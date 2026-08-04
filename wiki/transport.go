package wiki

import (
	"context"
	cryptotls "crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
)

// TLSProfile 指定 Wiki HTTPS 请求使用的 TLS ClientHello。
type TLSProfile string

const (
	TLSProfileStandard   TLSProfile = "standard"
	TLSProfileChrome133  TLSProfile = "chrome-133"
	TLSProfileFirefox120 TLSProfile = "firefox-120"
	TLSProfileSafari160  TLSProfile = "safari-16.0"
)

type dialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)
type proxyFunc func(*http.Request) (*url.URL, error)
type clientHelloSpecFactory func() (utls.ClientHelloSpec, error)

type profileRoundTripper struct {
	base    http.RoundTripper
	profile TLSProfile
}

func (t profileRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	switch t.profile {
	case TLSProfileSafari160:
		clone.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Safari/605.1.15")
		if clone.Header.Get("Accept") == "" {
			clone.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		}
		clone.Header.Set("Accept-Language", "zh-CN,zh-Hans;q=0.9")
	case TLSProfileFirefox120:
		clone.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:120.0) Gecko/20100101 Firefox/120.0")
		if clone.Header.Get("Accept") == "" {
			clone.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		}
		clone.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	case TLSProfileChrome133:
		clone.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36")
		if clone.Header.Get("Accept") == "" {
			clone.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		}
		clone.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	}
	clone.Header.Set("From", "sts2bot authorized crawler")
	return t.base.RoundTrip(clone)
}

// ParseTLSProfile 解析并验证 TLS profile。
func ParseTLSProfile(value string) (TLSProfile, error) {
	profile := TLSProfile(strings.ToLower(strings.TrimSpace(value)))
	switch profile {
	case TLSProfileStandard, TLSProfileChrome133, TLSProfileFirefox120, TLSProfileSafari160:
		return profile, nil
	default:
		return "", fmt.Errorf("不支持的 Wiki TLS profile %q，可选值为 standard、chrome-133、firefox-120、safari-16.0", value)
	}
}

// NewHTTPClient 创建使用指定 TLS profile 和请求间隔的可复用 HTTP 客户端。
func NewHTTPClient(baseURL string, profile TLSProfile, timeout, requestInterval time.Duration) (*http.Client, error) {
	client, err := newHTTPClient(baseURL, profile, timeout, nil, nil, http.ProxyFromEnvironment)
	if err != nil {
		return nil, err
	}
	if requestInterval <= 0 {
		return nil, fmt.Errorf("Wiki 请求间隔必须大于 0")
	}
	client.Transport = &intervalTransport{base: client.Transport, interval: requestInterval}
	return client, nil
}

func newHTTPClient(baseURL string, profile TLSProfile, timeout time.Duration, roots *x509.CertPool, dial dialContextFunc, proxy proxyFunc) (*http.Client, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("HTTP 客户端超时必须大于 0")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("无效的 Wiki base URL %q", baseURL)
	}

	switch profile {
	case TLSProfileStandard:
		transport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, fmt.Errorf("默认 HTTP transport 类型异常")
		}
		return &http.Client{Transport: transport.Clone(), Timeout: timeout}, nil
	case TLSProfileChrome133, TLSProfileFirefox120, TLSProfileSafari160:
		if parsed.Scheme != "https" {
			return nil, fmt.Errorf("TLS profile %q 仅支持 HTTPS Wiki URL", profile)
		}
		if proxy != nil {
			proxyURL, proxyErr := proxy(&http.Request{URL: parsed})
			if proxyErr != nil {
				return nil, fmt.Errorf("检查 Wiki 代理配置: %w", proxyErr)
			}
			if proxyURL != nil {
				return nil, fmt.Errorf("TLS profile %q 仅支持直连，Wiki URL 当前命中了代理 %s；请设置 NO_PROXY=%s", profile, proxyURL.Redacted(), parsed.Hostname())
			}
		}
		if dial == nil {
			netDialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
			dial = netDialer.DialContext
		}
		helloID := utls.HelloChrome_133
		var specFactory clientHelloSpecFactory
		switch profile {
		case TLSProfileFirefox120:
			helloID = utls.HelloFirefox_120
		case TLSProfileSafari160:
			helloID = utls.HelloSafari_16_0
			specFactory = func() (utls.ClientHelloSpec, error) {
				// Cloudflare 的部分边缘在 Safari TLS 1.3 CertificateVerify 路径上
				// 返回无法验证的 RSA 签名；TLS 1.2 保留 Safari 压缩扩展可稳定通过。
				return safariClientHelloSpec(true, utls.VersionTLS12, utls.VersionTLS12)
			}
		}
		transport := newUTLSHTTP2Transport(helloID, specFactory, roots, dial)
		return &http.Client{Transport: profileRoundTripper{base: transport, profile: profile}, Timeout: timeout}, nil
	default:
		return nil, fmt.Errorf("不支持的 Wiki TLS profile %q", profile)
	}
}

