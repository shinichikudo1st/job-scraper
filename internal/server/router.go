package server

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/shinichikudo1st/job-scraper/internal/ai"
	"github.com/shinichikudo1st/job-scraper/internal/api"
	webassets "github.com/shinichikudo1st/job-scraper/web"
	"gorm.io/gorm"
)

// NewRouter registers API routes and the matcher UI.
// webRoot is an optional development override directory containing index.html.
// If webRoot is empty, the embedded UI is served.
// db may be nil in tests; matched-jobs routes are only registered when db is non-nil.
//
// Note: Gin's Static("/", ...) catch-all conflicts with /api in recent Gin versions, so we serve
// the shell page explicitly at GET /.
func NewRouter(db *gorm.DB, webRoot string, aiStores ...*ai.ConfigStore) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.RedirectTrailingSlash = false

	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	if db != nil {
		reader := &api.GormMatchedJobsReader{DB: db}
		api.RegisterMatchedJobsRoutes(r, reader)
		profileStore := &api.GormProfileStore{DB: db}
		api.RegisterProfileRoutes(r, profileStore)
		api.RegisterScraperSettingsRoutes(r, db)
		api.RegisterScrapeRunRoutes(r, db)
	}
	if len(aiStores) > 0 && aiStores[0] != nil {
		api.RegisterAISettingsRoutes(r, aiStores[0])
	}

	if webRoot == "" {
		r.GET("/", func(c *gin.Context) {
			c.Data(http.StatusOK, "text/html; charset=utf-8", webassets.IndexHTML())
		})
	} else {
		root := webRoot
		if abs, err := filepath.Abs(webRoot); err == nil {
			root = abs
		}
		indexPath := filepath.Join(root, "index.html")
		r.GET("/", func(c *gin.Context) {
			c.File(indexPath)
		})
	}

	return r
}
