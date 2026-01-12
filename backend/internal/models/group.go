package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type GroupRole string

const (
	GroupRoleOwner  GroupRole = "owner"
	GroupRoleAdmin  GroupRole = "admin"
	GroupRoleMember GroupRole = "member"
)

type Group struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description,omitempty"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	CreatedBy   string    `gorm:"not null" json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Creator User          `gorm:"foreignKey:CreatedBy" json:"-"`
	Members []GroupMember `gorm:"foreignKey:GroupID" json:"members,omitempty"`
}

func (g *Group) BeforeCreate(tx *gorm.DB) error {
	if g.ID == "" {
		g.ID = uuid.New().String()
	}
	return nil
}

type GroupMember struct {
	ID       string    `gorm:"primaryKey" json:"id"`
	GroupID  string    `gorm:"not null;index;uniqueIndex:idx_group_user" json:"group_id"`
	UserID   string    `gorm:"not null;index;uniqueIndex:idx_group_user" json:"user_id"`
	Role     GroupRole `gorm:"default:member" json:"role"`
	JoinedAt time.Time `json:"joined_at"`

	Group Group `gorm:"foreignKey:GroupID" json:"-"`
	User  User  `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (gm *GroupMember) BeforeCreate(tx *gorm.DB) error {
	if gm.ID == "" {
		gm.ID = uuid.New().String()
	}
	if gm.JoinedAt.IsZero() {
		gm.JoinedAt = time.Now()
	}
	return nil
}

type GroupWithMemberCount struct {
	Group
	MemberCount int `json:"member_count"`
}

type GroupResponse struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	AvatarURL   string        `json:"avatar_url,omitempty"`
	CreatedBy   string        `json:"created_by"`
	MemberCount int           `json:"member_count"`
	Members     []GroupMember `json:"members,omitempty"`
	MyRole      GroupRole     `json:"my_role,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
}
