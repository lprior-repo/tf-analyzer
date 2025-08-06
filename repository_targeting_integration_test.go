package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// REPOSITORY TARGETING INTEGRATION TESTS - Real GitHub Repository Tests
// ============================================================================
// These tests verify end-to-end repository targeting functionality with REAL
// GitHub repositories. They focus ONLY on functionality not covered by existing
// unit tests and integration tests - specifically the actual targeting of real repos.
// 
// NOTE: Configuration validation is already tested in repository_targeting_test.go
// NOTE: Basic integration patterns are already tested in integration_test.go

const (
	// Test timeout for integration tests
	integrationTestTimeout = 15 * time.Minute
	
	// Known repositories with extensive Terraform content
	hashicorpVaultRepo        = "terraform-aws-vault"
	terraformAWSVPCRepo      = "terraform-aws-vpc"  
	terraformAWSProviderRepo = "terraform-provider-aws"
	
	// Test organizations
	hashicorpOrg           = "hashicorp"
	terraformAWSModulesOrg = "terraform-aws-modules"
)

// skipIfNoGitHubToken skips the test if no GitHub token is available
func skipIfNoGitHubToken(t *testing.T) string {
	t.Helper()
	
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("GITHUB_TOKEN environment variable not set - skipping integration test")
	}
	
	return token
}

// TestTargetSpecificHashiCorpRepository tests targeting a specific HashiCorp repository
// This test will FAIL initially until we implement the targeting functionality
func TestTargetSpecificHashiCorpRepository(t *testing.T) {
	token := skipIfNoGitHubToken(t)
	
	t.Run("target hashicorp terraform-aws-vault repository", func(t *testing.T) {
		// GIVEN: Configuration targeting a specific HashiCorp repository
		config := Config{
			GitHubToken:      token,
			Organizations:    []string{hashicorpOrg},
			TargetRepos:      []string{hashicorpVaultRepo},
			MaxGoroutines:    10,
			CloneConcurrency: 5,
			ProcessTimeout:   integrationTestTimeout,
		}
		
		// WHEN: Analysis is performed with repository targeting
		ctx, cancel := context.WithTimeout(context.Background(), integrationTestTimeout)
		defer cancel()
		
		results, err := executeTargetedAnalysisWorkflow(ctx, config)
		
		// THEN: Analysis should succeed
		if err != nil {
			t.Fatalf("Targeted analysis should succeed, got error: %v", err)
		}
		
		// AND: Only the targeted repository should be analyzed
		if len(results) != 1 {
			t.Fatalf("Expected exactly 1 repository result, got %d", len(results))
		}
		
		result := results[0]
		if result.RepoName != hashicorpVaultRepo {
			t.Errorf("Expected repository '%s', got '%s'", hashicorpVaultRepo, result.RepoName)
		}
		
		if result.Organization != hashicorpOrg {
			t.Errorf("Expected organization '%s', got '%s'", hashicorpOrg, result.Organization)
		}
		
		// AND: Repository should contain Terraform content
		if result.Analysis.ResourceAnalysis.TotalResourceCount == 0 {
			t.Error("Expected to find Terraform resources in HashiCorp vault repository")
		}
		
		if result.Analysis.Providers.UniqueProviderCount == 0 {
			t.Error("Expected to find Terraform providers in HashiCorp vault repository")
		}
	})
}

// TestTargetRepositoriesWithRegexPattern tests repository targeting using regex patterns
// This test will FAIL initially until we implement regex targeting
func TestTargetRepositoriesWithRegexPattern(t *testing.T) {
	token := skipIfNoGitHubToken(t)
	
	t.Run("target repositories matching terraform-aws pattern", func(t *testing.T) {
		// GIVEN: Configuration with regex pattern for terraform-aws modules
		config := Config{
			GitHubToken:      token,
			Organizations:    []string{terraformAWSModulesOrg},
			MatchRegex:       "^terraform-aws-vpc$", // Specific match for vpc module
			MaxGoroutines:    10,
			CloneConcurrency: 5,
			ProcessTimeout:   integrationTestTimeout,
		}
		
		// WHEN: Analysis is performed with regex targeting
		ctx, cancel := context.WithTimeout(context.Background(), integrationTestTimeout)
		defer cancel()
		
		results, err := executeTargetedAnalysisWorkflow(ctx, config)
		
		// THEN: Analysis should succeed
		if err != nil {
			t.Fatalf("Regex targeted analysis should succeed, got error: %v", err)
		}
		
		// AND: Only repositories matching the pattern should be analyzed
		if len(results) == 0 {
			t.Fatal("Expected at least 1 repository matching the regex pattern")
		}
		
		// AND: All results should match the pattern
		for _, result := range results {
			if !strings.Contains(result.RepoName, "vpc") {
				t.Errorf("Repository '%s' does not match expected pattern", result.RepoName)
			}
			
			if result.Organization != terraformAWSModulesOrg {
				t.Errorf("Expected organization '%s', got '%s'", terraformAWSModulesOrg, result.Organization)
			}
		}
		
		// AND: Repository should contain VPC-related Terraform resources
		hasVPCResources := false
		for _, result := range results {
			if result.Analysis.ResourceAnalysis.TotalResourceCount > 0 {
				hasVPCResources = true
				break
			}
		}
		
		if !hasVPCResources {
			t.Error("Expected to find Terraform resources in VPC module repository")
		}
	})
}

