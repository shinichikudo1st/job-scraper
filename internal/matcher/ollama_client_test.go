package matcher

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestOllamaClientGenerateSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/generate" {
			t.Fatalf("expected /api/generate, got %s", r.URL.Path)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		opts, ok := body["options"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected options object")
		}
		if _, exists := opts["num_predict"]; !exists {
			t.Fatalf("expected num_predict in options")
		}
		if v, ok := body["think"].(bool); !ok || v != false {
			t.Fatalf("expected think=false in request body, got %v (%T)", body["think"], body["think"])
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"response": "fit json payload",
		})
	}))
	defer ts.Close()

	client := NewOllamaClient(ts.URL, "qwen3.5:latest")
	got, err := client.Generate("test prompt")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got != "fit json payload" {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestOllamaClientGenerateRetriesOn500(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			http.Error(w, `{"error":"temporary failure"}`, http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"response": "ok-after-retry",
		})
	}))
	defer ts.Close()

	client := NewOllamaClient(ts.URL, "qwen3.5:latest")
	client.RetryBackoff = 10 * time.Millisecond
	client.MaxRetries = 2

	got, err := client.Generate("test prompt")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got != "ok-after-retry" {
		t.Fatalf("unexpected response: %q", got)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestOllamaClientGenerateTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(120 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"response": "late-response",
		})
	}))
	defer ts.Close()

	client := NewOllamaClient(ts.URL, "qwen3.5:latest")
	client.MaxRetries = 0
	client.HTTPClient = &http.Client{Timeout: 50 * time.Millisecond}

	_, err := client.Generate("test prompt")
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
}
