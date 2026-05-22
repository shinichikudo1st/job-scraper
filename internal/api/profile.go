package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shinichikudo1st/job-scraper/internal/db"
	"github.com/shinichikudo1st/job-scraper/internal/models"
	"gorm.io/gorm"
)

type ProfileStore interface {
	GetActiveProfile() (*models.Profile, error)
	SaveActiveProfile(name, cvText string) (*models.Profile, error)
}

type GormProfileStore struct {
	DB *gorm.DB
}

func (s *GormProfileStore) GetActiveProfile() (*models.Profile, error) {
	return db.GetActiveProfile(s.DB)
}

func (s *GormProfileStore) SaveActiveProfile(name, cvText string) (*models.Profile, error) {
	return db.UpsertActiveProfile(s.DB, name, cvText)
}

type ProfileHandler struct {
	Store ProfileStore
}

type profileJSON struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	CVText     string `json:"cv_text"`
	IsActive   bool   `json:"is_active"`
	Configured bool   `json:"configured"`
}

type saveProfileRequest struct {
	Name   string `json:"name"`
	CVText string `json:"cv_text"`
}

func RegisterProfileRoutes(r gin.IRoutes, store ProfileStore) {
	h := &ProfileHandler{Store: store}
	r.GET("/api/profile/active", h.GetActive)
	r.POST("/api/profile/active", h.SaveActive)
}

func (h *ProfileHandler) GetActive(c *gin.Context) {
	if h.Store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "profile store not configured"})
		return
	}

	profile, err := h.Store.GetActiveProfile()
	if err != nil {
		if errors.Is(err, db.ErrProfileNotFound) {
			c.JSON(http.StatusOK, profileJSON{Configured: false})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toProfileJSON(profile))
}

func (h *ProfileHandler) SaveActive(c *gin.Context) {
	if h.Store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "profile store not configured"})
		return
	}

	var req saveProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile payload"})
		return
	}

	if strings.TrimSpace(req.CVText) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cv_text is required"})
		return
	}

	profile, err := h.Store.SaveActiveProfile(req.Name, req.CVText)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toProfileJSON(profile))
}

func toProfileJSON(profile *models.Profile) profileJSON {
	if profile == nil {
		return profileJSON{Configured: false}
	}
	return profileJSON{
		ID:         profile.ID,
		Name:       profile.Name,
		CVText:     profile.CVText,
		IsActive:   profile.IsActive,
		Configured: true,
	}
}
