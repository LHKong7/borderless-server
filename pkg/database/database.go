package database

import (
	"fmt"
	"log"

	"borderless_coding_server/config"
	"borderless_coding_server/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDB(cfg *config.Config) error {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort, cfg.DBSSLMode)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Enable UUID extension
	if err := DB.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"").Error; err != nil {
		log.Printf("Warning: Could not create uuid-ossp extension: %v", err)
	}

	// Enable pgcrypto extension for gen_random_uuid()
	if err := DB.Exec("CREATE EXTENSION IF NOT EXISTS pgcrypto").Error; err != nil {
		log.Printf("Warning: Could not create pgcrypto extension: %v", err)
	}

	// Enable citext extension for case-insensitive emails
	if err := DB.Exec("CREATE EXTENSION IF NOT EXISTS citext").Error; err != nil {
		log.Printf("Warning: Could not create citext extension: %v", err)
	}

	// Enable pg_trgm extension for fuzzy search
	if err := DB.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm").Error; err != nil {
		log.Printf("Warning: Could not create pg_trgm extension: %v", err)
	}

	// Ensure required custom enum types exist (used by models)
	if err := DB.Exec(`DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'project_visibility') THEN
        CREATE TYPE project_visibility AS ENUM ('private','unlisted','public');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'chat_sender') THEN
        CREATE TYPE chat_sender AS ENUM ('user','assistant','system','tool');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'auth_provider') THEN
        CREATE TYPE auth_provider AS ENUM ('password','google');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'token_purpose') THEN
        CREATE TYPE token_purpose AS ENUM ('email_verify','email_login','phone_verify','phone_login','reset_password');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_level') THEN
        CREATE TYPE user_level AS ENUM ('free','entry','senior','staff','principal');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'storage_type') THEN
        CREATE TYPE storage_type AS ENUM ('git_remote','local_fs','network_fs');
    END IF;
END$$;`).Error; err != nil {
		log.Printf("Warning: Could not ensure custom enums: %v", err)
	}

	log.Println("Database connected successfully")
	return nil
}

func AutoMigrate() error {
	if DB == nil {
		return fmt.Errorf("database connection not initialized")
	}

	// Auto-migrate all models
	err := DB.AutoMigrate(
		&models.User{},
		&models.UserPhone{},
		&models.AuthIdentity{},
		&models.UserCredential{},
		&models.VerificationToken{},
		&models.Session{},
		&models.AuthAudit{},
		&models.Project{},
		&models.ChatSession{},
		&models.ChatMessage{},
		&models.Build{},
		&models.BuildLog{},
		&models.StorageLocation{},
		&models.BuildResult{},
	)

	if err != nil {
		return fmt.Errorf("failed to auto-migrate: %w", err)
	}

	log.Println("Database migration completed successfully")
	return nil
}

func CloseDB() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}
