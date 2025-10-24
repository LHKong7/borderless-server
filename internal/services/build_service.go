package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"borderless_coding_server/internal/models"
	"borderless_coding_server/pkg/database"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type BuildService struct {
	claudeCLIPath string
	logger        *logrus.Logger
	activeBuilds  map[uuid.UUID]*exec.Cmd
	buildMutex    sync.RWMutex
}

// ClaudeCLIOptions represents options for Claude CLI
type ClaudeCLIOptions struct {
	SessionID      string         `json:"session_id,omitempty"`
	ProjectPath    string         `json:"project_path,omitempty"`
	CWD            string         `json:"cwd,omitempty"`
	Resume         bool           `json:"resume,omitempty"`
	ToolsSettings  *ToolsSettings `json:"tools_settings,omitempty"`
	PermissionMode string         `json:"permission_mode,omitempty"`
	Images         []ImageData    `json:"images,omitempty"`
	Model          string         `json:"model,omitempty"`
}

// ToolsSettings represents tool permission settings
type ToolsSettings struct {
	AllowedTools    []string `json:"allowed_tools,omitempty"`
	DisallowedTools []string `json:"disallowed_tools,omitempty"`
	SkipPermissions bool     `json:"skip_permissions,omitempty"`
}

// ImageData represents image data for Claude CLI
type ImageData struct {
	Data     string `json:"data"` // base64 encoded image data
	MimeType string `json:"mime_type,omitempty"`
}

func NewBuildService(claudeCLIPath string, logger *logrus.Logger) *BuildService {
	return &BuildService{
		claudeCLIPath: claudeCLIPath,
		logger:        logger,
		activeBuilds:  make(map[uuid.UUID]*exec.Cmd),
	}
}

// StartBuild starts a new build process
func (s *BuildService) StartBuild(ctx context.Context, userID, projectID uuid.UUID, sessionID *uuid.UUID, command string) (*models.Build, error) {
	// Create working directory
	workingDir := s.getWorkingDirectory(userID, projectID, sessionID)
	if err := os.MkdirAll(workingDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create working directory: %w", err)
	}

	// Create build record
	build := &models.Build{
		UserID:     userID,
		ProjectID:  projectID,
		SessionID:  sessionID,
		Command:    command,
		WorkingDir: workingDir,
		Status:     models.BuildStatusPending,
		Metadata:   make(models.JSONB),
	}

	if err := database.DB.Create(build).Error; err != nil {
		return nil, fmt.Errorf("failed to create build record: %w", err)
	}

	// Start build process in background
	go s.executeBuild(ctx, build)

	return build, nil
}

// executeBuild executes the build command
func (s *BuildService) executeBuild(ctx context.Context, build *models.Build) {
	// Update status to running
	now := time.Now()
	build.Status = models.BuildStatusRunning
	build.StartedAt = &now
	database.DB.Save(build)

	// Log build start
	s.logBuildEvent(build.ID, "info", "Build started", map[string]interface{}{
		"command":     build.Command,
		"working_dir": build.WorkingDir,
	})

	// Parse command and arguments
	parts := strings.Fields(build.Command)
	if len(parts) == 0 {
		s.failBuild(build, "Empty command")
		return
	}

	cmdName := parts[0]
	cmdArgs := parts[1:]

	// Create command
	cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
	cmd.Dir = build.WorkingDir

	// Set environment variables
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("USER_ID=%s", build.UserID.String()),
		fmt.Sprintf("PROJECT_ID=%s", build.ProjectID.String()),
		fmt.Sprintf("WORKING_DIR=%s", build.WorkingDir),
	)

	// Store active build
	s.buildMutex.Lock()
	s.activeBuilds[build.ID] = cmd
	s.buildMutex.Unlock()

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.failBuild(build, fmt.Sprintf("Failed to create stdout pipe: %v", err))
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.failBuild(build, fmt.Sprintf("Failed to create stderr pipe: %v", err))
		return
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		s.failBuild(build, fmt.Sprintf("Failed to start command: %v", err))
		return
	}

	// Store process ID
	processID := cmd.Process.Pid
	build.ProcessID = &processID
	database.DB.Save(build)

	s.logBuildEvent(build.ID, "info", fmt.Sprintf("Process started with PID: %d", processID), nil)

	// Stream output
	var outputBuffer bytes.Buffer
	var wg sync.WaitGroup

	// Stream stdout
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			outputBuffer.WriteString(line + "\n")
			s.logBuildEvent(build.ID, "info", line, nil)
		}
	}()

	// Stream stderr
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			outputBuffer.WriteString(line + "\n")
			s.logBuildEvent(build.ID, "error", line, nil)
		}
	}()

	// Wait for command to complete
	wg.Wait()
	err = cmd.Wait()

	// Remove from active builds
	s.buildMutex.Lock()
	delete(s.activeBuilds, build.ID)
	s.buildMutex.Unlock()

	// Update build status
	completedAt := time.Now()
	build.CompletedAt = &completedAt
	build.Output = outputBuffer.String()

	if err != nil {
		// Command failed
		exitCode := 1
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
		build.Status = models.BuildStatusFailed
		build.ExitCode = &exitCode
		errorMsg := err.Error()
		build.Error = &errorMsg

		s.logBuildEvent(build.ID, "error", fmt.Sprintf("Build failed with exit code %d: %v", exitCode, err), nil)
	} else {
		// Command succeeded
		build.Status = models.BuildStatusCompleted
		exitCode := 0
		build.ExitCode = &exitCode

		s.logBuildEvent(build.ID, "info", "Build completed successfully", nil)
	}

	database.DB.Save(build)
}

