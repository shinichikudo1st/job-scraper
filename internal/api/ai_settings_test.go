package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shinichikudo1st/job-scraper/internal/ai"
)

func TestAISettingsSaveDoesNotReturnAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store, err := ai.NewConfigStore(ai.Config{Provider: ai.ProviderOllama})
	if err != nil {
		t.Fatalf("NewConfigStore() error = %v", err)
	}

	r := gin.New()
	RegisterAISettingsRoutes(r, store)

	body := strings.NewReader(`{"provider":"openai","model":"gpt-test","api_key":"secret-key"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/settings", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret-key") {
		t.Fatalf("response leaked api key: %s", rec.Body.String())
	}
	var got ai.PublicConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if got.Provider != ai.ProviderOpenAI || !got.HasAPIKey {
		t.Fatalf("unexpected public config: %+v", got)
	}
}

func TestAISettingsInvalidProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store, err := ai.NewConfigStore(ai.Config{Provider: ai.ProviderOllama})
	if err != nil {
		t.Fatalf("NewConfigStore() error = %v", err)
	}

	r := gin.New()
	RegisterAISettingsRoutes(r, store)

	body := strings.NewReader(`{"provider":"bad"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/settings", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}
