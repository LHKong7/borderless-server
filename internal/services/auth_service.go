package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"borderless_coding_server/internal/models"
	"borderless_coding_server/pkg/database"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

type AuthService struct {
	googleConfig *oauth2.Config
	jwtSecret    string
}

func NewAuthService(googleClientID, googleClientSecret, redirectURL, jwtSecret string) *AuthService {
	config := &oauth2.Config{
		ClientID:     googleClientID,
		ClientSecret: googleClientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"openid",
			"email",
			"profile",
		},
		Endpoint: google.Endpoint,
	}

	return &AuthService{
		googleConfig: config,
		jwtSecret:    jwtSecret,
	}
}

// GoogleOIDCFlow handles the Google OIDC authentication flow
func (s *AuthService) GoogleOIDCFlow(ctx context.Context, code string, userAgent, ipAddress string) (*models.User, *models.Session, error) {
	// Exchange code for token
	token, err := s.googleConfig.Exchange(ctx, code)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	// Get user info from Google
	client := s.googleConfig.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("failed to get user info: status %d", resp.StatusCode)
	}

	var googleUser struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		Name          string `json:"name"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Picture       string `json:"picture"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return nil, nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	// Find or create auth identity
	authIdentity, err := s.findOrCreateAuthIdentity(ctx, googleUser.ID, googleUser.Email, googleUser.VerifiedEmail)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find or create auth identity: %w", err)
	}

	// Get or create user
	user, err := s.getOrCreateUser(ctx, authIdentity, googleUser)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get or create user: %w", err)
	}

	// Create session
	session, err := s.createSession(ctx, user.ID, userAgent, ipAddress)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Log authentication event
	provider := models.AuthProviderGoogle
	s.logAuthEvent(ctx, user.ID, "login_success", &provider, ipAddress, userAgent, map[string]interface{}{
		"google_user_id": googleUser.ID,
		"email":          googleUser.Email,
	})

	return user, session, nil
}

// findOrCreateAuthIdentity finds an existing auth identity or creates a new one
func (s *AuthService) findOrCreateAuthIdentity(ctx context.Context, providerUID, email string, emailVerified bool) (*models.AuthIdentity, error) {
	var authIdentity models.AuthIdentity

	// Try to find existing identity
	err := database.DB.Where("provider = ? AND provider_uid = ?", models.AuthProviderGoogle, providerUID).First(&authIdentity).Error
	if err == nil {
		return &authIdentity, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Check if email is already used by another account
	if email != "" {
		var existingUser models.User
		err = database.DB.Where("email = ? AND deleted_at IS NULL", email).First(&existingUser).Error
		if err == nil {
			// Email exists, check if it has Google auth identity
			var existingIdentity models.AuthIdentity
			err = database.DB.Where("user_id = ? AND provider = ?", existingUser.ID, models.AuthProviderGoogle).First(&existingIdentity).Error
			if err == nil {
				// User already has Google auth, return existing identity
				return &existingIdentity, nil
			}
			// User exists but no Google auth, we'll link it
		}
	}

	// Create new auth identity
	authIdentity = models.AuthIdentity{
		Provider:      models.AuthProviderGoogle,
		ProviderUID:   providerUID,
		EmailAtSignup: &email,
		EmailVerified: &emailVerified,
	}

	// If we found an existing user with this email, link the identity to that user
	if email != "" {
		var existingUser models.User
		err = database.DB.Where("email = ? AND deleted_at IS NULL", email).First(&existingUser).Error
		if err == nil {
			authIdentity.UserID = existingUser.ID
		}
	}

	if err := database.DB.Create(&authIdentity).Error; err != nil {
		return nil, err
	}

	return &authIdentity, nil
}

// getOrCreateUser gets an existing user or creates a new one
func (s *AuthService) getOrCreateUser(ctx context.Context, authIdentity *models.AuthIdentity, googleUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
}) (*models.User, error) {
	// If auth identity already has a user, return that user
	if authIdentity.UserID != uuid.Nil {
		var user models.User
		err := database.DB.Where("id = ? AND deleted_at IS NULL", authIdentity.UserID).First(&user).Error
		if err == nil {
			return &user, nil
		}
	}

	// Create new user
	user := models.User{
		Email:       &googleUser.Email,
		DisplayName: &googleUser.Name,
		IsActive:    true,
		Metadata: models.JSONB{
			"google_id":   googleUser.ID,
			"picture":     googleUser.Picture,
			"given_name":  googleUser.GivenName,
			"family_name": googleUser.FamilyName,
		},
	}

	if err := database.DB.Create(&user).Error; err != nil {
		return nil, err
	}

	// Update auth identity with user ID
	authIdentity.UserID = user.ID
	if err := database.DB.Save(authIdentity).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

// createSession creates a new session for the user
func (s *AuthService) createSession(ctx context.Context, userID uuid.UUID, userAgent, ipAddress string) (*models.Session, error) {
	// Generate refresh token
	refreshToken, err := s.generateRefreshToken()
	if err != nil {
		return nil, err
	}

	// Hash the refresh token
	refreshTokenHash := s.hashToken(refreshToken)

	// Create session
	session := models.Session{
		UserID:           userID,
		UserAgent:        &userAgent,
		IPNet:            &ipAddress,
		RefreshTokenHash: refreshTokenHash,
		ValidUntil:       time.Now().Add(30 * 24 * time.Hour), // 30 days
	}

	if err := database.DB.Create(&session).Error; err != nil {
		return nil, err
	}

	return &session, nil
}

// generateRefreshToken generates a secure random refresh token
func (s *AuthService) generateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// hashToken hashes a token using SHA-256
func (s *AuthService) hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// logAuthEvent logs an authentication event
func (s *AuthService) logAuthEvent(ctx context.Context, userID uuid.UUID, event string, provider *models.AuthProvider, ipAddress, userAgent string, details map[string]interface{}) {
	audit := models.AuthAudit{
		UserID:    &userID,
		Event:     event,
		Provider:  provider,
		IPNet:     &ipAddress,
		UserAgent: &userAgent,
		Details:   models.JSONB(details),
	}

	// Log asynchronously to avoid blocking the main flow
	go func() {
		database.DB.Create(&audit)
	}()
}

// ValidateSession validates a session and returns the user
func (s *AuthService) ValidateSession(ctx context.Context, refreshToken string) (*models.User, *models.Session, error) {
	refreshTokenHash := s.hashToken(refreshToken)

	var session models.Session
	err := database.DB.Where("refresh_token_hash = ? AND revoked_at IS NULL", refreshTokenHash).First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, errors.New("invalid session")
		}
		return nil, nil, err
	}

	// Check if session is still valid
	if !session.IsValid() {
		return nil, nil, errors.New("session expired")
	}

	// Get user
	var user models.User
	err = database.DB.Where("id = ? AND deleted_at IS NULL", session.UserID).First(&user).Error
	if err != nil {
		return nil, nil, err
	}

	return &user, &session, nil
}

