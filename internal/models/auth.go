package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuthProvider represents the authentication provider type
type AuthProvider string

const (
	AuthProviderPassword AuthProvider = "password"
	AuthProviderGoogle   AuthProvider = "google"
)

// TokenPurpose represents the purpose of a verification token
type TokenPurpose string

const (
	TokenPurposeEmailVerify   TokenPurpose = "email_verify"
	TokenPurposeEmailLogin    TokenPurpose = "email_login"
	TokenPurposePhoneVerify   TokenPurpose = "phone_verify"
	TokenPurposePhoneLogin    TokenPurpose = "phone_login"
	TokenPurposeResetPassword TokenPurpose = "reset_password"
)

// UserPhone represents a phone number linked to a user
type UserPhone struct {
	ID         uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID     uuid.UUID  `json:"user_id" gorm:"type:uuid;not null"`
	E164       string     `json:"e164" gorm:"not null;uniqueIndex"` // +8869xxxxxxx (E.164)
	IsPrimary  bool       `json:"is_primary" gorm:"default:false"`
	VerifiedAt *time.Time `json:"verified_at"`
	CreatedAt  time.Time  `json:"created_at"`

	// Relationships
	User User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName returns the table name for the UserPhone model
func (UserPhone) TableName() string {
	return "user_phones"
}

// BeforeCreate hook to set timestamps
func (up *UserPhone) BeforeCreate(tx *gorm.DB) error {
	if up.ID == uuid.Nil {
		up.ID = uuid.New()
	}
	now := time.Now()
	up.CreatedAt = now
	return nil
}

// IsVerified checks if the phone number is verified
func (up *UserPhone) IsVerified() bool {
	return up.VerifiedAt != nil
}

// AuthIdentity represents an external authentication identity
type AuthIdentity struct {
	ID                    uuid.UUID    `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID                uuid.UUID    `json:"user_id" gorm:"type:uuid;not null"`
	Provider              AuthProvider `json:"provider" gorm:"type:auth_provider;not null"`
	ProviderUID           string       `json:"provider_uid" gorm:"not null"`
	EmailAtSignup         *string      `json:"email_at_signup" gorm:"type:citext"`
	EmailVerified         *bool        `json:"email_verified"`
	RefreshTokenEncrypted []byte       `json:"-"` // Hidden from JSON
	CreatedAt             time.Time    `json:"created_at"`

	// Relationships
	User User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName returns the table name for the AuthIdentity model
func (AuthIdentity) TableName() string {
	return "auth_identities"
}

// BeforeCreate hook to set timestamps
func (ai *AuthIdentity) BeforeCreate(tx *gorm.DB) error {
	if ai.ID == uuid.Nil {
		ai.ID = uuid.New()
	}
	now := time.Now()
	ai.CreatedAt = now
	return nil
}

// UserCredential represents user credentials for password authentication
type UserCredential struct {
	UserID        uuid.UUID  `json:"user_id" gorm:"type:uuid;primary_key"`
	PasswordHash  *string    `json:"-"` // Hidden from JSON (server-side H2)
	Salt          *string    `json:"-"` // Server-generated salt used in H2
	PasswordSetAt *time.Time `json:"password_set_at"`

	// Relationships
	User User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName returns the table name for the UserCredential model
func (UserCredential) TableName() string {
	return "user_credentials"
}

// HasPassword checks if the user has a password set
func (uc *UserCredential) HasPassword() bool {
	return uc.PasswordHash != nil && *uc.PasswordHash != ""
}

// VerificationToken represents a one-time verification token
type VerificationToken struct {
	ID           uuid.UUID    `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID       *uuid.UUID   `json:"user_id" gorm:"type:uuid"`
	Purpose      TokenPurpose `json:"purpose" gorm:"type:token_purpose;not null"`
	SentTo       string       `json:"sent_to" gorm:"not null"` // email or E.164 phone
	TokenHash    string       `json:"-" gorm:"not null"`       // Hidden from JSON
	ExpiresAt    time.Time    `json:"expires_at" gorm:"not null"`
	ConsumedAt   *time.Time   `json:"consumed_at"`
	AttemptCount int          `json:"attempt_count" gorm:"default:0"`
	Metadata     JSONB        `json:"metadata" gorm:"type:jsonb;default:'{}'"`
	CreatedAt    time.Time    `json:"created_at"`

	// Relationships
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName returns the table name for the VerificationToken model
func (VerificationToken) TableName() string {
	return "verification_tokens"
}

// BeforeCreate hook to set timestamps
func (vt *VerificationToken) BeforeCreate(tx *gorm.DB) error {
	if vt.ID == uuid.Nil {
		vt.ID = uuid.New()
	}
	now := time.Now()
	vt.CreatedAt = now
	return nil
}

// IsExpired checks if the token is expired
func (vt *VerificationToken) IsExpired() bool {
	return time.Now().After(vt.ExpiresAt)
}

// IsConsumed checks if the token has been consumed
func (vt *VerificationToken) IsConsumed() bool {
	return vt.ConsumedAt != nil
}

// IsValid checks if the token is valid (not expired and not consumed)
func (vt *VerificationToken) IsValid() bool {
	return !vt.IsExpired() && !vt.IsConsumed()
}

// Session represents a user session
type Session struct {
	ID               uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID           uuid.UUID  `json:"user_id" gorm:"type:uuid;not null"`
	UserAgent        *string    `json:"user_agent"`
	IPNet            *string    `json:"ip_net" gorm:"type:inet"`
	RefreshTokenHash string     `json:"-" gorm:"not null"` // Hidden from JSON
	ValidUntil       time.Time  `json:"valid_until" gorm:"not null"`
	CreatedAt        time.Time  `json:"created_at"`
	RevokedAt        *time.Time `json:"revoked_at"`

	// Relationships
	User User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName returns the table name for the Session model
func (Session) TableName() string {
	return "sessions"
}

// BeforeCreate hook to set timestamps
func (s *Session) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	now := time.Now()
	s.CreatedAt = now
	return nil
}

// IsValid checks if the session is valid (not expired and not revoked)
func (s *Session) IsValid() bool {
	return time.Now().Before(s.ValidUntil) && s.RevokedAt == nil
}

// IsExpired checks if the session is expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ValidUntil)
}

// IsRevoked checks if the session is revoked
func (s *Session) IsRevoked() bool {
	return s.RevokedAt != nil
}

// Revoke revokes the session
func (s *Session) Revoke() {
	now := time.Now()
	s.RevokedAt = &now
}

// AuthAudit represents an authentication audit log entry
type AuthAudit struct {
	ID        uuid.UUID     `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    *uuid.UUID    `json:"user_id" gorm:"type:uuid"`
	Event     string        `json:"event" gorm:"not null"` // 'login_success','otp_sent','logout', etc.
	Provider  *AuthProvider `json:"provider" gorm:"type:auth_provider"`
	IPNet     *string       `json:"ip_net" gorm:"type:inet"`
	UserAgent *string       `json:"user_agent"`
	Details   JSONB         `json:"details" gorm:"type:jsonb;default:'{}'"`
	CreatedAt time.Time     `json:"created_at"`

	// Relationships
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName returns the table name for the AuthAudit model
func (AuthAudit) TableName() string {
	return "auth_audit"
}

// BeforeCreate hook to set timestamps
func (aa *AuthAudit) BeforeCreate(tx *gorm.DB) error {
	if aa.ID == uuid.Nil {
		aa.ID = uuid.New()
	}
	now := time.Now()
	aa.CreatedAt = now
	return nil
}
