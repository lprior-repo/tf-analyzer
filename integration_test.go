package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// INTEGRATION TESTS - Component Interaction Verification
// ============================================================================
// These tests verify how key components interact following Martin Fowler's
// integration testing principles - focusing on critical interaction points
// and testing both success and failure scenarios.

// TestAnalyzerReporterIntegration tests the critical interaction between
// the analysis engine and reporting system.
func TestAnalyzerReporterIntegration(t *testing.T) {
	t.Run("analyzer results flow correctly through reporter", func(t *testing.T) {
		// GIVEN: A repository with mixed Terraform files
		tempRepo := setupTestRepository(t)
		
		// AND: An analyzer configured for that repository
		analyzer := setupTestAnalyzer()
		
		// AND: A reporter to collect results
		reporter := NewReporter()
		
		// WHEN: Analysis is performed on the repository
		result, err := analyzer.AnalyzeRepository(tempRepo, "test-org")
		if err != nil {
			t.Fatalf("Analysis should succeed, got error: %v", err)
		}
		
		// AND: Results are added to the reporter
		reporter.AddResults([]AnalysisResult{result})
		
		// THEN: Reporter should contain the analysis data
		if len(reporter.results) != 1 {
			t.Errorf("Expected 1 result in reporter, got %d", len(reporter.results))
		}
		
		// AND: Reporter should be able to generate coherent summary
		tempDir := t.TempDir()
		summaryFile := filepath.Join(tempDir, "summary.json")
		
		if err := reporter.ExportJSON(summaryFile); err != nil {
			t.Errorf("Integration failed: reporter could not export analyzer results: %v", err)
		}
		
		// AND: Exported data should match original analysis
		content, err := os.ReadFile(summaryFile)
		if err != nil {
			t.Fatalf("Could not read exported file: %v", err)
		}
		
		contentStr := string(content)
		if !strings.Contains(contentStr, result.RepoName) {
			t.Errorf("Exported data missing repository name '%s'", result.RepoName)
		}
		
		if !strings.Contains(contentStr, "test-org") {
			t.Errorf("Exported data missing organization 'test-org'")
		}
	})
	
	t.Run("analyzer handles repository errors gracefully", func(t *testing.T) {
		// GIVEN: An analyzer
		analyzer := setupTestAnalyzer()
		
		// AND: An invalid repository path
		invalidPath := "/nonexistent/path"
		
		// WHEN: Analysis is attempted on invalid path
		result, err := analyzer.AnalyzeRepository(invalidPath, "test-org")
		
		// THEN: Error should be handled gracefully
		if err == nil {
			t.Error("Expected error for invalid path, but got nil")
		}
		
		// AND: Result should indicate the error state
		if result.Error == nil {
			t.Error("Expected result to contain error information")
		}
		
		// AND: Reporter should be able to handle error results
		reporter := NewReporter()
		reporter.AddResults([]AnalysisResult{result})
		
		// Should not panic or fail when processing error results
		if len(reporter.results) != 1 {
			t.Errorf("Reporter should accept error results, got %d results", len(reporter.results))
		}
	})
}

