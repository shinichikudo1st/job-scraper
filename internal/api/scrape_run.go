package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shinichikudo1st/job-scraper/internal/db"
	"github.com/shinichikudo1st/job-scraper/internal/models"
	"github.com/shinichikudo1st/job-scraper/internal/scraper"
	"gorm.io/gorm"
)

type ScrapeRunHandler struct {
	DB *gorm.DB
}

type scrapeRunResponse struct {
	Queued int `json:"queued"`
}

func RegisterScrapeRunRoutes(r gin.IRoutes, conn *gorm.DB) {
	h := &ScrapeRunHandler{DB: conn}
	r.POST("/api/scraper/run", h.Run)
}

func (h *ScrapeRunHandler) Run(c *gin.Context) {
	if h.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "scraper store not configured"})
		return
	}

	settings, err := db.GetJSONSetting(h.DB, scraperSettingsKey, DefaultScraperSettings())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	settings = normalizeScraperSettings(settings)
	s := scraper.NewOnlineJobsPHScraper(scraper.OnlineJobsPHConfig{
		SearchURL:        settings.SearchURL,
		Keywords:         settings.Keywords,
		ExcludedKeywords: settings.ExcludedKeywords,
		MaxPages:         settings.MaxPages,
		MinSalary:        settings.MinSalary,
		RequestDelay:     time.Duration(settings.RequestDelayMS) * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()

	details, err := s.ScrapeNewDetails(ctx, &db.GormSeenJobStore{DB: h.DB})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	for _, detail := range details {
		if err := db.UpsertQueuedJob(h.DB, queuedJobFromDetail(detail)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, scrapeRunResponse{Queued: len(details)})
}

func queuedJobFromDetail(detail scraper.JobDetail) models.Job {
	description := strings.TrimSpace(detail.Description)
	salary := strings.TrimSpace(detail.Salary)
	job := models.Job{
		ExternalID: detail.ExternalID,
		Title:      strings.TrimSpace(detail.Title),
		URL:        strings.TrimSpace(detail.URL),
		ScrapedAt:  time.Now().UTC(),
		Status:     "queued",
	}
	if description != "" {
		job.Description = &description
	}
	if salary != "" {
		job.Salary = &salary
	}
	return job
}
