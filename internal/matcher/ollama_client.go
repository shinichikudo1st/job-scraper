package matcher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultHTTPTimeout = 90 * time.Second
	defaultMaxRetries  = 2
	defaultNumPredict  = 512 // Increased from 256 to allow full JSON response with long CV/job desc
)

type OllamaClient struct {
	BaseURL      string
	Model        string
	HTTPClient   *http.Client
	MaxRetries   int
	NumPredict   int
	RetryBackoff time.Duration
	// Think controls Ollama's thinking mode on /api/generate (see https://ollama.com/blog/thinking).
	// false: model answers directly in "response" (what we need for JSON).
	// true: thinking may appear in "thinking" and "response" may be empty until the final answer.
	Think bool
}

type ollamaGenerateRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	Think   bool                   `json:"think"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
	Thinking string `json:"thinking"`
	Error    string `json:"error,omitempty"`
}

func NewOllamaClient(baseURL, model string) *OllamaClient {
	return &OllamaClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Model:   model,
		HTTPClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
		MaxRetries:   defaultMaxRetries,
		NumPredict:   defaultNumPredict,
		RetryBackoff: 500 * time.Millisecond,
	}
}

func (c *OllamaClient) Generate(prompt string) (string, error) {
	return c.GenerateWithContext(context.Background(), prompt)
}

func (c *OllamaClient) GenerateWithContext(ctx context.Context, prompt string) (string, error) {
	if c == nil {
		return "", errors.New("ollama client is nil")
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return "", errors.New("ollama base URL is required")
	}
	if strings.TrimSpace(c.Model) == "" {
		return "", errors.New("ollama model is required")
	}
	if strings.TrimSpace(prompt) == "" {
		return "", errors.New("prompt is required")
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}

	maxRetries := c.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}

	numPredict := c.NumPredict
	if numPredict <= 0 {
		numPredict = defaultNumPredict
	}

	backoff := c.RetryBackoff
	if backoff <= 0 {
		backoff = 500 * time.Millisecond
	}

	reqBody := ollamaGenerateRequest{
		Model:   c.Model,
		Prompt:  prompt,
		Stream:  false,
		Think:   c.Think,
		Options: map[string]interface{}{"num_predict": numPredict},
	}

	endpoint := c.BaseURL + "/api/generate"

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return "", fmt.Errorf("marshal ollama request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
		if err != nil {
			return "", fmt.Errorf("create ollama request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("call ollama: %w", err)
		} else {
			respBody, readErr := io.ReadAll(resp.Body)
			closeErr := resp.Body.Close()
			if readErr != nil {
				lastErr = fmt.Errorf("read ollama response: %w", readErr)
			} else if closeErr != nil {
				lastErr = fmt.Errorf("close ollama response body: %w", closeErr)
			} else if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
				lastErr = fmt.Errorf("ollama server error %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
			} else if resp.StatusCode != http.StatusOK {
				return "", fmt.Errorf("ollama unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
			} else {
				var parsed ollamaGenerateResponse
				if err := json.Unmarshal(respBody, &parsed); err != nil {
					return "", fmt.Errorf("decode ollama response: %w", err)
				}
				if strings.TrimSpace(parsed.Error) != "" {
					return "", fmt.Errorf("ollama returned error: %s", parsed.Error)
				}
				out := strings.TrimSpace(parsed.Response)
				// qwen models with thinking enabled put output in "thinking" field
				if out == "" && strings.TrimSpace(parsed.Thinking) != "" {
					out = strings.TrimSpace(parsed.Thinking)
				}
				if out == "" {
					return "", fmt.Errorf("ollama returned empty response (runner often crashes mid-request on GPU/driver issues; try MATCHER_WORKERS=1, CPU mode, or a smaller model)")
				}
				return out, nil
			}
		}

		if attempt < maxRetries {
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			case <-timer.C:
			}
		}
	}

	if lastErr == nil {
		lastErr = errors.New("ollama request failed")
	}
	return "", lastErr
}
