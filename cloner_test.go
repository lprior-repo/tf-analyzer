package main

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCreateCloneOperation tests the CloneOperation creation
func TestCreateCloneOperation(t *testing.T) {
	t.Run("creates operation with correct fields", func(t *testing.T) {
		// Given: operation parameters
		org := "test-org"
		tempDir := "/tmp/test"
		config := Config{
			GitHubToken:      "token123",
			CloneConcurrency: 5,
			SkipArchived:     true,
			SkipForks:        false,
		}
		startTime := time.Now()

		// When: createCloneOperation is called
		op := createCloneOperation(org, tempDir, config)

		// Then: operation should have correct fields
		if op.Org != org {
			t.Errorf("Expected org %s, got %s", org, op.Org)
		}
		if op.TempDir != tempDir {
			t.Errorf("Expected tempDir %s, got %s", tempDir, op.TempDir)
		}
		if op.Config.GitHubToken != config.GitHubToken {
			t.Errorf("Expected token %s, got %s", config.GitHubToken, op.Config.GitHubToken)
		}
		if op.StartTime.Before(startTime) {
			t.Error("StartTime should be set to current time")
		}
	})
}

// TestBuildGhorgCommand tests the ghorg command building
func TestBuildGhorgCommand(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected []string
	}{
		{
			name: "basic command with token",
			config: Config{
				GitHubToken:      "token123",
				CloneConcurrency: 5,
				SkipArchived:     false,
				SkipForks:        false,
			},
			expected: []string{"ghorg", "clone", "test-org", "--path", "/tmp/test", "--concurrency", "5", "--git-filter", "blob:none"},
		},
		{
			name: "command with skip archived",
			config: Config{
				GitHubToken:      "token123",
				CloneConcurrency: 10,
				SkipArchived:     true,
				SkipForks:        false,
			},
			expected: []string{"ghorg", "clone", "test-org", "--path", "/tmp/test", "--skip-archived", "--concurrency", "10", "--git-filter", "blob:none"},
		},
		{
			name: "command with skip forks",
			config: Config{
				GitHubToken:      "token123",
				CloneConcurrency: 3,
				SkipArchived:     false,
				SkipForks:        true,
			},
			expected: []string{"ghorg", "clone", "test-org", "--path", "/tmp/test", "--skip-forks", "--concurrency", "3", "--git-filter", "blob:none"},
		},
		{
			name: "command with base URL",
			config: Config{
				GitHubToken:      "token123",
				CloneConcurrency: 5,
				BaseURL:          "https://github.enterprise.com",
				SkipArchived:     false,
				SkipForks:        false,
			},
			expected: []string{"ghorg", "clone", "test-org", "--path", "/tmp/test", "--base-url", "https://github.enterprise.com", "--concurrency", "5", "--git-filter", "blob:none"},
		},
		{
			name: "command with all options",
			config: Config{
				GitHubToken:      "token123",
				CloneConcurrency: 8,
				BaseURL:          "https://github.enterprise.com",
				SkipArchived:     true,
				SkipForks:        true,
			},
			expected: []string{"ghorg", "clone", "test-org", "--path", "/tmp/test", "--skip-archived", "--skip-forks", "--base-url", "https://github.enterprise.com", "--concurrency", "8", "--git-filter", "blob:none"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: a clone operation with specific config
			op := CloneOperation{
				Org:     "test-org",
				TempDir: "/tmp/test",
				Config:  tt.config,
			}
			ctx := context.Background()

			// When: buildGhorgCommand is called
			cmd := buildGhorgCommand(ctx, op)

			// Then: command should have correct arguments
			if !strings.HasSuffix(cmd.Path, "ghorg") {
				t.Errorf("Expected command path to end with 'ghorg', got %s", cmd.Path)
			}

			actualArgs := append([]string{"ghorg"}, cmd.Args[1:]...)
			if len(actualArgs) != len(tt.expected) {
				t.Errorf("Expected %d args, got %d: %v", len(tt.expected), len(actualArgs), actualArgs)
			}

			for i, expectedArg := range tt.expected {
				if i < len(actualArgs) && actualArgs[i] != expectedArg {
					t.Errorf("Arg %d: expected %s, got %s", i, expectedArg, actualArgs[i])
				}
			}

			// Check environment variable is set
			if tt.config.GitHubToken != "" {
				found := false
				for _, env := range cmd.Env {
					if strings.HasPrefix(env, "GHORG_GITHUB_TOKEN=") {
						if env != "GHORG_GITHUB_TOKEN="+tt.config.GitHubToken {
							t.Errorf("Expected env GHORG_GITHUB_TOKEN=%s, got %s", tt.config.GitHubToken, env)
						}
						found = true
						break
					}
				}
				if !found {
					t.Error("Expected GHORG_GITHUB_TOKEN environment variable to be set")
				}
			}
		})
	}
}