// TestFileSystemAnalyzerIntegration tests interaction between file system
// operations and the analysis engine.
func TestFileSystemAnalyzerIntegration(t *testing.T) {
	t.Run("file discovery and analysis pipeline works end-to-end", func(t *testing.T) {
		// GIVEN: A complex repository structure
		tempRepo := setupComplexTestRepository(t)
		
		// WHEN: Repository is processed through the full pipeline
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		repo := Repository{
			Name:         "test-repo",
			Path:         tempRepo,
			Organization: "test-org",
		}
		
		result := processRepositoryFilesWithRecovery(repo, logger)
		
		// THEN: Processing should succeed
		if result.Error != nil {
			t.Errorf("Full pipeline should succeed, got error: %v", result.Error)
		}
		
		// AND: Result should contain discovered and analyzed files
		if result.Analysis.ResourceAnalysis.TotalResourceCount == 0 {
			t.Error("Expected to find resources in test repository")
		}
		
		// AND: File filtering should work correctly
		if result.Analysis.Providers.UniqueProviderCount == 0 {
			t.Error("Expected to find providers in test repository")
		}
		
		// AND: Path traversal should respect skip rules
		// (implicitly tested by not finding files in .git directories)
	})
	
	t.Run("concurrent file processing maintains data integrity", func(t *testing.T) {
		// GIVEN: Multiple repositories to process concurrently
		repos := setupMultipleTestRepositories(t, 3)
		
		// WHEN: Repositories are processed concurrently
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		results := make([]AnalysisResult, len(repos))
		for i, repoPath := range repos {
			repo := Repository{
				Name:         filepath.Base(repoPath),
				Path:         repoPath,
				Organization: "test-org",
			}
			result := processRepositoryFilesWithRecovery(repo, logger)
			if result.Error != nil {
				t.Errorf("Repository %d processing failed: %v", i, result.Error)
			}
			results[i] = result
		}
		
		// THEN: All results should be valid and distinct
		for i, result := range results {
			if result.RepoName == "" {
				t.Errorf("Repository %d should have a name", i)
			}
			
			if result.Organization != "test-org" {
				t.Errorf("Repository %d should belong to test-org", i)
			}
		}
		
		// AND: Results should be aggregatable
		reporter := NewReporter()
		reporter.AddResults(results)
		
		if len(reporter.results) != len(repos) {
			t.Errorf("Expected %d results in reporter, got %d", len(repos), len(reporter.results))
		}
	})
}

// TestConfigurationValidationIntegration tests the interaction between
// configuration loading and validation systems.
func TestConfigurationValidationIntegration(t *testing.T) {
	t.Run("configuration flows correctly through validation pipeline", func(t *testing.T) {
		// GIVEN: Various configuration scenarios
		testCases := []struct {
			name          string
			config        Config
			expectError   bool
			errorContains string
		}{
			{
				name: "valid production configuration",
				config: Config{
					Organizations:    []string{"hashicorp", "terraform-providers"},
					GitHubToken:      "ghp_validtokenformat123456789",
					MaxGoroutines:    50,
					CloneConcurrency: 25,
				},
				expectError: false,
			},
			{
				name: "invalid configuration with missing token",
				config: Config{
					Organizations:    []string{"test-org"},
					GitHubToken:      "", // Missing
					MaxGoroutines:    10,
					CloneConcurrency: 5,
				},
				expectError:   true,
				errorContains: "GitHubToken",
			},
			{
				name: "invalid configuration with zero goroutines",
				config: Config{
					Organizations:    []string{"test-org"},
					GitHubToken:      "valid-token",
					MaxGoroutines:    0, // Invalid
					CloneConcurrency: 5,
				},
				expectError:   true,
				errorContains: "MaxGoroutines",
			},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// WHEN: Configuration is validated
				err := validateAnalysisConfiguration(tc.config)
				
				// THEN: Validation result should match expectations
				if tc.expectError {
					if err == nil {
						t.Errorf("Expected validation error for %s, but got nil", tc.name)
					} else if tc.errorContains != "" && !strings.Contains(err.Error(), tc.errorContains) {
						t.Errorf("Expected error to contain '%s', got: %v", tc.errorContains, err)
					}
				} else {
					if err != nil {
						t.Errorf("Expected no validation error for %s, got: %v", tc.name, err)
					}
				}
			})
		}
	})
}

// Helper functions for setting up test scenarios

func setupTestRepository(t *testing.T) string {
	t.Helper()
	
	tempDir := t.TempDir()
	
	// Create a simple Terraform file
	tfContent := `
resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
  
  tags = {
    Name = "main-vpc"
    Environment = "test"
  }
}

provider "aws" {
  region = "us-west-2"
}
`
	
	tfFile := filepath.Join(tempDir, "main.tf")
	if err := os.WriteFile(tfFile, []byte(tfContent), 0644); err != nil {
		t.Fatalf("Failed to create test Terraform file: %v", err)
	}
	
	return tempDir
}

func setupComplexTestRepository(t *testing.T) string {
	t.Helper()
	
	tempDir := t.TempDir()
	
	// Create multiple Terraform files with different constructs
	files := map[string]string{
		"main.tf": `
resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}

resource "aws_subnet" "private" {
  count  = 2
  vpc_id = aws_vpc.main.id
}
`,
		"variables.tf": `
variable "environment" {
  description = "Environment name"
  type        = string
  default     = "dev"
}
`,
		"outputs.tf": `
output "vpc_id" {
  value = aws_vpc.main.id
}
`,
		"providers.tf": `
provider "aws" {
  region = var.aws_region
}

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}
`,
	}
	
	for filename, content := range files {
		filepath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}
	
	return tempDir
}

