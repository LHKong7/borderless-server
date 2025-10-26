package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserLevel enumerates subscription/entitlement levels
type UserLevel string

const (
	UserLevelFree      UserLevel = "free"
	UserLevelEntry     UserLevel = "entry"
	UserLevelSenior    UserLevel = "senior"
	UserLevelStaff     UserLevel = "staff"
	UserLevelPrincipal UserLevel = "principal"
)

// User represents a user in the system
type User struct {
	ID           uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Email        *string        `json:"email" gorm:"type:citext"` // NULL for phone-only accounts
	DisplayName  *string        `json:"display_name"`
	PasswordHash *string        `json:"-"` // Hidden from JSON
	IsActive     bool           `json:"is_active" gorm:"default:true"`
	Level        UserLevel      `json:"level" gorm:"type:user_level;default:'free'"`
	Metadata     JSONB          `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`

	// Relationships
	Projects           []Project           `json:"projects,omitempty" gorm:"foreignKey:OwnerID"`
	ChatSessions       []ChatSession       `json:"chat_sessions,omitempty" gorm:"foreignKey:UserID"`
	Phones             []UserPhone         `json:"phones,omitempty" gorm:"foreignKey:UserID"`
	AuthIdentities     []AuthIdentity      `json:"auth_identities,omitempty" gorm:"foreignKey:UserID"`
	Credentials        *UserCredential     `json:"credentials,omitempty" gorm:"foreignKey:UserID"`
	Sessions           []Session           `json:"sessions,omitempty" gorm:"foreignKey:UserID"`
	VerificationTokens []VerificationToken `json:"verification_tokens,omitempty" gorm:"foreignKey:UserID"`
	AuthAudits         []AuthAudit         `json:"auth_audits,omitempty" gorm:"foreignKey:UserID"`
}

// TableName returns the table name for the User model
func (User) TableName() string {
	return "users"
}

// BeforeCreate hook to set timestamps
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now
	if u.Level == "" {
		u.Level = UserLevelFree
	}
	return nil
}

// BeforeUpdate hook to update timestamp
func (u *User) BeforeUpdate(tx *gorm.DB) error {
	u.UpdatedAt = time.Now()
	return nil
}

// JSONB represents a JSONB field
type JSONB map[string]interface{}

// Scan implements the Scanner interface for JSONB
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONB)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return gorm.ErrInvalidData
	}

	return json.Unmarshal(bytes, j)
}

// Value implements the driver Valuer interface for JSONB
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}