// TestTargetRepositoriesWithPrefixMatch tests repository targeting using prefix matching
// This test will FAIL initially until we implement prefix targeting
func TestTargetRepositoriesWithPrefixMatch(t *testing.T) {
	token := skipIfNoGitHubToken(t)
	
	t.Run("target repositories with terraform prefix", func(t *testing.T) {
		// GIVEN: Configuration with prefix matching for terraform repositories
		config := Config{
			GitHubToken:      token,
			Organizations:    []string{hashicorpOrg},
			MatchPrefix:      []string{"terraform-"},
			MaxGoroutines:    10,
			CloneConcurrency: 5,
			ProcessTimeout:   integrationTestTimeout,
		}
		
		// WHEN: Analysis is performed with prefix targeting
		ctx, cancel := context.WithTimeout(context.Background(), integrationTestTimeout)
		defer cancel()
		
		results, err := executeTargetedAnalysisWorkflow(ctx, config)
		
		// THEN: Analysis should succeed
		if err != nil {
			t.Fatalf("Prefix targeted analysis should succeed, got error: %v", err)
		}
		
		// AND: Results should contain repositories with terraform prefix
		if len(results) == 0 {
			t.Fatal("Expected at least 1 repository with terraform prefix")
		}
		
		// AND: All results should have terraform prefix
		for _, result := range results {
			if !strings.HasPrefix(result.RepoName, "terraform-") {
				t.Errorf("Repository '%s' does not have terraform prefix", result.RepoName)
			}
			
			if result.Organization != hashicorpOrg {
				t.Errorf("Expected organization '%s', got '%s'", hashicorpOrg, result.Organization)
			}
		}
	})
}

// TestAnalyzeRealTerraformModules tests analysis of real Terraform modules for specific constructs
func TestAnalyzeRealTerraformModules(t *testing.T) {
	token := skipIfNoGitHubToken(t)
	
	t.Run("analyze terraform-aws-vpc module for AWS constructs", func(t *testing.T) {
		// GIVEN: Configuration targeting the VPC module
		config := Config{
			GitHubToken:      token,
			Organizations:    []string{terraformAWSModulesOrg},  
			TargetRepos:      []string{terraformAWSVPCRepo},
			MaxGoroutines:    10,
			CloneConcurrency: 5,
			ProcessTimeout:   integrationTestTimeout,
		}
		
		// WHEN: Analysis is performed on the VPC module
		ctx, cancel := context.WithTimeout(context.Background(), integrationTestTimeout)
		defer cancel()
		
		results, err := executeTargetedAnalysisWorkflow(ctx, config)
		
		// THEN: Analysis should succeed
		if err != nil {
			t.Fatalf("VPC module analysis should succeed, got error: %v", err)
		}
		
		if len(results) != 1 {
			t.Fatalf("Expected exactly 1 result, got %d", len(results))
		}
		
		result := results[0]
		
		// AND: Should detect AWS provider usage (VPC modules must use AWS provider)
		if result.Analysis.Providers.UniqueProviderCount == 0 {
			t.Error("Expected to find AWS provider in VPC module")
		}
		
		// AND: Should detect VPC-related resources (VPC modules must have resources)
		if result.Analysis.ResourceAnalysis.TotalResourceCount == 0 {
			t.Error("Expected to find VPC resources in terraform-aws-vpc module")
		}
		
		// AND: Should have proper module structure with variables and outputs
		validateTerraformModuleStructure(t, result)
	})
}

