package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
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

func executeCloneCommandWithRealTimeProgress(ctx context.Context, op CloneOperation, logger *slog.Logger, tuiProgress *TUIProgressChannel) error {
	cmd := buildGhorgCommand(ctx, op)
	
	logger.Debug("Executing ghorg command with real-time progress", 
		"command", cmd.String(),
		"args", cmd.Args)

	return executeCommandWithProgressTracking(ctx, cmd, op, logger, tuiProgress)
}

func executeCloneCommandWithProgress(ctx context.Context, op CloneOperation, logger *slog.Logger, tuiProgress *TUIProgressChannel) error {
	if tuiProgress != nil {
		tuiProgress.UpdateProgressWithPhase(op.Org, op.Org, "Starting clone operation...", 0, 1, 0, 0)
	}

	err := executeCloneCommandWithRealTimeProgress(ctx, op, logger, tuiProgress)
	
	if tuiProgress != nil {
		if err != nil {
			tuiProgress.UpdateProgressWithPhase(op.Org, op.Org, "❌ Clone operation failed", 0, 1, 0, 0)
		} else {
			tuiProgress.UpdateProgressWithPhase(op.Org, op.Org, "✅ Clone operation completed", 1, 1, 0, 0)
		}
	}

	return err
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

func executeClonePhaseWithProgress(ctx context.Context, operation CloneOperation, logger *slog.Logger, tuiProgress *TUIProgressChannel) error {
	logger.Info("Starting clone operation", 
		"organization", operation.Org,
		"temp_dir", operation.TempDir,
		"concurrency", operation.Config.CloneConcurrency)

	cloneErr := executeCloneCommandWithProgress(ctx, operation, logger, tuiProgress)
	if cloneErr != nil {
		return fmt.Errorf("ghorg clone failed: %w", cloneErr)
	}

	logger.Info("Clone operation completed successfully", 
		"organization", operation.Org,
		"duration", time.Since(operation.StartTime))
	return nil
}

func executeCommandWithProgressTracking(ctx context.Context, cmd *exec.Cmd, op CloneOperation, logger *slog.Logger, tuiProgress *TUIProgressChannel) error {
	// Check context before starting
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled before starting command: %w", ctx.Err())
	default:
	}
	
	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ghorg command: %w", err)
	}

	// Track progress from both stdout and stderr
	progressTracker := createProgressTracker(op, logger, tuiProgress)
	
	// Use go routines to read from both pipes concurrently
	done := make(chan error, 2)
	
	go func() {
		done <- progressTracker.trackOutputWithContext(ctx, stdout, "stdout")
	}()
	
	go func() {
		done <- progressTracker.trackOutputWithContext(ctx, stderr, "stderr") 
	}()

	// Wait for both goroutines to finish or context cancellation
	waitDone := make(chan struct{})
	go func() {
		defer close(waitDone)
		for i := 0; i < 2; i++ {
			select {
			case err := <-done:
				if err != nil {
					logger.Debug("Output tracking completed", "error", err)
				}
			case <-ctx.Done():
				logger.Debug("Context cancelled during output tracking")
				return
			}
		}
	}()

	// Wait for command to complete or context cancellation
	cmdDone := make(chan error, 1)
	go func() {
		cmdDone <- cmd.Wait()
	}()

	select {
	case err := <-cmdDone:
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				logger.Error("ghorg command failed", 
					"exit_code", exitError.ExitCode(),
					"organization", op.Org)
				return fmt.Errorf("ghorg exited with code %d", exitError.ExitCode())
			}
			return fmt.Errorf("ghorg command failed: %w", err)
		}
	case <-ctx.Done():
		// Context cancelled - kill the process
		if killErr := cmd.Process.Kill(); killErr != nil {
			logger.Warn("Failed to kill process after context cancellation", "error", killErr)
		}
		return fmt.Errorf("command cancelled: %w", ctx.Err())
	}

	// Ensure output tracking completes
	select {
	case <-waitDone:
	case <-time.After(1 * time.Second):
		logger.Debug("Output tracking timeout")
	}

	return nil
}

type ProgressTracker struct {
	operation   CloneOperation
	logger      *slog.Logger
	tuiProgress *TUIProgressChannel
	repoRegex   *regexp.Regexp
	totalRepos  int
	clonedRepos int
}

func createProgressTracker(op CloneOperation, logger *slog.Logger, tuiProgress *TUIProgressChannel) *ProgressTracker {
	// Regex patterns to match ghorg output
	repoRegex := regexp.MustCompile(`(?i)(?:cloning|cloned|processing)\s+(?:repository\s+)?([^/\s]+/[^/\s]+)`)
	
	return &ProgressTracker{
		operation:   op,
		logger:      logger,
		tuiProgress: tuiProgress,
		repoRegex:   repoRegex,
	}
}

func (pt *ProgressTracker) trackOutputWithContext(ctx context.Context, reader io.Reader, source string) error {
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)
	
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		
		pt.logger.Debug("ghorg output", "source", source, "line", line)
		pt.parseProgressLine(line)
	}
	
	return scanner.Err()
}

func (pt *ProgressTracker) parseProgressLine(line string) {
	line = strings.ToLower(line)
	
	// Update phase based on ghorg output patterns
	if strings.Contains(line, "fetching") && strings.Contains(line, "repositories") {
		pt.updateProgress("Fetching repository list...", 0, 1)
	} else if strings.Contains(line, "total") && strings.Contains(line, "repositories") {
		pt.extractTotalRepos(line)
	} else if strings.Contains(line, "cloning") || strings.Contains(line, "cloned") {
		repoName := pt.extractRepoName(line)
		if repoName != "" {
			pt.clonedRepos++
			phase := fmt.Sprintf("Cloning: %s", repoName)
			pt.updateProgress(phase, pt.clonedRepos, pt.totalRepos)
			pt.logger.Debug("Repository cloned", "repo", repoName, "progress", fmt.Sprintf("%d/%d", pt.clonedRepos, pt.totalRepos))
		}
	} else if strings.Contains(line, "authentication") {
		pt.updateProgress("Authenticating with GitHub...", 0, 1)
	} else if strings.Contains(line, "complete") || strings.Contains(line, "finished") {
		pt.updateProgress("Clone operation completed", pt.totalRepos, pt.totalRepos)
	}
}

func (pt *ProgressTracker) extractTotalRepos(line string) {
	// Look for patterns like "found 25 repositories" or "total: 25"
	re := regexp.MustCompile(`(?:found|total)[\s:]*(\d+)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		if total, err := strconv.Atoi(matches[1]); err == nil {
			pt.totalRepos = total
			pt.updateProgress("Found repositories", 0, total)
			pt.logger.Info("Total repositories discovered", "count", total, "organization", pt.operation.Org)
		}
	}
}

func (pt *ProgressTracker) extractRepoName(line string) string {
	matches := pt.repoRegex.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (pt *ProgressTracker) updateProgress(phase string, completed, total int) {
	if pt.tuiProgress != nil {
		pt.tuiProgress.UpdateProgressWithPhase(
			pt.operation.Org, 
			pt.operation.Org, 
			phase, 
			completed, 
			total, 
			completed, 
			total)
	}
}
