package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ChatSession represents a chat session in the system
type ChatSession struct {
	ID         uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID     uuid.UUID  `json:"user_id" gorm:"type:uuid;not null"`
	ProjectID  uuid.UUID  `json:"project_id" gorm:"type:uuid;not null;unique"`
	Title      *string    `json:"title"`
	ModelHint  *string    `json:"model_hint"`
	Meta       JSONB      `json:"meta" gorm:"type:jsonb;default:'{}'"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	ArchivedAt *time.Time `json:"archived_at"`

	// Relationships
	User     User          `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Project  *Project      `json:"project,omitempty" gorm:"foreignKey:ProjectID"`
	Messages []ChatMessage `json:"messages,omitempty" gorm:"foreignKey:SessionID"`
}

// TableName returns the table name for the ChatSession model
func (ChatSession) TableName() string {
	return "chat_sessions"
}

// BeforeCreate hook to set timestamps
func (cs *ChatSession) BeforeCreate(tx *gorm.DB) error {
	if cs.ID == uuid.Nil {
		cs.ID = uuid.New()
	}
	now := time.Now()
	cs.CreatedAt = now
	cs.UpdatedAt = now
	return nil
}

// BeforeUpdate hook to update timestamp
func (cs *ChatSession) BeforeUpdate(tx *gorm.DB) error {
	cs.UpdatedAt = time.Now()
	return nil
}

// IsArchived checks if the session is archived
func (cs *ChatSession) IsArchived() bool {
	return cs.ArchivedAt != nil
}

// Archive marks the session as archived
func (cs *ChatSession) Archive() {
	now := time.Now()
	cs.ArchivedAt = &now
}

// Unarchive removes the archived status
func (cs *ChatSession) Unarchive() {
	cs.ArchivedAt = nil
}
