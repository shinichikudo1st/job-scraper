package db

import (
	"errors"
	"strings"
	"time"

	"github.com/shinichikudo1st/job-scraper/internal/models"
	"gorm.io/gorm"
)

var ErrProfileNotFound = errors.New("active profile not found")

func FetchPendingJobs(conn *gorm.DB, limit int, postedAfter time.Time) ([]models.Job, error) {
	if limit <= 0 {
		limit = 20
	}

	var jobs []models.Job
	err := conn.
		Where("match_score IS NULL").
		Where("posted_at >= ?", postedAfter).
		Order("posted_at DESC").
		Limit(limit).
		Find(&jobs).Error
	if err != nil {
		return nil, err
	}

	return jobs, nil
}

func UpdateJobMatch(conn *gorm.DB, id int, isMatch bool, score int, reason string) error {
	now := time.Now().UTC()

	return conn.
		Model(&models.Job{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"is_match":     isMatch,
			"match_score":  score,
			"match_reason": reason,
			"analyzed_at":  now,
		}).Error
}

func GetMatchedJobs(conn *gorm.DB, notified bool, limit int) ([]models.Job, error) {
	jobs, _, err := GetMatchedJobsPaginated(conn, notified, limit, 0)
	return jobs, err
}

// GetMatchedJobsPaginated returns is_match=true rows filtered by notified, ordered for UI.
func GetMatchedJobsPaginated(conn *gorm.DB, notified bool, limit, offset int) ([]models.Job, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	base := conn.Model(&models.Job{}).
		Where("is_match = ?", true).
		Where("notified = ?", notified)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var jobs []models.Job
	err := conn.
		Where("is_match = ?", true).
		Where("notified = ?", notified).
		Order("match_score DESC NULLS LAST, posted_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&jobs).Error
	if err != nil {
		return nil, 0, err
	}

	return jobs, total, nil
}

func GetActiveProfile(conn *gorm.DB) (*models.Profile, error) {
	var profile models.Profile
	err := conn.
		Where("is_active = ?", true).
		Order("updated_at DESC, id DESC").
		First(&profile).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProfileNotFound
		}
		return nil, err
	}
	return &profile, nil
}

func UpsertActiveProfile(conn *gorm.DB, name, cvText string) (*models.Profile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Default"
	}
	cvText = strings.TrimSpace(cvText)
	if cvText == "" {
		return nil, errors.New("cv text is required")
	}

	var saved models.Profile
	err := conn.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()

		var existing models.Profile
		err := tx.
			Where("is_active = ?", true).
			Order("updated_at DESC, id DESC").
			First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if existing.ID != 0 {
			existing.Name = name
			existing.CVText = cvText
			existing.IsActive = true
			existing.UpdatedAt = now
			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
			saved = existing
			return nil
		}

		if err := tx.Model(&models.Profile{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
			return err
		}
		profile := models.Profile{
			Name:      name,
			CVText:    cvText,
			IsActive:  true,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := tx.Create(&profile).Error; err != nil {
			return err
		}
		saved = profile
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

type GormProfileCVProvider struct {
	DB *gorm.DB
}

func (p *GormProfileCVProvider) ActiveCVText() (string, error) {
	if p == nil || p.DB == nil {
		return "", errors.New("profile cv provider is not configured")
	}
	profile, err := GetActiveProfile(p.DB)
	if err != nil {
		return "", err
	}
	cvText := strings.TrimSpace(profile.CVText)
	if cvText == "" {
		return "", errors.New("active profile cv text is empty")
	}
	return cvText, nil
}

func HasSeenJob(conn *gorm.DB, externalID string) (bool, error) {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return false, errors.New("external id is required")
	}
	var count int64
	if err := conn.Model(&models.SeenJob{}).Where("external_id = ?", externalID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func MarkSeenJob(conn *gorm.DB, externalID, url, title, status string) error {
	externalID = strings.TrimSpace(externalID)
	url = strings.TrimSpace(url)
	title = strings.TrimSpace(title)
	status = strings.TrimSpace(status)
	if externalID == "" {
		return errors.New("external id is required")
	}
	if url == "" {
		return errors.New("url is required")
	}
	if title == "" {
		title = url
	}
	if status == "" {
		status = "seen"
	}

	now := time.Now().UTC()
	seen := models.SeenJob{
		ExternalID:  externalID,
		URL:         url,
		Title:       title,
		Status:      status,
		FirstSeenAt: now,
		LastSeenAt:  now,
	}
	return conn.Transaction(func(tx *gorm.DB) error {
		var existing models.SeenJob
		err := tx.Where("external_id = ?", externalID).First(&existing).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return tx.Create(&seen).Error
			}
			return err
		}
		return tx.Model(&models.SeenJob{}).
			Where("external_id = ?", externalID).
			Updates(map[string]any{
				"url":          url,
				"title":        title,
				"status":       status,
				"last_seen_at": now,
			}).Error
	})
}

type GormSeenJobStore struct {
	DB *gorm.DB
}

func (s *GormSeenJobStore) IsSeen(externalID string) (bool, error) {
	if s == nil || s.DB == nil {
		return false, errors.New("seen job store is not configured")
	}
	return HasSeenJob(s.DB, externalID)
}

func (s *GormSeenJobStore) MarkSeen(externalID, url, title string) error {
	if s == nil || s.DB == nil {
		return errors.New("seen job store is not configured")
	}
	return MarkSeenJob(s.DB, externalID, url, title, "seen")
}
