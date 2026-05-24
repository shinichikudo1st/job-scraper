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
	ListJobsByStatus(status, search string, limit, offset int) ([]models.Job, int64, error)
	CountJobsByStatus() (map[string]int64, error)
	UpdateJobStatus(id int, status string) error
	RequeueJob(id int) error
}

type GormMatchedJobsReader struct {
	DB *gorm.DB
}

func (r *GormMatchedJobsReader) ListMatchedJobs(notified bool, limit, offset int) ([]models.Job, int64, error) {
	return db.GetMatchedJobsPaginated(r.DB, notified, limit, offset)
}

func (r *GormMatchedJobsReader) ListJobsByStatus(status, search string, limit, offset int) ([]models.Job, int64, error) {
	return db.GetJobsByStatusPaginated(r.DB, status, search, limit, offset)
}

func (r *GormMatchedJobsReader) CountJobsByStatus() (map[string]int64, error) {
	return db.CountJobsByStatus(r.DB)
}

func (r *GormMatchedJobsReader) UpdateJobStatus(id int, status string) error {
	return db.UpdateJobWorkflowStatus(r.DB, id, status)
}

func (r *GormMatchedJobsReader) RequeueJob(id int) error {
	return db.RequeueJobForAnalysis(r.DB, id)
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
	Status      string     `json:"status"`
	LastError   *string    `json:"analysis_last_error,omitempty"`
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
	r.GET("/api/jobs", h.ListByStatus)
	r.GET("/api/jobs/stats", h.Stats)
	r.POST("/api/jobs/:id/status", h.UpdateStatus)
	r.POST("/api/jobs/:id/reanalyze", h.Reanalyze)
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

func (h *MatchedJobsHandler) ListByStatus(c *gin.Context) {
	if h.Reader == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "jobs reader not configured"})
		return
	}
	status := strings.ToLower(strings.TrimSpace(c.Query("status")))
	if status == "" || status == "new" {
		status = "matched"
	}
	search := c.Query("search")
	limit := parseQueryInt(c, "limit", defaultMatchedLimit, 1, maxMatchedLimit)
	offset := parseQueryInt(c, "offset", 0, 0, 1_000_000)

	jobs, total, err := h.Reader.ListJobsByStatus(status, search, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]matchedJobJSON, 0, len(jobs))
	for _, j := range jobs {
		items = append(items, toMatchedJobJSON(j))
	}
	c.JSON(http.StatusOK, matchedListResponse{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *MatchedJobsHandler) Stats(c *gin.Context) {
	if h.Reader == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "jobs reader not configured"})
		return
	}
	counts, err := h.Reader.CountJobsByStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"counts": counts})
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

func (h *MatchedJobsHandler) UpdateStatus(c *gin.Context) {
	if h.Reader == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "jobs reader not configured"})
		return
	}
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status payload"})
		return
	}
	if err := h.Reader.UpdateJobStatus(id, req.Status); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *MatchedJobsHandler) Reanalyze(c *gin.Context) {
	if h.Reader == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "jobs reader not configured"})
		return
	}
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	if err := h.Reader.RequeueJob(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
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
		Status:      j.Status,
		LastError:   j.LastError,
	}
}

func parsePathID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return 0, false
	}
	return id, true
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
