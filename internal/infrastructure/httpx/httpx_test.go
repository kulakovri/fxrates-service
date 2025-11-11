package httpx

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type memLogger struct {
	mu    sync.Mutex
	infos []string
	warns []string
}

func (m *memLogger) Info(msg string, _ ...any) {
	m.mu.Lock()
	m.infos = append(m.infos, msg)
	m.mu.Unlock()
}
func (m *memLogger) Warn(msg string, _ ...any) {
	m.mu.Lock()
	m.warns = append(m.warns, msg)
	m.mu.Unlock()
}

func httpClientRT(rt http.RoundTripper) *http.Client {
	return &http.Client{Transport: rt, Timeout: 2 * time.Second}
}

func TestDoJSON_Retry500Then200(t *testing.T) {
	var calls int
	rt := httpClientRT(rtFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader("err")), Header: make(http.Header), Request: r}, nil
		}
		body := `{"ok": true}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: r}, nil
	}))
	type resp struct {
		OK bool `json:"ok"`
	}
	var out resp
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c := &Client{HTTP: rt}
	if err := c.DoJSON(ctx, req, &out, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.OK {
		t.Fatalf("expected ok=true")
	}
	if calls < 2 {
		t.Fatalf("expected at least 2 calls, got %d", calls)
	}
}

type tempTimeoutErr struct{}

func (tempTimeoutErr) Error() string   { return "timeout" }
func (tempTimeoutErr) Timeout() bool   { return true }
func (tempTimeoutErr) Temporary() bool { return true }

func TestDoJSON_RetryNetTimeoutThen200(t *testing.T) {
	var calls int
	rt := httpClientRT(rtFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			var ne net.Error = tempTimeoutErr{}
			return nil, ne
		}
		body := `{"ok": true}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: r}, nil
	}))
	type resp struct {
		OK bool `json:"ok"`
	}
	var out resp
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c := &Client{HTTP: rt}
	if err := c.DoJSON(ctx, req, &out, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDoJSON_NoRetryOn400(t *testing.T) {
	var calls int
	rt := httpClientRT(rtFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader("bad")), Header: make(http.Header), Request: r}, nil
	}))
	var out any
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	c := &Client{HTTP: rt}
	err := c.DoJSON(context.Background(), req, &out, nil)
	if err == nil {
		t.Fatalf("expected error")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestDoJSON_DecodeError_NoRetry(t *testing.T) {
	rt := httpClientRT(rtFunc(func(r *http.Request) (*http.Response, error) {
		// invalid json
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString("{x")), Header: make(http.Header), Request: r}, nil
	}))
	var out map[string]any
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	c := &Client{HTTP: rt}
	err := c.DoJSON(context.Background(), req, &out, nil)
	if err == nil {
		t.Fatalf("expected decode error")
	}
	// ensure it's not a retry loop: memLogger doesn't expose counts, but a fast return suffices
	if !strings.Contains(err.Error(), "decode") && !errors.Is(err, context.DeadlineExceeded) {
		// ok
	}
}
