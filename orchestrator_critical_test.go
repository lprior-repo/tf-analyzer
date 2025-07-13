package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDiscoverRepositories tests repository discovery
func TestDiscoverRepositories(t *testing.T) {
	t.Run("discovers repositories correctly", func(t *testing.T) {
		// Given: a temporary directory with repositories
		tempDir := t.TempDir()
		orgDir := filepath.Join(tempDir, "test-org")
		os.MkdirAll(orgDir, 0755)
		
		// Create some test repositories
		os.Mkdir(filepath.Join(orgDir, "repo1"), 0755)
		os.Mkdir(filepath.Join(orgDir, "repo2"), 0755)
		os.WriteFile(filepath.Join(orgDir, "notarepo.txt"), []byte("test"), 0644)

		// When: discoverRepositories is called
		repos, err := discoverRepositories(orgDir, "test-org")

		// Then: should discover only directories
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(repos) != 2 {
			t.Errorf("Expected 2 repositories, got %d", len(repos))
		}
	})

	t.Run("handles non-existent directory", func(t *testing.T) {
		// Given: a non-existent directory
		nonExistentDir := "/non/existent/path"

		// When: discoverRepositories is called
		_, err := discoverRepositories(nonExistentDir, "test-org")

		// Then: should return error
		if err == nil {
			t.Error("Expected error for non-existent directory")
		}
	})
}

// TestConfigureWaitGroup tests wait group configuration
func TestConfigureWaitGroup(t *testing.T) {
	t.Run("configures wait group correctly", func(t *testing.T) {
		// Given: a list of repositories
		repos := []Repository{
			{Name: "repo1", Path: "/path1", Organization: "org1"},
			{Name: "repo2", Path: "/path2", Organization: "org1"},
		}

		// When: configureWaitGroup is called
		wg := configureWaitGroup(len(repos))

		// Then: should not panic
		if wg == nil {
			t.Error("Expected non-nil wait group")
		}
		// Note: We can't easily test the internal counter without race conditions
	})
}

// TestWaitAndCloseChannel tests channel closing
func TestWaitAndCloseChannel(t *testing.T) {
	t.Run("closes channel correctly", func(t *testing.T) {
		// Given: a channel and wait group
		ch := make(chan AnalysisResult, 1)
		repos := []Repository{{Name: "repo1", Path: "/path1", Organization: "org1"}}
		wg := configureWaitGroup(len(repos))

		// When: waitAndCloseChannel is called in a goroutine
		go waitAndCloseChannel(wg, ch)

		// The pool will handle completion automatically for our test

		// Then: channel should be closed
		select {
		case _, ok := <-ch:
			if ok {
				t.Error("Expected channel to be closed")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Channel was not closed within timeout")
		}
	})
}

// TestLoadEnvironmentFile tests environment file loading
func TestLoadEnvironmentFile(t *testing.T) {
	t.Run("loads environment file without panic", func(t *testing.T) {
		// Given: a temporary environment file
		tempFile, err := os.CreateTemp("", "test.env")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())
		
		tempFile.WriteString("TEST_VAR=test_value\n")
		tempFile.Close()

		// When: loadEnvironmentFile is called
		// Then: should not panic
		loadEnvironmentFile(tempFile.Name())
	})
}

// TestLogStats tests statistics logging
func TestLogStats(t *testing.T) {
	t.Run("logs stats without panic", func(t *testing.T) {
		// Given: analysis stats
		stats := ProcessingStats{
			TotalOrgs:      2,
			TotalRepos:     10,
			ProcessedRepos: 8,
			FailedRepos:    2,
			TotalFiles:     50,
			Duration:       30 * time.Second,
		}

		// When: logStats is called
		// Then: should not panic
		logStats(stats)
	})
}

// TestFinalizeProcessing tests processing finalization
func TestFinalizeProcessing(t *testing.T) {
	t.Run("finalizes processing without panic", func(t *testing.T) {
		// Given: analysis results and stats
		results := []AnalysisResult{
			{RepoName: "repo1", Organization: "org1"},
		}
		startTime := time.Now().Add(-10 * time.Second)

		// When: finalizeProcessing is called
		// Then: should not panic
		finalizeProcessing(results, startTime)
	})
}

// TestHandleApplicationError tests error handling
func TestHandleApplicationError(t *testing.T) {
	t.Run("handles error without panic", func(t *testing.T) {
		// Given: an error
		err := os.ErrNotExist

		// When: handleApplicationError is called
		// Then: should not panic
		handleApplicationError(err)
	})
}

// TestLogConfiguration tests configuration logging
func TestLogConfiguration(t *testing.T) {
	t.Run("logs configuration without panic", func(t *testing.T) {
		// Given: a configuration
		config := Config{
			Organizations:    []string{"test-org"},
			GitHubToken:      "test-token",
			MaxGoroutines:    10,
			CloneConcurrency: 5,
		}

		// When: logConfiguration is called
		// Then: should not panic
		logConfiguration(config)
	})
}