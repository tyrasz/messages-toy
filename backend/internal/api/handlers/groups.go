package handlers

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
	"messenger/internal/websocket"
)

type GroupsHandler struct {
	hub *websocket.Hub
}

func NewGroupsHandler(hub *websocket.Hub) *GroupsHandler {
	return &GroupsHandler{hub: hub}
}

type CreateGroupInput struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	MemberIDs   []string `json:"member_ids,omitempty"` // Initial members to add
}

func (h *GroupsHandler) Create(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var input CreateGroupInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Group name is required",
		})
	}

	// Create group
	group := models.Group{
		Name:        input.Name,
		Description: input.Description,
		CreatedBy:   userID,
	}

	if err := database.DB.Create(&group).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create group",
		})
	}

	// Add creator as owner
	ownerMember := models.GroupMember{
		GroupID: group.ID,
		UserID:  userID,
		Role:    models.GroupRoleOwner,
	}
	database.DB.Create(&ownerMember)

	// Add initial members
	for _, memberID := range input.MemberIDs {
		if memberID == userID {
			continue // Skip creator
		}
		member := models.GroupMember{
			GroupID: group.ID,
			UserID:  memberID,
			Role:    models.GroupRoleMember,
		}
		database.DB.Create(&member)

		// Notify new member via WebSocket
		h.notifyGroupEvent(memberID, "group_added", group.ID, group.Name)
	}

	// Load members for response
	var members []models.GroupMember
	database.DB.Preload("User").Where("group_id = ?", group.ID).Find(&members)

	return c.Status(fiber.StatusCreated).JSON(models.GroupResponse{
		ID:          group.ID,
		Name:        group.Name,
		Description: group.Description,
		CreatedBy:   group.CreatedBy,
		MemberCount: len(members),
		Members:     members,
		MyRole:      models.GroupRoleOwner,
		CreatedAt:   group.CreatedAt,
	})
}

func (h *GroupsHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	// Get groups where user is a member
	var memberships []models.GroupMember
	if err := database.DB.Where("user_id = ?", userID).Find(&memberships).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch groups",
		})
	}

	groupIDs := make([]string, len(memberships))
	roleMap := make(map[string]models.GroupRole)
	for i, m := range memberships {
		groupIDs[i] = m.GroupID
		roleMap[m.GroupID] = m.Role
	}

	if len(groupIDs) == 0 {
		return c.JSON(fiber.Map{"groups": []interface{}{}})
	}

	var groups []models.Group
	database.DB.Where("id IN ?", groupIDs).Find(&groups)

	// Get member counts
	type CountResult struct {
		GroupID string
		Count   int
	}
	var counts []CountResult
	database.DB.Model(&models.GroupMember{}).
		Select("group_id, count(*) as count").
		Where("group_id IN ?", groupIDs).
		Group("group_id").
		Find(&counts)

	countMap := make(map[string]int)
	for _, c := range counts {
		countMap[c.GroupID] = c.Count
	}

	response := make([]models.GroupResponse, len(groups))
	for i, g := range groups {
		response[i] = models.GroupResponse{
			ID:          g.ID,
			Name:        g.Name,
			Description: g.Description,
			AvatarURL:   g.AvatarURL,
			CreatedBy:   g.CreatedBy,
			MemberCount: countMap[g.ID],
			MyRole:      roleMap[g.ID],
			CreatedAt:   g.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{"groups": response})
}

func (h *GroupsHandler) Get(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	groupID := c.Params("id")

	// Check membership
	var membership models.GroupMember
	if err := database.DB.Where("group_id = ? AND user_id = ?", groupID, userID).First(&membership).Error; err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You are not a member of this group",
		})
	}

	var group models.Group
	if err := database.DB.First(&group, "id = ?", groupID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Group not found",
		})
	}

	// Get members
	var members []models.GroupMember
	database.DB.Preload("User").Where("group_id = ?", groupID).Find(&members)

	return c.JSON(models.GroupResponse{
		ID:          group.ID,
		Name:        group.Name,
		Description: group.Description,
		AvatarURL:   group.AvatarURL,
		CreatedBy:   group.CreatedBy,
		MemberCount: len(members),
		Members:     members,
		MyRole:      membership.Role,
		CreatedAt:   group.CreatedAt,
	})
}

type AddMemberInput struct {
	UserID string `json:"user_id"`
}

