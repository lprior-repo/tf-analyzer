package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// REPOSITORY TARGETING WORKFLOW TESTS - Mock Workflow Validation
// ============================================================================
// These tests validate the workflow implementation without requiring GitHub access

// TestExecuteTargetedAnalysisWorkflowValidation tests the workflow function
// with invalid configurations to ensure proper error handling
func TestExecuteTargetedAnalysisWorkflowValidation(t *testing.T) {
	t.Run("workflow validates targeting configuration", func(t *testing.T) {
		// GIVEN: Invalid targeting configuration
		config := Config{
			GitHubToken:      "test-token", 
			Organizations:    []string{"test-org"},
			MaxGoroutines:    10,
			CloneConcurrency: 5,
			ProcessTimeout:   30 * time.Second,
			// Invalid: both target repos and regex specified
			TargetRepos:     []string{"repo1"},
			TargetReposFile: "/path/to/file", // Conflict!
		}
		
		// WHEN: Workflow is executed
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		results, err := executeTargetedAnalysisWorkflow(ctx, config)
		
		// THEN: Should fail with validation error
		if err == nil {
			t.Error("Expected validation error for conflicting configuration")
		}
		
		if results != nil {
			t.Error("Expected nil results when validation fails")
		}
		
		if err != nil && !containsAny(err.Error(), []string{"targeting configuration", "cannot specify both"}) {
			t.Errorf("Expected targeting configuration error, got: %v", err)
		}
	})
	
	t.Run("workflow validates analysis configuration", func(t *testing.T) {
		// GIVEN: Invalid analysis configuration
		config := Config{
			// Missing required fields
			Organizations:    []string{}, // Empty!
			MaxGoroutines:    0,          // Invalid!
			CloneConcurrency: 5,
			ProcessTimeout:   30 * time.Second,
		}
		
		// WHEN: Workflow is executed
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		results, err := executeTargetedAnalysisWorkflow(ctx, config)
		
		// THEN: Should fail with analysis configuration error
		if err == nil {
			t.Error("Expected analysis configuration error")
		}
		
		if results != nil {
			t.Error("Expected nil results when validation fails")
		}
	})
}

// TestWorkflowComponentIntegration tests integration between workflow components
func TestWorkflowComponentIntegration(t *testing.T) {
	t.Run("workflow creates and releases processing context", func(t *testing.T) {
		// GIVEN: Valid minimal configuration
		config := Config{
			GitHubToken:      "test-token",
			Organizations:    []string{"test-org"},
			MaxGoroutines:    10,
			CloneConcurrency: 5,
			ProcessTimeout:   30 * time.Second,
		}
		
		// WHEN: Processing context is created
		processingCtx, err := createProcessingContext(config)
		
		// THEN: Should succeed
		if err != nil {
			t.Fatalf("Processing context creation should succeed, got: %v", err)
		}
		
		// AND: Should have proper configuration
		if processingCtx.Config.MaxGoroutines != 10 {
			t.Errorf("Expected MaxGoroutines 10, got %d", processingCtx.Config.MaxGoroutines)
		}
		
		if processingCtx.Pool == nil {
			t.Error("Expected goroutine pool to be created")
		}
		
		// AND: Should be releasable without errors
		releaseProcessingContext(processingCtx)
	})
	
	t.Run("reporter workflow integration", func(t *testing.T) {
		// GIVEN: A reporter and test results
		reporter := NewReporter()
		testResults := []AnalysisResult{
			{
				RepoName:     "test-repo",
				Organization: "test-org",
				Analysis: RepositoryAnalysis{
					RepositoryPath:   "/path/to/repo",
					ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 5},
					Providers:        ProvidersAnalysis{UniqueProviderCount: 2},
				},
			},
		}
		
		// WHEN: Results are added and retrieved
		reporter.AddResults(testResults)
		retrievedResults := reporter.GetResults()
		
		// THEN: Results should be properly stored and retrieved
		if len(retrievedResults) != 1 {
			t.Errorf("Expected 1 result, got %d", len(retrievedResults))
		}
		
		if retrievedResults[0].RepoName != "test-repo" {
			t.Errorf("Expected repo name 'test-repo', got '%s'", retrievedResults[0].RepoName)
		}
		
		if retrievedResults[0].Organization != "test-org" {
			t.Errorf("Expected organization 'test-org', got '%s'", retrievedResults[0].Organization)
		}
	})
}

// Helper functions

func containsAny(str string, substrings []string) bool {
	for _, substr := range substrings {
		if strings.Contains(str, substr) {
			return true
		}
	}
	return false
}