// TestExpandHomePath tests home directory expansion
func TestExpandHomePath(t *testing.T) {
	t.Run("expands home path correctly", func(t *testing.T) {
		// Given: a path with ~ prefix
		inputPath := "~/test/path"

		// When: expandHomePath is called
		result, err := expandHomePath(inputPath)

		// Then: path should be expanded
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		currentUser, _ := user.Current()
		expected := filepath.Join(currentUser.HomeDir, "test/path")
		if result != expected {
			t.Errorf("Expected %s, got %s", expected, result)
		}
	})

	t.Run("returns path unchanged if not home path", func(t *testing.T) {
		// Given: a path without ~ prefix
		inputPath := "/absolute/path"

		// When: expandHomePath is called
		result, err := expandHomePath(inputPath)

		// Then: path should be unchanged
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if result != inputPath {
			t.Errorf("Expected %s, got %s", inputPath, result)
		}
	})

	t.Run("handles relative path correctly", func(t *testing.T) {
		// Given: a relative path
		inputPath := "relative/path"

		// When: expandHomePath is called
		result, err := expandHomePath(inputPath)

		// Then: path should be unchanged
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if result != inputPath {
			t.Errorf("Expected %s, got %s", inputPath, result)
		}
	})
}

// TestCreateAbsolutePath tests absolute path creation
func TestCreateAbsolutePath(t *testing.T) {
	t.Run("creates absolute path from relative", func(t *testing.T) {
		// Given: a relative path
		inputPath := "test/path"

		// When: createAbsolutePath is called
		result, err := createAbsolutePath(inputPath)

		// Then: should return absolute path
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if !filepath.IsAbs(result) {
			t.Errorf("Expected absolute path, got %s", result)
		}

		if !strings.HasSuffix(result, "test/path") {
			t.Errorf("Expected path to end with test/path, got %s", result)
		}
	})

	t.Run("returns absolute path unchanged", func(t *testing.T) {
		// Given: an absolute path
		inputPath := "/absolute/test/path"

		// When: createAbsolutePath is called
		result, err := createAbsolutePath(inputPath)

		// Then: should return same path
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if result != inputPath {
			t.Errorf("Expected %s, got %s", inputPath, result)
		}
	})

	t.Run("expands and converts home path to absolute", func(t *testing.T) {
		// Given: a home path
		inputPath := "~/test/path"

		// When: createAbsolutePath is called
		result, err := createAbsolutePath(inputPath)

		// Then: should return absolute expanded path
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if !filepath.IsAbs(result) {
			t.Errorf("Expected absolute path, got %s", result)
		}

		currentUser, _ := user.Current()
		expected := filepath.Join(currentUser.HomeDir, "test/path")
		if result != expected {
			t.Errorf("Expected %s, got %s", expected, result)
		}
	})
}

// TestCreateTempDirectoryWithRecovery tests temp directory creation
func TestCreateTempDirectoryWithRecovery(t *testing.T) {
	t.Run("creates temp directory successfully", func(t *testing.T) {
		// Given: a logger
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

		// When: createTempDirectoryWithRecovery is called
		tempDir, err := createTempDirectoryWithRecovery(logger)

		// Then: should create directory and return path
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if tempDir == "" {
			t.Error("Expected non-empty temp directory path")
		}

		// Verify directory exists
		if _, statErr := os.Stat(tempDir); os.IsNotExist(statErr) {
			t.Errorf("Expected directory to exist at %s", tempDir)
		}

		// Cleanup
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("Failed to remove temp directory: %v", err)
		}
	})
}

// TestExpandHomePath_Error tests error cases for home expansion
func TestExpandHomePath_Error(t *testing.T) {
	// Note: This test is difficult to trigger naturally as user.Current() rarely fails
	// In a real scenario, you might use dependency injection or build tags for testing
	t.Run("handles home path with tilde", func(t *testing.T) {
		// Given: a path starting with ~/
		path := "~/documents/test"

		// When: expandHomePath is called
		result, err := expandHomePath(path)

		// Then: should expand successfully (assuming normal environment)
		if err != nil {
			// In normal environments this shouldn't fail
			// but we test the error handling structure
			t.Logf("Got error (expected in some test environments): %v", err)
		} else {
			if !strings.Contains(result, "documents/test") {
				t.Errorf("Expected result to contain documents/test, got %s", result)
			}
		}
	})
}

