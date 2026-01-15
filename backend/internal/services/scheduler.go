package services

import (
	"log"
	"time"

	"messenger/internal/database"
	"messenger/internal/models"
)

// MessageDeliveryFunc is a callback for delivering scheduled messages
type MessageDeliveryFunc func(msg *models.Message)

// SchedulerService handles scheduled message delivery
type SchedulerService struct {
	deliverFunc MessageDeliveryFunc
	stopChan    chan struct{}
}

// NewSchedulerService creates a new scheduler service
func NewSchedulerService(deliverFunc MessageDeliveryFunc) *SchedulerService {
	return &SchedulerService{
		deliverFunc: deliverFunc,
		stopChan:    make(chan struct{}),
	}
}

// Start begins the scheduler loop
func (s *SchedulerService) Start() {
	go s.run()
}

// Stop halts the scheduler
func (s *SchedulerService) Stop() {
	close(s.stopChan)
}

func (s *SchedulerService) run() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.processScheduledMessages()
		case <-s.stopChan:
			return
		}
	}
}

func (s *SchedulerService) processScheduledMessages() {
	var messages []models.Message
	now := time.Now()

	// Find messages scheduled for now or earlier that haven't been delivered yet
	err := database.DB.
		Where("scheduled_at IS NOT NULL AND scheduled_at <= ? AND status = ?", now, models.MessageStatusSent).
		Where("deleted_at IS NULL").
		Find(&messages).Error

	if err != nil {
		log.Printf("Scheduler: Error fetching scheduled messages: %v", err)
		return
	}

	for _, msg := range messages {
		// Clear scheduled_at to mark as processed
		database.DB.Model(&msg).Update("scheduled_at", nil)

		// Deliver via callback
		if s.deliverFunc != nil {
			s.deliverFunc(&msg)
		}

		log.Printf("Scheduler: Delivered scheduled message %s", msg.ID)
	}
}

// ScheduleMessage creates a scheduled message
func ScheduleMessage(senderID string, recipientID *string, groupID *string, content string, mediaID *string, scheduledAt time.Time) (*models.Message, error) {
	msg := &models.Message{
		SenderID:    senderID,
		RecipientID: recipientID,
		GroupID:     groupID,
		Content:     content,
		MediaID:     mediaID,
		ScheduledAt: &scheduledAt,
		Status:      models.MessageStatusSent, // Will be updated when delivered
	}

	if err := database.DB.Create(msg).Error; err != nil {
		return nil, err
	}

	return msg, nil
}

// GetScheduledMessages returns all pending scheduled messages for a user
func GetScheduledMessages(userID string) ([]models.Message, error) {
	var messages []models.Message
	err := database.DB.
		Where("sender_id = ? AND scheduled_at IS NOT NULL AND scheduled_at > ? AND deleted_at IS NULL", userID, time.Now()).
		Order("scheduled_at ASC").
		Find(&messages).Error
	return messages, err
}

// CancelScheduledMessage cancels a scheduled message
func CancelScheduledMessage(messageID, userID string) error {
	return database.DB.
		Where("id = ? AND sender_id = ? AND scheduled_at IS NOT NULL AND scheduled_at > ?", messageID, userID, time.Now()).
		Delete(&models.Message{}).Error
}
