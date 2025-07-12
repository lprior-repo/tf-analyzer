package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

// ============================================================================
// CLONER - Repository cloning functionality
// ============================================================================

type CloneOperation struct {
	Org       string
	TempDir   string
	Config    Config
	StartTime time.Time
}

func createCloneOperation(org, tempDir string, config Config) CloneOperation {
	return CloneOperation{
		Org:       org,
		TempDir:   tempDir,
		Config:    config,
		StartTime: time.Now(),
	}
}

func buildGhorgCommand(ctx context.Context, op CloneOperation) *exec.Cmd {
	args := []string{"clone", op.Org, "--path", op.TempDir}

	if op.Config.SkipArchived {
		args = append(args, "--skip-archived")
	}
	if op.Config.SkipForks {
		args = append(args, "--skip-forks")
	}
	if op.Config.BaseURL != "" {
		args = append(args, "--base-url", op.Config.BaseURL)
	}

	args = append(args, "--concurrency", fmt.Sprintf("%d", op.Config.CloneConcurrency))
	args = append(args, "--git-filter", "blob:none")

	cmd := exec.CommandContext(ctx, "ghorg", args...)

	if op.Config.GitHubToken != "" {
		cmd.Env = append(os.Environ(), "GHORG_GITHUB_TOKEN="+op.Config.GitHubToken)
	}

	return cmd
}

func expandHomePath(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") {
		return path, nil
	}

	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	return filepath.Join(currentUser.HomeDir, path[2:]), nil
}

func createAbsolutePath(path string) (string, error) {
	expandedPath, err := expandHomePath(path)
	if err != nil {
		return "", err
	}

	if filepath.IsAbs(expandedPath) {
		return expandedPath, nil
	}

	return filepath.Abs(expandedPath)
}


func createTempDirectoryWithRecovery(logger *slog.Logger) (string, error) {
	baseDir := "~/src/gh-repos-clone"

	expandedPath, err := createAbsolutePath(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to expand path %s: %w", baseDir, err)
	}

	logger.Debug("Creating base directory", "path", expandedPath)

	if err := os.MkdirAll(expandedPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create base directory %s: %w", expandedPath, err)
	}

	tempDir := expandedPath

	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory %s: %w", tempDir, err)
	}

	logger.Debug("Temp directory created successfully", "path", tempDir)
	return tempDir, nil
}


func removeTempDirectoryWithRecovery(path string, logger *slog.Logger) error {
	if path == "" {
		return nil
	}
	
	logger.Info("Starting comprehensive cleanup of temp directory", "path", path)
	
	// Force removal with multiple attempts
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		err := os.RemoveAll(path)
		if err == nil {
			logger.Info("Temp directory removed successfully", "path", path, "attempt", attempt)
			return nil
		}
		
		lastErr = err
		logger.Warn("Failed to remove temp directory, retrying", 
			"path", path, 
			"attempt", attempt, 
			"error", err)
		
		// Wait before retry
		time.Sleep(time.Duration(attempt) * time.Second)
		
		// Try to force permissions on retry
		if attempt > 1 {
			if chmodErr := makeDirectoryWritable(path, logger); chmodErr != nil {
				logger.Debug("Failed to make directory writable", "path", path, "error", chmodErr)
			}
		}
	}
	
	logger.Error("Failed to remove temp directory after all attempts", "path", path, "error", lastErr)
	return lastErr
}

func makeDirectoryWritable(path string, logger *slog.Logger) error {
	return filepath.WalkDir(path, func(walkPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Continue walking despite errors
		}
		
		// Make directories and files writable
		if chmodErr := os.Chmod(walkPath, 0755); chmodErr != nil {
			logger.Debug("Failed to chmod path", "path", walkPath, "error", chmodErr)
		}
		
		return nil
	})
}

func findOrgDirectory(tempDir string) (string, error) {
	entries, err := readDirectory(tempDir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(tempDir, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("no org directory found in %s", tempDir)
}


func executeCloneCommandWithRecovery(ctx context.Context, op CloneOperation, logger *slog.Logger) error {
	cmd := buildGhorgCommand(ctx, op)
	
	logger.Debug("Executing ghorg command", 
		"command", cmd.String(),
		"args", cmd.Args)

	// Capture both stdout and stderr for better error reporting
	stdout, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			logger.Error("ghorg command failed", 
				"exit_code", exitError.ExitCode(),
				"stderr", string(exitError.Stderr),
				"stdout", string(stdout))
			return fmt.Errorf("ghorg exited with code %d: %s", exitError.ExitCode(), string(exitError.Stderr))
		}
		logger.Error("ghorg command execution failed", "error", err)
		return fmt.Errorf("failed to execute ghorg: %w", err)
	}

	logger.Debug("ghorg command completed successfully", "stdout", string(stdout))
	return nil
}


func setupWorkspaceWithRecovery(logger *slog.Logger) (string, func(), error) {
	tempDir, err := createTempDirectoryWithRecovery(logger)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	logger.Debug("Workspace setup completed", "temp_dir", tempDir)

	cleanup := func() {
		if cleanupErr := removeTempDirectoryWithRecovery(tempDir, logger); cleanupErr != nil {
			logger.Warn("Failed to cleanup temp directory", "temp_dir", tempDir, "error", cleanupErr)
		}
	}

	return tempDir, cleanup, nil
}


func executeClonePhaseWithRecovery(ctx context.Context, operation CloneOperation, logger *slog.Logger) error {
	logger.Info("Starting clone operation", 
		"organization", operation.Org,
		"temp_dir", operation.TempDir,
		"concurrency", operation.Config.CloneConcurrency)

	cloneErr := executeCloneCommandWithRecovery(ctx, operation, logger)
	if cloneErr != nil {
		return fmt.Errorf("ghorg clone failed: %w", cloneErr)
	}

	logger.Info("Clone operation completed successfully", 
		"organization", operation.Org,
		"duration", time.Since(operation.StartTime))
	return nil
}
