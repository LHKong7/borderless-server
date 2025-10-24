package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ChatSender represents the sender type of a chat message
type ChatSender string

const (
	ChatSenderUser      ChatSender = "user"
	ChatSenderAssistant ChatSender = "assistant"
	ChatSenderSystem    ChatSender = "system"
	ChatSenderTool      ChatSender = "tool"
)

// ChatMessage represents a message in a chat session
type ChatMessage struct {
	ID         uuid.UUID   `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	SessionID  uuid.UUID   `json:"session_id" gorm:"type:uuid;not null"`
	Sender     ChatSender  `json:"sender" gorm:"type:chat_sender;not null"`
	Text       *string     `json:"text"`
	Content    JSONB       `json:"content" gorm:"type:jsonb;not null"`
	TokensUsed *int        `json:"tokens_used"`
	ToolName   *string     `json:"tool_name"`
	ReplyTo    *uuid.UUID  `json:"reply_to" gorm:"type:uuid"`
	CreatedAt  time.Time   `json:"created_at"`

	// Relationships
	Session ChatSession `json:"session,omitempty" gorm:"foreignKey:SessionID"`
	Reply   *ChatMessage `json:"reply,omitempty" gorm:"foreignKey:ReplyTo"`
}

// TableName returns the table name for the ChatMessage model
func (ChatMessage) TableName() string {
	return "chat_messages"
}

// BeforeCreate hook to set timestamps
func (cm *ChatMessage) BeforeCreate(tx *gorm.DB) error {
	if cm.ID == uuid.Nil {
		cm.ID = uuid.New()
	}
	now := time.Now()
	cm.CreatedAt = now
	return nil
}

// IsFromTool checks if the message is from a tool
func (cm *ChatMessage) IsFromTool() bool {
	return cm.Sender == ChatSenderTool
}

// IsFromUser checks if the message is from a user
func (cm *ChatMessage) IsFromUser() bool {
	return cm.Sender == ChatSenderUser
}

// IsFromAssistant checks if the message is from an assistant
func (cm *ChatMessage) IsFromAssistant() bool {
	return cm.Sender == ChatSenderAssistant
}

// IsFromSystem checks if the message is from the system
func (cm *ChatMessage) IsFromSystem() bool {
	return cm.Sender == ChatSenderSystem
}