// failBuild marks a build as failed
func (s *BuildService) failBuild(build *models.Build, errorMsg string) {
	now := time.Now()
	build.Status = models.BuildStatusFailed
	build.CompletedAt = &now
	build.Error = &errorMsg
	exitCode := 1
	build.ExitCode = &exitCode

	database.DB.Save(build)
	s.logBuildEvent(build.ID, "error", errorMsg, nil)
}

// CancelBuild cancels a running build
func (s *BuildService) CancelBuild(buildID uuid.UUID) error {
	s.buildMutex.Lock()
	cmd, exists := s.activeBuilds[buildID]
	s.buildMutex.Unlock()

	if !exists {
		return fmt.Errorf("build not found or not running")
	}

	// Kill the process
	if err := cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}

	// Update build status
	var build models.Build
	if err := database.DB.Where("id = ?", buildID).First(&build).Error; err != nil {
		return err
	}

	now := time.Now()
	build.Status = models.BuildStatusCancelled
	build.CompletedAt = &now
	database.DB.Save(&build)

	s.logBuildEvent(buildID, "warn", "Build cancelled by user", nil)

	return nil
}

// GetBuild retrieves a build by ID
func (s *BuildService) GetBuild(buildID uuid.UUID) (*models.Build, error) {
	var build models.Build
	err := database.DB.Where("id = ?", buildID).First(&build).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("build not found")
		}
		return nil, err
	}
	return &build, nil
}

// GetUserBuilds retrieves builds for a user
func (s *BuildService) GetUserBuilds(userID uuid.UUID, limit int) ([]models.Build, error) {
	var builds []models.Build
	err := database.DB.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&builds).Error
	return builds, err
}

// GetProjectBuilds retrieves builds for a project
func (s *BuildService) GetProjectBuilds(projectID uuid.UUID, limit int) ([]models.Build, error) {
	var builds []models.Build
	err := database.DB.Where("project_id = ?", projectID).
		Order("created_at DESC").
		Limit(limit).
		Find(&builds).Error
	return builds, err
}

