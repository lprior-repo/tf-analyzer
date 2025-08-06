package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/samber/lo"
)

// Config Builders - Pure functions for test configuration creation

// newTestConfig creates a basic test configuration with specified orgs and token
func newTestConfig(orgs []string, token string) Config {
	return Config{
		Organizations:    orgs,
		GitHubToken:      token,
		CloneConcurrency: 10,
		MaxGoroutines:    50,
		ProcessTimeout:   30 * time.Second,
		RetryDelay:       1 * time.Second,
		SkipArchived:     false,
		SkipForks:        false,
		BaseURL:          "https://api.github.com",
		TargetRepos:      []string{},
		MatchPrefix:      []string{},
	}
}

// newValidConfig creates a valid configuration for testing
func newValidConfig() Config {
	return newTestConfig([]string{"test-org"}, "valid-token")
}

// newInvalidConfig creates an invalid configuration for error testing
func newInvalidConfig() Config {
	return Config{
		Organizations:    []string{},
		GitHubToken:      "",
		CloneConcurrency: 0,
		MaxGoroutines:    0,
		ProcessTimeout:   0,
		RetryDelay:       0,
		BaseURL:          "",
		TargetRepos:      []string{},
		MatchPrefix:      []string{},
	}
}

// newTargetingConfig creates configuration with target repositories
func newTargetingConfig(targetRepos []string) Config {
	config := newValidConfig()
	config.TargetRepos = targetRepos
	return config
}

// File System Helpers - Functions for creating test files and directories

// createTempTerraformFile creates a temporary .tf file with given content
func createTempTerraformFile(t *testing.T, content string) string {
	t.Helper()
	
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.tf")
	
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp terraform file: %v", err)
	}
	
	return filePath
}

// createTempTerraformRepo creates a temporary repository with terraform files
func createTempTerraformRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	
	repoDir := t.TempDir()
	
	for fileName, content := range files {
		filePath := filepath.Join(repoDir, fileName)
		
		// Create directory if needed
		if dir := filepath.Dir(filePath); dir != repoDir {
			err := os.MkdirAll(dir, 0755)
			if err != nil {
				t.Fatalf("Failed to create directory %s: %v", dir, err)
			}
		}
		
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", fileName, err)
		}
	}
	
	return repoDir
}

// createTempConfigFile creates a temporary configuration file
func createTempConfigFile(t *testing.T, content string) string {
	t.Helper()
	
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "config.yaml")
	
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}
	
	return filePath
}

// Data Builders - Pure functions for creating test data structures

// newTestAnalysisResult creates a standard analysis result for testing
func newTestAnalysisResult(repoName, org string) AnalysisResult {
	return AnalysisResult{
		RepoName:     repoName,
		Organization: org,
		Analysis:     standardRepositoryAnalysis(),
		Error:        nil,
	}
}

// newTestReporter creates a test reporter with standard configuration
func newTestReporter() *Reporter {
	return NewReporter()
}

// standardRepositoryAnalysis creates a standard repository analysis
func standardRepositoryAnalysis() RepositoryAnalysis {
	return RepositoryAnalysis{
		RepositoryPath: "/tmp/test-repo",
		BackendConfig:  nil,
		Providers: ProvidersAnalysis{
			UniqueProviderCount: 0,
			ProviderDetails:     []ProviderDetail{},
		},
		Modules: ModulesAnalysis{
			TotalModuleCalls:  0,
			UniqueModuleCount: 0,
			UniqueModules:     []ModuleDetail{},
		},
		ResourceAnalysis: ResourceAnalysis{
			TotalResourceCount:      0,
			UniqueResourceTypeCount: 0,
			ResourceTypes:           []ResourceType{},
			UntaggedResources:       []UntaggedResource{},
		},
		VariableAnalysis: VariableAnalysis{
			DefinedVariables: []VariableDefinition{},
		},
		OutputAnalysis: OutputAnalysis{
			OutputCount: 0,
			Outputs:     []string{},
		},
	}
}

