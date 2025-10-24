package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"borderless_coding_server/internal/models"
	"borderless_coding_server/internal/services"
	"borderless_coding_server/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type BuildHandler struct {
	buildService *services.BuildService
	logger       *logrus.Logger
}

func NewBuildHandler(buildService *services.BuildService, logger *logrus.Logger) *BuildHandler {
	return &BuildHandler{
		buildService: buildService,
		logger:       logger,
	}
}

// StartBuildRequest represents the request to start a build
type StartBuildRequest struct {
	Command string `json:"command" binding:"required"`
}

// StartBuildWithClaudeRequest represents the request to start a build with Claude CLI
type StartBuildWithClaudeRequest struct {
	UserInput string                     `json:"user_input" binding:"required"`
	Options   *services.ClaudeCLIOptions `json:"options,omitempty"`
}

// StartBuild starts a new build
func (h *BuildHandler) StartBuild(c *gin.Context) {
	// Get user ID from context
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

	// Get project ID from URL
	projectIDStr := c.Param("project_id")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	// Get session ID (optional)
	var sessionID *uuid.UUID
	if sessionIDStr := c.Query("session_id"); sessionIDStr != "" {
		if parsedID, err := uuid.Parse(sessionIDStr); err == nil {
			sessionID = &parsedID
		}
	}

	var req StartBuildRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Start build
	build, err := h.buildService.StartBuild(c.Request.Context(), userID, projectID, sessionID, req.Command)
	if err != nil {
		h.logger.WithError(err).Error("Failed to start build")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start build"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Build started successfully",
		"build":   build,
	})
}

// StartBuildWithClaude starts a build using Claude CLI to process user input
func (h *BuildHandler) StartBuildWithClaude(c *gin.Context) {
	// Get user ID from context
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

	// Get project ID from URL
	projectIDStr := c.Param("project_id")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	// Get session ID (optional)
	var sessionID *uuid.UUID
	if sessionIDStr := c.Query("session_id"); sessionIDStr != "" {
		if parsedID, err := uuid.Parse(sessionIDStr); err == nil {
			sessionID = &parsedID
		}
	}

	var req StartBuildWithClaudeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set default options if not provided
	if req.Options == nil {
		req.Options = &services.ClaudeCLIOptions{
			Model: "sonnet",
		}
	}

	// Set working directory in options
	if req.Options.CWD == "" {
		req.Options.CWD = h.buildService.GetWorkingDirectory(userID, projectID, sessionID)
	}

	// Start build with Claude CLI
	build, err := h.buildService.StartBuildWithClaudeCLI(c.Request.Context(), userID, projectID, sessionID, req.UserInput, req.Options)
	if err != nil {
		h.logger.WithError(err).Error("Failed to start build with Claude CLI")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start build with Claude CLI"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Build started successfully with Claude CLI",
		"build":   build,
	})
}

// GetBuild retrieves a build by ID
func (h *BuildHandler) GetBuild(c *gin.Context) {
	buildIDStr := c.Param("id")
	buildID, err := uuid.Parse(buildIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid build ID"})
		return
	}

	build, err := h.buildService.GetBuild(buildID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"build": build})
}

// CancelBuild cancels a running build
func (h *BuildHandler) CancelBuild(c *gin.Context) {
	buildIDStr := c.Param("id")
	buildID, err := uuid.Parse(buildIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid build ID"})
		return
	}

	err = h.buildService.CancelBuild(buildID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to cancel build")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Build cancelled successfully"})
}

// GetUserBuilds retrieves builds for a user
func (h *BuildHandler) GetUserBuilds(c *gin.Context) {
	// Get user ID from context
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

	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 20
	}

	builds, err := h.buildService.GetUserBuilds(userID, limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get user builds")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve builds"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"builds": builds})
}

// GetProjectBuilds retrieves builds for a project
func (h *BuildHandler) GetProjectBuilds(c *gin.Context) {
	projectIDStr := c.Param("project_id")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 20
	}

	builds, err := h.buildService.GetProjectBuilds(projectID, limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get project builds")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve builds"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"builds": builds})
}

