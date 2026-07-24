package wiki

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	utls "github.com/refraction-networking/utls"
)

func TestParseTLSProfile(t *testing.T) {
	for input, want := range map[string]TLSProfile{
		" standard ":    TLSProfileStandard,
		"CHROME-133":    TLSProfileChrome133,
		"firefox-120":   TLSProfileFirefox120,
		" safari-16.0 ": TLSProfileSafari160,
	} {
		got, err := ParseTLSProfile(input)
		if err != nil || got != want {
			t.Fatalf("ParseTLSProfile(%q) = %q, %v", input, got, err)
		}
	}
	if _, err := ParseTLSProfile("random"); err == nil {
		t.Fatal("未知 TLS profile 未返回错误")
	}
}

func TestSafariCompatibilitySpec(t *testing.T) {
	spec, err := safariClientHelloSpec(true, utls.VersionTLS12, utls.VersionTLS12)
	if err != nil {
		t.Fatal(err)
	}
	if spec.TLSVersMin != utls.VersionTLS12 || spec.TLSVersMax != utls.VersionTLS12 {
		t.Fatalf("TLS 版本范围 = %x-%x", spec.TLSVersMin, spec.TLSVersMax)
	}
	foundCompression := false
	foundVersions := false
	for _, extension := range spec.Extensions {
		switch typed := extension.(type) {
		case *utls.UtlsCompressCertExtension:
			foundCompression = len(typed.Algorithms) > 0
		case *utls.SupportedVersionsExtension:
			foundVersions = true
			for _, version := range typed.Versions {
				if version == utls.VersionTLS10 || version == utls.VersionTLS11 || version == utls.VersionTLS13 {
					t.Fatalf("TLS 1.2 兼容 spec 含不允许版本 %x", version)
				}
			}
		}
	}
	if !foundCompression || !foundVersions {
		t.Fatalf("compression=%v supported_versions=%v", foundCompression, foundVersions)
	}
}

func TestSafariSpecWithoutCompression(t *testing.T) {
	spec, err := safariClientHelloSpec(false, utls.VersionTLS12, utls.VersionTLS13)
	if err != nil {
		t.Fatal(err)
	}
	for _, extension := range spec.Extensions {
		if _, ok := extension.(*utls.UtlsCompressCertExtension); ok {
			t.Fatal("no-compression spec 仍包含证书压缩扩展")
		}
	}
}

func TestStandardHTTPClient(t *testing.T) {
	client, err := NewHTTPClient("https://example.test", TLSProfileStandard, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport = %T", client.Transport)
	}
	if transport == http.DefaultTransport || transport.Proxy == nil || client.Timeout != 3*time.Second {
		t.Fatalf("标准客户端未正确 clone: %+v", client)
	}
}

func TestUTLSHTTP2Client(t *testing.T) {
	var newConnections atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor != 2 {
			t.Errorf("协议 = %s", r.Proto)
		}
		_, _ = io.WriteString(w, "ok")
	}))
	server.EnableHTTP2 = true
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			newConnections.Add(1)
		}
	}
	server.StartTLS()
	defer server.Close()

	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	client, err := newHTTPClient(server.URL, TLSProfileChrome133, 5*time.Second, roots, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil || string(body) != "ok" {
			t.Fatalf("响应 = %q, %v", body, readErr)
		}
	}
	if got := newConnections.Load(); got != 1 {
		t.Fatalf("新连接数 = %d，期望复用一条 HTTP/2 连接", got)
	}
}

func TestUTLSVerifiesCertificate(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	client, err := newHTTPClient(server.URL, TLSProfileChrome133, 3*time.Second, x509.NewCertPool(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Get(server.URL); err == nil {
		t.Fatal("未受信任的服务端证书被接受")
	}
}

func TestUTLSRejectsHTTP1ALPN(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.TLS = &tls.Config{NextProtos: []string{"http/1.1"}}
	server.StartTLS()
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	client, err := newHTTPClient(server.URL, TLSProfileChrome133, 3*time.Second, roots, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Get(server.URL)
	if err == nil || !strings.Contains(err.Error(), "ALPN") {
		t.Fatalf("error = %v", err)
	}
}

func TestUTLSRejectsConfiguredProxy(t *testing.T) {
	proxyURL, _ := url.Parse("http://proxy.example:8080")
	_, err := newHTTPClient("https://example.test", TLSProfileChrome133, time.Second, nil, nil, func(*http.Request) (*url.URL, error) {
		return proxyURL, nil
	})
	if err == nil || !strings.Contains(err.Error(), "仅支持直连") {
		t.Fatalf("error = %v", err)
	}
}

func TestUTLSHandshakeHonorsContext(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	dial := func(context.Context, string, string) (net.Conn, error) { return clientConn, nil }
	client, err := newHTTPClient("https://example.test", TLSProfileChrome133, time.Second, nil, dial, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.test", nil)
	_, err = client.Do(req)
	if err == nil || ctx.Err() == nil {
		t.Fatalf("error = %v, context = %v", err, ctx.Err())
	}
}
