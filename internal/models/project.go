package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProjectVisibility represents the visibility level of a project
type ProjectVisibility string

const (
	ProjectVisibilityPrivate  ProjectVisibility = "private"
	ProjectVisibilityUnlisted ProjectVisibility = "unlisted"
	ProjectVisibilityPublic   ProjectVisibility = "public"
)

// Project represents a project in the system
type Project struct {
	ID                uuid.UUID         `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OwnerID           uuid.UUID         `json:"owner_id" gorm:"type:uuid;not null"`
	Name              string            `json:"name" gorm:"not null"`
	Slug              string            `json:"slug" gorm:"type:text;generated"`
	Description       *string           `json:"description"`
	Visibility        ProjectVisibility `json:"visibility" gorm:"type:project_visibility;default:'private'"`
	RootBucket        string            `json:"root_bucket" gorm:"not null"`
	RootPrefix        string            `json:"root_prefix" gorm:"not null"`
	StorageQuotaBytes int64             `json:"storage_quota_bytes" gorm:"default:0"`
	Meta              JSONB             `json:"meta" gorm:"type:jsonb;default:'{}'"`
	Version           int               `json:"version" gorm:"default:1"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	DeletedAt         gorm.DeletedAt    `json:"-" gorm:"index"`

	// Relationships
	Owner         User           `json:"owner,omitempty" gorm:"foreignKey:OwnerID"`
	ChatSessions  []ChatSession  `json:"chat_sessions,omitempty" gorm:"foreignKey:ProjectID"`
}

// TableName returns the table name for the Project model
func (Project) TableName() string {
	return "projects"
}

// BeforeCreate hook to set timestamps
func (p *Project) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

// BeforeUpdate hook to update timestamp
func (p *Project) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = time.Now()
	return nil
}

// IsUnlimitedStorage checks if the project has unlimited storage
func (p *Project) IsUnlimitedStorage() bool {
	return p.StorageQuotaBytes == 0
}