// GetBuildLogs retrieves logs for a build
func (h *BuildHandler) GetBuildLogs(c *gin.Context) {
	buildIDStr := c.Param("id")
	buildID, err := uuid.Parse(buildIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid build ID"})
		return
	}

	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 1000 {
		limit = 100
	}

	logs, err := h.buildService.GetBuildLogs(buildID, limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get build logs")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve build logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

// StreamBuildOutput streams build output using Server-Sent Events
func (h *BuildHandler) StreamBuildOutput(c *gin.Context) {
	buildIDStr := c.Param("id")
	buildID, err := uuid.Parse(buildIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid build ID"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")

	// Create a channel for build updates
	updateChan := make(chan models.BuildLog, 100)
	defer close(updateChan)

	// Start goroutine to monitor build logs
	go h.monitorBuildLogs(c.Request.Context(), buildID, updateChan)

	// Stream events to client
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Streaming not supported"})
		return
	}

	// Send initial build status
	build, err := h.buildService.GetBuild(buildID)
	if err == nil {
		h.sendSSEEvent(c, "build_status", build)
		flusher.Flush()
	}

	// Stream build logs
	for {
		select {
		case log, ok := <-updateChan:
			if !ok {
				// Channel closed, send final event and exit
				h.sendSSEEvent(c, "build_complete", nil)
				flusher.Flush()
				return
			}

			// Check if this is a Claude CLI response
			if log.Level == "info" && strings.Contains(log.Message, "Claude response:") {
				// Extract JSON from the log message
				jsonStart := strings.Index(log.Message, "{")
				if jsonStart != -1 {
					jsonStr := log.Message[jsonStart:]
					var claudeResponse map[string]interface{}
					if err := json.Unmarshal([]byte(jsonStr), &claudeResponse); err == nil {
						// Send as Claude response event
						h.sendSSEEvent(c, "claude_response", claudeResponse)

						// Check for session ID
						if sessionID, ok := claudeResponse["session_id"].(string); ok {
							h.sendSSEEvent(c, "session_created", map[string]interface{}{
								"session_id": sessionID,
							})
						}
					} else {
						// Send as regular log
						h.sendSSEEvent(c, "build_log", log)
					}
				} else {
					// Send as regular log
					h.sendSSEEvent(c, "build_log", log)
				}
			} else {
				// Send as regular log
				h.sendSSEEvent(c, "build_log", log)
			}
			flusher.Flush()

		case <-c.Request.Context().Done():
			// Client disconnected
			return

		case <-time.After(30 * time.Second):
			// Send keepalive
			h.sendSSEEvent(c, "keepalive", nil)
			flusher.Flush()
		}
	}
}

// monitorBuildLogs monitors build logs and sends updates to the channel
func (h *BuildHandler) monitorBuildLogs(ctx context.Context, buildID uuid.UUID, updateChan chan<- models.BuildLog) {
	defer close(updateChan)

	// Get initial logs
	logs, err := h.buildService.GetBuildLogs(buildID, 100)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get initial build logs")
		return
	}

	// Send initial logs
	for _, log := range logs {
		select {
		case updateChan <- log:
		case <-ctx.Done():
			return
		}
	}

	// Monitor for new logs
	lastLogID := uuid.Nil
	if len(logs) > 0 {
		lastLogID = logs[len(logs)-1].ID
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check for new logs
			var newLogs []models.BuildLog
			query := database.DB.Where("build_id = ?", buildID).Order("timestamp ASC")
			if lastLogID != uuid.Nil {
				query = query.Where("id > ?", lastLogID)
			}

			if err := query.Find(&newLogs).Error; err != nil {
				h.logger.WithError(err).Error("Failed to get new build logs")
				continue
			}

			// Send new logs
			for _, log := range newLogs {
				select {
				case updateChan <- log:
					lastLogID = log.ID
				case <-ctx.Done():
					return
				}
			}

			// Check if build is completed
			var build models.Build
			if err := database.DB.Where("id = ?", buildID).First(&build).Error; err == nil {
				if build.IsCompleted() {
					// Build completed, close channel after a short delay
					time.Sleep(2 * time.Second)
					return
				}
			}
		}
	}
}

// sendSSEEvent sends a Server-Sent Event
func (h *BuildHandler) sendSSEEvent(c *gin.Context, eventType string, data interface{}) {
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			h.logger.WithError(err).Error("Failed to marshal SSE data")
			return
		}
		c.Writer.WriteString(fmt.Sprintf("event: %s\n", eventType))
		c.Writer.WriteString(fmt.Sprintf("data: %s\n\n", string(jsonData)))
	} else {
		c.Writer.WriteString(fmt.Sprintf("event: %s\n", eventType))
		c.Writer.WriteString("data: {}\n\n")
	}
}
