package handlers

import (
	"net/http"
	"time"

	"borderless_coding_server/pkg/cache"
	"borderless_coding_server/pkg/database"
	"borderless_coding_server/pkg/storage"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type HealthHandler struct {
	logger *logrus.Logger
}

func NewHealthHandler(logger *logrus.Logger) *HealthHandler {
	return &HealthHandler{
		logger: logger,
	}
}

// HealthCheck returns the health status of the service
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"service":   "borderless-coding-server",
	})
}

// ReadinessCheck checks if all dependencies are ready
func (h *HealthHandler) ReadinessCheck(c *gin.Context) {
	ctx := c.Request.Context()
	status := gin.H{
		"status":    "ready",
		"timestamp": time.Now().UTC(),
		"checks":    make(map[string]string),
	}

	// Check database
	if database.DB != nil {
		sqlDB, err := database.DB.DB()
		if err == nil {
			err = sqlDB.PingContext(ctx)
		}
		if err != nil {
			status["checks"].(map[string]string)["database"] = "unhealthy"
			status["status"] = "not ready"
		} else {
			status["checks"].(map[string]string)["database"] = "healthy"
		}
	} else {
		status["checks"].(map[string]string)["database"] = "not connected"
		status["status"] = "not ready"
	}

	// Check Redis
	if cache.RedisClient != nil {
		_, err := cache.RedisClient.Ping(ctx).Result()
		if err != nil {
			status["checks"].(map[string]string)["redis"] = "unhealthy"
			status["status"] = "not ready"
		} else {
			status["checks"].(map[string]string)["redis"] = "healthy"
		}
	} else {
		status["checks"].(map[string]string)["redis"] = "not connected"
		status["status"] = "not ready"
	}

	// Check MinIO
	if storage.MinIOClient != nil {
		_, err := storage.MinIOClient.ListBuckets(ctx)
		if err != nil {
			status["checks"].(map[string]string)["minio"] = "unhealthy"
			status["status"] = "not ready"
		} else {
			status["checks"].(map[string]string)["minio"] = "healthy"
		}
	} else {
		status["checks"].(map[string]string)["minio"] = "not connected"
		status["status"] = "not ready"
	}

	// Determine HTTP status code
	httpStatus := http.StatusOK
	if status["status"] == "not ready" {
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, status)
}

// LivenessCheck checks if the service is alive
func (h *HealthHandler) LivenessCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "alive",
		"timestamp": time.Now().UTC(),
	})
}
