package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Poll represents a poll that can be sent in a chat
type Poll struct {
	ID          string       `gorm:"primaryKey" json:"id"`
	CreatorID   string       `gorm:"not null;index" json:"creator_id"`
	MessageID   *string      `gorm:"index" json:"message_id,omitempty"` // Associated message
	GroupID     *string      `gorm:"index" json:"group_id,omitempty"`   // If in a group
	RecipientID *string      `gorm:"index" json:"recipient_id,omitempty"` // If in a DM
	Question    string       `gorm:"not null" json:"question"`
	MultiSelect bool         `gorm:"default:false" json:"multi_select"` // Allow multiple selections
	Anonymous   bool         `gorm:"default:false" json:"anonymous"`    // Hide who voted
	Closed      bool         `gorm:"default:false" json:"closed"`       // No more voting
	ExpiresAt   *time.Time   `json:"expires_at,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`

	Options []PollOption `gorm:"foreignKey:PollID" json:"options"`
	Creator User         `gorm:"foreignKey:CreatorID" json:"-"`
}

func (p *Poll) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// PollOption represents an option in a poll
type PollOption struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	PollID    string    `gorm:"not null;index" json:"poll_id"`
	Text      string    `gorm:"not null" json:"text"`
	Position  int       `gorm:"not null" json:"position"` // Order of display
	CreatedAt time.Time `json:"created_at"`

	Votes []PollVote `gorm:"foreignKey:OptionID" json:"votes,omitempty"`
}

func (o *PollOption) BeforeCreate(tx *gorm.DB) error {
	if o.ID == "" {
		o.ID = uuid.New().String()
	}
	return nil
}

// PollVote represents a user's vote on a poll option
type PollVote struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	PollID    string    `gorm:"not null;index;uniqueIndex:idx_poll_user_option" json:"poll_id"`
	OptionID  string    `gorm:"not null;index;uniqueIndex:idx_poll_user_option" json:"option_id"`
	UserID    string    `gorm:"not null;index;uniqueIndex:idx_poll_user_option" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`

	Option PollOption `gorm:"foreignKey:OptionID" json:"-"`
	User   User       `gorm:"foreignKey:UserID" json:"-"`
}

func (v *PollVote) BeforeCreate(tx *gorm.DB) error {
	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	return nil
}

// PollResponse is the API response format for a poll
type PollResponse struct {
	ID          string               `json:"id"`
	CreatorID   string               `json:"creator_id"`
	Question    string               `json:"question"`
	MultiSelect bool                 `json:"multi_select"`
	Anonymous   bool                 `json:"anonymous"`
	Closed      bool                 `json:"closed"`
	TotalVotes  int                  `json:"total_votes"`
	Options     []PollOptionResponse `json:"options"`
	MyVotes     []string             `json:"my_votes,omitempty"` // Option IDs the current user voted for
	ExpiresAt   *string              `json:"expires_at,omitempty"`
	CreatedAt   string               `json:"created_at"`
}

type PollOptionResponse struct {
	ID         string   `json:"id"`
	Text       string   `json:"text"`
	VoteCount  int      `json:"vote_count"`
	Percentage float64  `json:"percentage"`
	Voters     []string `json:"voters,omitempty"` // User IDs (only if not anonymous)
}

// ToPollResponse converts a Poll to its API response format
func (p *Poll) ToPollResponse(db *gorm.DB, currentUserID string) PollResponse {
	response := PollResponse{
		ID:          p.ID,
		CreatorID:   p.CreatorID,
		Question:    p.Question,
		MultiSelect: p.MultiSelect,
		Anonymous:   p.Anonymous,
		Closed:      p.Closed,
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
	}

	if p.ExpiresAt != nil {
		s := p.ExpiresAt.Format(time.RFC3339)
		response.ExpiresAt = &s
	}

	// Count total votes
	var totalVotes int64
	db.Model(&PollVote{}).Where("poll_id = ?", p.ID).Count(&totalVotes)
	response.TotalVotes = int(totalVotes)

	// Get user's votes
	var userVotes []PollVote
	db.Where("poll_id = ? AND user_id = ?", p.ID, currentUserID).Find(&userVotes)
	for _, v := range userVotes {
		response.MyVotes = append(response.MyVotes, v.OptionID)
	}

	// Build options with vote counts
	for _, opt := range p.Options {
		var voteCount int64
		db.Model(&PollVote{}).Where("option_id = ?", opt.ID).Count(&voteCount)

		optResponse := PollOptionResponse{
			ID:        opt.ID,
			Text:      opt.Text,
			VoteCount: int(voteCount),
		}

		if totalVotes > 0 {
			optResponse.Percentage = float64(voteCount) / float64(totalVotes) * 100
		}

		// Include voters if not anonymous
		if !p.Anonymous {
			var votes []PollVote
			db.Where("option_id = ?", opt.ID).Find(&votes)
			for _, v := range votes {
				optResponse.Voters = append(optResponse.Voters, v.UserID)
			}
		}

		response.Options = append(response.Options, optResponse)
	}

	return response
}

// CreatePoll creates a new poll with options
func CreatePoll(db *gorm.DB, creatorID string, question string, options []string, multiSelect, anonymous bool, groupID, recipientID *string) (*Poll, error) {
	poll := &Poll{
		CreatorID:   creatorID,
		Question:    question,
		MultiSelect: multiSelect,
		Anonymous:   anonymous,
		GroupID:     groupID,
		RecipientID: recipientID,
	}

	if err := db.Create(poll).Error; err != nil {
		return nil, err
	}

	// Create options
	for i, text := range options {
		option := &PollOption{
			PollID:   poll.ID,
			Text:     text,
			Position: i,
		}
		if err := db.Create(option).Error; err != nil {
			return nil, err
		}
		poll.Options = append(poll.Options, *option)
	}

	return poll, nil
}

// Vote adds or removes a vote on a poll option
func (p *Poll) Vote(db *gorm.DB, userID, optionID string) error {
	if p.Closed {
		return gorm.ErrInvalidData
	}

	// Check if option belongs to this poll
	var option PollOption
	if err := db.Where("id = ? AND poll_id = ?", optionID, p.ID).First(&option).Error; err != nil {
		return err
	}

	// Check for existing vote on this option
	var existingVote PollVote
	err := db.Where("poll_id = ? AND option_id = ? AND user_id = ?", p.ID, optionID, userID).First(&existingVote).Error

	if err == nil {
		// Vote exists, remove it (toggle off)
		return db.Delete(&existingVote).Error
	}

	// If not multi-select, remove any existing votes first
	if !p.MultiSelect {
		db.Where("poll_id = ? AND user_id = ?", p.ID, userID).Delete(&PollVote{})
	}

	// Add new vote
	vote := &PollVote{
		PollID:   p.ID,
		OptionID: optionID,
		UserID:   userID,
	}
	return db.Create(vote).Error
}

// Close closes the poll for voting
func (p *Poll) Close(db *gorm.DB) error {
	return db.Model(p).Update("closed", true).Error
}
