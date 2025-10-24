package services

import (
	"errors"

	"borderless_coding_server/internal/models"
	"borderless_coding_server/pkg/database"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserService struct{}

func NewUserService() *UserService {
	return &UserService{}
}

// CreateUser creates a new user
func (s *UserService) CreateUser(user *models.User) error {
	if user.Email == nil || *user.Email == "" {
		return errors.New("email is required")
	}

	// Check if user already exists
	var existingUser models.User
	if err := database.DB.Where("email = ?", *user.Email).First(&existingUser).Error; err == nil {
		return errors.New("user with this email already exists")
	}

	// Set default metadata if not provided
	if user.Metadata == nil {
		user.Metadata = make(models.JSONB)
	}

	return database.DB.Create(user).Error
}

// GetUserByID retrieves a user by ID
func (s *UserService) GetUserByID(id uuid.UUID) (*models.User, error) {
	var user models.User
	err := database.DB.Where("id = ? AND deleted_at IS NULL", id).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

// GetUserByEmail retrieves a user by email
func (s *UserService) GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	err := database.DB.Where("email = ? AND deleted_at IS NULL", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

// UpdateUser updates an existing user
func (s *UserService) UpdateUser(id uuid.UUID, updates map[string]interface{}) error {
	// Check if user exists
	var user models.User
	if err := database.DB.Where("id = ? AND deleted_at IS NULL", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return err
	}

	// Prevent updating email to an existing one
	if email, exists := updates["email"]; exists {
		var existingUser models.User
		if err := database.DB.Where("email = ? AND id != ?", email, id).First(&existingUser).Error; err == nil {
			return errors.New("email already exists")
		}
	}

	return database.DB.Model(&user).Updates(updates).Error
}

// DeleteUser soft deletes a user
func (s *UserService) DeleteUser(id uuid.UUID) error {
	var user models.User
	if err := database.DB.Where("id = ? AND deleted_at IS NULL", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return err
	}

	return database.DB.Delete(&user).Error
}

// ListUsers retrieves a paginated list of users
func (s *UserService) ListUsers(offset, limit int) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	// Count total users
	if err := database.DB.Model(&models.User{}).Where("deleted_at IS NULL").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get users with pagination
	err := database.DB.Where("deleted_at IS NULL").
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&users).Error

	return users, total, err
}

// ActivateUser activates a user account
func (s *UserService) ActivateUser(id uuid.UUID) error {
	return s.UpdateUser(id, map[string]interface{}{
		"is_active": true,
	})
}

// DeactivateUser deactivates a user account
func (s *UserService) DeactivateUser(id uuid.UUID) error {
	return s.UpdateUser(id, map[string]interface{}{
		"is_active": false,
	})
}

// GetUserProjects retrieves all projects owned by a user
func (s *UserService) GetUserProjects(userID uuid.UUID) ([]models.Project, error) {
	var projects []models.Project
	err := database.DB.Where("owner_id = ? AND deleted_at IS NULL", userID).
		Order("created_at DESC").
		Find(&projects).Error
	return projects, err
}

// GetUserChatSessions retrieves all chat sessions for a user
func (s *UserService) GetUserChatSessions(userID uuid.UUID) ([]models.ChatSession, error) {
	var sessions []models.ChatSession
	err := database.DB.Where("user_id = ? AND archived_at IS NULL", userID).
		Order("created_at DESC").
		Find(&sessions).Error
	return sessions, err
}

// UpdateUserMetadata updates user metadata
func (s *UserService) UpdateUserMetadata(id uuid.UUID, metadata models.JSONB) error {
	return s.UpdateUser(id, map[string]interface{}{
		"metadata": metadata,
	})
}

// SearchUsers searches users by email or display name
func (s *UserService) SearchUsers(query string, limit int) ([]models.User, error) {
	var users []models.User
	err := database.DB.Where("deleted_at IS NULL AND (email ILIKE ? OR display_name ILIKE ?)",
		"%"+query+"%", "%"+query+"%").
		Limit(limit).
		Find(&users).Error
	return users, err
}