// TestExcludeRepositoriesWithPattern tests excluding repositories using patterns
// This test will FAIL initially until we implement exclusion functionality
func TestExcludeRepositoriesWithPattern(t *testing.T) {
	token := skipIfNoGitHubToken(t)
	
	t.Run("exclude test repositories using regex pattern", func(t *testing.T) {
		// GIVEN: Configuration that excludes test repositories
		config := Config{
			GitHubToken:      token,
			Organizations:    []string{hashicorpOrg},
			MatchPrefix:      []string{"terraform-"},  // Include terraform repos
			ExcludeRegex:     ".*test.*",              // Exclude anything with 'test'
			MaxGoroutines:    10,
			CloneConcurrency: 5,
			ProcessTimeout:   integrationTestTimeout,
		}
		
		// WHEN: Analysis is performed with exclusion pattern
		ctx, cancel := context.WithTimeout(context.Background(), integrationTestTimeout)
		defer cancel()
		
		results, err := executeTargetedAnalysisWorkflow(ctx, config)
		
		// THEN: Analysis should succeed
		if err != nil {
			t.Fatalf("Exclusion targeted analysis should succeed, got error: %v", err)
		}
		
		// AND: No results should contain 'test' in the name
		for _, result := range results {
			if strings.Contains(strings.ToLower(result.RepoName), "test") {
				t.Errorf("Repository '%s' should have been excluded by test pattern", result.RepoName)
			}
		}
		
		// AND: Should still have some terraform repositories
		if len(results) == 0 {
			t.Error("Expected some terraform repositories after exclusion")
		}
	})
}

// TestPerformanceAndErrorHandling tests timeout and error scenarios
func TestPerformanceAndErrorHandling(t *testing.T) {
	token := skipIfNoGitHubToken(t)
	
	t.Run("handle timeout gracefully", func(t *testing.T) {
		// GIVEN: Configuration with very short timeout
		config := Config{
			GitHubToken:      token,
			Organizations:    []string{hashicorpOrg},
			TargetRepos:      []string{terraformAWSProviderRepo}, // Large repository
			MaxGoroutines:    1,  // Slow processing
			CloneConcurrency: 1,  // Slow cloning
			ProcessTimeout:   1 * time.Second, // Very short timeout
		}
		
		// WHEN: Analysis is performed with short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		
		results, err := executeTargetedAnalysisWorkflow(ctx, config)
		
		// THEN: Should handle timeout gracefully (either success or proper error)
		if err != nil {
			// Error is acceptable for timeout scenarios
			if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "context") {
				t.Errorf("Expected timeout-related error, got: %v", err)
			}
		}
		
		// If results are returned, they should be valid
		for _, result := range results {
			if result.Organization == "" {
				t.Error("Result should have valid organization even on timeout")
			}
		}
	})
	
	t.Run("handle invalid repository names", func(t *testing.T) {
		// GIVEN: Configuration with invalid repository names
		config := Config{
			GitHubToken:      token,
			Organizations:    []string{hashicorpOrg},
			TargetRepos:      []string{"nonexistent-repo-12345"},
			MaxGoroutines:    10,
			CloneConcurrency: 5,
			ProcessTimeout:   integrationTestTimeout,
		}
		
		// WHEN: Analysis is performed with invalid repository
		ctx, cancel := context.WithTimeout(context.Background(), integrationTestTimeout)
		defer cancel()
		
		results, err := executeTargetedAnalysisWorkflow(ctx, config)
		
		// THEN: Should handle gracefully (may return empty results or error)
		if err != nil {
			// Error is acceptable for invalid repositories
			t.Logf("Expected error for invalid repository: %v", err)
		}
		
		// If results are returned, they should be valid
		for _, result := range results {
			if result.RepoName == "nonexistent-repo-12345" && result.Error == nil {
				t.Error("Nonexistent repository should have error in result")
			}
		}
	})
}

// Helper Functions (These will FAIL initially and drive our implementation)

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
	logger := createQuietLogger()
	
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

// createQuietLogger creates a logger that minimizes output for tests
func createQuietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors in integration tests
	}))
}

// validateTerraformModuleStructure validates that a Terraform module has expected structure
func validateTerraformModuleStructure(t *testing.T, result AnalysisResult) {
	t.Helper()
	
	// A well-structured Terraform module should have:
	
	// 1. Variables for customization
	if len(result.Analysis.VariableAnalysis.DefinedVariables) == 0 {
		t.Error("Expected Terraform module to have variable definitions")
	}
	
	// 2. Outputs for other modules to consume
	if result.Analysis.OutputAnalysis.OutputCount == 0 {
		t.Error("Expected Terraform module to have output definitions")
	}
	
	// 3. At least one resource or module call
	hasContent := result.Analysis.ResourceAnalysis.TotalResourceCount > 0 || 
				 result.Analysis.Modules.TotalModuleCalls > 0
	
	if !hasContent {
		t.Error("Expected Terraform module to have either resources or module calls")
	}
	
	// 4. Provider configuration (at least one provider)
	if result.Analysis.Providers.UniqueProviderCount == 0 {
		t.Error("Expected Terraform module to specify at least one provider")
	}
}

// NOTE: Configuration validation tests already exist in repository_targeting_test.go
// This file focuses ONLY on end-to-end integration with real GitHub repositories