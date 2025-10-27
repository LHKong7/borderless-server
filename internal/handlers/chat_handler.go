package handlers

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"borderless_coding_server/config"
	"borderless_coding_server/internal/models"
	"borderless_coding_server/internal/services"
	"borderless_coding_server/pkg/database"
	"borderless_coding_server/pkg/storage"
	"borderless_coding_server/pkg/utils"

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
	UserID    uuid.UUID    `json:"user_id" binding:"required"`
	ProjectID *uuid.UUID   `json:"project_id" binding:"required"`
	Title     *string      `json:"title"`
	ModelHint *string      `json:"model_hint"`
	Meta      models.JSONB `json:"meta"`
}

// UpdateChatSessionRequest represents the request payload for updating a chat session
type UpdateChatSessionRequest struct {
	Title     *string      `json:"title"`
	ModelHint *string      `json:"model_hint"`
	Meta      models.JSONB `json:"meta"`
}

// CreateChatMessageRequest represents the request payload for creating a chat message
type CreateChatMessageRequest struct {
	SessionID  uuid.UUID         `json:"session_id" binding:"required"`
	Sender     models.ChatSender `json:"sender" binding:"required"`
	Text       *string           `json:"text"`
	Content    models.JSONB      `json:"content"`
	TokensUsed *int              `json:"tokens_used"`
	ToolName   *string           `json:"tool_name"`
	ReplyTo    *uuid.UUID        `json:"reply_to"`
}

// UpdateChatMessageRequest represents the request payload for updating a chat message
type UpdateChatMessageRequest struct {
	Text       *string      `json:"text"`
	Content    models.JSONB `json:"content"`
	TokensUsed *int         `json:"tokens_used"`
	ToolName   *string      `json:"tool_name"`
}

