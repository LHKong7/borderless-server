package handlers

import (
	"net/http"
	"strconv"

	"borderless_coding_server/internal/models"
	"borderless_coding_server/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type ChatHandler struct {
	chatService *services.ChatService
	logger      *logrus.Logger
}

func NewChatHandler(chatService *services.ChatService, logger *logrus.Logger) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		logger:      logger,
	}
}

// CreateChatSessionRequest represents the request payload for creating a chat session
type CreateChatSessionRequest struct {
	UserID     uuid.UUID      `json:"user_id" binding:"required"`
	ProjectID  *uuid.UUID     `json:"project_id"`
	Title      *string        `json:"title"`
	ModelHint  *string        `json:"model_hint"`
	Meta       models.JSONB   `json:"meta"`
}

// UpdateChatSessionRequest represents the request payload for updating a chat session
type UpdateChatSessionRequest struct {
	Title     *string      `json:"title"`
	ModelHint *string      `json:"model_hint"`
	Meta      models.JSONB  `json:"meta"`
}

// CreateChatMessageRequest represents the request payload for creating a chat message
type CreateChatMessageRequest struct {
	SessionID  uuid.UUID      `json:"session_id" binding:"required"`
	Sender     models.ChatSender `json:"sender" binding:"required"`
	Text       *string        `json:"text"`
	Content    models.JSONB    `json:"content"`
	TokensUsed *int           `json:"tokens_used"`
	ToolName   *string        `json:"tool_name"`
	ReplyTo    *uuid.UUID     `json:"reply_to"`
}

// UpdateChatMessageRequest represents the request payload for updating a chat message
type UpdateChatMessageRequest struct {
	Text       *string     `json:"text"`
	Content    models.JSONB `json:"content"`
	TokensUsed *int        `json:"tokens_used"`
	ToolName   *string     `json:"tool_name"`
}

// CreateChatSession creates a new chat session
func (h *ChatHandler) CreateChatSession(c *gin.Context) {
	var req CreateChatSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session := &models.ChatSession{
		UserID:    req.UserID,
		ProjectID: req.ProjectID,
		Title:     req.Title,
		ModelHint: req.ModelHint,
		Meta:      req.Meta,
	}

	if session.Meta == nil {
		session.Meta = make(models.JSONB)
	}

	if err := h.chatService.CreateChatSession(session); err != nil {
		h.logger.WithError(err).Error("Failed to create chat session")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Chat session created successfully",
		"session": session,
	})
}

// GetChatSession retrieves a chat session by ID
func (h *ChatHandler) GetChatSession(c *gin.Context) {
	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	session, err := h.chatService.GetChatSessionByID(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"session": session})
}

// UpdateChatSession updates an existing chat session
func (h *ChatHandler) UpdateChatSession(c *gin.Context) {
	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	var req UpdateChatSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.ModelHint != nil {
		updates["model_hint"] = *req.ModelHint
	}
	if req.Meta != nil {
		updates["meta"] = req.Meta
	}

	if err := h.chatService.UpdateChatSession(sessionID, updates); err != nil {
		h.logger.WithError(err).Error("Failed to update chat session")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get updated session
	session, err := h.chatService.GetChatSessionByID(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve updated session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Chat session updated successfully",
		"session": session,
	})
}

