package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LinkPreview struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	URL         string    `gorm:"not null;uniqueIndex" json:"url"`
	Title       string    `json:"title,omitempty"`
	Description string    `json:"description,omitempty"`
	ImageURL    string    `json:"image_url,omitempty"`
	SiteName    string    `json:"site_name,omitempty"`
	FaviconURL  string    `json:"favicon_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (lp *LinkPreview) BeforeCreate(tx *gorm.DB) error {
	if lp.ID == "" {
		lp.ID = uuid.New().String()
	}
	return nil
}

// GetOrCreateLinkPreview fetches existing preview or creates placeholder
func GetOrCreateLinkPreview(db *gorm.DB, url string) (*LinkPreview, bool, error) {
	var preview LinkPreview
	err := db.Where("url = ?", url).First(&preview).Error

	if err == nil {
		return &preview, false, nil // existing
	}

	if err == gorm.ErrRecordNotFound {
		// Create new preview (will be populated by fetcher)
		preview = LinkPreview{
			URL: url,
		}
		if err := db.Create(&preview).Error; err != nil {
			return nil, false, err
		}
		return &preview, true, nil // new
	}

	return nil, false, err
}

// UpdateLinkPreview updates preview with fetched metadata
func UpdateLinkPreview(db *gorm.DB, id string, title, description, imageURL, siteName, faviconURL string) error {
	return db.Model(&LinkPreview{}).Where("id = ?", id).Updates(map[string]interface{}{
		"title":       title,
		"description": description,
		"image_url":   imageURL,
		"site_name":   siteName,
		"favicon_url": faviconURL,
	}).Error
}
