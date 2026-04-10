package models

import "time"

type Job struct {
	ID          int        `json:"id" gorm:"column:id"`
	ExternalID  string     `json:"external_id" gorm:"column:external_id"`
	Title       string     `json:"title" gorm:"column:title"`
	Company     *string    `json:"company" gorm:"column:company"`
	Location    *string    `json:"location" gorm:"column:location"`
	Salary      *string    `json:"salary" gorm:"column:salary"`
	Description *string    `json:"description" gorm:"column:description"`
	URL         string     `json:"url" gorm:"column:url"`
	PostedAt    *time.Time `json:"posted_at" gorm:"column:posted_at"`
	ScrapedAt   time.Time  `json:"scraped_at" gorm:"column:scraped_at"`
	AnalyzedAt  *time.Time `json:"analyzed_at" gorm:"column:analyzed_at"`
	IsMatch     bool       `json:"is_match" gorm:"column:is_match"`
	MatchScore  *int       `json:"match_score" gorm:"column:match_score"`
	MatchReason *string    `json:"match_reason" gorm:"column:match_reason"`
	Notified    bool       `json:"notified" gorm:"column:notified"`
}

func (Job) TableName() string {
	return "jobs"
}
