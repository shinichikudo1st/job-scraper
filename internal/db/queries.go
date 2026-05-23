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
		Where("status = ?", "queued").
		Where("(posted_at IS NULL OR posted_at >= ?)", postedAfter).
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

	status := "analyzed"
	if isMatch {
		status = "matched"
	}

	return conn.
		Model(&models.Job{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"is_match":             isMatch,
			"match_score":          score,
			"match_reason":         reason,
			"analyzed_at":          now,
			"status":               status,
			"analysis_retry_count": 0,
			"analysis_last_error":  nil,
		}).Error
}

func MarkJobAnalysisFailed(conn *gorm.DB, id int, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "analysis failed"
	}
	return conn.
		Model(&models.Job{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":               "failed",
			"analysis_retry_count": gorm.Expr("analysis_retry_count + 1"),
			"analysis_last_error":  reason,
		}).Error
}

func UpsertQueuedJob(conn *gorm.DB, job models.Job) error {
	job.ExternalID = strings.TrimSpace(job.ExternalID)
	job.Title = strings.TrimSpace(job.Title)
	job.URL = strings.TrimSpace(job.URL)
	if job.ExternalID == "" {
		return errors.New("external id is required")
	}
	if job.Title == "" {
		return errors.New("job title is required")
	}
	if job.URL == "" {
		return errors.New("job url is required")
	}
	if job.ScrapedAt.IsZero() {
		job.ScrapedAt = time.Now().UTC()
	}
	job.Status = "queued"

	var existing models.Job
	err := conn.Where("external_id = ?", job.ExternalID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return conn.Create(&job).Error
		}
		return err
	}

	return conn.Model(&models.Job{}).
		Where("external_id = ?", job.ExternalID).
		Updates(map[string]any{
			"title":       job.Title,
			"company":     job.Company,
			"location":    job.Location,
			"salary":      job.Salary,
			"description": job.Description,
			"url":         job.URL,
			"posted_at":   job.PostedAt,
			"scraped_at":  job.ScrapedAt,
			"status":      "queued",
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

func PruneSeenJobs(conn *gorm.DB, status string, olderThan time.Time) (int64, error) {
	status = strings.TrimSpace(status)
	if status == "" {
		return 0, errors.New("status is required")
	}
	result := conn.
		Where("status = ?", status).
		Where("last_seen_at < ?", olderThan).
		Delete(&models.SeenJob{})
	return result.RowsAffected, result.Error
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

func (s *GormSeenJobStore) MarkWithStatus(externalID, url, title, status string) error {
	if s == nil || s.DB == nil {
		return errors.New("seen job store is not configured")
	}
	return MarkSeenJob(s.DB, externalID, url, title, status)
}

type GormPipelineRepository struct {
	DB *gorm.DB
}

func (r *GormPipelineRepository) IsSeen(externalID string) (bool, error) {
	if r == nil || r.DB == nil {
		return false, errors.New("pipeline repository is not configured")
	}
	return HasSeenJob(r.DB, externalID)
}

func (r *GormPipelineRepository) MarkSeen(externalID, url, title, status string) error {
	if r == nil || r.DB == nil {
		return errors.New("pipeline repository is not configured")
	}
	return MarkSeenJob(r.DB, externalID, url, title, status)
}

func (r *GormPipelineRepository) SaveQueuedJob(job models.Job) error {
	if r == nil || r.DB == nil {
		return errors.New("pipeline repository is not configured")
	}
	return UpsertQueuedJob(r.DB, job)
}
