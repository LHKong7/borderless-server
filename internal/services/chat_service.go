package services

import (
	"errors"
	"time"

	"borderless_coding_server/internal/models"
	"borderless_coding_server/pkg/database"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ChatService struct{}

func NewChatService() *ChatService {
	return &ChatService{}
}

// ===================== Chat Session Methods =====================

// CreateChatSession creates a new chat session
func (s *ChatService) CreateChatSession(session *models.ChatSession) error {
	if session.UserID == uuid.Nil {
		return errors.New("user ID is required")
	}

	// Check if user exists
	var user models.User
	if err := database.DB.Where("id = ? AND deleted_at IS NULL", session.UserID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return err
	}

	// Check if project exists (if specified)
	if session.ProjectID != nil {
		var project models.Project
		if err := database.DB.Where("id = ? AND deleted_at IS NULL", *session.ProjectID).First(&project).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("project not found")
			}
			return err
		}
	}

	// Set default metadata if not provided
	if session.Meta == nil {
		session.Meta = make(models.JSONB)
	}

	// Set default title if not provided
	if session.Title == nil || *session.Title == "" {
		defaultTitle := "New Chat Session"
		session.Title = &defaultTitle
	}

	return database.DB.Create(session).Error
}

// GetChatSessionByID retrieves a chat session by ID
func (s *ChatService) GetChatSessionByID(id uuid.UUID) (*models.ChatSession, error) {
	var session models.ChatSession
	err := database.DB.Where("id = ?", id).First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("chat session not found")
		}
		return nil, err
	}
	return &session, nil
}

// UpdateChatSession updates an existing chat session
func (s *ChatService) UpdateChatSession(id uuid.UUID, updates map[string]interface{}) error {
	// Check if session exists
	var session models.ChatSession
	if err := database.DB.Where("id = ?", id).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("chat session not found")
		}
		return err
	}

	return database.DB.Model(&session).Updates(updates).Error
}

// DeleteChatSession deletes a chat session (hard delete)
func (s *ChatService) DeleteChatSession(id uuid.UUID) error {
	var session models.ChatSession
	if err := database.DB.Where("id = ?", id).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("chat session not found")
		}
		return err
	}

	return database.DB.Delete(&session).Error
}

// ArchiveChatSession archives a chat session
func (s *ChatService) ArchiveChatSession(id uuid.UUID) error {
	now := time.Now()
	return s.UpdateChatSession(id, map[string]interface{}{
		"archived_at": &now,
	})
}

// UnarchiveChatSession unarchives a chat session
func (s *ChatService) UnarchiveChatSession(id uuid.UUID) error {
	return s.UpdateChatSession(id, map[string]interface{}{
		"archived_at": nil,
	})
}

// ListChatSessions retrieves chat sessions for a user
func (s *ChatService) ListChatSessions(userID uuid.UUID, projectID *uuid.UUID, includeArchived bool, offset, limit int) ([]models.ChatSession, int64, error) {
	var sessions []models.ChatSession
	var total int64

	query := database.DB.Model(&models.ChatSession{}).Where("user_id = ?", userID)

	// Filter by project if specified
	if projectID != nil {
		query = query.Where("project_id = ?", *projectID)
	}

	// Filter archived sessions
	if !includeArchived {
		query = query.Where("archived_at IS NULL")
	}

	// Count total sessions
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get sessions with pagination
	err := query.Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&sessions).Error

	return sessions, total, err
}

// GetProjectChatSessions retrieves chat sessions for a project
func (s *ChatService) GetProjectChatSessions(projectID uuid.UUID, includeArchived bool) ([]models.ChatSession, error) {
	var sessions []models.ChatSession

	query := database.DB.Where("project_id = ?", projectID)

	// Filter archived sessions
	if !includeArchived {
		query = query.Where("archived_at IS NULL")
	}

	err := query.Order("created_at DESC").Find(&sessions).Error
	return sessions, err
}

// ===================== Chat Message Methods =====================

