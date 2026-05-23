package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shinichikudo1st/job-scraper/internal/db"
	"gorm.io/gorm"
)

func TestScraperSettingsSaveAndGet(t *testing.T) {
	conn := openTestDB(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterScraperSettingsRoutes(r, conn)

	body := strings.NewReader(`{
		"search_url":"https://www.onlinejobs.ph/jobseekers/jobsearch",
		"keywords":["go","backend","go"],
		"excluded_keywords":["caller"],
		"max_pages":3,
		"min_salary":1000,
		"request_delay_ms":100
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/settings/scraper", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST status = %d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/settings/scraper", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d body=%s", rec.Code, rec.Body.String())
	}

	var got scraperSettingsJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if got.MaxPages != 3 || got.MinSalary != 1000 {
		t.Fatalf("unexpected numeric settings: %+v", got)
	}
	if len(got.Keywords) != 2 {
		t.Fatalf("expected deduped keywords, got %+v", got.Keywords)
	}
	if got.RequestDelayMS != 500 {
		t.Fatalf("request delay should be clamped to 500ms, got %d", got.RequestDelayMS)
	}
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	conn, err := db.ConnectConfiguredDB(&db.DBConfig{
		Driver:     db.DriverSQLite,
		SQLitePath: filepath.Join(t.TempDir(), "settings.db"),
	})
	if err != nil {
		t.Fatalf("ConnectConfiguredDB() error = %v", err)
	}
	if err := db.RunSQLiteMigrations(conn); err != nil {
		t.Fatalf("RunSQLiteMigrations() error = %v", err)
	}
	sqlDB, err := conn.DB()
	if err != nil {
		t.Fatalf("DB() error = %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	return conn
}