// standardResourceType creates a standard resource type for testing
func standardResourceType() ResourceType {
	return ResourceType{
		Type:  "aws_s3_bucket",
		Count: 1,
	}
}

// Assertion Helpers - Functions for common test assertions

// assertNoError verifies that no error occurred
func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}
}

// assertError verifies that an error occurred with expected message
func assertError(t *testing.T, err error, expectedMsg string) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected error, but got nil")
	}
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Fatalf("Expected error containing '%s', but got: %v", expectedMsg, err)
	}
}

// assertFileExists verifies that a file exists at the given path
func assertFileExists(t *testing.T, filePath string) {
	t.Helper()
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("Expected file to exist: %s", filePath)
	}
}

// assertFileContains verifies that a file contains expected content
func assertFileContains(t *testing.T, filePath, expectedContent string) {
	t.Helper()
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", filePath, err)
	}
	if !strings.Contains(string(content), expectedContent) {
		t.Fatalf("File %s does not contain expected content: %s", filePath, expectedContent)
	}
}

// Environment Helpers - Functions for managing test environment

// withEnvVars temporarily sets environment variables for a test
func withEnvVars(t *testing.T, envVars map[string]string) {
	t.Helper()
	
	// Store original values for cleanup
	originalValues := make(map[string]string)
	var keysToUnset []string
	
	for key, value := range envVars {
		if original, exists := os.LookupEnv(key); exists {
			originalValues[key] = original
		} else {
			keysToUnset = append(keysToUnset, key)
		}
		err := os.Setenv(key, value)
		if err != nil {
			t.Fatalf("Failed to set env var %s: %v", key, err)
		}
	}
	
	// Cleanup function
	t.Cleanup(func() {
		// Restore original values
		for key, value := range originalValues {
			os.Setenv(key, value)
		}
		// Unset keys that didn't exist before
		for _, key := range keysToUnset {
			os.Unsetenv(key)
		}
	})
}

// Collection Helpers - Functional operations on test data

// filterTerraformFiles filters a list of files to only include terraform files
func filterTerraformFiles(files []string) []string {
	return lo.Filter(files, func(file string, _ int) bool {
		ext := strings.ToLower(filepath.Ext(file))
		return lo.Contains([]string{".tf", ".tfvars", ".hcl"}, ext)
	})
}

// mapFileNames extracts file names from a list of file paths
func mapFileNames(filePaths []string) []string {
	return lo.Map(filePaths, func(path string, _ int) string {
		return filepath.Base(path)
	})
}

// reduceFileSize calculates total size of files
func reduceFileSize(filePaths []string) int64 {
	return lo.Reduce(filePaths, func(acc int64, path string, _ int) int64 {
		if info, err := os.Stat(path); err == nil {
			return acc + info.Size()
		}
		return acc
	}, 0)
}

// Validation Helpers - Functions for validating test conditions

// isValidTerraformFile checks if a file is a valid terraform file
func isValidTerraformFile(filePath string) bool {
	filteredFiles := filterTerraformFiles([]string{filePath})
	if len(filteredFiles) == 0 {
		return false
	}
	
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}
	
	// Basic validation - file should contain terraform syntax
	contentStr := string(content)
	return strings.Contains(contentStr, "resource") ||
		   strings.Contains(contentStr, "variable") ||
		   strings.Contains(contentStr, "output") ||
		   strings.Contains(contentStr, "data")
}

// hasValidGitHubToken checks if a valid GitHub token is available
func hasValidGitHubToken() bool {
	token := os.Getenv("GITHUB_TOKEN")
	return token != "" && len(token) > 10
}

// Test Skip Helpers - Functions for conditional test execution

// Note: skipIfNoGitHubToken already exists in acceptance_test.go
// Removed duplicate to avoid redeclaration error