// DeleteChatSession deletes a chat session
func (h *ChatHandler) DeleteChatSession(c *gin.Context) {
	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	if err := h.chatService.DeleteChatSession(sessionID); err != nil {
		h.logger.WithError(err).Error("Failed to delete chat session")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Chat session deleted successfully"})
}

// ArchiveChatSession archives a chat session
func (h *ChatHandler) ArchiveChatSession(c *gin.Context) {
	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	if err := h.chatService.ArchiveChatSession(sessionID); err != nil {
		h.logger.WithError(err).Error("Failed to archive chat session")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Chat session archived successfully"})
}

// UnarchiveChatSession unarchives a chat session
func (h *ChatHandler) UnarchiveChatSession(c *gin.Context) {
	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	if err := h.chatService.UnarchiveChatSession(sessionID); err != nil {
		h.logger.WithError(err).Error("Failed to unarchive chat session")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Chat session unarchived successfully"})
}

// ListChatSessions retrieves chat sessions for a user
func (h *ChatHandler) ListChatSessions(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	offsetStr := c.DefaultQuery("offset", "0")
	limitStr := c.DefaultQuery("limit", "10")

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 10
	}

	includeArchived := c.Query("include_archived") == "true"

	var projectID *uuid.UUID
	if projectIDStr := c.Query("project_id"); projectIDStr != "" {
		if parsedID, err := uuid.Parse(projectIDStr); err == nil {
			projectID = &parsedID
		}
	}

	sessions, total, err := h.chatService.ListChatSessions(userID, projectID, includeArchived, offset, limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to list chat sessions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve chat sessions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"pagination": gin.H{
			"offset": offset,
			"limit":  limit,
			"total":  total,
		},
	})
}

// GetProjectChatSessions retrieves chat sessions for a project
func (h *ChatHandler) GetProjectChatSessions(c *gin.Context) {
	projectIDStr := c.Param("project_id")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	includeArchived := c.Query("include_archived") == "true"

	sessions, err := h.chatService.GetProjectChatSessions(projectID, includeArchived)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get project chat sessions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve project chat sessions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// CreateChatMessage creates a new chat message
func (h *ChatHandler) CreateChatMessage(c *gin.Context) {
	var req CreateChatMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	message := &models.ChatMessage{
		SessionID:  req.SessionID,
		Sender:     req.Sender,
		Text:       req.Text,
		Content:    req.Content,
		TokensUsed: req.TokensUsed,
		ToolName:   req.ToolName,
		ReplyTo:    req.ReplyTo,
	}

	if message.Content == nil {
		message.Content = make(models.JSONB)
	}

	if err := h.chatService.CreateChatMessage(message); err != nil {
		h.logger.WithError(err).Error("Failed to create chat message")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": message,
	})
}

// GetChatMessage retrieves a chat message by ID
func (h *ChatHandler) GetChatMessage(c *gin.Context) {
	messageIDStr := c.Param("id")
	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}

	message, err := h.chatService.GetChatMessageByID(messageID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": message})
}

// GetChatMessages retrieves messages for a chat session
func (h *ChatHandler) GetChatMessages(c *gin.Context) {
	sessionIDStr := c.Param("session_id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	offsetStr := c.DefaultQuery("offset", "0")
	limitStr := c.DefaultQuery("limit", "50")

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 50
	}

	includeReplies := c.Query("include_replies") == "true"

	var messages []models.ChatMessage
	var total int64

	if includeReplies {
		messages, total, err = h.chatService.GetChatMessagesWithReplies(sessionID, offset, limit)
	} else {
		messages, total, err = h.chatService.GetChatMessages(sessionID, offset, limit)
	}

	if err != nil {
		h.logger.WithError(err).Error("Failed to get chat messages")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve chat messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
		"pagination": gin.H{
			"offset": offset,
			"limit":  limit,
			"total":  total,
		},
	})
}

// UpdateChatMessage updates a chat message
func (h *ChatHandler) UpdateChatMessage(c *gin.Context) {
	messageIDStr := c.Param("id")
	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}

	var req UpdateChatMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.Text != nil {
		updates["text"] = *req.Text
	}
	if req.Content != nil {
		updates["content"] = req.Content
	}
	if req.TokensUsed != nil {
		updates["tokens_used"] = *req.TokensUsed
	}
	if req.ToolName != nil {
		updates["tool_name"] = *req.ToolName
	}

	if err := h.chatService.UpdateChatMessage(messageID, updates); err != nil {
		h.logger.WithError(err).Error("Failed to update chat message")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get updated message
	message, err := h.chatService.GetChatMessageByID(messageID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve updated message"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Chat message updated successfully",
		"data":    message,
	})
}

// DeleteChatMessage deletes a chat message
func (h *ChatHandler) DeleteChatMessage(c *gin.Context) {
	messageIDStr := c.Param("id")
	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}

	if err := h.chatService.DeleteChatMessage(messageID); err != nil {
		h.logger.WithError(err).Error("Failed to delete chat message")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Chat message deleted successfully"})
}

// GetChatSessionWithMessages retrieves a chat session with its messages
func (h *ChatHandler) GetChatSessionWithMessages(c *gin.Context) {
	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	messageLimitStr := c.DefaultQuery("message_limit", "50")
	messageLimit, err := strconv.Atoi(messageLimitStr)
	if err != nil || messageLimit <= 0 || messageLimit > 200 {
		messageLimit = 50
	}

	session, messages, err := h.chatService.GetChatSessionWithMessages(sessionID, messageLimit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get chat session with messages")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve chat session with messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session":  session,
		"messages": messages,
	})
}

// GetRecentChatSessions retrieves recent chat sessions for a user
func (h *ChatHandler) GetRecentChatSessions(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 50 {
		limit = 10
	}

	sessions, err := h.chatService.GetRecentChatSessions(userID, limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get recent chat sessions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve recent chat sessions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}
