package main

import (
	"context"
	"fmt"
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

func createTempDirectory() (string, error) {
	return createTempDirectoryWithRecovery(slog.Default())
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

func removeTempDirectory(path string) error {
	return removeTempDirectoryWithRecovery(path, slog.Default())
}

func removeTempDirectoryWithRecovery(path string, logger *slog.Logger) error {
	if path == "" {
		return nil
	}
	
	logger.Debug("Removing temp directory", "path", path)
	
	err := os.RemoveAll(path)
	if err != nil {
		logger.Warn("Failed to remove temp directory", "path", path, "error", err)
		return err
	}
	
	logger.Debug("Temp directory removed successfully", "path", path)
	return nil
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

func executeCloneCommand(ctx context.Context, op CloneOperation) error {
	return executeCloneCommandWithRecovery(ctx, op, slog.Default())
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

func setupWorkspace() (string, func(), error) {
	return setupWorkspaceWithRecovery(slog.Default())
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

func executeClonePhase(ctx context.Context, operation CloneOperation) error {
	return executeClonePhaseWithRecovery(ctx, operation, slog.Default())
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
