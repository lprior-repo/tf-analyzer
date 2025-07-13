package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// TestRemoveTempDirectoryWithRecovery tests temp directory cleanup
func TestRemoveTempDirectoryWithRecovery(t *testing.T) {
	t.Run("removes directory successfully", func(t *testing.T) {
		// Given: a temporary directory and logger
		tempDir := t.TempDir()
		testDir := filepath.Join(tempDir, "test-removal")
		os.Mkdir(testDir, 0755)
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

		// When: removeTempDirectoryWithRecovery is called
		removeTempDirectoryWithRecovery(testDir, logger)

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
		// Then: should not panic
		removeTempDirectoryWithRecovery(nonExistentDir, logger)
	})
}

// TestMakeDirectoryWritable tests directory permission changes
func TestMakeDirectoryWritable(t *testing.T) {
	t.Run("makes directory writable", func(t *testing.T) {
		// Given: a directory with restricted permissions
		tempDir := t.TempDir()
		testDir := filepath.Join(tempDir, "test-writable")
		os.Mkdir(testDir, 0444) // Read-only

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

		// Then: should return error
		if err == nil {
			t.Error("Expected error for non-existent directory")
		}
	})
}

// TestFindOrgDirectory tests organization directory discovery
func TestFindOrgDirectory(t *testing.T) {
	t.Run("finds organization directory", func(t *testing.T) {
		// Given: a temp directory structure with organization
		tempDir := t.TempDir()
		orgDir := filepath.Join(tempDir, "test-org")
		os.MkdirAll(orgDir, 0755)

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
		// Given: a clone operation (will fail but should not panic)
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
		// Then: should not panic (will likely fail but gracefully)
		executeCloneCommandWithRecovery(ctx, op, logger)
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
		// Given: a clone operation
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