// GetBuildLogs retrieves logs for a build
func (s *BuildService) GetBuildLogs(buildID uuid.UUID, limit int) ([]models.BuildLog, error) {
	var logs []models.BuildLog
	err := database.DB.Where("build_id = ?", buildID).
		Order("timestamp ASC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// logBuildEvent logs a build event
func (s *BuildService) logBuildEvent(buildID uuid.UUID, level, message string, metadata map[string]interface{}) {
	log := models.BuildLog{
		BuildID:  buildID,
		Level:    level,
		Message:  message,
		Metadata: models.JSONB(metadata),
	}

	if err := database.DB.Create(&log).Error; err != nil {
		s.logger.WithError(err).Error("Failed to create build log")
	}
}

// GetWorkingDirectory returns the working directory for a build (public method for handlers)
func (s *BuildService) GetWorkingDirectory(userID, projectID uuid.UUID, sessionID *uuid.UUID) string {
	return s.getWorkingDirectory(userID, projectID, sessionID)
}

// getWorkingDirectory returns the working directory for a build
func (s *BuildService) getWorkingDirectory(userID, projectID uuid.UUID, sessionID *uuid.UUID) string {
	baseDir := filepath.Join("/tmp", "borderless-coding", "builds", userID.String(), projectID.String())
	if sessionID != nil {
		baseDir = filepath.Join(baseDir, sessionID.String())
	}
	return baseDir
}

// StartBuildWithClaudeCLI starts a build using Claude CLI to process user input
func (s *BuildService) StartBuildWithClaudeCLI(ctx context.Context, userID, projectID uuid.UUID, sessionID *uuid.UUID, userInput string, options *ClaudeCLIOptions) (*models.Build, error) {
	// Create working directory
	workingDir := s.getWorkingDirectory(userID, projectID, sessionID)
	if err := os.MkdirAll(workingDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create working directory: %w", err)
	}

	// Create build record
	build := &models.Build{
		UserID:     userID,
		ProjectID:  projectID,
		SessionID:  sessionID,
		Command:    userInput, // Store the user input as the command
		WorkingDir: workingDir,
		Status:     models.BuildStatusPending,
		Metadata:   make(models.JSONB),
	}

	if err := database.DB.Create(build).Error; err != nil {
		return nil, fmt.Errorf("failed to create build record: %w", err)
	}

	// Start Claude CLI process in background
	go s.executeClaudeCLI(ctx, build, options)

	return build, nil
}

// executeClaudeCLI executes the Claude CLI command
func (s *BuildService) executeClaudeCLI(ctx context.Context, build *models.Build, options *ClaudeCLIOptions) {
	// Update status to running
	now := time.Now()
	build.Status = models.BuildStatusRunning
	build.StartedAt = &now
	database.DB.Save(build)

	// Log build start
	s.logBuildEvent(build.ID, "info", "Claude CLI build started", map[string]interface{}{
		"user_input":  build.Command,
		"working_dir": build.WorkingDir,
		"session_id":  options.SessionID,
		"resume":      options.Resume,
	})

	// Build Claude CLI arguments
	args := s.buildClaudeCLIArgs(build.Command, options)

	// Use Claude CLI path from config or default to 'claude'
	claudePath := s.claudeCLIPath
	if claudePath == "" {
		claudePath = "claude"
	}

	s.logger.WithFields(logrus.Fields{
		"claude_path": claudePath,
		"args":        args,
		"working_dir": build.WorkingDir,
	}).Info("Spawning Claude CLI")

	// Create command
	cmd := exec.CommandContext(ctx, claudePath, args...)
	cmd.Dir = build.WorkingDir

	// Set environment variables
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("USER_ID=%s", build.UserID.String()),
		fmt.Sprintf("PROJECT_ID=%s", build.ProjectID.String()),
		fmt.Sprintf("WORKING_DIR=%s", build.WorkingDir),
	)

	// Store active build
	s.buildMutex.Lock()
	s.activeBuilds[build.ID] = cmd
	s.buildMutex.Unlock()

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.failBuild(build, fmt.Sprintf("Failed to create stdout pipe: %v", err))
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.failBuild(build, fmt.Sprintf("Failed to create stderr pipe: %v", err))
		return
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		s.failBuild(build, fmt.Sprintf("Failed to start Claude CLI: %v", err))
		return
	}

	// Store process ID
	processID := cmd.Process.Pid
	build.ProcessID = &processID
	database.DB.Save(build)

	s.logBuildEvent(build.ID, "info", fmt.Sprintf("Claude CLI process started with PID: %d", processID), nil)

	// Stream output
	var outputBuffer bytes.Buffer
	var wg sync.WaitGroup

	// Stream stdout (JSON responses from Claude CLI)
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			outputBuffer.WriteString(line + "\n")

			// Try to parse as JSON
			var response map[string]interface{}
			if err := json.Unmarshal([]byte(line), &response); err == nil {
				// Valid JSON response from Claude CLI
				s.logBuildEvent(build.ID, "info", fmt.Sprintf("Claude response: %s", line), response)

				// Check for session ID in response
				if sessionID, ok := response["session_id"].(string); ok {
					s.logBuildEvent(build.ID, "info", fmt.Sprintf("Session ID captured: %s", sessionID), nil)
				}
			} else {
				// Non-JSON output
				s.logBuildEvent(build.ID, "info", line, nil)
			}
		}
	}()

	// Stream stderr
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			outputBuffer.WriteString(line + "\n")
			s.logBuildEvent(build.ID, "error", line, nil)
		}
	}()

	// Wait for command to complete
	wg.Wait()
	err = cmd.Wait()

	// Remove from active builds
	s.buildMutex.Lock()
	delete(s.activeBuilds, build.ID)
	s.buildMutex.Unlock()

	// Update build status
	completedAt := time.Now()
	build.CompletedAt = &completedAt
	build.Output = outputBuffer.String()

	if err != nil {
		// Command failed
		exitCode := 1
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
		build.Status = models.BuildStatusFailed
		build.ExitCode = &exitCode
		errorMsg := err.Error()
		build.Error = &errorMsg

		s.logBuildEvent(build.ID, "error", fmt.Sprintf("Claude CLI build failed with exit code %d: %v", exitCode, err), nil)
	} else {
		// Command succeeded
		build.Status = models.BuildStatusCompleted
		exitCode := 0
		build.ExitCode = &exitCode

		s.logBuildEvent(build.ID, "info", "Claude CLI build completed successfully", nil)
	}

	database.DB.Save(build)
}

