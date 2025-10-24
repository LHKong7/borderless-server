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

type UserHandler struct {
	userService *services.UserService
	logger      *logrus.Logger
}

func NewUserHandler(userService *services.UserService, logger *logrus.Logger) *UserHandler {
	return &UserHandler{
		userService: userService,
		logger:      logger,
	}
}

// CreateUserRequest represents the request payload for creating a user
type CreateUserRequest struct {
	Email       string       `json:"email" binding:"required,email"`
	DisplayName *string      `json:"display_name"`
	Password    *string      `json:"password"`
	Metadata    models.JSONB `json:"metadata"`
}

// UpdateUserRequest represents the request payload for updating a user
type UpdateUserRequest struct {
	Email       *string      `json:"email"`
	DisplayName *string      `json:"display_name"`
	IsActive    *bool        `json:"is_active"`
	Metadata    models.JSONB `json:"metadata"`
}

// CreateUser creates a new user
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := &models.User{
		Email:        &req.Email,
		DisplayName:  req.DisplayName,
		PasswordHash: req.Password, // In production, hash this password
		Metadata:     req.Metadata,
		IsActive:     true,
	}

	if err := h.userService.CreateUser(user); err != nil {
		h.logger.WithError(err).Error("Failed to create user")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully",
		"user":    user,
	})
}

// GetUser retrieves a user by ID
func (h *UserHandler) GetUser(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := h.userService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user})
}

// UpdateUser updates an existing user
func (h *UserHandler) UpdateUser(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.Metadata != nil {
		updates["metadata"] = req.Metadata
	}

	if err := h.userService.UpdateUser(userID, updates); err != nil {
		h.logger.WithError(err).Error("Failed to update user")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get updated user
	user, err := h.userService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve updated user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User updated successfully",
		"user":    user,
	})
}

// DeleteUser deletes a user
func (h *UserHandler) DeleteUser(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := h.userService.DeleteUser(userID); err != nil {
		h.logger.WithError(err).Error("Failed to delete user")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// ListUsers retrieves a paginated list of users
func (h *UserHandler) ListUsers(c *gin.Context) {
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

	users, total, err := h.userService.ListUsers(offset, limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to list users")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"pagination": gin.H{
			"offset": offset,
			"limit":  limit,
			"total":  total,
		},
	})
}

// GetUserProjects retrieves all projects for a user
func (h *UserHandler) GetUserProjects(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	projects, err := h.userService.GetUserProjects(userID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get user projects")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user projects"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

// GetUserChatSessions retrieves all chat sessions for a user
func (h *UserHandler) GetUserChatSessions(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	sessions, err := h.userService.GetUserChatSessions(userID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get user chat sessions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user chat sessions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"chat_sessions": sessions})
}

// SearchUsers searches users by email or display name
func (h *UserHandler) SearchUsers(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query is required"})
		return
	}

	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 50 {
		limit = 10
	}

	users, err := h.userService.SearchUsers(query, limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to search users")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"users": users})
}

// ActivateUser activates a user account
func (h *UserHandler) ActivateUser(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := h.userService.ActivateUser(userID); err != nil {
		h.logger.WithError(err).Error("Failed to activate user")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User activated successfully"})
}

// DeactivateUser deactivates a user account
func (h *UserHandler) DeactivateUser(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := h.userService.DeactivateUser(userID); err != nil {
		h.logger.WithError(err).Error("Failed to deactivate user")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deactivated successfully"})
}
