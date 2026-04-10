package server

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/shinichikudo1st/job-scraper/internal/api"
	"gorm.io/gorm"
)

// NewRouter registers API routes and the matcher UI.
// webRoot is the directory containing index.html (e.g. "web"). If empty, no UI routes are mounted.
// db may be nil in tests; matched-jobs routes are only registered when db is non-nil.
//
// Note: Gin's Static("/", ...) catch-all conflicts with /api in recent Gin versions, so we serve
// the shell page explicitly at GET /.
func NewRouter(db *gorm.DB, webRoot string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.RedirectTrailingSlash = false

	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	if db != nil {
		reader := &api.GormMatchedJobsReader{DB: db}
		api.RegisterMatchedJobsRoutes(r, reader)
	}

	if webRoot != "" {
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