// CreateChatMessage creates a new chat message
func (s *ChatService) CreateChatMessage(message *models.ChatMessage) error {
	if message.SessionID == uuid.Nil {
		return errors.New("session ID is required")
	}
	if message.Sender == "" {
		return errors.New("sender is required")
	}
	if message.Content == nil {
		message.Content = make(models.JSONB)
	}

	// Check if session exists
	var session models.ChatSession
	if err := database.DB.Where("id = ?", message.SessionID).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("chat session not found")
		}
		return err
	}

	// Check if reply_to message exists (if specified)
	if message.ReplyTo != nil {
		var replyMessage models.ChatMessage
		if err := database.DB.Where("id = ?", *message.ReplyTo).First(&replyMessage).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("reply message not found")
			}
			return err
		}
	}

	return database.DB.Create(message).Error
}

// GetChatMessageByID retrieves a chat message by ID
func (s *ChatService) GetChatMessageByID(id uuid.UUID) (*models.ChatMessage, error) {
	var message models.ChatMessage
	err := database.DB.Where("id = ?", id).First(&message).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("chat message not found")
		}
		return nil, err
	}
	return &message, nil
}

// GetChatMessages retrieves messages for a chat session
func (s *ChatService) GetChatMessages(sessionID uuid.UUID, offset, limit int) ([]models.ChatMessage, int64, error) {
	var messages []models.ChatMessage
	var total int64

	// Count total messages
	if err := database.DB.Model(&models.ChatMessage{}).Where("session_id = ?", sessionID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get messages with pagination
	err := database.DB.Where("session_id = ?", sessionID).
		Offset(offset).
		Limit(limit).
		Order("created_at ASC").
		Find(&messages).Error

	return messages, total, err
}

// GetChatMessagesWithReplies retrieves messages with their replies
func (s *ChatService) GetChatMessagesWithReplies(sessionID uuid.UUID, offset, limit int) ([]models.ChatMessage, int64, error) {
	var messages []models.ChatMessage
	var total int64

	// Count total messages
	if err := database.DB.Model(&models.ChatMessage{}).Where("session_id = ?", sessionID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get messages with replies
	err := database.DB.Preload("Reply").
		Where("session_id = ?", sessionID).
		Offset(offset).
		Limit(limit).
		Order("created_at ASC").
		Find(&messages).Error

	return messages, total, err
}

// UpdateChatMessage updates a chat message
func (s *ChatService) UpdateChatMessage(id uuid.UUID, updates map[string]interface{}) error {
	// Check if message exists
	var message models.ChatMessage
	if err := database.DB.Where("id = ?", id).First(&message).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("chat message not found")
		}
		return err
	}

	return database.DB.Model(&message).Updates(updates).Error
}

// DeleteChatMessage deletes a chat message
func (s *ChatService) DeleteChatMessage(id uuid.UUID) error {
	var message models.ChatMessage
	if err := database.DB.Where("id = ?", id).First(&message).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("chat message not found")
		}
		return err
	}

	return database.DB.Delete(&message).Error
}

// GetChatSessionWithMessages retrieves a chat session with its messages
func (s *ChatService) GetChatSessionWithMessages(sessionID uuid.UUID, messageLimit int) (*models.ChatSession, []models.ChatMessage, error) {
	var session models.ChatSession
	var messages []models.ChatMessage

	// Get session
	if err := database.DB.Where("id = ?", sessionID).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, errors.New("chat session not found")
		}
		return nil, nil, err
	}

	// Get messages
	err := database.DB.Where("session_id = ?", sessionID).
		Order("created_at ASC").
		Limit(messageLimit).
		Find(&messages).Error

	return &session, messages, err
}

// GetRecentChatSessions retrieves recent chat sessions for a user
func (s *ChatService) GetRecentChatSessions(userID uuid.UUID, limit int) ([]models.ChatSession, error) {
	var sessions []models.ChatSession
	err := database.DB.Where("user_id = ? AND archived_at IS NULL", userID).
		Order("updated_at DESC").
		Limit(limit).
		Find(&sessions).Error
	return sessions, err
}

// UpdateSessionTitle updates the title of a chat session
func (s *ChatService) UpdateSessionTitle(sessionID uuid.UUID, title string) error {
	return s.UpdateChatSession(sessionID, map[string]interface{}{
		"title": title,
	})
}

// UpdateSessionModelHint updates the model hint of a chat session
func (s *ChatService) UpdateSessionModelHint(sessionID uuid.UUID, modelHint string) error {
	return s.UpdateChatSession(sessionID, map[string]interface{}{
		"model_hint": modelHint,
	})
}
