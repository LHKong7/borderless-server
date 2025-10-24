package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BuildStatus represents the status of a build
type BuildStatus string

const (
	BuildStatusPending   BuildStatus = "pending"
	BuildStatusRunning   BuildStatus = "running"
	BuildStatusCompleted BuildStatus = "completed"
	BuildStatusFailed    BuildStatus = "failed"
	BuildStatusCancelled BuildStatus = "cancelled"
)

// Build represents a build process
type Build struct {
	ID          uuid.UUID   `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID      uuid.UUID   `json:"user_id" gorm:"type:uuid;not null"`
	ProjectID   uuid.UUID   `json:"project_id" gorm:"type:uuid;not null"`
	SessionID   *uuid.UUID  `json:"session_id" gorm:"type:uuid"`
	Command     string      `json:"command" gorm:"not null"`
	WorkingDir  string      `json:"working_dir" gorm:"not null"`
	Status      BuildStatus `json:"status" gorm:"type:text;default:'pending'"`
	Output      string      `json:"output" gorm:"type:text"`
	Error       *string     `json:"error"`
	ExitCode    *int        `json:"exit_code"`
	ProcessID   *int        `json:"process_id"`
	StartedAt   *time.Time  `json:"started_at"`
	CompletedAt *time.Time  `json:"completed_at"`
	Metadata    JSONB       `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`

	// Relationships
	User      User         `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Project   Project      `json:"project,omitempty" gorm:"foreignKey:ProjectID"`
	Session   *ChatSession `json:"session,omitempty" gorm:"foreignKey:SessionID"`
	BuildLogs []BuildLog   `json:"build_logs,omitempty" gorm:"foreignKey:BuildID"`
}

// TableName returns the table name for the Build model
func (Build) TableName() string {
	return "builds"
}

// BeforeCreate hook to set timestamps
func (b *Build) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	now := time.Now()
	b.CreatedAt = now
	b.UpdatedAt = now
	return nil
}

// BeforeUpdate hook to update timestamp
func (b *Build) BeforeUpdate(tx *gorm.DB) error {
	b.UpdatedAt = time.Now()
	return nil
}

// IsRunning checks if the build is currently running
func (b *Build) IsRunning() bool {
	return b.Status == BuildStatusRunning
}

// IsCompleted checks if the build is completed
func (b *Build) IsCompleted() bool {
	return b.Status == BuildStatusCompleted || b.Status == BuildStatusFailed || b.Status == BuildStatusCancelled
}

// Duration returns the build duration
func (b *Build) Duration() *time.Duration {
	if b.StartedAt == nil {
		return nil
	}

	endTime := b.CompletedAt
	if endTime == nil {
		endTime = &time.Time{}
		*endTime = time.Now()
	}

	duration := endTime.Sub(*b.StartedAt)
	return &duration
}

// BuildLog represents a log entry for a build
type BuildLog struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	BuildID   uuid.UUID `json:"build_id" gorm:"type:uuid;not null"`
	Level     string    `json:"level" gorm:"not null"` // info, warn, error, debug
	Message   string    `json:"message" gorm:"not null"`
	Timestamp time.Time `json:"timestamp" gorm:"not null"`
	Metadata  JSONB     `json:"metadata" gorm:"type:jsonb;default:'{}'"`

	// Relationships
	Build Build `json:"build,omitempty" gorm:"foreignKey:BuildID"`
}

// TableName returns the table name for the BuildLog model
func (BuildLog) TableName() string {
	return "build_logs"
}

// BeforeCreate hook to set timestamps
func (bl *BuildLog) BeforeCreate(tx *gorm.DB) error {
	if bl.ID == uuid.Nil {
		bl.ID = uuid.New()
	}
	now := time.Now()
	bl.Timestamp = now
	return nil
}