type intervalTransport struct {
	base     http.RoundTripper
	interval time.Duration
	mu       sync.Mutex
	next     time.Time
}

func (t *intervalTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	now := time.Now()
	waitFor := t.next.Sub(now)
	if waitFor > 0 {
		timer := time.NewTimer(waitFor)
		select {
		case <-timer.C:
		case <-req.Context().Done():
			if !timer.Stop() {
				<-timer.C
			}
			t.mu.Unlock()
			return nil, req.Context().Err()
		}
	}
	t.next = time.Now().Add(t.interval)
	t.mu.Unlock()
	return t.base.RoundTrip(req)
}

func safariClientHelloSpec(includeCertCompression bool, minVersion, maxVersion uint16) (utls.ClientHelloSpec, error) {
	spec, err := utls.UTLSIdToSpec(utls.HelloSafari_16_0)
	if err != nil {
		return utls.ClientHelloSpec{}, err
	}
	spec.TLSVersMin = minVersion
	spec.TLSVersMax = maxVersion
	versions := []uint16{utls.GREASE_PLACEHOLDER}
	if maxVersion >= utls.VersionTLS13 {
		versions = append(versions, utls.VersionTLS13)
	}
	if minVersion <= utls.VersionTLS12 && maxVersion >= utls.VersionTLS12 {
		versions = append(versions, utls.VersionTLS12)
	}

	extensions := make([]utls.TLSExtension, 0, len(spec.Extensions))
	for _, extension := range spec.Extensions {
		switch typed := extension.(type) {
		case *utls.UtlsCompressCertExtension:
			if includeCertCompression {
				extensions = append(extensions, extension)
			}
		case *utls.SupportedVersionsExtension:
			extensions = append(extensions, &utls.SupportedVersionsExtension{Versions: append([]uint16(nil), versions...)})
		default:
			extensions = append(extensions, typed)
		}
	}
	spec.Extensions = extensions
	return spec, nil
}

func newUTLSHTTP2Transport(helloID utls.ClientHelloID, specFactory clientHelloSpecFactory, roots *x509.CertPool, dial dialContextFunc) *http2.Transport {
	return &http2.Transport{
		IdleConnTimeout: 90 * time.Second,
		ReadIdleTimeout: 30 * time.Second,
		PingTimeout:     15 * time.Second,
		DialTLSContext: func(ctx context.Context, network, address string, cfg *cryptotls.Config) (net.Conn, error) {
			rawConn, err := dial(ctx, network, address)
			if err != nil {
				return nil, err
			}
			closeOnError := true
			defer func() {
				if closeOnError {
					_ = rawConn.Close()
				}
			}()

			serverName := ""
			if cfg != nil {
				serverName = cfg.ServerName
			}
			if serverName == "" {
				serverName, _, err = net.SplitHostPort(address)
				if err != nil {
					return nil, fmt.Errorf("解析 TLS 目标地址 %q: %w", address, err)
				}
			}

			config := &utls.Config{
				ServerName: serverName,
				RootCAs:    roots,
				MinVersion: utls.VersionTLS12,
				MaxVersion: utls.VersionTLS13,
				NextProtos: []string{"h2", "http/1.1"},
			}
			uconn := utls.UClient(rawConn, config, helloID)
			if specFactory != nil {
				spec, specErr := specFactory()
				if specErr != nil {
					return nil, fmt.Errorf("构造自定义 ClientHello 失败: %w", specErr)
				}
				uconn = utls.UClient(rawConn, config, utls.HelloCustom)
				if err := uconn.ApplyPreset(&spec); err != nil {
					return nil, fmt.Errorf("应用自定义 ClientHello 失败: %w", err)
				}
			}
			if err := uconn.HandshakeContext(ctx); err != nil {
				return nil, fmt.Errorf("uTLS 握手失败: %w", err)
			}
			if protocol := uconn.ConnectionState().NegotiatedProtocol; protocol != http2.NextProtoTLS {
				return nil, fmt.Errorf("uTLS ALPN 协商得到 %q，需要 %q", protocol, http2.NextProtoTLS)
			}
			closeOnError = false
			return uconn, nil
		},
	}
}
