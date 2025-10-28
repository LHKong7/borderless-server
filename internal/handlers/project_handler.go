package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

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

type ProjectHandler struct {
	projectService *services.ProjectService
	logger         *logrus.Logger
}

func NewProjectHandler(projectService *services.ProjectService, logger *logrus.Logger) *ProjectHandler {
	return &ProjectHandler{
		projectService: projectService,
		logger:         logger,
	}
}

// CreateProjectRequest represents the request payload for creating a project
type CreateProjectRequest struct {
	Name              string                   `json:"name" binding:"required"`
	Description       *string                  `json:"description"`
	Visibility        models.ProjectVisibility `json:"visibility"`
	RootBucket        string                   `json:"root_bucket"`
	RootPrefix        string                   `json:"root_prefix"`
	StorageQuotaBytes int64                    `json:"storage_quota_bytes"`
	Meta              models.JSONB             `json:"meta"`
}

// UpdateProjectRequest represents the request payload for updating a project
type UpdateProjectRequest struct {
	Name              *string                   `json:"name"`
	Description       *string                   `json:"description"`
	Visibility        *models.ProjectVisibility `json:"visibility"`
	StorageQuotaBytes *int64                    `json:"storage_quota_bytes"`
	Meta              models.JSONB              `json:"meta"`
}

// CreateProject creates a new project
func (h *ProjectHandler) CreateProject(c *gin.Context) {
	ownerIDStr := c.Param("id")
	ownerID, err := uuid.Parse(ownerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid owner ID"})
		return
	}

	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project := &models.Project{
		OwnerID:           ownerID,
		Name:              req.Name,
		Description:       req.Description,
		Visibility:        req.Visibility,
		RootBucket:        req.RootBucket,
		RootPrefix:        req.RootPrefix,
		StorageQuotaBytes: req.StorageQuotaBytes,
		Meta:              req.Meta,
	}

	if project.Visibility == "" {
		project.Visibility = models.ProjectVisibilityPrivate
	}

	if err := h.projectService.CreateProject(project); err != nil {
		h.logger.WithError(err).Error("Failed to create project")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Project created successfully",
		"project": project,
	})
}

// GetProject retrieves a project by ID
func (h *ProjectHandler) GetProject(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	project, err := h.projectService.GetProjectByID(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Prepare enriched response
	var chatSessionID *uuid.UUID
	if project.ChatSession != nil {
		chatSessionID = &project.ChatSession.ID
	}

	var storage any
	if project.StorageLocation != nil {
		storage = gin.H{
			"type":         project.StorageLocation.Type,
			"git_url":      project.StorageLocation.GitURL,
			"git_branch":   project.StorageLocation.GitBranch,
			"local_path":   project.StorageLocation.LocalPath,
			"network_path": project.StorageLocation.NetworkPath,
		}
	}

	var build any
	if project.BuildResult != nil {
		build = gin.H{
			"built_url": project.BuildResult.BuiltURL,
			"meta":      project.BuildResult.Meta,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"project":          project,
		"chat_session_id":  chatSessionID,
		"storage_location": storage,
		"build_result":     build,
	})
}

// GetProjectBySlug retrieves a project by slug
func (h *ProjectHandler) GetProjectBySlug(c *gin.Context) {
	ownerIDStr := c.Param("id")
	ownerID, err := uuid.Parse(ownerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid owner ID"})
		return
	}

	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Slug is required"})
		return
	}

	project, err := h.projectService.GetProjectBySlug(ownerID, slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"project": project})
}

// UpdateProject updates an existing project
func (h *ProjectHandler) UpdateProject(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Visibility != nil {
		updates["visibility"] = *req.Visibility
	}
	if req.StorageQuotaBytes != nil {
		updates["storage_quota_bytes"] = *req.StorageQuotaBytes
	}
	if req.Meta != nil {
		updates["meta"] = req.Meta
	}

	if err := h.projectService.UpdateProject(projectID, updates); err != nil {
		h.logger.WithError(err).Error("Failed to update project")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get updated project
	project, err := h.projectService.GetProjectByID(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve updated project"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Project updated successfully",
		"project": project,
	})
}

