package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MediaStatus string

const (
	MediaStatusPending  MediaStatus = "pending"
	MediaStatusApproved MediaStatus = "approved"
	MediaStatusRejected MediaStatus = "rejected"
	MediaStatusReview   MediaStatus = "review"
)

type Media struct {
	ID          string      `gorm:"primaryKey" json:"id"`
	UploaderID  string      `gorm:"not null;index" json:"uploader_id"`
	Filename    string      `gorm:"not null" json:"filename"`
	ContentType string      `gorm:"not null" json:"content_type"`
	Size        int64       `json:"size"`
	Status      MediaStatus `gorm:"default:pending" json:"status"`
	ScanResult  string      `gorm:"type:text" json:"scan_result,omitempty"`
	StoragePath string      `json:"storage_path,omitempty"`
	URL         string      `json:"url,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`

	Uploader User `gorm:"foreignKey:UploaderID" json:"-"`
}

func (m *Media) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	if m.Status == "" {
		m.Status = MediaStatusPending
	}
	return nil
}
