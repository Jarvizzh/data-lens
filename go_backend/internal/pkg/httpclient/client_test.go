package httpclient

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestClient_RetryOnServerError(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cnt := atomic.AddInt32(&callCount, 1)
		if cnt < 2 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("502 bad gateway"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))
	defer server.Close()

	client := NewClient(
		WithMaxRetries(3),
		WithBaseRetryInterval(10*time.Millisecond),
	)

	req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("expected success on attempt 2, got err: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "success" {
		t.Fatalf("expected 'success', got '%s'", string(body))
	}

	if atomic.LoadInt32(&callCount) != 2 {
		t.Fatalf("expected 2 calls, got %d", callCount)
	}
}

func TestClient_RetryPreservesRequestBody(t *testing.T) {
	var callCount int32
	var lastReceivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cnt := atomic.AddInt32(&callCount, 1)
		body, _ := io.ReadAll(r.Body)
		lastReceivedBody = string(body)

		if cnt == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := NewClient(
		WithMaxRetries(3),
		WithBaseRetryInterval(10*time.Millisecond),
	)

	payload := `{"test_key":"hello_world"}`
	req, err := http.NewRequestWithContext(context.Background(), "POST", server.URL, bytes.NewReader([]byte(payload)))
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	defer resp.Body.Close()

	if lastReceivedBody != payload {
		t.Fatalf("expected body '%s' on attempt 2, got '%s'", payload, lastReceivedBody)
	}
	if atomic.LoadInt32(&callCount) != 2 {
		t.Fatalf("expected 2 attempts, got %d", callCount)
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(
		WithMaxRetries(5),
		WithBaseRetryInterval(100*time.Millisecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	_, err := client.Do(req)
	if err == nil {
		t.Fatal("expected context timeout error, got nil")
	}
}
