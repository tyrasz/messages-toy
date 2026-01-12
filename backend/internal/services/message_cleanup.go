package services

import (
	"log"
	"time"

	"gorm.io/gorm"
	"messenger/internal/models"
)

// MessageCleanupService handles periodic cleanup of expired messages
type MessageCleanupService struct {
	db       *gorm.DB
	interval time.Duration
	stopChan chan struct{}
}

// NewMessageCleanupService creates a new cleanup service
func NewMessageCleanupService(db *gorm.DB, interval time.Duration) *MessageCleanupService {
	return &MessageCleanupService{
		db:       db,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

// Start begins the periodic cleanup
func (s *MessageCleanupService) Start() {
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		// Run once at startup
		s.cleanupExpiredMessages()

		for {
			select {
			case <-ticker.C:
				s.cleanupExpiredMessages()
			case <-s.stopChan:
				return
			}
		}
	}()
	log.Printf("Message cleanup service started (interval: %v)", s.interval)
}

// Stop halts the cleanup service
func (s *MessageCleanupService) Stop() {
	close(s.stopChan)
	log.Println("Message cleanup service stopped")
}

// cleanupExpiredMessages deletes messages that have expired
func (s *MessageCleanupService) cleanupExpiredMessages() {
	now := time.Now()

	// Soft delete expired messages (set deleted_at)
	result := s.db.Model(&models.Message{}).
		Where("expires_at IS NOT NULL AND expires_at < ? AND deleted_at IS NULL", now).
		Update("deleted_at", now)

	if result.Error != nil {
		log.Printf("Error cleaning up expired messages: %v", result.Error)
		return
	}

	if result.RowsAffected > 0 {
		log.Printf("Cleaned up %d expired messages", result.RowsAffected)
	}
}

// CleanupNow triggers an immediate cleanup (useful for testing)
func (s *MessageCleanupService) CleanupNow() {
	s.cleanupExpiredMessages()
}
