package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// StorageType enumerates storage backends
type StorageType string

const (
	StorageTypeGitRemote StorageType = "git_remote"
	StorageTypeLocalFS   StorageType = "local_fs"
	StorageTypeNetworkFS StorageType = "network_fs"
)

// StorageLocation represents a project's source storage configuration (one-to-one)
type StorageLocation struct {
	ID        uuid.UUID   `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ProjectID uuid.UUID   `json:"project_id" gorm:"type:uuid;not null;unique"`
	Type      StorageType `json:"type" gorm:"type:storage_type;not null"`

	// Git remote details
	GitURL    *string `json:"git_url"`
	GitBranch *string `json:"git_branch"`

	// Local filesystem path
	LocalPath *string `json:"local_path"`

	// Network filesystem path (e.g., NFS, SMB)
	NetworkPath *string `json:"network_path"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relationships
	Project Project `json:"project,omitempty" gorm:"foreignKey:ProjectID"`
}

func (StorageLocation) TableName() string { return "storage_locations" }

// ensure gorm import is used (avoids linter false-positive)
func _useGormTypes(db *gorm.DB) *gorm.DB { return db }

func (s *StorageLocation) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	now := time.Now()
	s.CreatedAt = now
	s.UpdatedAt = now
	return nil
}

func (s *StorageLocation) BeforeUpdate(tx *gorm.DB) error {
	s.UpdatedAt = time.Now()
	return nil
}
