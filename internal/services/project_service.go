package services

import (
	"errors"
	"fmt"
	"strings"

	"borderless_coding_server/internal/models"
	"borderless_coding_server/pkg/database"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProjectService struct{}

func NewProjectService() *ProjectService {
	return &ProjectService{}
}

// CreateProject creates a new project
func (s *ProjectService) CreateProject(project *models.Project) error {
	if project.Name == "" {
		return errors.New("project name is required")
	}
	if project.OwnerID == uuid.Nil {
		return errors.New("owner ID is required")
	}

	// Check if user exists
	var user models.User
	if err := database.DB.Where("id = ? AND deleted_at IS NULL", project.OwnerID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("owner not found")
		}
		return err
	}

	// Check if project name already exists for this owner
	var existingProject models.Project
	if err := database.DB.Where("owner_id = ? AND name = ? AND deleted_at IS NULL", project.OwnerID, project.Name).First(&existingProject).Error; err == nil {
		return errors.New("project with this name already exists for this owner")
	}

	// Set default values
	if project.Meta == nil {
		project.Meta = make(models.JSONB)
	}
	if project.RootBucket == "" {
		project.RootBucket = "borderless-coding"
	}
	if project.RootPrefix == "" {
		project.RootPrefix = fmt.Sprintf("users/%s/projects/%s/", project.OwnerID.String(), project.ID.String())
	}

	return database.DB.Create(project).Error
}

// GetProjectByID retrieves a project by ID
func (s *ProjectService) GetProjectByID(id uuid.UUID) (*models.Project, error) {
	var project models.Project
	err := database.DB.
		Preload("StorageLocation").
		Preload("ChatSession").
		Preload("BuildResult").
		Where("id = ? AND deleted_at IS NULL", id).First(&project).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}
	return &project, nil
}

// GetProjectBySlug retrieves a project by slug
func (s *ProjectService) GetProjectBySlug(ownerID uuid.UUID, slug string) (*models.Project, error) {
	var project models.Project
	err := database.DB.Where("owner_id = ? AND slug = ? AND deleted_at IS NULL", ownerID, slug).First(&project).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}
	return &project, nil
}

// UpdateProject updates an existing project
func (s *ProjectService) UpdateProject(id uuid.UUID, updates map[string]interface{}) error {
	// Check if project exists
	var project models.Project
	if err := database.DB.Where("id = ? AND deleted_at IS NULL", id).First(&project).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("project not found")
		}
		return err
	}

	// Check for name conflicts if name is being updated
	if name, exists := updates["name"]; exists {
		var existingProject models.Project
		if err := database.DB.Where("owner_id = ? AND name = ? AND id != ? AND deleted_at IS NULL",
			project.OwnerID, name, id).First(&existingProject).Error; err == nil {
			return errors.New("project name already exists for this owner")
		}
	}

	// Increment version for optimistic locking
	updates["version"] = project.Version + 1

	return database.DB.Model(&project).Updates(updates).Error
}

// DeleteProject soft deletes a project
func (s *ProjectService) DeleteProject(id uuid.UUID) error {
	var project models.Project
	if err := database.DB.Where("id = ? AND deleted_at IS NULL", id).First(&project).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("project not found")
		}
		return err
	}

	return database.DB.Delete(&project).Error
}

// ListProjects retrieves a paginated list of projects
func (s *ProjectService) ListProjects(ownerID *uuid.UUID, visibility *models.ProjectVisibility, offset, limit int) ([]models.Project, int64, error) {
	var projects []models.Project
	var total int64

	query := database.DB.Model(&models.Project{}).Where("deleted_at IS NULL")

	// Filter by owner if specified
	if ownerID != nil {
		query = query.Where("owner_id = ?", *ownerID)
	}

	// Filter by visibility if specified
	if visibility != nil {
		query = query.Where("visibility = ?", *visibility)
	}

	// Count total projects
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get projects with pagination
	err := query.Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&projects).Error

	return projects, total, err
}

// GetPublicProjects retrieves public projects
func (s *ProjectService) GetPublicProjects(offset, limit int) ([]models.Project, int64, error) {
	public := models.ProjectVisibilityPublic
	return s.ListProjects(nil, &public, offset, limit)
}

// GetUserProjects retrieves all projects for a specific user
func (s *ProjectService) GetUserProjects(userID uuid.UUID) ([]models.Project, error) {
	var projects []models.Project
	err := database.DB.Where("owner_id = ? AND deleted_at IS NULL", userID).
		Order("created_at DESC").
		Find(&projects).Error
	return projects, err
}

// UpdateProjectVisibility updates project visibility
func (s *ProjectService) UpdateProjectVisibility(id uuid.UUID, visibility models.ProjectVisibility) error {
	return s.UpdateProject(id, map[string]interface{}{
		"visibility": visibility,
	})
}

// UpdateProjectStorageQuota updates project storage quota
func (s *ProjectService) UpdateProjectStorageQuota(id uuid.UUID, quotaBytes int64) error {
	return s.UpdateProject(id, map[string]interface{}{
		"storage_quota_bytes": quotaBytes,
	})
}

// SearchProjects searches projects by name or description
func (s *ProjectService) SearchProjects(query string, ownerID *uuid.UUID, visibility *models.ProjectVisibility, limit int) ([]models.Project, error) {
	var projects []models.Project

	dbQuery := database.DB.Where("deleted_at IS NULL AND (name ILIKE ? OR description ILIKE ?)",
		"%"+query+"%", "%"+query+"%")

	// Filter by owner if specified
	if ownerID != nil {
		dbQuery = dbQuery.Where("owner_id = ?", *ownerID)
	}

	// Filter by visibility if specified
	if visibility != nil {
		dbQuery = dbQuery.Where("visibility = ?", *visibility)
	}

	err := dbQuery.Limit(limit).
		Order("created_at DESC").
		Find(&projects).Error

	return projects, err
}

// GetProjectChatSessions retrieves all chat sessions for a project
func (s *ProjectService) GetProjectChatSessions(projectID uuid.UUID) ([]models.ChatSession, error) {
	var sessions []models.ChatSession
	err := database.DB.Where("project_id = ? AND archived_at IS NULL", projectID).
		Order("created_at DESC").
		Find(&sessions).Error
	return sessions, err
}

// UpdateProjectMetadata updates project metadata
func (s *ProjectService) UpdateProjectMetadata(id uuid.UUID, metadata models.JSONB) error {
	return s.UpdateProject(id, map[string]interface{}{
		"meta": metadata,
	})
}

// GenerateSlug generates a URL-friendly slug from a name
func (s *ProjectService) GenerateSlug(name string) string {
	// Convert to lowercase and replace non-alphanumeric characters with hyphens
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")

	// Remove multiple consecutive hyphens
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	// Remove leading/trailing hyphens
	slug = strings.Trim(slug, "-")

	return slug
}
