package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shinichikudo1st/job-scraper/internal/ai"
)

type AIConfigStore interface {
	Get() ai.Config
	Update(config ai.Config) (ai.Config, error)
}

type AISettingsHandler struct {
	Store AIConfigStore
}

type saveAISettingsRequest struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key"`
	Think    bool   `json:"think"`
}

func RegisterAISettingsRoutes(r gin.IRoutes, store AIConfigStore) {
	h := &AISettingsHandler{Store: store}
	r.GET("/api/ai/settings", h.Get)
	r.POST("/api/ai/settings", h.Save)
}

func (h *AISettingsHandler) Get(c *gin.Context) {
	if h.Store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ai settings store not configured"})
		return
	}
	c.JSON(http.StatusOK, ai.ToPublicConfig(h.Store.Get()))
}

func (h *AISettingsHandler) Save(c *gin.Context) {
	if h.Store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ai settings store not configured"})
		return
	}

	var req saveAISettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ai settings payload"})
		return
	}

	config, err := h.Store.Update(ai.Config{
		Provider: req.Provider,
		BaseURL:  req.BaseURL,
		Model:    req.Model,
		APIKey:   req.APIKey,
		Think:    req.Think,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ai.ToPublicConfig(config))
}
