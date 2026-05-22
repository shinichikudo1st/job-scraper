package ai

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
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		opts, ok := body["options"].(map[string]any)
		if !ok {
			t.Fatalf("expected options object")
		}
		if _, exists := opts["num_predict"]; !exists {
			t.Fatalf("expected num_predict in options")
		}
		if v, ok := body["think"].(bool); !ok || v {
			t.Fatalf("expected think=false in request body, got %v (%T)", body["think"], body["think"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"response": "fit json payload",
		})
	}))
	defer ts.Close()

	client := NewOllamaClient(ts.URL, "qwen3.5:latest", false)
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
		_ = json.NewEncoder(w).Encode(map[string]any{
			"response": "ok-after-retry",
		})
	}))
	defer ts.Close()

	client := NewOllamaClient(ts.URL, "qwen3.5:latest", false)
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
		_ = json.NewEncoder(w).Encode(map[string]any{
			"response": "late-response",
		})
	}))
	defer ts.Close()

	client := NewOllamaClient(ts.URL, "qwen3.5:latest", false)
	client.MaxRetries = 0
	client.HTTPClient = &http.Client{Timeout: 50 * time.Millisecond}

	_, err := client.Generate("test prompt")
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
}

func TestOpenAICompatibleClientGenerateSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": `{"fit":true,"score":90,"reason":"ok"}`}},
			},
		})
	}))
	defer ts.Close()

	client := NewOpenAICompatibleClient(ts.URL, "gpt-test", "test-key")
	got, err := client.Generate("prompt")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got == "" {
		t.Fatalf("expected content")
	}
}

func TestAnthropicClientGenerateSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("X-API-Key"); got != "test-key" {
			t.Fatalf("X-API-Key = %q", got)
		}
		if got := r.Header.Get("Anthropic-Version"); got == "" {
			t.Fatalf("missing Anthropic-Version header")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": `{"fit":true,"score":90,"reason":"ok"}`},
			},
		})
	}))
	defer ts.Close()

	client := NewAnthropicClient(ts.URL, "claude-test", "test-key")
	got, err := client.Generate("prompt")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got == "" {
		t.Fatalf("expected content")
	}
}

func TestNormalizeConfigRequiresCloudKeys(t *testing.T) {
	_, err := NormalizeConfig(Config{Provider: ProviderOpenAI, Model: "gpt-test"})
	if err == nil {
		t.Fatalf("expected OpenAI key validation error")
	}
	_, err = NormalizeConfig(Config{Provider: ProviderAnthropic, Model: "claude-test"})
	if err == nil {
		t.Fatalf("expected Anthropic key validation error")
	}
}