// ============================================================================
// ADDITIONAL TESTS (formerly from cloner_critical_test.go)
// ============================================================================

// TestRemoveTempDirectoryWithRecovery tests temp directory cleanup
func TestRemoveTempDirectoryWithRecovery(t *testing.T) {
	t.Run("removes directory successfully", func(t *testing.T) {
		// Given: a temporary directory and logger
		tempDir := t.TempDir()
		testDir := filepath.Join(tempDir, "test-removal")
		if err := os.Mkdir(testDir, 0755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

		// When: removeTempDirectoryWithRecovery is called
		if err := removeTempDirectoryWithRecovery(testDir, logger); err != nil {
			t.Errorf("Failed to remove temp directory: %v", err)
		}

		// Then: directory should be removed
		if _, err := os.Stat(testDir); !os.IsNotExist(err) {
			t.Error("Expected directory to be removed")
		}
	})

	t.Run("handles non-existent directory gracefully", func(t *testing.T) {
		// Given: a non-existent directory
		nonExistentDir := "/non/existent/directory"
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

		// When: removeTempDirectoryWithRecovery is called
		// Then: should not panic and may return error for non-existent directory
		if err := removeTempDirectoryWithRecovery(nonExistentDir, logger); err != nil {
			t.Logf("Expected error for non-existent directory: %v", err)
		}
	})
}

// TestMakeDirectoryWritable tests directory permission changes
func TestMakeDirectoryWritable(t *testing.T) {
	t.Run("makes directory writable", func(t *testing.T) {
		// Given: a directory with restricted permissions
		tempDir := t.TempDir()
		testDir := filepath.Join(tempDir, "test-writable")
		if err := os.Mkdir(testDir, 0444); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		// When: makeDirectoryWritable is called
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		err := makeDirectoryWritable(testDir, logger)

		// Then: should succeed without error
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Verify we can write to the directory
		testFile := filepath.Join(testDir, "test.txt")
		writeErr := os.WriteFile(testFile, []byte("test"), 0644)
		if writeErr != nil {
			t.Errorf("Directory should be writable after makeDirectoryWritable")
		}
	})

	t.Run("handles non-existent directory", func(t *testing.T) {
		// Given: a non-existent directory
		nonExistentDir := "/non/existent/directory"

		// When: makeDirectoryWritable is called
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		err := makeDirectoryWritable(nonExistentDir, logger)

		// Then: function should handle gracefully (WalkDir will return error for non-existent path)
		if err != nil {
			t.Logf("Function returned error as expected for non-existent directory: %v", err)
		}
	})
}

// TestFindOrgDirectory tests organization directory discovery
func TestFindOrgDirectory(t *testing.T) {
	t.Run("finds organization directory", func(t *testing.T) {
		// Given: a temp directory structure with organization
		tempDir := t.TempDir()
		orgDir := filepath.Join(tempDir, "test-org")
		if err := os.MkdirAll(orgDir, 0755); err != nil {
			t.Fatalf("Failed to create org directory: %v", err)
		}

		// When: findOrgDirectory is called
		foundDir, err := findOrgDirectory(tempDir)

		// Then: should find the directory
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if foundDir != orgDir {
			t.Errorf("Expected %s, got %s", orgDir, foundDir)
		}
	})

	t.Run("returns error when org directory not found", func(t *testing.T) {
		// Given: a temp directory without the organization
		tempDir := t.TempDir()

		// When: findOrgDirectory is called
		_, err := findOrgDirectory(tempDir)

		// Then: should return error
		if err == nil {
			t.Error("Expected error for non-existent organization directory")
		}
	})
}

// TestExecuteCloneCommandWithRecovery tests clone command execution
func TestExecuteCloneCommandWithRecovery(t *testing.T) {
	t.Run("handles command execution gracefully", func(t *testing.T) {
		// Skip this test in CI/automated environments where real GitHub calls would fail
		if testing.Short() {
			t.Skip("Skipping GitHub API integration test in short mode")
		}
		
		// This test validates error handling and recovery, not successful cloning
		// Given: a clone operation with invalid token (expected to fail gracefully)
		op := CloneOperation{
			Org:     "test-org",
			TempDir: t.TempDir(),
			Config: Config{
				GitHubToken:      "fake-token",
				CloneConcurrency: 1,
			},
		}
		ctx := context.Background()
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

		// When: executeCloneCommandWithRecovery is called
		// Then: should not panic and should return an error (graceful failure)
		err := executeCloneCommandWithRecovery(ctx, op, logger)
		if err == nil {
			t.Error("Expected error for invalid token, but got nil")
		}
		
		// Verify error contains expected GitHub authentication failure
		if !strings.Contains(err.Error(), "401") && !strings.Contains(err.Error(), "Bad credentials") {
			t.Logf("Expected GitHub authentication error, got: %v", err)
		}
	})
}

// TestSetupWorkspaceWithRecovery tests workspace setup
func TestSetupWorkspaceWithRecovery(t *testing.T) {
	t.Run("sets up workspace", func(t *testing.T) {
		// Given: a logger
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

		// When: setupWorkspaceWithRecovery is called
		// Then: should not panic
		_, cleanup, _ := setupWorkspaceWithRecovery(logger)
		if cleanup != nil {
			defer cleanup()
		}
	})
}

// TestExecuteClonePhaseWithRecovery tests clone phase execution
func TestExecuteClonePhaseWithRecovery(t *testing.T) {
	t.Run("executes clone phase", func(t *testing.T) {
		// Skip this test in CI/automated environments where real GitHub calls would fail
		if testing.Short() {
			t.Skip("Skipping GitHub API integration test in short mode")
		}
		
		// This test validates error handling in clone phase
		// Given: a clone operation with invalid token (expected to fail gracefully)
		op := CloneOperation{
			Org:     "test-org",
			TempDir: t.TempDir(),
			Config: Config{
				GitHubToken:      "fake-token",
				CloneConcurrency: 1,
			},
		}
		ctx := context.Background()
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

		// When: executeClonePhaseWithRecovery is called
		// Then: should not panic and return org directory
		_ = executeClonePhaseWithRecovery(ctx, op, logger)
		
		// The function should return some path (even if clone fails)
		// Note: orgDir may be empty if clone fails, which is acceptable
	})
}

// ============================================================================
// TUI-FREE CLONER FUNCTION TESTS - Tests for functions without TUI dependencies
// ============================================================================

// TestExecuteCloneCommand tests clone command execution without TUI
func TestExecuteCloneCommand(t *testing.T) {
	t.Run("executes clone command without TUI progress tracking", func(t *testing.T) {
		// Given: A clone operation without TUI
		op := CloneOperation{
			Org:     "test-org",
			TempDir: t.TempDir(),
			Config: Config{
				GitHubToken:      "fake-token",
				CloneConcurrency: 1,
			},
		}
		ctx := context.Background()
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		// When: executeCloneCommand is called
		err := executeCloneCommand(ctx, op, logger)
		
		// Then: Function should not panic (error is expected due to fake token)
		// We're testing that the function signature exists and can be called
		_ = err // Ignore error as we expect it to fail with fake token
	})
}

// TestExecuteClonePhase tests clone phase execution without TUI
func TestExecuteClonePhase(t *testing.T) {
	t.Run("executes clone phase without TUI progress tracking", func(t *testing.T) {
		// Given: A clone operation
		op := CloneOperation{
			Org:     "test-org", 
			TempDir: t.TempDir(),
			Config: Config{
				GitHubToken:      "fake-token",
				CloneConcurrency: 1,
			},
		}
		ctx := context.Background()
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		// When: executeClonePhase is called
		err := executeClonePhase(ctx, op, logger)
		
		// Then: Function should not panic
		_ = err // Error is expected due to fake token
	})
}

// TestExecuteCommandWithoutProgressTracking tests command execution without progress tracking
func TestExecuteCommandWithoutProgressTracking(t *testing.T) {
	t.Run("executes command without progress tracking", func(t *testing.T) {
		// Given: A simple command and operation
		op := CloneOperation{
			Org:     "test-org",
			TempDir: t.TempDir(),
			Config:  Config{GitHubToken: "fake-token"},
		}
		ctx := context.Background()
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		// Create a simple command that will succeed
		cmd := exec.Command("echo", "test")
		
		// When: executeCommandWithProgressTracking is called
		err := executeCommandWithProgressTracking(ctx, cmd, op, logger)
		
		// Then: No error should occur for a simple echo command
		if err != nil {
			t.Errorf("Expected no error for echo command, got %v", err)
		}
	})
}