// DeleteProject deletes a project
func (h *ProjectHandler) DeleteProject(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	if err := h.projectService.DeleteProject(projectID); err != nil {
		h.logger.WithError(err).Error("Failed to delete project")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Project deleted successfully"})
}

// ListProjects retrieves a paginated list of projects
func (h *ProjectHandler) ListProjects(c *gin.Context) {
	// Always scope to the authenticated user
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

	// Force owner to current user
	ownerID := &userID

	var visibility *models.ProjectVisibility
	if visibilityStr := c.Query("visibility"); visibilityStr != "" {
		vis := models.ProjectVisibility(visibilityStr)
		visibility = &vis
	}

	projects, total, err := h.projectService.ListProjects(ownerID, visibility, offset, limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to list projects")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve projects"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"projects": projects,
		"pagination": gin.H{
			"offset": offset,
			"limit":  limit,
			"total":  total,
		},
	})
}

// GetPublicProjects retrieves public projects
func (h *ProjectHandler) GetPublicProjects(c *gin.Context) {
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

	projects, total, err := h.projectService.GetPublicProjects(offset, limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get public projects")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve public projects"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"projects": projects,
		"pagination": gin.H{
			"offset": offset,
			"limit":  limit,
			"total":  total,
		},
	})
}

// GetUserProjects retrieves all projects for a specific user
func (h *ProjectHandler) GetUserProjects(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	projects, err := h.projectService.GetUserProjects(userID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get user projects")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user projects"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

// UpdateProjectVisibility updates project visibility
func (h *ProjectHandler) UpdateProjectVisibility(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	var req struct {
		Visibility models.ProjectVisibility `json:"visibility" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.projectService.UpdateProjectVisibility(projectID, req.Visibility); err != nil {
		h.logger.WithError(err).Error("Failed to update project visibility")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Project visibility updated successfully"})
}

// UpdateProjectStorageQuota updates project storage quota
func (h *ProjectHandler) UpdateProjectStorageQuota(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	var req struct {
		StorageQuotaBytes int64 `json:"storage_quota_bytes" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.projectService.UpdateProjectStorageQuota(projectID, req.StorageQuotaBytes); err != nil {
		h.logger.WithError(err).Error("Failed to update project storage quota")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Project storage quota updated successfully"})
}

// SearchProjects searches projects by name or description
func (h *ProjectHandler) SearchProjects(c *gin.Context) {
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

	var ownerID *uuid.UUID
	if ownerIDStr := c.Query("owner_id"); ownerIDStr != "" {
		if parsedID, err := uuid.Parse(ownerIDStr); err == nil {
			ownerID = &parsedID
		}
	}

	var visibility *models.ProjectVisibility
	if visibilityStr := c.Query("visibility"); visibilityStr != "" {
		vis := models.ProjectVisibility(visibilityStr)
		visibility = &vis
	}

	projects, err := h.projectService.SearchProjects(query, ownerID, visibility, limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to search projects")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search projects"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

// GetProjectChatSessions retrieves all chat sessions for a project
func (h *ProjectHandler) GetProjectChatSessions(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	sessions, err := h.projectService.GetProjectChatSessions(projectID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get project chat sessions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve project chat sessions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"chat_sessions": sessions})
}

// BuildPreview builds the project and deploys it to static preview folder
// POST /api/v1/projects/:id/preview
func (h *ProjectHandler) BuildPreview(c *gin.Context) {
	// 1. Authenticate user and parse project ID
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

	projectIDStr := c.Param("id")
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	// 2. Get project and check ownership
	project, err := h.projectService.GetProjectByID(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}
	if project.OwnerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not own this project"})
		return
	}

	// 3. Check if StorageLocation.NetworkPath exists
	if project.StorageLocation == nil || project.StorageLocation.NetworkPath == nil || *project.StorageLocation.NetworkPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No network path available for this project (project not saved to MinIO yet)"})
		return
	}
	networkPath := *project.StorageLocation.NetworkPath

	// 4. Parse MinIO path (bucket/object)
	bucket, objectName := parseMinIOPathLocal(networkPath)
	if bucket == "" || objectName == "" {
		h.logger.Error("Failed to parse network path: empty bucket or object")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid network path format"})
		return
	}

	// 5. Download zip from MinIO to temp file
	tmpZipFile, err := os.CreateTemp("", "preview-*.zip")
	if err != nil {
		h.logger.WithError(err).Error("Failed to create temp zip file")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp file"})
		return
	}
	tmpZipPath := tmpZipFile.Name()
	tmpZipFile.Close()
	defer os.Remove(tmpZipPath)

	h.logger.Infof("Downloading from MinIO: bucket=%s, object=%s", bucket, objectName)
	if err := storage.DownloadFile(context.Background(), bucket, objectName, tmpZipPath); err != nil {
		h.logger.WithError(err).Error("Failed to download zip from MinIO")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to download project from MinIO"})
		return
	}

	// 6. Unzip to a temp working directory
	tmpWorkDir, err := os.MkdirTemp("", "preview-work-*")
	if err != nil {
		h.logger.WithError(err).Error("Failed to create temp work dir")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp work directory"})
		return
	}
	defer os.RemoveAll(tmpWorkDir)

	h.logger.Infof("Unzipping %s to %s", tmpZipPath, tmpWorkDir)
	if err := utils.UnZipFile(tmpZipPath, tmpWorkDir); err != nil {
		h.logger.WithError(err).Error("Failed to unzip project")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unzip project"})
		return
	}

	// 7. Run npm install && npm run build
	h.logger.Infof("Running npm install in %s", tmpWorkDir)
	installCmd := exec.Command("npm", "install")
	installCmd.Dir = tmpWorkDir
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		h.logger.WithError(err).Error("npm install failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "npm install failed"})
		return
	}

	h.logger.Infof("Running npm run build in %s", tmpWorkDir)
	buildCmd := exec.Command("npm", "run", "build")
	buildCmd.Dir = tmpWorkDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		h.logger.WithError(err).Error("npm run build failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "npm run build failed"})
		return
	}

	// 8. Copy dist folder to StaticFolderPath/<project-id>/
	cfg := config.LoadConfig()
	staticDest := filepath.Join(cfg.StaticFolderPath, projectID.String())
	distSrc := filepath.Join(tmpWorkDir, "dist")

	if _, err := os.Stat(distSrc); os.IsNotExist(err) {
		h.logger.WithError(err).Error("dist folder not found after build")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Build succeeded but dist folder not found"})
		return
	}

	// Remove old static files if they exist
	if err := os.RemoveAll(staticDest); err != nil && !os.IsNotExist(err) {
		h.logger.WithError(err).Warn("Failed to remove old static files")
	}

	h.logger.Infof("Copying dist from %s to %s", distSrc, staticDest)
	if err := utils.CopyDir(distSrc, staticDest); err != nil {
		h.logger.WithError(err).Error("Failed to copy dist folder")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to copy built files"})
		return
	}

	// 9. Update project.preview_url in database
	previewURL := fmt.Sprintf("/static/%s/index.html", projectID.String())
	if err := database.DB.Model(&models.Project{}).Where("id = ?", projectID).Update("preview_url", previewURL).Error; err != nil {
		h.logger.WithError(err).Error("Failed to update preview_url")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update preview URL"})
		return
	}

	h.logger.Infof("Preview built successfully for project %s", projectID)
	c.JSON(http.StatusOK, gin.H{
		"message":     "Preview built successfully",
		"preview_url": previewURL,
	})
}

// parseMinIOPathLocal parses a path like "bucket/path/to/object.zip" into bucket and object
// Returns empty strings if invalid
func parseMinIOPathLocal(p string) (string, string) {
	for i := 0; i < len(p); i++ {
		if p[i] == '/' {
			return p[:i], p[i+1:]
		}
	}
	return "", ""
}
