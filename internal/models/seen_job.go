package models

import "time"

type SeenJob struct {
	ExternalID  string    `json:"external_id" gorm:"column:external_id;primaryKey"`
	URL         string    `json:"url" gorm:"column:url"`
	Title       string    `json:"title" gorm:"column:title"`
	Status      string    `json:"status" gorm:"column:status"`
	FirstSeenAt time.Time `json:"first_seen_at" gorm:"column:first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at" gorm:"column:last_seen_at"`
}

func (SeenJob) TableName() string {
	return "seen_jobs"
}