// buildClaudeCLIArgs builds the command line arguments for Claude CLI
func (s *BuildService) buildClaudeCLIArgs(command string, options *ClaudeCLIOptions) []string {
	args := []string{}

	// Add resume flag if resuming
	if options.Resume && options.SessionID != "" {
		args = append(args, "--resume", options.SessionID)
	}

	// Add basic flags
	args = append(args, "--output-format", "stream-json", "--verbose")

	// Add MCP config flag if MCP servers are configured
	if s.hasMCPServers() {
		configPath := s.getMCPConfigPath()
		if configPath != "" {
			args = append(args, "--mcp-config", configPath)
			s.logger.Info("Added MCP config:", configPath)
		}
	}

	// Add model for new sessions
	if !options.Resume {
		model := options.Model
		if model == "" {
			model = "sonnet"
		}
		args = append(args, "--model", model)
	}

	// Add permission mode if specified
	if options.PermissionMode != "" && options.PermissionMode != "default" {
		args = append(args, "--permission-mode", options.PermissionMode)
		s.logger.Info("Using permission mode:", options.PermissionMode)
	}

	// Add tools settings flags
	if options.ToolsSettings != nil {
		settings := options.ToolsSettings

		// Don't use --dangerously-skip-permissions when in plan mode
		if settings.SkipPermissions && options.PermissionMode != "plan" {
			args = append(args, "--dangerously-skip-permissions")
			s.logger.Info("Using --dangerously-skip-permissions")
		} else {
			// Add allowed tools
			if len(settings.AllowedTools) > 0 {
				for _, tool := range settings.AllowedTools {
					args = append(args, "--allowedTools", tool)
				}
			}

			// Add disallowed tools
			if len(settings.DisallowedTools) > 0 {
				for _, tool := range settings.DisallowedTools {
					args = append(args, "--disallowedTools", tool)
				}
			}

			// Add plan mode specific tools
			if options.PermissionMode == "plan" {
				planModeTools := []string{"Read", "Task", "exit_plan_mode", "TodoRead", "TodoWrite"}
				for _, tool := range planModeTools {
					args = append(args, "--allowedTools", tool)
				}
			}
		}
	}

	// Add print flag with command if we have a command
	if command != "" {
		args = append(args, "--print", "--", command)
	}

	return args
}

// hasMCPServers checks if MCP servers are configured
func (s *BuildService) hasMCPServers() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	configPath := filepath.Join(homeDir, ".claude.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return false
	}

	// Read and parse config to check for MCP servers
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}

	var config map[string]interface{}
	if err := json.Unmarshal(configData, &config); err != nil {
		return false
	}

	// Check global MCP servers
	if mcpServers, ok := config["mcpServers"].(map[string]interface{}); ok && len(mcpServers) > 0 {
		return true
	}

	// Check project-specific MCP servers
	if claudeProjects, ok := config["claudeProjects"].(map[string]interface{}); ok {
		for _, project := range claudeProjects {
			if projectMap, ok := project.(map[string]interface{}); ok {
				if mcpServers, ok := projectMap["mcpServers"].(map[string]interface{}); ok && len(mcpServers) > 0 {
					return true
				}
			}
		}
	}

	return false
}

// getMCPConfigPath returns the path to the MCP config file
func (s *BuildService) getMCPConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	configPath := filepath.Join(homeDir, ".claude.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return ""
	}

	return configPath
}
