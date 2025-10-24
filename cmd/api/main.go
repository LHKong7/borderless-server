package main

import (
	"borderless_coding_server/config"
	"borderless_coding_server/internal/handlers"
	"borderless_coding_server/internal/middleware"
	"borderless_coding_server/internal/services"
	"borderless_coding_server/pkg/cache"
	"borderless_coding_server/pkg/database"
	"borderless_coding_server/pkg/storage"
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// "borderless_coding_server/internal/handlers"
	// "borderless_coding_server/internal/middleware"
	// "borderless_coding_server/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Setup logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	logger.Info("configuration is ...", cfg)

	// Set Gin mode
	gin.SetMode(cfg.GinMode)

	// Initialize database
	if err := database.ConnectDB(cfg); err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.CloseDB()

	// Run database migrations
	if err := database.AutoMigrate(); err != nil {
		logger.Fatalf("Failed to run database migrations: %v", err)
	}

	// Initialize Redis
	if err := cache.ConnectRedis(cfg); err != nil {
		logger.Warnf("Failed to connect to Redis: %v", err)
	}
	defer cache.CloseRedis()

	// Initialize MinIO
	if err := storage.ConnectMinIO(cfg); err != nil {
		logger.Warnf("Failed to connect to MinIO: %v", err)
	}

	// Setup Gin router
	router := gin.New()

	// Add middleware
	router.Use(middleware.Logger(logger))
	router.Use(middleware.Recovery(logger))
	router.Use(middleware.CORS())
	router.Use(middleware.RequestID())

	// Initialize services
	userService := services.NewUserService()
	projectService := services.NewProjectService()
	chatService := services.NewChatService()
	authService := services.NewAuthService(
		cfg.GoogleClientID,
		cfg.GoogleClientSecret,
		cfg.GoogleRedirectURL,
		cfg.JWTSecret,
	)
	jwtService := services.NewJWTService(cfg.JWTSecret)
	buildService := services.NewBuildService(cfg.ClaudeCLIPath, logger)

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(logger)
	userHandler := handlers.NewUserHandler(userService, logger)
	projectHandler := handlers.NewProjectHandler(projectService, logger)
	chatHandler := handlers.NewChatHandler(chatService, logger)
	authHandler := handlers.NewAuthHandler(authService, jwtService, logger)
	buildHandler := handlers.NewBuildHandler(buildService, logger)

	// Setup routes
	setupRoutes(router, healthHandler, userHandler, projectHandler, chatHandler, authHandler, buildHandler, jwtService)

	// Create HTTP server
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		logger.Infof("Server starting on port %s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	// Give outstanding requests 60 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatalf("Server forced to shutdown: %v", err)
	}

	logger.Info("Server exited")
}