// skipIfShortTest skips test if running in short mode
func skipIfShortTest(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}
}

// Mock Data Creators - Functions for creating test fixtures

// createMockTerraformContent creates realistic terraform content for testing
func createMockTerraformContent(resourceType, resourceName string) string {
	return `resource "` + resourceType + `" "` + resourceName + `" {
  name        = "test-resource"
  environment = "development"
  
  tags = {
    Project = "tf-analyzer"
    Owner   = "test"
  }
}`
}

// createMockVariableContent creates terraform variable content
func createMockVariableContent(varName, varType, description string) string {
	return `variable "` + varName + `" {
  type        = ` + varType + `
  description = "` + description + `"
  default     = ""
}`
}

// createMockOutputContent creates terraform output content
func createMockOutputContent(outputName, value, description string) string {
	return `output "` + outputName + `" {
  value       = ` + value + `
  description = "` + description + `"
}`
}

// Directory Structure Helpers - Functions for creating complex test structures

// createMockTerraformModule creates a complete terraform module structure
func createMockTerraformModule(t *testing.T, moduleName string) string {
	t.Helper()
	
	moduleDir := filepath.Join(t.TempDir(), moduleName)
	err := os.MkdirAll(moduleDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create module directory: %v", err)
	}
	
	files := map[string]string{
		"main.tf": createMockTerraformContent("aws_s3_bucket", "main_bucket"),
		"variables.tf": createMockVariableContent("bucket_name", "string", 
			"Name of the S3 bucket"),
		"outputs.tf": createMockOutputContent("bucket_id", "aws_s3_bucket.main_bucket.id", 
			"ID of the created bucket"),
		"versions.tf": `terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}`,
	}
	
	for fileName, content := range files {
		filePath := filepath.Join(moduleDir, fileName)
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", fileName, err)
		}
	}
	
	return moduleDir
}

// createMockRepositoryStructure creates a realistic repository structure
func createMockRepositoryStructure(t *testing.T, repoName string) string {
	t.Helper()
	
	repoDir := filepath.Join(t.TempDir(), repoName)
	
	// Create nested directory structure
	directories := []string{
		"modules/vpc",
		"modules/security-groups",
		"environments/dev",
		"environments/prod",
		"examples",
	}
	
	for _, dir := range directories {
		fullPath := filepath.Join(repoDir, dir)
		err := os.MkdirAll(fullPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}
	
	// Create files in different locations
	files := map[string]string{
		"README.md": "# " + repoName + "\n\nTest repository for tf-analyzer",
		"main.tf": createMockTerraformContent("aws_vpc", "main"),
		"variables.tf": createMockVariableContent("vpc_cidr", "string", "VPC CIDR block"),
		"modules/vpc/main.tf": createMockTerraformContent("aws_vpc", "vpc"),
		"modules/vpc/variables.tf": createMockVariableContent("cidr_block", "string", 
			"CIDR block for VPC"),
		"environments/dev/terraform.tfvars": `vpc_cidr = "10.0.0.0/16"
environment = "development"`,
		"environments/prod/terraform.tfvars": `vpc_cidr = "10.1.0.0/16"
environment = "production"`,
	}
	
	for filePath, content := range files {
		fullPath := filepath.Join(repoDir, filePath)
		
		// Ensure directory exists
		dir := filepath.Dir(fullPath)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory for file %s: %v", filePath, err)
		}
		
		err = os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", filePath, err)
		}
	}
	
	return repoDir
}

// Functional Test Utilities - Higher-order functions for test operations

// withTempRepo executes a test function with a temporary repository
func withTempRepo(t *testing.T, repoFiles map[string]string, 
	testFunc func(repoPath string)) {
	t.Helper()
	
	repoPath := createTempTerraformRepo(t, repoFiles)
	testFunc(repoPath)
}