func (h *GroupsHandler) AddMember(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	groupID := c.Params("id")

	// Check if requester is admin/owner
	var membership models.GroupMember
	if err := database.DB.Where("group_id = ? AND user_id = ?", groupID, userID).First(&membership).Error; err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You are not a member of this group",
		})
	}

	if membership.Role != models.GroupRoleOwner && membership.Role != models.GroupRoleAdmin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Only admins can add members",
		})
	}

	var input AddMemberInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Check if user exists
	var user models.User
	if err := database.DB.First(&user, "id = ?", input.UserID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	// Check if already a member
	var existingMember models.GroupMember
	if err := database.DB.Where("group_id = ? AND user_id = ?", groupID, input.UserID).First(&existingMember).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "User is already a member",
		})
	}

	// Add member
	newMember := models.GroupMember{
		GroupID: groupID,
		UserID:  input.UserID,
		Role:    models.GroupRoleMember,
	}

	if err := database.DB.Create(&newMember).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to add member",
		})
	}

	// Get group name for notification
	var group models.Group
	database.DB.First(&group, "id = ?", groupID)

	// Notify new member
	h.notifyGroupEvent(input.UserID, "group_added", groupID, group.Name)

	// Notify other members
	h.broadcastToGroup(groupID, userID, "member_joined", map[string]interface{}{
		"user_id":  input.UserID,
		"username": user.Username,
	})

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Member added successfully",
		"member": fiber.Map{
			"user_id": newMember.UserID,
			"role":    newMember.Role,
		},
	})
}

func (h *GroupsHandler) RemoveMember(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	groupID := c.Params("id")
	targetUserID := c.Params("userId")

	// Check if requester is admin/owner
	var membership models.GroupMember
	if err := database.DB.Where("group_id = ? AND user_id = ?", groupID, userID).First(&membership).Error; err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You are not a member of this group",
		})
	}

	// Get target membership
	var targetMembership models.GroupMember
	if err := database.DB.Where("group_id = ? AND user_id = ?", groupID, targetUserID).First(&targetMembership).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User is not a member of this group",
		})
	}

	// Permission check: owner can remove anyone, admin can remove members only
	if membership.Role == models.GroupRoleMember {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Only admins can remove members",
		})
	}

	if membership.Role == models.GroupRoleAdmin && targetMembership.Role != models.GroupRoleMember {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Admins cannot remove other admins or the owner",
		})
	}

	if targetMembership.Role == models.GroupRoleOwner {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Cannot remove the group owner",
		})
	}

	// Remove member
	database.DB.Delete(&targetMembership)

	// Notify removed user
	var group models.Group
	database.DB.First(&group, "id = ?", groupID)
	h.notifyGroupEvent(targetUserID, "group_removed", groupID, group.Name)

	// Notify other members
	h.broadcastToGroup(groupID, userID, "member_left", map[string]interface{}{
		"user_id": targetUserID,
	})

	return c.JSON(fiber.Map{
		"message": "Member removed successfully",
	})
}

func (h *GroupsHandler) Leave(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	groupID := c.Params("id")

	var membership models.GroupMember
	if err := database.DB.Where("group_id = ? AND user_id = ?", groupID, userID).First(&membership).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "You are not a member of this group",
		})
	}

	// Owner cannot leave, must transfer ownership or delete group
	if membership.Role == models.GroupRoleOwner {
		// Check if there are other members
		var memberCount int64
		database.DB.Model(&models.GroupMember{}).Where("group_id = ?", groupID).Count(&memberCount)

		if memberCount > 1 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Owner cannot leave. Transfer ownership first or remove all members.",
			})
		}

		// If owner is the only member, delete the group
		database.DB.Delete(&membership)
		database.DB.Delete(&models.Group{}, "id = ?", groupID)

		return c.JSON(fiber.Map{
			"message": "Group deleted (you were the only member)",
		})
	}

	// Remove membership
	database.DB.Delete(&membership)

	// Notify other members
	h.broadcastToGroup(groupID, userID, "member_left", map[string]interface{}{
		"user_id": userID,
	})

	return c.JSON(fiber.Map{
		"message": "Left group successfully",
	})
}

func (h *GroupsHandler) GetMessages(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	groupID := c.Params("id")

	// Check membership
	var membership models.GroupMember
	if err := database.DB.Where("group_id = ? AND user_id = ?", groupID, userID).First(&membership).Error; err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You are not a member of this group",
		})
	}

	// Get messages
	var messages []models.Message
	database.DB.Preload("Media").
		Preload("ReplyTo").
		Where("group_id = ?", groupID).
		Order("created_at DESC").
		Limit(100).
		Find(&messages)

	return c.JSON(fiber.Map{
		"messages": messages,
	})
}

// Helper to notify a single user about group events
func (h *GroupsHandler) notifyGroupEvent(userID, eventType, groupID, groupName string) {
	if h.hub == nil {
		return
	}

	msg := map[string]interface{}{
		"type":       eventType,
		"group_id":   groupID,
		"group_name": groupName,
	}

	msgBytes, _ := json.Marshal(msg)
	h.hub.SendToUser(userID, msgBytes)
}

// Helper to broadcast to all group members
func (h *GroupsHandler) broadcastToGroup(groupID, excludeUserID, eventType string, data map[string]interface{}) {
	if h.hub == nil {
		return
	}

	msg := map[string]interface{}{
		"type":     eventType,
		"group_id": groupID,
		"data":     data,
	}

	msgBytes, _ := json.Marshal(msg)
	h.hub.SendToGroup(groupID, excludeUserID, msgBytes)
}
