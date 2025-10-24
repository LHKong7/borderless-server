package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"borderless_coding_server/internal/models"
	"borderless_coding_server/internal/services"
	"borderless_coding_server/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AuthMiddleware creates authentication middleware
func AuthMiddleware(jwtService *services.JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		tokenString, err := jwtService.ExtractTokenFromHeader(authHeader)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		claims, err := jwtService.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Set user information in context
		c.Set("user_id", claims.UserID)
		c.Set("session_id", claims.SessionID)
		c.Set("claims", claims)

		c.Next()
	}
}

// OptionalAuthMiddleware creates optional authentication middleware
func OptionalAuthMiddleware(jwtService *services.JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		tokenString, err := jwtService.ExtractTokenFromHeader(authHeader)
		if err != nil {
			c.Next()
			return
		}

		claims, err := jwtService.ValidateToken(tokenString)
		if err != nil {
			c.Next()
			return
		}

		// Set user information in context if token is valid
		c.Set("user_id", claims.UserID)
		c.Set("session_id", claims.SessionID)
		c.Set("claims", claims)

		c.Next()
	}
}

// RequireRole creates middleware that requires specific roles
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDInterface, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		userID, ok := userIDInterface.(uuid.UUID)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
			c.Abort()
			return
		}

		// Get user from database to check roles
		// This is a simplified implementation - you might want to cache user roles
		var user models.User
		err := database.DB.Where("id = ? AND deleted_at IS NULL", userID).First(&user).Error
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		// Check if user has required role
		userRoles, ok := user.Metadata["roles"].([]interface{})
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}

		hasRole := false
		for _, role := range roles {
			for _, userRole := range userRoles {
				if roleStr, ok := userRole.(string); ok && roleStr == role {
					hasRole = true
					break
				}
			}
			if hasRole {
				break
			}
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireActiveUser creates middleware that requires an active user
func RequireActiveUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDInterface, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		userID, ok := userIDInterface.(uuid.UUID)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
			c.Abort()
			return
		}

		// Check if user is active
		var user models.User
		err := database.DB.Where("id = ? AND is_active = true AND deleted_at IS NULL", userID).First(&user).Error
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User account is inactive"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitMiddleware creates a simple rate limiting middleware
func RateLimitMiddleware(maxRequests int, windowMinutes int) gin.HandlerFunc {
	// This is a simplified implementation
	// In production, you should use Redis or a proper rate limiting library
	return func(c *gin.Context) {
		// Get client IP
		clientIP := c.ClientIP()
		fmt.Println("clientIP is ", clientIP)

		// Simple in-memory rate limiting (not suitable for production)
		// You should implement proper rate limiting with Redis
		c.Next()
	}
}

// CORSMiddleware creates CORS middleware with authentication support
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Allow specific origins or all origins in development
		allowedOrigins := []string{
			"http://localhost:3000",
			"http://localhost:3001",
			"https://yourdomain.com",
		}

		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}

		if allowed || strings.Contains(origin, "localhost") {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// SecurityHeadersMiddleware adds security headers
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'")

		c.Next()
	}
}
