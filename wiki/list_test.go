package wiki

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

func TestListFollowsContinuation(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("eititle") != "模板:卡牌信息框" {
			t.Errorf("eititle=%q", r.URL.Query().Get("eititle"))
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("eicontinue") == "" {
			_, _ = io.WriteString(w, `{"continue":{"eicontinue":"next"},"query":{"embeddedin":[{"pageid":2,"ns":0,"title":"打击"}]}}`)
		} else {
			_, _ = io.WriteString(w, `{"query":{"embeddedin":[{"pageid":1,"ns":0,"title":"防御"},{"pageid":2,"ns":0,"title":"打击"}]}}`)
		}
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, server.Client(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	items, err := client.List(context.Background(), EntityCard)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(items) != 2 || items[0].Name != "打击" || items[1].Name != "防御" {
		t.Fatalf("calls=%d items=%+v", calls, items)
	}
}

func TestIntervalTransportSpacesConcurrentStarts(t *testing.T) {
	var mu sync.Mutex
	var starts []time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()
	baseURL, _ := url.Parse(server.URL)
	transport := &intervalTransport{base: http.DefaultTransport, interval: 30 * time.Millisecond}
	client := &http.Client{Transport: transport}
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := client.Get(baseURL.String())
			if err != nil {
				t.Error(err)
				return
			}
			_ = response.Body.Close()
		}()
	}
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	if len(starts) != 3 {
		t.Fatalf("starts=%d", len(starts))
	}
	for i := 1; i < len(starts); i++ {
		if starts[i].Sub(starts[i-1]) < 25*time.Millisecond {
			t.Fatalf("gap=%v starts=%v", starts[i].Sub(starts[i-1]), starts)
		}
	}
}

func TestListRejectsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"error":{"code":"bad","info":"details"}}`)
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, server.Client(), nil)
	if _, err := client.List(context.Background(), EntityRelic); !IsKind(err, KindUpstream) {
		t.Fatalf("error=%v", err)
	}
}