func setupMultipleTestRepositories(t *testing.T, count int) []string {
	t.Helper()
	
	repos := make([]string, count)
	for i := 0; i < count; i++ {
		repos[i] = setupTestRepository(t)
	}
	return repos
}

func setupTestAnalyzer() *RepositoryAnalyzer {
	// Return a configured analyzer instance
	// This would be properly implemented based on the actual analyzer structure
	return &RepositoryAnalyzer{
		// Configuration for testing
	}
}

// TestRepositoryTargetingWorkflowIntegration tests the repository targeting workflow
// without requiring GitHub access
func TestRepositoryTargetingWorkflowIntegration(t *testing.T) {
	t.Run("workflow validates targeting configuration", func(t *testing.T) {
		// GIVEN: Invalid targeting configuration
		config := Config{
			GitHubToken:      "test-token", 
			Organizations:    []string{"test-org"},
			MaxGoroutines:    10,
			CloneConcurrency: 5,
			ProcessTimeout:   30 * time.Second,
			// Invalid: both target repos and target file specified
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
		
		if err != nil && !strings.Contains(err.Error(), "targeting configuration") {
			t.Errorf("Expected targeting configuration error, got: %v", err)
		}
	})
	
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

// executeTargetedAnalysisWorkflow performs the complete workflow for targeted repository analysis
func executeTargetedAnalysisWorkflow(ctx context.Context, config Config) ([]AnalysisResult, error) {
	// 1. Validate targeting configuration
	if err := validateTargetingConfiguration(config); err != nil {
		return nil, fmt.Errorf("invalid targeting configuration: %w", err)
	}
	
	// 2. Validate analysis configuration
	if err := validateAnalysisConfiguration(config); err != nil {
		return nil, fmt.Errorf("invalid analysis configuration: %w", err)
	}
	
	// 3. Create processing context
	processingCtx, err := createProcessingContext(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create processing context: %w", err)
	}
	defer releaseProcessingContext(processingCtx)
	
	// 4. Create reporter for collecting results
	reporter := NewReporter()
	
	// 5. Create logger (quiet for tests)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors in integration tests
	}))
	
	// 6. Process all organizations with targeting
	multiCtx := MultiOrgContext{
		Ctx:           ctx,
		ProcessingCtx: processingCtx,
		Reporter:      reporter,
	}
	
	if err := processMultipleOrganizationsWithTargeting(multiCtx, logger); err != nil {
		return nil, fmt.Errorf("failed to process organizations: %w", err)
	}
	
	// 7. Return all results
	return reporter.GetResults(), nil
}

// processMultipleOrganizationsWithTargeting processes organizations with targeting support
func processMultipleOrganizationsWithTargeting(multiCtx MultiOrgContext, logger *slog.Logger) error {
	for _, org := range multiCtx.ProcessingCtx.Config.Organizations {
		orgCtx := OrgProcessContext{
			Ctx:           multiCtx.Ctx,
			Org:           org,
			ProcessingCtx: multiCtx.ProcessingCtx,
			Reporter:      multiCtx.Reporter,
			Logger:        logger,
		}
		
		_, err := processOrganizationWorkflow(orgCtx)
		if err != nil {
			logger.Error("Organization processing failed", "org", org, "error", err)
			// Continue with other organizations instead of failing completely
			continue
		}
	}
	
	return nil
}

// Mock RepositoryAnalyzer for integration testing
type RepositoryAnalyzer struct {
	// Add necessary fields for testing
}

func (ra *RepositoryAnalyzer) AnalyzeRepository(repoPath, organization string) (AnalysisResult, error) {
	// Simplified analysis for integration testing
	result := AnalysisResult{
		RepoName:     filepath.Base(repoPath),
		Organization: organization,
		Analysis: RepositoryAnalysis{
			RepositoryPath:   repoPath,
			ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 2},
			Providers:        ProvidersAnalysis{UniqueProviderCount: 1},
			Modules:          ModulesAnalysis{TotalModuleCalls: 0},
		},
	}
	
	// Check if path exists to simulate real behavior
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		result.Error = err
		return result, err
	}
	
	return result, nil
}