// RevokeSession revokes a session
func (s *AuthService) RevokeSession(ctx context.Context, refreshToken string) error {
	refreshTokenHash := s.hashToken(refreshToken)

	var session models.Session
	err := database.DB.Where("refresh_token_hash = ?", refreshTokenHash).First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("session not found")
		}
		return err
	}

	session.Revoke()
	return database.DB.Save(&session).Error
}

// RevokeAllUserSessions revokes all sessions for a user
func (s *AuthService) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	now := time.Now()
	return database.DB.Model(&models.Session{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", &now).Error
}

// GetGoogleAuthURL returns the Google OAuth URL
func (s *AuthService) GetGoogleAuthURL(state string) string {
	return s.googleConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// CreateVerificationToken creates a verification token
func (s *AuthService) CreateVerificationToken(ctx context.Context, userID *uuid.UUID, purpose models.TokenPurpose, sentTo string, expiresIn time.Duration) (*models.VerificationToken, string, error) {
	// Generate token
	token, err := s.generateRefreshToken()
	if err != nil {
		return nil, "", err
	}

	tokenHash := s.hashToken(token)

	verificationToken := models.VerificationToken{
		UserID:    userID,
		Purpose:   purpose,
		SentTo:    sentTo,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(expiresIn),
		Metadata:  make(models.JSONB),
	}

	if err := database.DB.Create(&verificationToken).Error; err != nil {
		return nil, "", err
	}

	return &verificationToken, token, nil
}

// ValidateVerificationToken validates a verification token
func (s *AuthService) ValidateVerificationToken(ctx context.Context, token string, purpose models.TokenPurpose) (*models.VerificationToken, error) {
	tokenHash := s.hashToken(token)

	var verificationToken models.VerificationToken
	err := database.DB.Where("token_hash = ? AND purpose = ?", tokenHash, purpose).First(&verificationToken).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid token")
		}
		return nil, err
	}

	// Check if token is valid
	if !verificationToken.IsValid() {
		return nil, errors.New("token expired or already used")
	}

	// Increment attempt count
	verificationToken.AttemptCount++
	database.DB.Save(&verificationToken)

	return &verificationToken, nil
}

// ConsumeVerificationToken marks a verification token as consumed
func (s *AuthService) ConsumeVerificationToken(ctx context.Context, tokenID uuid.UUID) error {
	now := time.Now()
	return database.DB.Model(&models.VerificationToken{}).
		Where("id = ?", tokenID).
		Update("consumed_at", &now).Error
}
