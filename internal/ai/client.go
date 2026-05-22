package ai

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
	defaultNumPredict  = 512
)

type PromptClient interface {
	Generate(prompt string) (string, error)
}

type contextPromptClient interface {
	GenerateWithContext(ctx context.Context, prompt string) (string, error)
}

type DynamicClient struct {
	Store      *ConfigStore
	HTTPClient *http.Client
}

func (c *DynamicClient) Generate(prompt string) (string, error) {
	return c.GenerateWithContext(context.Background(), prompt)
}

func (c *DynamicClient) GenerateWithContext(ctx context.Context, prompt string) (string, error) {
	if c == nil || c.Store == nil {
		return "", errors.New("ai client is not configured")
	}
	client, err := NewClient(c.Store.Get())
	if err != nil {
		return "", err
	}
	if c.HTTPClient != nil {
		setHTTPClient(client, c.HTTPClient)
	}
	if withContext, ok := client.(contextPromptClient); ok {
		return withContext.GenerateWithContext(ctx, prompt)
	}
	return client.Generate(prompt)
}

func NewClient(config Config) (PromptClient, error) {
	normalized, err := NormalizeConfig(config)
	if err != nil {
		return nil, err
	}
	switch normalized.Provider {
	case ProviderOllama:
		return NewOllamaClient(normalized.BaseURL, normalized.Model, normalized.Think), nil
	case ProviderOpenAI:
		return NewOpenAICompatibleClient(normalized.BaseURL, normalized.Model, normalized.APIKey), nil
	case ProviderOpenAICompatible:
		return NewOpenAICompatibleClient(normalized.BaseURL, normalized.Model, normalized.APIKey), nil
	case ProviderAnthropic:
		return NewAnthropicClient(normalized.BaseURL, normalized.Model, normalized.APIKey), nil
	default:
		return nil, errors.New("unsupported AI_PROVIDER " + normalized.Provider)
	}
}

func setHTTPClient(client PromptClient, httpClient *http.Client) {
	switch c := client.(type) {
	case *OllamaClient:
		c.HTTPClient = httpClient
	case *OpenAICompatibleClient:
		c.HTTPClient = httpClient
	case *AnthropicClient:
		c.HTTPClient = httpClient
	}
}

type OllamaClient struct {
	BaseURL      string
	Model        string
	HTTPClient   *http.Client
	MaxRetries   int
	NumPredict   int
	RetryBackoff time.Duration
	Think        bool
}

type ollamaGenerateRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Stream  bool           `json:"stream"`
	Think   bool           `json:"think"`
	Options map[string]any `json:"options,omitempty"`
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
	Thinking string `json:"thinking"`
	Error    string `json:"error,omitempty"`
}