func setupRoutes(router *gin.Engine, healthHandler *handlers.HealthHandler, userHandler *handlers.UserHandler, projectHandler *handlers.ProjectHandler, chatHandler *handlers.ChatHandler, authHandler *handlers.AuthHandler, buildHandler *handlers.BuildHandler, jwtService *services.JWTService) {
	// Health check routes
	router.GET("/health", healthHandler.HealthCheck)
	router.GET("/health/ready", healthHandler.ReadinessCheck)
	router.GET("/health/live", healthHandler.LivenessCheck)

	// Authentication routes (public)
	auth := router.Group("/auth")
	{
		auth.GET("/pbkey", authHandler.ObtainPublicKey)
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/google/url", authHandler.GetGoogleAuthURL)
		auth.POST("/google/callback", authHandler.GoogleCallback)
		auth.POST("/refresh", authHandler.RefreshToken)
		auth.POST("/logout", authHandler.Logout)
		auth.POST("/logout-all", middleware.AuthMiddleware(jwtService), authHandler.LogoutAll)
		auth.GET("/validate", authHandler.ValidateToken)
		auth.GET("/profile", middleware.AuthMiddleware(jwtService), authHandler.GetProfile)
		auth.GET("/sessions", middleware.AuthMiddleware(jwtService), authHandler.GetSessions)
		auth.DELETE("/sessions/:session_id", middleware.AuthMiddleware(jwtService), authHandler.RevokeSession)
	}

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Ping endpoint
		v1.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message":   "pong",
				"timestamp": time.Now().UTC(),
			})
		})

		// Public routes (no authentication required)
		public := v1.Group("/public")
		{
			public.GET("/projects", projectHandler.GetPublicProjects)
		}

		// Protected routes (authentication required)
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware(jwtService))
		protected.Use(middleware.RequireActiveUser())
		{
			// User routes
			users := protected.Group("/users")
			{
				users.GET("", userHandler.ListUsers)
				users.GET("/search", userHandler.SearchUsers)
				users.GET("/:id", userHandler.GetUser)
				users.PUT("/:id", userHandler.UpdateUser)
				users.DELETE("/:id", userHandler.DeleteUser)
				users.POST("/:id/activate", userHandler.ActivateUser)
				users.POST("/:id/deactivate", userHandler.DeactivateUser)
				users.GET("/:id/projects", userHandler.GetUserProjects)
				users.GET("/:id/chat-sessions", userHandler.GetUserChatSessions)
			}

			// Project routes
			projects := protected.Group("/projects")
			{
				projects.GET("", projectHandler.ListProjects)
				projects.GET("/search", projectHandler.SearchProjects)
				projects.GET("/:id", projectHandler.GetProject)
				projects.PUT("/:id", projectHandler.UpdateProject)
				projects.DELETE("/:id", projectHandler.DeleteProject)
				projects.PUT("/:id/visibility", projectHandler.UpdateProjectVisibility)
				projects.PUT("/:id/storage-quota", projectHandler.UpdateProjectStorageQuota)
				projects.GET("/:id/chat-sessions", projectHandler.GetProjectChatSessions)
			}

			// User-specific project routes
			userProjects := protected.Group("/users/:id/projects")
			{
				userProjects.POST("", projectHandler.CreateProject)
				userProjects.GET("/slug/:slug", projectHandler.GetProjectBySlug)
			}

			// Chat session routes
			chatSessions := protected.Group("/chat-sessions")
			{
				chatSessions.POST("", chatHandler.CreateChatSession)
				chatSessions.GET("/:id", chatHandler.GetChatSession)
				chatSessions.PUT("/:id", chatHandler.UpdateChatSession)
				chatSessions.DELETE("/:id", chatHandler.DeleteChatSession)
				chatSessions.POST("/:id/archive", chatHandler.ArchiveChatSession)
				chatSessions.POST("/:id/unarchive", chatHandler.UnarchiveChatSession)
				chatSessions.GET("/:id/messages", chatHandler.GetChatMessages)
				chatSessions.GET("/:id/with-messages", chatHandler.GetChatSessionWithMessages)
			}

			// User-specific chat session routes
			// Avoid conflict with users group by using the same ":id" wildcard
			// and only adding non-duplicate subroutes
			userChatSessions := protected.Group("/users/:id/chat-sessions")
			{
				// Listing is already handled at users.GET(":id/chat-sessions") above
				userChatSessions.GET("/recent", chatHandler.GetRecentChatSessions)
			}

			// Project-specific chat session routes are already handled above via
			// projects.GET(":id/chat-sessions", projectHandler.GetProjectChatSessions)

			// Chat message routes
			chatMessages := protected.Group("/chat-messages")
			{
				chatMessages.POST("", chatHandler.CreateChatMessage)
				chatMessages.GET("/:id", chatHandler.GetChatMessage)
				chatMessages.PUT("/:id", chatHandler.UpdateChatMessage)
				chatMessages.DELETE("/:id", chatHandler.DeleteChatMessage)
			}

			// Build routes
			builds := protected.Group("/builds")
			{
				builds.POST("", buildHandler.StartBuild)
				builds.POST("/claude", buildHandler.StartBuildWithClaude)
				builds.GET("/:id", buildHandler.GetBuild)
				builds.DELETE("/:id", buildHandler.CancelBuild)
				builds.GET("/:id/logs", buildHandler.GetBuildLogs)
				builds.GET("/:id/stream", buildHandler.StreamBuildOutput)
				builds.GET("", buildHandler.GetUserBuilds)
			}

			// Project-specific build routes
			projectBuilds := protected.Group("/projects/:id/builds")
			{
				projectBuilds.POST("", buildHandler.StartBuild)
				projectBuilds.POST("/claude", buildHandler.StartBuildWithClaude)
				projectBuilds.GET("", buildHandler.GetProjectBuilds)
			}
		}
	}

	// Root route
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message":   "Welcome to Borderless Coding Server",
			"version":   "1.0.0",
			"timestamp": time.Now().UTC(),
		})
	})
}