// withMockConfig executes a test function with a mock configuration
func withMockConfig(t *testing.T, configModifier func(*Config), 
	testFunc func(config Config)) {
	t.Helper()
	
	config := newValidConfig()
	if configModifier != nil {
		configModifier(&config)
	}
	testFunc(config)
}

// withEnvAndCleanup sets environment variables and ensures cleanup
func withEnvAndCleanup(t *testing.T, envVars map[string]string, 
	testFunc func()) {
	t.Helper()
	
	withEnvVars(t, envVars)
	testFunc()
}

// Comparison Helpers - Functions for comparing test results

// assertResultsEqual compares two AnalysisResult slices for equality
func assertResultsEqual(t *testing.T, expected, actual []AnalysisResult) {
	t.Helper()
	
	if len(expected) != len(actual) {
		t.Fatalf("Expected %d results, got %d", len(expected), len(actual))
	}
	
	for i, expectedResult := range expected {
		actualResult := actual[i]
		if expectedResult.RepoName != actualResult.RepoName {
			t.Errorf("Result %d: expected repository %s, got %s", 
				i, expectedResult.RepoName, actualResult.RepoName)
		}
		if expectedResult.Organization != actualResult.Organization {
			t.Errorf("Result %d: expected org %s, got %s", 
				i, expectedResult.Organization, actualResult.Organization)
		}
	}
}

// assertConfigValid validates that a configuration is valid
func assertConfigValid(t *testing.T, config Config) {
	t.Helper()
	
	if len(config.Organizations) == 0 {
		t.Error("Expected non-empty Organizations")
	}
	if config.GitHubToken == "" {
		t.Error("Expected non-empty GitHubToken")
	}
	if config.CloneConcurrency <= 0 {
		t.Error("Expected positive CloneConcurrency")
	}
	if config.MaxGoroutines <= 0 {
		t.Error("Expected positive MaxGoroutines")
	}
}

// CMD Test Specific Helpers - Specialized helpers for command-line testing

// assertStringSlicesEqual compares two string slices for equality
func assertStringSlicesEqual(t *testing.T, expected, actual []string) {
	t.Helper()
	
	if len(expected) != len(actual) {
		t.Errorf("Expected %d items, got %d", len(expected), len(actual))
		return
	}
	
	for i, expectedItem := range expected {
		if actual[i] != expectedItem {
			t.Errorf("Item %d: expected %s, got %s", i, expectedItem, actual[i])
		}
	}
}

// assertConfigField checks a specific config field matches expected value
func assertConfigField(t *testing.T, fieldName string, expected, actual interface{}) {
	t.Helper()
	
	if expected != actual {
		t.Errorf("Expected %s to be %v, got %v", fieldName, expected, actual)
	}
}

// assertConfigComplete validates all major config fields match expectations
func assertConfigComplete(t *testing.T, config Config, expectations map[string]interface{}) {
	t.Helper()
	
	if expected, exists := expectations["organizations"]; exists {
		assertStringSlicesEqual(t, expected.([]string), config.Organizations)
	}
	if expected, exists := expectations["token"]; exists {
		assertConfigField(t, "GitHubToken", expected, config.GitHubToken)
	}
	if expected, exists := expectations["max_goroutines"]; exists {
		assertConfigField(t, "MaxGoroutines", expected, config.MaxGoroutines)
	}
	if expected, exists := expectations["clone_concurrency"]; exists {
		assertConfigField(t, "CloneConcurrency", expected, config.CloneConcurrency)
	}
	if expected, exists := expectations["timeout"]; exists {
		assertConfigField(t, "ProcessTimeout", expected, config.ProcessTimeout)
	}
}

// Viper Test Helpers - Helpers for viper configuration testing

// withViperReset executes a test function with viper reset before and after
func withViperReset(t *testing.T, testFunc func()) {
	t.Helper()
	
	// Clear viper state before test
	// Note: viper.Reset() would be called here if viper was imported
	testFunc()
	// Clean up viper state after test if needed
}