func NewOllamaClient(baseURL, model string, think bool) *OllamaClient {
	return &OllamaClient{
		BaseURL:      strings.TrimRight(baseURL, "/"),
		Model:        model,
		HTTPClient:   &http.Client{Timeout: defaultHTTPTimeout},
		MaxRetries:   defaultMaxRetries,
		NumPredict:   defaultNumPredict,
		RetryBackoff: 500 * time.Millisecond,
		Think:        think,
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

	reqBody := ollamaGenerateRequest{
		Model:   c.Model,
		Prompt:  prompt,
		Stream:  false,
		Think:   c.Think,
		Options: map[string]any{"num_predict": positiveOrDefault(c.NumPredict, defaultNumPredict)},
	}

	var parsed ollamaGenerateResponse
	if err := postJSONWithRetries(ctx, c.HTTPClient, c.MaxRetries, c.RetryBackoff, c.BaseURL+"/api/generate", "", reqBody, &parsed, "ollama"); err != nil {
		return "", err
	}
	if strings.TrimSpace(parsed.Error) != "" {
		return "", fmt.Errorf("ollama returned error: %s", parsed.Error)
	}
	out := strings.TrimSpace(parsed.Response)
	if out == "" && strings.TrimSpace(parsed.Thinking) != "" {
		out = strings.TrimSpace(parsed.Thinking)
	}
	if out == "" {
		return "", errors.New("ollama returned empty response")
	}
	return out, nil
}

type OpenAICompatibleClient struct {
	BaseURL      string
	Model        string
	APIKey       string
	HTTPClient   *http.Client
	MaxRetries   int
	RetryBackoff time.Duration
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func NewOpenAICompatibleClient(baseURL, model, apiKey string) *OpenAICompatibleClient {
	return &OpenAICompatibleClient{
		BaseURL:      strings.TrimRight(baseURL, "/"),
		Model:        model,
		APIKey:       apiKey,
		HTTPClient:   &http.Client{Timeout: defaultHTTPTimeout},
		MaxRetries:   defaultMaxRetries,
		RetryBackoff: 500 * time.Millisecond,
	}
}

func (c *OpenAICompatibleClient) Generate(prompt string) (string, error) {
	return c.GenerateWithContext(context.Background(), prompt)
}

func (c *OpenAICompatibleClient) GenerateWithContext(ctx context.Context, prompt string) (string, error) {
	if c == nil {
		return "", errors.New("openai-compatible client is nil")
	}
	reqBody := chatCompletionRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: 0,
	}
	var parsed chatCompletionResponse
	if err := postJSONWithRetries(ctx, c.HTTPClient, c.MaxRetries, c.RetryBackoff, c.BaseURL+"/chat/completions", c.APIKey, reqBody, &parsed, "openai-compatible"); err != nil {
		return "", err
	}
	if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
		return "", errors.New(parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("openai-compatible provider returned no choices")
	}
	out := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if out == "" {
		return "", errors.New("openai-compatible provider returned empty content")
	}
	return out, nil
}

type AnthropicClient struct {
	BaseURL      string
	Model        string
	APIKey       string
	HTTPClient   *http.Client
	MaxRetries   int
	RetryBackoff time.Duration
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func NewAnthropicClient(baseURL, model, apiKey string) *AnthropicClient {
	return &AnthropicClient{
		BaseURL:      strings.TrimRight(baseURL, "/"),
		Model:        model,
		APIKey:       apiKey,
		HTTPClient:   &http.Client{Timeout: defaultHTTPTimeout},
		MaxRetries:   defaultMaxRetries,
		RetryBackoff: 500 * time.Millisecond,
	}
}

func (c *AnthropicClient) Generate(prompt string) (string, error) {
	return c.GenerateWithContext(context.Background(), prompt)
}

func (c *AnthropicClient) GenerateWithContext(ctx context.Context, prompt string) (string, error) {
	if c == nil {
		return "", errors.New("anthropic client is nil")
	}
	reqBody := anthropicRequest{
		Model:     c.Model,
		MaxTokens: defaultNumPredict,
		Messages:  []anthropicMessage{{Role: "user", Content: prompt}},
	}
	var parsed anthropicResponse
	if err := postAnthropicJSON(ctx, c.HTTPClient, c.MaxRetries, c.RetryBackoff, c.BaseURL+"/v1/messages", c.APIKey, reqBody, &parsed); err != nil {
		return "", err
	}
	if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
		return "", errors.New(parsed.Error.Message)
	}
	for _, content := range parsed.Content {
		if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
			return strings.TrimSpace(content.Text), nil
		}
	}
	return "", errors.New("anthropic returned no text content")
}

func postJSONWithRetries(ctx context.Context, client *http.Client, maxRetries int, backoff time.Duration, endpoint, apiKey string, reqBody any, out any, providerName string) error {
	return postWithRetries(ctx, client, maxRetries, backoff, endpoint, reqBody, out, providerName, func(req *http.Request) {
		req.Header.Set("Content-Type", "application/json")
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
	})
}

func postAnthropicJSON(ctx context.Context, client *http.Client, maxRetries int, backoff time.Duration, endpoint, apiKey string, reqBody any, out any) error {
	return postWithRetries(ctx, client, maxRetries, backoff, endpoint, reqBody, out, "anthropic", func(req *http.Request) {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Anthropic-Version", "2023-06-01")
		if apiKey != "" {
			req.Header.Set("X-API-Key", apiKey)
		}
	})
}

func postWithRetries(ctx context.Context, client *http.Client, maxRetries int, backoff time.Duration, endpoint string, reqBody any, out any, providerName string, setHeaders func(*http.Request)) error {
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	if backoff <= 0 {
		backoff = 500 * time.Millisecond
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal %s request: %w", providerName, err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("create %s request: %w", providerName, err)
		}
		setHeaders(req)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("call %s: %w", providerName, err)
		} else {
			respBody, readErr := io.ReadAll(resp.Body)
			closeErr := resp.Body.Close()
			if readErr != nil {
				lastErr = fmt.Errorf("read %s response: %w", providerName, readErr)
			} else if closeErr != nil {
				lastErr = fmt.Errorf("close %s response body: %w", providerName, closeErr)
			} else if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
				lastErr = fmt.Errorf("%s server error %d: %s", providerName, resp.StatusCode, strings.TrimSpace(string(respBody)))
			} else if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("%s unexpected status %d: %s", providerName, resp.StatusCode, strings.TrimSpace(string(respBody)))
			} else if err := json.Unmarshal(respBody, out); err != nil {
				return fmt.Errorf("decode %s response: %w", providerName, err)
			} else {
				return nil
			}
		}

		if attempt < maxRetries {
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	if lastErr == nil {
		lastErr = errors.New(providerName + " request failed")
	}
	return lastErr
}

func positiveOrDefault(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
