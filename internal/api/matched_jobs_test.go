package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shinichikudo1st/job-scraper/internal/models"
)

type fakeMatchedReader struct {
	jobs  []models.Job
	total int64
	err   error
}

func (f *fakeMatchedReader) ListMatchedJobs(notified bool, limit, offset int) ([]models.Job, int64, error) {
	if f.err != nil {
		return nil, 0, f.err
	}
	_ = notified
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = defaultMatchedLimit
	}
	if offset >= len(f.jobs) {
		return []models.Job{}, f.total, nil
	}
	end := offset + limit
	if end > len(f.jobs) {
		end = len(f.jobs)
	}
	return f.jobs[offset:end], f.total, nil
}

func TestListMatchedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reason := "Strong Go fit"
	score := 92
	company := "Acme"
	salary := "50k"
	posted := time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC)

	reader := &fakeMatchedReader{
		jobs: []models.Job{
			{
				ID:          7,
				Title:       "Backend Engineer",
				Company:     &company,
				Salary:      &salary,
				URL:         "https://www.onlinejobs.ph/jobseekers/job/foo-7",
				MatchScore:  &score,
				MatchReason: &reason,
				PostedAt:    &posted,
				IsMatch:     true,
			},
		},
		total: 1,
	}

	r := gin.New()
	RegisterMatchedJobsRoutes(r, reader)

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/matched?notified=false&limit=20&offset=0", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var body matchedListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if body.Total != 1 || body.Limit != 20 || body.Offset != 0 {
		t.Fatalf("unexpected meta: %+v", body)
	}
	if len(body.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(body.Items))
	}
	item := body.Items[0]
	if item.ID != 7 || item.Title != "Backend Engineer" || item.URL == "" {
		t.Fatalf("unexpected item: %+v", item)
	}
	if item.MatchScore == nil || *item.MatchScore != 92 {
		t.Fatalf("unexpected match_score: %+v", item.MatchScore)
	}
	if item.MatchReason == nil || *item.MatchReason != reason {
		t.Fatalf("unexpected match_reason: %+v", item.MatchReason)
	}
	if item.Company == nil || *item.Company != company {
		t.Fatalf("unexpected company: %+v", item.Company)
	}
}

func TestExportMatchedCSV(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reader := &fakeMatchedReader{
		jobs: []models.Job{
			{ID: 1, Title: "A", URL: "https://example.com/a", IsMatch: true},
		},
		total: 1,
	}
	r := gin.New()
	RegisterMatchedJobsRoutes(r, reader)

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/matched/export?format=csv&notified=false", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "text/csv; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	body := rec.Body.String()
	if len(body) < 10 {
		t.Fatalf("unexpected csv body: %q", body)
	}
	if body[:3] != "id," {
		t.Fatalf("expected csv header starting with id,: %q", body)
	}
}

func TestExportMatchedXLSX(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reader := &fakeMatchedReader{
		jobs: []models.Job{
			{ID: 2, Title: "B", URL: "https://example.com/b", IsMatch: true},
		},
		total: 1,
	}
	r := gin.New()
	RegisterMatchedJobsRoutes(r, reader)

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/matched/export?format=xlsx&notified=false", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	b := rec.Body.Bytes()
	if len(b) < 4 || string(b[0:2]) != "PK" {
		t.Fatalf("expected zip/xlsx magic PK, got %d bytes", len(b))
	}
}

func TestExportMatchedInvalidFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterMatchedJobsRoutes(r, &fakeMatchedReader{jobs: nil, total: 0})

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/matched/export?format=pdf", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
