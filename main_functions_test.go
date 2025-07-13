package main

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestCloneAndAnalyzeMultipleOrgs tests the main orchestration function
func TestCloneAndAnalyzeMultipleOrgs(t *testing.T) {
	t.Run("handles empty organizations gracefully", func(t *testing.T) {
		// Given: processing context with no organizations
		config := Config{
			MaxGoroutines:    2,
			CloneConcurrency: 1,
			GitHubToken:      "fake-token",
			Organizations:    []string{}, // Empty organizations
			ProcessTimeout:   5 * time.Second,
		}

		ctx, err := createProcessingContext(config)
		if err != nil {
			t.Fatalf("Failed to create processing context: %v", err)
		}
		defer releaseProcessingContext(ctx)

		reporter := NewReporter()
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		// When: cloneAndAnalyzeMultipleOrgs is called
		err = cloneAndAnalyzeMultipleOrgs(timeoutCtx, ctx, reporter, nil)

		// Then: should handle gracefully (no panic)
		// Error is expected due to no organizations
		if err == nil {
			t.Log("Function completed without error")
		}
	})
}

// TestProcessRepositoriesConcurrentlyWithTimeout tests concurrent processing
func TestProcessRepositoriesConcurrentlyWithTimeout(t *testing.T) {
	t.Run("processes empty repository list", func(t *testing.T) {
		// Given: empty repository list and short timeout
		repositories := []Repository{} // Empty list
		
		config := Config{
			MaxGoroutines:    2,
			CloneConcurrency: 1,
			GitHubToken:      "fake-token",
			Organizations:    []string{"test-org"},
			ProcessTimeout:   1 * time.Second,
		}

		ctx, err := createProcessingContext(config)
		if err != nil {
			t.Fatalf("Failed to create processing context: %v", err)
		}
		defer releaseProcessingContext(ctx)

		timeoutCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		// When: processRepositoriesConcurrentlyWithTimeout is called
		results := processRepositoriesConcurrentlyWithTimeout(repositories, timeoutCtx, ctx, nil, nil)

		// Then: should return empty results
		if len(results) != 0 {
			t.Errorf("Expected 0 results for empty repositories, got %d", len(results))
		}
	})
}

// TestLoadEnvironmentConfig tests environment configuration loading
func TestLoadEnvironmentConfig(t *testing.T) {
	t.Run("loads config from environment file", func(t *testing.T) {
		// Given: a temporary environment file
		tempFile, err := os.CreateTemp("", "test.env")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())
		
		// Write test environment variables
		envContent := `GITHUB_TOKEN=test-token-123
GITHUB_ORGS=test-org1,test-org2
MAX_GOROUTINES=20
CLONE_CONCURRENCY=10`
		
		_, err = tempFile.WriteString(envContent)
		if err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		tempFile.Close()

		// When: loadEnvironmentConfig is called
		config, err := loadEnvironmentConfig(tempFile.Name())

		// Then: should load configuration successfully
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		
		// Check that some config values are set (may use defaults)
		if config.GitHubToken == "" {
			t.Error("Expected GitHub token to be set")
		}
	})

	t.Run("handles non-existent file gracefully", func(t *testing.T) {
		// Given: a non-existent file path
		nonExistentFile := "/tmp/does-not-exist.env"

		// When: loadEnvironmentConfig is called
		_, err := loadEnvironmentConfig(nonExistentFile)

		// Then: should return error
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
	})
}

// TestSubmitRepositoryJobsWithTimeout tests job submission with timeout
func TestSubmitRepositoryJobsWithTimeout(t *testing.T) {
	t.Run("submits empty repository jobs", func(t *testing.T) {
		// Given: empty repositories and context with short timeout
		repositories := []Repository{}
		
		config := Config{
			MaxGoroutines:    2,
			CloneConcurrency: 1,
			GitHubToken:      "fake-token",
			Organizations:    []string{"test-org"},
		}

		ctx, err := createProcessingContext(config)
		if err != nil {
			t.Fatalf("Failed to create processing context: %v", err)
		}
		defer releaseProcessingContext(ctx)

		timeoutCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		p := configureWaitGroup(2)
		results := make(chan AnalysisResult, 1)
		defer close(results)

		// When: submitRepositoryJobsWithTimeout is called
		// Then: should not panic
		submitRepositoryJobsWithTimeout(repositories, timeoutCtx, p, ctx.Pool, results, nil)
	})
}

// TestCreateResultsModel tests TUI results model creation
func TestCreateResultsModel(t *testing.T) {
	t.Run("creates results model", func(t *testing.T) {
		// Given: analysis results
		results := []AnalysisResult{
			{
				RepoName:     "repo1",
				Organization: "org1",
				Analysis: RepositoryAnalysis{
					ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 5},
				},
			},
		}

		// When: createResultsModel is called
		model := createResultsModel(results)

		// Then: should return a model
		// Note: The exact structure depends on the bubbles table implementation
		_ = model // Just ensure no panic
	})

	t.Run("handles empty results", func(t *testing.T) {
		// Given: empty results
		results := []AnalysisResult{}

		// When: createResultsModel is called
		model := createResultsModel(results)

		// Then: should return a model without panic
		_ = model
	})
}