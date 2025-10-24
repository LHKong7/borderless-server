package handlers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"net/http"
	"time"

	"borderless_coding_server/internal/models"
	"borderless_coding_server/internal/services"
	"borderless_coding_server/pkg/database"

	"borderless_coding_server/config"
	"crypto/sha256"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type AuthHandler struct {
	authService *services.AuthService
	jwtService  *services.JWTService
	logger      *logrus.Logger
}

func NewAuthHandler(authService *services.AuthService, jwtService *services.JWTService, logger *logrus.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		jwtService:  jwtService,
		logger:      logger,
	}
}

// RegisterRequest represents the payload to register a new user
type RegisterRequest struct {
	Email       string  `json:"email" binding:"required"`
	Cipher      string  `json:"cipher" binding:"required"` // base64 or raw PEM-block encoded from client
	DisplayName *string `json:"display_name"`
}

// LoginRequest represents the payload to login with email + encrypted password
type LoginRequest struct {
	Email  string `json:"email" binding:"required"`
	Cipher string `json:"cipher" binding:"required"`
}

// Register registers a new user account (email + password)
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.logger.Info("register request is ...", req)

	// 1) Server loads RSA private key
	cfg := config.LoadConfig()
	if cfg.PasswordEncPrivateKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "encryption not configured"})
		return
	}

	// 2) Decrypt client cipher → random || passwordHash (client-side hashed), discard random
	//    Expect base64 string from client
	cipherBytes, decErr := base64.StdEncoding.DecodeString(req.Cipher)
	if decErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cipher must be base64"})
		return
	}
	plain, err := decryptWithRSAPrivateKey([]byte(cfg.PasswordEncPrivateKey), cipherBytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cipher"})
		return
	}
	h.logger.Info("plain is ...", plain)

	if len(plain) < 33 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cipher payload too short"})
		return
	}
	// Suppose first 16 bytes = random, following 32 bytes = SHA-256(password) (example)
	// Adjust to your client protocol as needed
	baseHash := plain[len(plain)-32:]

	// Check if email already exists
	var existing models.User
	if err := database.DB.Where("email = ? AND deleted_at IS NULL", req.Email).First(&existing).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email already registered"})
		return
	} else if err != nil && err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// 3) Server-side salt + hash: H2 = sha256( salt || baseHash ) and store {salt, H2}
	salt := uuid.New().String()
	combined := append([]byte(salt), baseHash...)
	h2 := sha256.Sum256(combined)

	// Create user + credentials in a transaction
	var user models.User
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		email := req.Email
		user = models.User{
			Email:       &email,
			DisplayName: req.DisplayName,
			IsActive:    true,
			Metadata:    make(models.JSONB),
		}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}

		now := time.Now()
		hashedStr := hex.EncodeToString(h2[:])
		saltCopy := salt
		cred := models.UserCredential{
			UserID:        user.ID,
			PasswordHash:  &hashedStr,
			Salt:          &saltCopy,
			PasswordSetAt: &now,
		}
		if err := tx.Create(&cred).Error; err != nil {
			return err
		}

		// Audit
		event := "register_success"
		provider := models.AuthProviderPassword
		ip := c.ClientIP()
		ua := c.Request.UserAgent()
		audit := models.AuthAudit{
			UserID:    &user.ID,
			Event:     event,
			Provider:  &provider,
			IPNet:     &ip,
			UserAgent: &ua,
			Details:   models.JSONB{"email": req.Email},
		}
		_ = tx.Create(&audit).Error

		return nil
	}); err != nil {
		h.logger.WithError(err).Error("failed to register user")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "registration successful",
		"user":    user,
	})
}

// Login authenticates a user using email + RSA-OAEP(SHA256) encrypted (16||SHA256(password))
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Load private key
	cfg := config.LoadConfig()
	if cfg.PasswordEncPrivateKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "encryption not configured"})
		return
	}

	// Base64 decode cipher
	cipherBytes, decErr := base64.StdEncoding.DecodeString(req.Cipher)
	if decErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cipher must be base64"})
		return
	}

	// Decrypt RSA-OAEP(SHA256)
	plain, err := decryptWithRSAPrivateKey([]byte(cfg.PasswordEncPrivateKey), cipherBytes)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if len(plain) < 33 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cipher payload too short"})
		return
	}
	baseHash := plain[len(plain)-32:]

	// Find user by email
	var user models.User
	if err := database.DB.Where("email = ? AND deleted_at IS NULL", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Load user credential
	var cred models.UserCredential
	if err := database.DB.Where("user_id = ?", user.ID).First(&cred).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if cred.Salt == nil || cred.PasswordHash == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Compute server-side H2 and compare
	combined := append([]byte(*cred.Salt), baseHash...)
	h2 := sha256.Sum256(combined)
	h2hex := hex.EncodeToString(h2[:])
	if h2hex != *cred.PasswordHash {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Create session (random refresh token -> store hash)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}
	refreshToken := base64.URLEncoding.EncodeToString(tokenBytes)
	rth := sha256.Sum256([]byte(refreshToken))
	rthHex := hex.EncodeToString(rth[:])

	ua := c.Request.UserAgent()
	ip := c.ClientIP()
	now := time.Now()
	session := models.Session{
		UserID:           user.ID,
		UserAgent:        &ua,
		IPNet:            &ip,
		RefreshTokenHash: rthHex,
		ValidUntil:       now.Add(30 * 24 * time.Hour),
	}
	if err := database.DB.Create(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}

	// Access token
	accessToken, err := h.jwtService.GenerateAccessToken(user.ID, session.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate access token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "login successful",
		"user":           user,
		"access_token":   accessToken,
		"refresh_token":  refreshToken,
		"access_expires": 15 * 60,
	})
}

