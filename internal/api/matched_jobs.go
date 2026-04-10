package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shinichikudo1st/job-scraper/internal/db"
	"github.com/shinichikudo1st/job-scraper/internal/export"
	"github.com/shinichikudo1st/job-scraper/internal/models"
	"gorm.io/gorm"
)

const (
	defaultMatchedLimit = 20
	maxMatchedLimit     = 100
	maxExportRows       = 10000
)

type MatchedJobsReader interface {
	ListMatchedJobs(notified bool, limit, offset int) ([]models.Job, int64, error)
}

type GormMatchedJobsReader struct {
	DB *gorm.DB
}

func (r *GormMatchedJobsReader) ListMatchedJobs(notified bool, limit, offset int) ([]models.Job, int64, error) {
	return db.GetMatchedJobsPaginated(r.DB, notified, limit, offset)
}

type MatchedJobsHandler struct {
	Reader MatchedJobsReader
}

type matchedJobJSON struct {
	ID          int        `json:"id"`
	Title       string     `json:"title"`
	Company     *string    `json:"company,omitempty"`
	Salary      *string    `json:"salary,omitempty"`
	URL         string     `json:"url"`
	MatchScore  *int       `json:"match_score"`
	MatchReason *string    `json:"match_reason"`
	PostedAt    *time.Time `json:"posted_at"`
}

type matchedListResponse struct {
	Items  []matchedJobJSON `json:"items"`
	Total  int64            `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

func RegisterMatchedJobsRoutes(r gin.IRoutes, reader MatchedJobsReader) {
	h := &MatchedJobsHandler{Reader: reader}
	r.GET("/api/jobs/matched", h.ListMatched)
	r.GET("/api/jobs/matched/export", h.ExportMatched)
}

func (h *MatchedJobsHandler) ListMatched(c *gin.Context) {
	if h.Reader == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "matched jobs reader not configured"})
		return
	}

	notified := parseQueryBool(c, "notified", false)
	limit := parseQueryInt(c, "limit", defaultMatchedLimit, 1, maxMatchedLimit)
	offset := parseQueryInt(c, "offset", 0, 0, 1_000_000)

	jobs, total, err := h.Reader.ListMatchedJobs(notified, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]matchedJobJSON, 0, len(jobs))
	for _, j := range jobs {
		items = append(items, toMatchedJobJSON(j))
	}

	c.JSON(http.StatusOK, matchedListResponse{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func (h *MatchedJobsHandler) ExportMatched(c *gin.Context) {
	if h.Reader == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "matched jobs reader not configured"})
		return
	}

	format := strings.ToLower(strings.TrimSpace(c.Query("format")))
	if format != "csv" && format != "xlsx" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query format must be csv or xlsx"})
		return
	}

	notified := parseQueryBool(c, "notified", false)
	jobs, _, err := h.Reader.ListMatchedJobs(notified, maxExportRows, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	filename := fmt.Sprintf("matched-jobs-%s", time.Now().UTC().Format("20060102-150405"))
	switch format {
	case "csv":
		b, err := export.ExportCSV(jobs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Disposition", `attachment; filename="`+filename+`.csv"`)
		c.Data(http.StatusOK, "text/csv; charset=utf-8", b)
	case "xlsx":
		b, err := export.ExportExcel(jobs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Disposition", `attachment; filename="`+filename+`.xlsx"`)
		c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", b)
	}
}

func toMatchedJobJSON(j models.Job) matchedJobJSON {
	return matchedJobJSON{
		ID:          j.ID,
		Title:       j.Title,
		Company:     j.Company,
		Salary:      j.Salary,
		URL:         j.URL,
		MatchScore:  j.MatchScore,
		MatchReason: j.MatchReason,
		PostedAt:    j.PostedAt,
	}
}

func parseQueryBool(c *gin.Context, key string, defaultVal bool) bool {
	s := strings.TrimSpace(strings.ToLower(c.Query(key)))
	if s == "" {
		return defaultVal
	}
	return s == "true" || s == "1" || s == "yes"
}

func parseQueryInt(c *gin.Context, key string, defaultVal, minVal, maxVal int) int {
	s := strings.TrimSpace(c.Query(key))
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < minVal {
		return defaultVal
	}
	if n > maxVal {
		return maxVal
	}
	return n
}
