package db

import (
	"time"

	"github.com/shinichikudo1st/job-scraper/internal/models"
	"gorm.io/gorm"
)

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