// decryptWithRSAPrivateKey decodes a PEM private key and decrypts the cipher input.
// This expects the client to encrypt using RSA-OAEP with SHA-256
// over the raw bytes of (random || baseHash).
func decryptWithRSAPrivateKey(pemKey []byte, cipher []byte) ([]byte, error) {
	block, _ := pem.Decode(pemKey)
	if block == nil {
		return nil, errors.New("invalid private key PEM")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// try PKCS8
		if k2, err2 := x509.ParsePKCS8PrivateKey(block.Bytes); err2 == nil {
			if rsaKey, ok := k2.(*rsa.PrivateKey); ok {
				key = rsaKey
			} else {
				return nil, errors.New("private key is not RSA")
			}
		} else {
			return nil, err
		}
	}
	// Decrypt using RSA-OAEP (SHA-256). If client base64-encodes, decode before calling.
	plain, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, key, cipher, nil)
	if err != nil {
		return nil, err
	}
	return plain, nil
}

// GoogleAuthRequest represents the request for Google OAuth
type GoogleAuthRequest struct {
	State string `json:"state"` // Optional state parameter for CSRF protection
}

// GoogleCallbackRequest represents the callback request from Google
type GoogleCallbackRequest struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state"`
}

// RefreshTokenRequest represents the request to refresh tokens
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// LogoutRequest represents the logout request
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// GetGoogleAuthURL returns the Google OAuth URL
func (h *AuthHandler) GetGoogleAuthURL(c *gin.Context) {
	var req GoogleAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate state if not provided
	state := req.State
	if state == "" {
		state = uuid.New().String()
	}

	authURL := h.authService.GetGoogleAuthURL(state)

	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
		"state":    state,
	})
}

// GoogleCallback handles the Google OAuth callback
func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	var req GoogleCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user agent and IP address
	userAgent := c.GetHeader("User-Agent")
	ipAddress := c.ClientIP()

	// Process Google OIDC flow
	user, session, err := h.authService.GoogleOIDCFlow(c.Request.Context(), req.Code, userAgent, ipAddress)
	if err != nil {
		h.logger.WithError(err).Error("Google OIDC flow failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authentication failed"})
		return
	}

	// Generate JWT tokens
	accessToken, err := h.jwtService.GenerateAccessToken(user.ID, session.ID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to generate access token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate access token"})
		return
	}

	refreshToken, err := h.jwtService.GenerateRefreshToken(user.ID, session.ID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to generate refresh token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Authentication successful",
		"user":          user,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    15 * 60, // 15 minutes in seconds
	})
}

// RefreshToken refreshes the access token using a refresh token
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate the refresh token
	user, _, err := h.authService.ValidateSession(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	// Generate new tokens
	accessToken, refreshToken, err := h.jwtService.RefreshTokenPair(req.RefreshToken)
	if err != nil {
		h.logger.WithError(err).Error("Failed to refresh tokens")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh tokens"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Tokens refreshed successfully",
		"user":          user,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    15 * 60, // 15 minutes in seconds
	})
}

// Logout logs out the user and revokes the session
func (h *AuthHandler) Logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Revoke the session
	err := h.authService.RevokeSession(c.Request.Context(), req.RefreshToken)
	if err != nil {
		h.logger.WithError(err).Error("Failed to revoke session")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to logout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// LogoutAll logs out the user from all sessions
func (h *AuthHandler) LogoutAll(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
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

	// Revoke all sessions
	err := h.authService.RevokeAllUserSessions(c.Request.Context(), userID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to revoke all sessions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout from all sessions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out from all sessions successfully",
	})
}

// GetProfile returns the current user's profile
func (h *AuthHandler) GetProfile(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
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

	// Get user from database
	userService := services.NewUserService()
	user, err := userService.GetUserByID(userID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get user profile")
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}

// ValidateToken validates an access token
func (h *AuthHandler) ValidateToken(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	tokenString, err := h.jwtService.ExtractTokenFromHeader(authHeader)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	claims, err := h.jwtService.ValidateToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":      true,
		"user_id":    claims.UserID,
		"session_id": claims.SessionID,
		"expires_at": claims.ExpiresAt.Time,
	})
}

// GetSessions returns all active sessions for the current user
func (h *AuthHandler) GetSessions(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
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

	// Get sessions from database
	var sessions []models.Session
	err := database.DB.Where("user_id = ? AND revoked_at IS NULL", userID).
		Order("created_at DESC").
		Find(&sessions).Error
	if err != nil {
		h.logger.WithError(err).Error("Failed to get user sessions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get sessions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
	})
}

// RevokeSession revokes a specific session
func (h *AuthHandler) RevokeSession(c *gin.Context) {
	sessionIDStr := c.Param("session_id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	// Get user ID from context (set by auth middleware)
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

	// Revoke the session
	now := time.Now()
	err = database.DB.Model(&models.Session{}).
		Where("id = ? AND user_id = ?", sessionID, userID).
		Update("revoked_at", &now).Error
	if err != nil {
		h.logger.WithError(err).Error("Failed to revoke session")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Session revoked successfully",
	})
}

func (h *AuthHandler) ObtainPublicKey(c *gin.Context) {
	cfg := config.LoadConfig()
	if cfg.PasswordEncPublicKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "encryption not configured"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"public_key": cfg.PasswordEncPublicKey})
}
