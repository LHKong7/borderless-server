package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	Port    string
	GinMode string

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// Redis
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	// MinIO
	MinIOEndpoint   string
	MinIOAccessKey  string
	MinIOSecretKey  string
	MinIOUseSSL     bool
	MinIOBucketName string

	// JWT
	JWTSecret     string
	JWTExpiration string

	// Google OAuth
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	// Claude CLI Configuration
	ClaudeCLIPath string

	// Password Encryption (client→server RSA)
	PasswordEncPublicKey  string
	PasswordEncPrivateKey string

	// local storage
	LocalStoragePath string

	// Project template zip for initializing new projects
	ProjectTemplateZip string
}

func LoadConfig() *Config {
	env := os.Getenv("BORDERLESS_CODING_SERVER_ENV")

	// Load .env file if it exists
	if err := godotenv.Load(".env." + env); err != nil {
		log.Println("No .env." + env + " file found, using environment variables")
	}

	config := &Config{
		// Server
		Port:    getEnv("PORT", "8080"),
		GinMode: getEnv("GIN_MODE", "debug"),

		// Database
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "password"),
		DBName:     getEnv("DB_NAME", "borderless_coding"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		// Redis
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvAsInt("REDIS_DB", 0),

		// MinIO
		MinIOEndpoint:   getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey:  getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey:  getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinIOUseSSL:     getEnvAsBool("MINIO_USE_SSL", false),
		MinIOBucketName: getEnv("MINIO_BUCKET_NAME", "borderless-coding"),

		// JWT
		JWTSecret:     getEnv("JWT_SECRET", "your-secret-key-here"),
		JWTExpiration: getEnv("JWT_EXPIRATION", "24h"),

		// Google OAuth
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/auth/google/callback"),

		// Claude CLI Configuration
		ClaudeCLIPath: getEnv("CLAUDE_CLI_PATH", "claude"),

		// Password Encryption (client→server RSA)
		PasswordEncPublicKey:  loadPublicKey(),
		PasswordEncPrivateKey: loadPrivateKey(),

		// local storage
		LocalStoragePath:   getEnv("LOCAL_STORAGE_PATH", "workspaces"),
		ProjectTemplateZip: getEnv("PROJECT_TEMPLATE_ZIP", "project_template.zip"),
	}

	return config
}

func (c Config) String() string {
	redact := func(s string) string {
		if s == "" {
			return ""
		}
		if len(s) <= 4 {
			return "****"
		}
		return "****" + s[len(s)-4:]
	}

	return fmt.Sprintf(
		"Config:\n"+
			"  Server:\n"+
			"    Port: %s\n"+
			"    GinMode: %s\n"+
			"  Database:\n"+
			"    Host: %s\n"+
			"    Port: %s\n"+
			"    User: %s\n"+
			"    Password: %s\n"+
			"    Name: %s\n"+
			"    SSLMode: %s\n"+
			"  Redis:\n"+
			"    Host: %s\n"+
			"    Port: %s\n"+
			"    Password: %s\n"+
			"    DB: %d\n"+
			"  MinIO:\n"+
			"    Endpoint: %s\n"+
			"    AccessKey: %s\n"+
			"    SecretKey: %s\n"+
			"    UseSSL: %t\n"+
			"    BucketName: %s\n"+
			"  JWT:\n"+
			"    Secret: %s\n"+
			"    Expiration: %s\n"+
			"  Google:\n"+
			"    ClientID: %s\n"+
			"    ClientSecret: %s\n"+
			"    RedirectURL: %s\n"+
			"  ClaudeCLI:\n"+
			"    Path: %s\n"+
			"  LocalStoragePath:%s",
		c.Port, c.GinMode,
		c.DBHost, c.DBPort, c.DBUser, redact(c.DBPassword), c.DBName, c.DBSSLMode,
		c.RedisHost, c.RedisPort, redact(c.RedisPassword), c.RedisDB,
		c.MinIOEndpoint, redact(c.MinIOAccessKey), redact(c.MinIOSecretKey), c.MinIOUseSSL, c.MinIOBucketName,
		redact(c.JWTSecret), c.JWTExpiration,
		c.GoogleClientID, redact(c.GoogleClientSecret), c.GoogleRedirectURL,
		c.ClaudeCLIPath, c.LocalStoragePath,
	)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// loadPrivateKey loads PASSWORD_ENC_PRIVATE_KEY from env, or reads from keypairs/private-key.pem if env is empty.
func loadPrivateKey() string {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		return ""
	}

	if v := os.Getenv("PASSWORD_ENC_PRIVATE_KEY"); v != "" {
		return normalizePEM(v)
	}
	// fallback file path
	path := filepath.Join(cwd, "keypairs", "private-key.pem")
	fmt.Println("path is ...", path)
	if b, err := os.ReadFile(path); err == nil {
		return string(b)
	}
	return ""
}

// loadPrivateKey loads PASSWORD_ENC_PRIVATE_KEY from env, or reads from keypairs/private-key.pem if env is empty.
func loadPublicKey() string {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		return ""
	}

	if v := os.Getenv("PASSWORD_ENC_PUBLIC_KEY"); v != "" {
		return normalizePEM(v)
	}
	// fallback file path
	path := filepath.Join(cwd, "keypairs", "public-key.pem")
	fmt.Println("path is ...", path)
	if b, err := os.ReadFile(path); err == nil {
		return string(b)
	}
	return ""
}

// normalize PEM if provided as single-line with escaped newlines
func normalizePEM(s string) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, "\\r\\n", "\n")
	s = strings.ReplaceAll(s, "\\n", "\n")
	return s
}
