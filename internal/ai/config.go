package ai

import (
	"errors"
	"os"
	"strings"
	"sync"
)

const (
	ProviderOllama           = "ollama"
	ProviderOpenAI           = "openai"
	ProviderAnthropic        = "anthropic"
	ProviderOpenAICompatible = "openai_compatible"
)

type Config struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	Model    string `json:"model"`
	APIKey   string `json:"-"`
	Think    bool   `json:"think"`
}

type PublicConfig struct {
	Provider     string `json:"provider"`
	BaseURL      string `json:"base_url"`
	Model        string `json:"model"`
	Think        bool   `json:"think"`
	HasAPIKey    bool   `json:"has_api_key"`
	SessionOnly  bool   `json:"session_only"`
	CloudWarning string `json:"cloud_warning,omitempty"`
}

type ConfigStore struct {
	mu     sync.RWMutex
	config Config
}

func NewConfigStore(config Config) (*ConfigStore, error) {
	normalized, err := NormalizeConfig(config)
	if err != nil {
		return nil, err
	}
	return &ConfigStore{config: normalized}, nil
}

func (s *ConfigStore) Get() Config {
	if s == nil {
		return Config{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

func (s *ConfigStore) Update(config Config) (Config, error) {
	if s == nil {
		return Config{}, errors.New("ai config store is not configured")
	}
	normalized, err := NormalizeConfig(config)
	if err != nil {
		return Config{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = normalized
	return normalized, nil
}

func LoadConfigFromEnv() Config {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("AI_PROVIDER")))
	if provider == "" {
		provider = ProviderOllama
	}

	baseURL := strings.TrimSpace(os.Getenv("AI_BASE_URL"))
	model := strings.TrimSpace(os.Getenv("AI_MODEL"))
	apiKey := strings.TrimSpace(os.Getenv("AI_API_KEY"))

	if provider == ProviderOllama {
		if baseURL == "" {
			baseURL = strings.TrimSpace(os.Getenv("OLLAMA_BASE_URL"))
		}
		if model == "" {
			model = strings.TrimSpace(os.Getenv("OLLAMA_MODEL"))
		}
	}
	if provider == ProviderOpenAI && apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	}
	if provider == ProviderAnthropic && apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	}

	return Config{
		Provider: provider,
		BaseURL:  baseURL,
		Model:    model,
		APIKey:   apiKey,
		Think:    envBool("AI_THINK") || envBool("OLLAMA_THINK"),
	}
}

func NormalizeConfig(config Config) (Config, error) {
	config.Provider = strings.ToLower(strings.TrimSpace(config.Provider))
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	config.Model = strings.TrimSpace(config.Model)
	config.APIKey = strings.TrimSpace(config.APIKey)

	if config.Provider == "" {
		config.Provider = ProviderOllama
	}

	switch config.Provider {
	case ProviderOllama:
		if config.BaseURL == "" {
			config.BaseURL = "http://127.0.0.1:11434"
		}
		if config.Model == "" {
			config.Model = "llama3.2:3b"
		}
	case ProviderOpenAI:
		if config.BaseURL == "" {
			config.BaseURL = "https://api.openai.com/v1"
		}
		if config.Model == "" {
			config.Model = "gpt-4.1-mini"
		}
		if config.APIKey == "" {
			return Config{}, errors.New("AI_API_KEY or OPENAI_API_KEY is required when AI_PROVIDER=openai")
		}
	case ProviderAnthropic:
		if config.BaseURL == "" {
			config.BaseURL = "https://api.anthropic.com"
		}
		if config.Model == "" {
			config.Model = "claude-3-5-haiku-latest"
		}
		if config.APIKey == "" {
			return Config{}, errors.New("AI_API_KEY or ANTHROPIC_API_KEY is required when AI_PROVIDER=anthropic")
		}
	case ProviderOpenAICompatible:
		if config.BaseURL == "" {
			return Config{}, errors.New("AI_BASE_URL is required when AI_PROVIDER=openai_compatible")
		}
		if config.Model == "" {
			return Config{}, errors.New("AI_MODEL is required when AI_PROVIDER=openai_compatible")
		}
	default:
		return Config{}, errors.New("unsupported AI_PROVIDER " + config.Provider)
	}

	return config, nil
}

func ToPublicConfig(config Config) PublicConfig {
	public := PublicConfig{
		Provider:    config.Provider,
		BaseURL:     config.BaseURL,
		Model:       config.Model,
		Think:       config.Think,
		HasAPIKey:   config.APIKey != "",
		SessionOnly: true,
	}
	if IsCloudProvider(config.Provider) {
		public.CloudWarning = "Cloud providers may receive your CV text and job descriptions."
	}
	return public
}

func IsCloudProvider(provider string) bool {
	return provider == ProviderOpenAI || provider == ProviderAnthropic
}

func envBool(key string) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes"
}
