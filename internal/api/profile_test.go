package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shinichikudo1st/job-scraper/internal/db"
	"github.com/shinichikudo1st/job-scraper/internal/models"
)

type fakeProfileStore struct {
	profile *models.Profile
	err     error
}

func (f *fakeProfileStore) GetActiveProfile() (*models.Profile, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.profile == nil {
		return nil, db.ErrProfileNotFound
	}
	return f.profile, nil
}

func (f *fakeProfileStore) SaveActiveProfile(name, cvText string) (*models.Profile, error) {
	if f.err != nil {
		return nil, f.err
	}
	if strings.TrimSpace(cvText) == "" {
		return nil, errors.New("cv text is required")
	}
	f.profile = &models.Profile{
		ID:       1,
		Name:     name,
		CVText:   strings.TrimSpace(cvText),
		IsActive: true,
	}
	return f.profile, nil
}

func TestGetActiveProfileNotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterProfileRoutes(r, &fakeProfileStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/profile/active", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var body profileJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if body.Configured {
		t.Fatalf("expected profile to be unconfigured: %+v", body)
	}
}

func TestSaveActiveProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &fakeProfileStore{}
	r := gin.New()
	RegisterProfileRoutes(r, store)

	body := strings.NewReader(`{"name":"Main","cv_text":"Go developer"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/profile/active", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var got profileJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if !got.Configured || got.CVText != "Go developer" || got.Name != "Main" {
		t.Fatalf("unexpected profile response: %+v", got)
	}
}