// CreateChatSession creates a new chat session
func (h *ChatHandler) CreateChatSession(c *gin.Context) {
	var req CreateChatSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ProjectID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id is required"})
		return
	}

	session := &models.ChatSession{
		UserID:    req.UserID,
		ProjectID: *req.ProjectID,
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

// StreamClaudeChatRequest represents the chat streaming request
type StreamClaudeChatRequest struct {
	Message string `json:"message" binding:"required"`
}

// StreamClaudeChat streams Claude CLI output for a project's single chat session (SSE)
func (h *ChatHandler) StreamClaudeChat(c *gin.Context) {
	// Auth user
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	userID, ok := userIDInterface.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Project ID
	projectIDStr := c.Param("id")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	// Ownership check
	var project models.Project
	if err := database.DB.Where("id = ? AND owner_id = ? AND deleted_at IS NULL", projectID, userID).First(&project).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Project not found or not owned by user"})
		return
	}

	// Get or create one-to-one chat session for project
	var session models.ChatSession
	if err := database.DB.Where("project_id = ?", projectID).First(&session).Error; err != nil {
		// create
		session = models.ChatSession{UserID: userID, ProjectID: projectID}
		if err := database.DB.Create(&session).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create chat session"})
			return
		}
	}

	// Parse request
	var req StreamClaudeChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Persist user message
	userMsg := models.ChatMessage{SessionID: session.ID, Sender: models.ChatSenderUser, Text: &req.Message, Content: models.JSONB{"text": req.Message}}
	if err := database.DB.Create(&userMsg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save message"})
		return
	}

	// Determine working directory from StorageLocation (local_fs) or fallback
	var storageLoc models.StorageLocation
	workDir := ""
	if err := database.DB.Where("project_id = ?", projectID).First(&storageLoc).Error; err == nil && storageLoc.LocalPath != nil {
		workDir = *storageLoc.LocalPath
		// Ensure directory exists for local filesystem storage
		if storageLoc.Type == models.StorageTypeLocalFS {
			_ = os.MkdirAll(workDir, 0755)
		}
	}
	if workDir == "" {
		// fallback temp path
		workDir = "/tmp/borderless-coding/chat/" + userID.String() + "/" + projectID.String()
		_ = os.MkdirAll(workDir, 0755)
	}

	// Sync from MinIO if network path exists; otherwise initialize from template
	if storageLoc.NetworkPath != nil && *storageLoc.NetworkPath != "" {
		// Download zip to temp, unzip into workDir
		tmpZip := workDir + "/.remote.zip"
		// NetworkPath format: bucket/objectKey
		bucket, object := parseMinIOPath(*storageLoc.NetworkPath)
		if bucket == "" || object == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid network path"})
			return
		}
		// Download to tmp and unzip
		if err := storage.DownloadFile(c.Request.Context(), bucket, object, tmpZip); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to download remote archive"})
			return
		}
		if err := utils.UnZipFile(tmpZip, workDir); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid zip archive"})
			os.Remove(tmpZip)
			return
		}
		_ = os.Remove(tmpZip)
	} else {
		// New user: copy project_template.zip into workDir and unzip
		templateZip := config.LoadConfig().ProjectTemplateZip
		if templateZip == "" {
			templateZip = "project_template.zip"
		}
		if _, err := os.Stat(templateZip); err == nil {
			if err := utils.UnZipFile(templateZip, workDir); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initialize template"})
				return
			}
		}
	}

	h.logger.Info("workDir is ...", workDir)

	// Prepare SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Streaming not supported"})
		return
	}

	// Spawn Claude CLI
	args := []string{"--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions", "-p", req.Message}
	claudePath := config.LoadConfig().ClaudeCLIPath
	if claudePath == "" {
		claudePath = "claude"
	}
	cmd := exec.CommandContext(c.Request.Context(), claudePath, args...)
	cmd.Dir = workDir
	h.logger.WithField("cmd", append([]string{claudePath}, args...)).Info("starting claude cli")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to attach stdout"})
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to attach stderr"})
		return
	}
	if err := cmd.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start Claude CLI"})
		return
	}

	type line struct {
		isErr bool
		text  string
	}
	outCh := make(chan line, 128)
	done := make(chan struct{})

	// Readers
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			outCh <- line{isErr: false, text: scanner.Text()}
		}
		done <- struct{}{}
	}()
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			outCh <- line{isErr: true, text: scanner.Text()}
		}
		done <- struct{}{}
	}()

	// Wait for readers and process output synchronously to keep connection open
	assistantAccum := ""
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	readersClosed := 0
	for {
		select {
		case l := <-outCh:
			if l.text == "" {
				continue
			}
			if l.isErr {
				c.Writer.WriteString("event: claude_error\n")
				b, _ := json.Marshal(gin.H{"error": l.text})
				c.Writer.WriteString("data: " + string(b) + "\n\n")
				flusher.Flush()
				continue
			}
			var payload map[string]interface{}
			if err := json.Unmarshal([]byte(l.text), &payload); err == nil {
				if t, ok := payload["text"].(string); ok {
					assistantAccum += t
				}
				c.Writer.WriteString("event: claude_response\n")
				b, _ := json.Marshal(payload)
				c.Writer.WriteString("data: " + string(b) + "\n\n")
			} else {
				assistantAccum += l.text + "\n"
				c.Writer.WriteString("event: claude_output\n")
				b, _ := json.Marshal(gin.H{"data": l.text})
				c.Writer.WriteString("data: " + string(b) + "\n\n")
			}
			flusher.Flush()
		case <-done:
			readersClosed++
			if readersClosed >= 2 {
				// All readers done; wait for process
				_ = cmd.Wait()
				// Persist assistant message
				if assistantAccum != "" {
					content := models.JSONB{"text": assistantAccum}
					msg := models.ChatMessage{SessionID: session.ID, Sender: models.ChatSenderAssistant, Text: &assistantAccum, Content: content}
					_ = database.DB.Create(&msg).Error
				}
				// Git commit changes with chat message (best effort)
				_ = runGitCommit(workDir, req.Message)
				// Zip and upload back to MinIO (compose path if empty)
				composedPath := getOrComposeNetworkPath(&storageLoc, userID, projectID, session.ID)
				bucket, object := parseMinIOPath(composedPath)
				tmpZip := filepath.Join(os.TempDir(), "upload-"+projectID.String()+".zip")
				if err := utils.ZipFolder(workDir, tmpZip); err != nil {
					h.logger.WithError(err).Error("zip failed")
					c.Writer.WriteString("event: upload_error\n")
					b, _ := json.Marshal(gin.H{"error": "zip failed"})
					c.Writer.WriteString("data: " + string(b) + "\n\n")
					flusher.Flush()
				} else {
					if err := storage.UploadFile(c.Request.Context(), bucket, object, tmpZip); err != nil {
						h.logger.WithError(err).Error("upload failed")
						c.Writer.WriteString("event: upload_error\n")
						b, _ := json.Marshal(gin.H{"error": "upload failed"})
						c.Writer.WriteString("data: " + string(b) + "\n\n")
						flusher.Flush()
					} else {
						// Persist composed network path if it was empty before
						if storageLoc.NetworkPath == nil || *storageLoc.NetworkPath == "" {
							np := composedPath
							_ = database.DB.Model(&models.StorageLocation{}).Where("id = ?", storageLoc.ID).Update("network_path", np).Error
						}
					}
					_ = os.Remove(tmpZip)
				}
				// Cleanup local folder
				_ = os.RemoveAll(workDir)
				c.Writer.WriteString("event: complete\n")
				c.Writer.WriteString("data: {}\n\n")
				flusher.Flush()
				return
			}
		case <-ticker.C:
			// keepalive
			c.Writer.WriteString("event: keepalive\n")
			c.Writer.WriteString("data: {}\n\n")
			flusher.Flush()
		case <-c.Request.Context().Done():
			_ = cmd.Process.Kill()
			return
		}
	}
}

// parseMinIOPath expects "bucket/objectKey"
func parseMinIOPath(p string) (string, string) {
	for i := 0; i < len(p); i++ {
		if p[i] == '/' {
			return p[:i], p[i+1:]
		}
	}
	return "", ""
}

func getOrComposeNetworkPath(loc *models.StorageLocation, userID, projectID, sessionID uuid.UUID) string {
	if loc != nil && loc.NetworkPath != nil && *loc.NetworkPath != "" {
		return *loc.NetworkPath
	}
	// Default bucket from config, path composed
	cfg := config.LoadConfig()
	bucket := cfg.MinIOBucketName
	object := "users/" + userID.String() + "/projects/" + projectID.String() + "/sessions/" + sessionID.String() + ".zip"
	np := bucket + "/" + object
	return np
}

func runGitCommit(dir, message string) error {
	// Initialize repo if needed
	if _, err := os.Stat(dir + "/.git"); os.IsNotExist(err) {
		cmd := exec.Command("git", "init")
		cmd.Dir = dir
		_ = cmd.Run()
	}
	// Add all
	cmdAdd := exec.Command("git", "add", ".")
	cmdAdd.Dir = dir
	if err := cmdAdd.Run(); err != nil {
		return err
	}
	// Commit
	if message == "" {
		message = "update via chat"
	}
	cmdCommit := exec.Command("git", "commit", "-m", message)
	cmdCommit.Dir = dir
	_ = cmdCommit.Run() // best effort
	return nil
}
