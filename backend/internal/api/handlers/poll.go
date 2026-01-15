package handlers

import (
	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
	"messenger/internal/websocket"
)

type PollHandler struct {
	hub *websocket.Hub
}

func NewPollHandler(hub *websocket.Hub) *PollHandler {
	return &PollHandler{hub: hub}
}

type CreatePollRequest struct {
	Question    string   `json:"question"`
	Options     []string `json:"options"`
	MultiSelect bool     `json:"multi_select"`
	Anonymous   bool     `json:"anonymous"`
	GroupID     *string  `json:"group_id,omitempty"`
	RecipientID *string  `json:"recipient_id,omitempty"` // For DM polls
}

// Create creates a new poll
func (h *PollHandler) Create(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req CreatePollRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Question == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Question is required",
		})
	}

	if len(req.Options) < 2 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "At least 2 options are required",
		})
	}

	if len(req.Options) > 10 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Maximum 10 options allowed",
		})
	}

	// Verify group membership if group poll
	if req.GroupID != nil {
		var membership models.GroupMember
		if err := database.DB.Where("group_id = ? AND user_id = ?", *req.GroupID, userID).First(&membership).Error; err != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "You are not a member of this group",
			})
		}
	}

	poll, err := models.CreatePoll(
		database.DB,
		userID,
		req.Question,
		req.Options,
		req.MultiSelect,
		req.Anonymous,
		req.GroupID,
		req.RecipientID,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create poll",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(poll.ToPollResponse(database.DB, userID))
}

// Get retrieves a poll by ID
func (h *PollHandler) Get(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	pollID := c.Params("id")

	var poll models.Poll
	if err := database.DB.Preload("Options").First(&poll, "id = ?", pollID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Poll not found",
		})
	}

	return c.JSON(poll.ToPollResponse(database.DB, userID))
}

type VoteRequest struct {
	OptionID string `json:"option_id"`
}

// Vote adds or removes a vote on a poll option
func (h *PollHandler) Vote(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	pollID := c.Params("id")

	var req VoteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	var poll models.Poll
	if err := database.DB.Preload("Options").First(&poll, "id = ?", pollID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Poll not found",
		})
	}

	if poll.Closed {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Poll is closed",
		})
	}

	// Check access
	if poll.GroupID != nil {
		var membership models.GroupMember
		if err := database.DB.Where("group_id = ? AND user_id = ?", *poll.GroupID, userID).First(&membership).Error; err != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "You don't have access to this poll",
			})
		}
	}

	if err := poll.Vote(database.DB, userID, req.OptionID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to vote",
		})
	}

	// Get updated poll response
	response := poll.ToPollResponse(database.DB, userID)

	// Broadcast vote update to participants
	h.broadcastPollUpdate(&poll, &response)

	return c.JSON(response)
}

// Close closes a poll for voting
func (h *PollHandler) Close(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	pollID := c.Params("id")

	var poll models.Poll
	if err := database.DB.Preload("Options").First(&poll, "id = ?", pollID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Poll not found",
		})
	}

	// Only creator can close
	if poll.CreatorID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Only the poll creator can close it",
		})
	}

	if err := poll.Close(database.DB); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to close poll",
		})
	}

	poll.Closed = true
	response := poll.ToPollResponse(database.DB, userID)

	// Broadcast poll closed
	h.broadcastPollUpdate(&poll, &response)

	return c.JSON(response)
}

func (h *PollHandler) broadcastPollUpdate(poll *models.Poll, response *models.PollResponse) {
	update := map[string]interface{}{
		"type": "poll_update",
		"poll": response,
	}

	if poll.GroupID != nil {
		// Broadcast to group
		h.hub.BroadcastToGroup(*poll.GroupID, update)
	} else if poll.RecipientID != nil {
		// Send to both participants in DM
		h.hub.SendJSONToUser(poll.CreatorID, update)
		h.hub.SendJSONToUser(*poll.RecipientID, update)
	}
}
