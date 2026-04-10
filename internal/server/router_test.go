package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNewRouter_HealthWithoutDB(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := NewRouter(nil, "")
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v, raw=%s", err, rec.Body.String())
	}
	if body["status"] != "ok" {
		t.Fatalf("unexpected body: %v", body)
	}
}

func TestNewRouter_StaticServesIndexHTML(t *testing.T) {
	gin.SetMode(gin.TestMode)

	webDir := filepath.Join("..", "..", "web")
	r := NewRouter(nil, webDir)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="jobs-table"`) {
		t.Fatalf("response does not look like index.html (missing jobs-table)")
	}
	if !strings.Contains(body, "/api/jobs/matched") {
		t.Fatalf("response missing API path reference")
	}
}

func TestNewRouter_NoMatchedAPIWhenDBNil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(nil, "")

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/matched", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 without db, got %d", rec.Code)
	}
}
