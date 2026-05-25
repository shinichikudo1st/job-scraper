package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shinichikudo1st/job-scraper/internal/db"
	"github.com/shinichikudo1st/job-scraper/internal/scraper"
	"gorm.io/gorm"
)

const scraperSettingsKey = "scraper.onlinejobsph"

type ScraperSettingsHandler struct {
	DB *gorm.DB
}

type scraperSettingsJSON struct {
	SearchURL        string   `json:"search_url"`
	Keywords         []string `json:"keywords"`
	ExcludedKeywords []string `json:"excluded_keywords"`
	MaxPages         int      `json:"max_pages"`
	MinSalary        int      `json:"min_salary"`
	RequestDelayMS   int      `json:"request_delay_ms"`
}

func RegisterScraperSettingsRoutes(r gin.IRoutes, conn *gorm.DB) {
	h := &ScraperSettingsHandler{DB: conn}
	r.GET("/api/settings/scraper", h.Get)
	r.POST("/api/settings/scraper", h.Save)
}

func DefaultScraperSettings() scraperSettingsJSON {
	return scraperSettingsJSON{
		SearchURL:        scraper.DefaultOnlineJobsPHBaseURL + "/jobseekers/jobsearch",
		Keywords:         []string{},
		ExcludedKeywords: []string{},
		MaxPages:         10,
		MinSalary:        0,
		RequestDelayMS:   1000,
	}
}

func (h *ScraperSettingsHandler) Get(c *gin.Context) {
	if h.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settings store not configured"})
		return
	}
	settings, err := db.GetJSONSetting(h.DB, scraperSettingsKey, DefaultScraperSettings())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *ScraperSettingsHandler) Save(c *gin.Context) {
	if h.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settings store not configured"})
		return
	}

	var req scraperSettingsJSON
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scraper settings payload"})
		return
	}
	req = normalizeScraperSettings(req)
	if err := db.SetJSONSetting(h.DB, scraperSettingsKey, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, req)
}

func normalizeScraperSettings(settings scraperSettingsJSON) scraperSettingsJSON {
	defaults := DefaultScraperSettings()
	settings.SearchURL = strings.TrimSpace(settings.SearchURL)
	if settings.SearchURL == "" {
		settings.SearchURL = defaults.SearchURL
	}
	settings.Keywords = cleanStringList(settings.Keywords)
	settings.ExcludedKeywords = cleanStringList(settings.ExcludedKeywords)
	if settings.MaxPages <= 0 {
		settings.MaxPages = defaults.MaxPages
	}
	if settings.MaxPages > 25 {
		settings.MaxPages = 25
	}
	if settings.MinSalary < 0 {
		settings.MinSalary = 0
	}
	if settings.RequestDelayMS <= 0 {
		settings.RequestDelayMS = defaults.RequestDelayMS
	}
	if settings.RequestDelayMS < int((500 * time.Millisecond).Milliseconds()) {
		settings.RequestDelayMS = int((500 * time.Millisecond).Milliseconds())
	}
	return settings
}

func cleanStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}
