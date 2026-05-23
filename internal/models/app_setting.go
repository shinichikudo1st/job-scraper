package models

import "time"

type AppSetting struct {
	Key       string    `json:"key" gorm:"column:key;primaryKey"`
	Value     string    `json:"value" gorm:"column:value"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

func (AppSetting) TableName() string {
	return "app_settings"
}
