package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseOrganizationsFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single organization",
			input:    "hashicorp",
			expected: []string{"hashicorp"},
		},
		{
			name:     "multiple organizations",
			input:    "hashicorp,terraform-providers,aws-samples",
			expected: []string{"hashicorp", "terraform-providers", "aws-samples"},
		},
		{
			name:     "organizations with spaces",
			input:    " hashicorp , terraform-providers , aws-samples ",
			expected: []string{"hashicorp", "terraform-providers", "aws-samples"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseOrganizations(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d organizations, got %d", len(tt.expected), len(result))
				return
			}
			for i, org := range result {
				if org != tt.expected[i] {
					t.Errorf("Expected organization %s, got %s", tt.expected[i], org)
				}
			}
		})
	}
}

func TestCreateConfigFromEnv(t *testing.T) {
	envVars := map[string]string{
		"GITHUB_TOKEN":    "test-token-123",
		"GITHUB_ORGS":     "org1,org2,org3",
		"GITHUB_BASE_URL": "https://github.enterprise.com",
	}

	config := createConfigFromEnv(envVars)

	// Test default values
	if config.MaxGoroutines != 100 {
		t.Errorf("Expected MaxGoroutines 100, got %d", config.MaxGoroutines)
	}
	if config.CloneConcurrency != 100 {
		t.Errorf("Expected CloneConcurrency 100, got %d", config.CloneConcurrency)
	}
	if config.ProcessTimeout != 30*time.Minute {
		t.Errorf("Expected ProcessTimeout 30m, got %v", config.ProcessTimeout)
	}
	if !config.SkipArchived {
		t.Error("Expected SkipArchived to be true")
	}
	if config.SkipForks {
		t.Error("Expected SkipForks to be false")
	}

	// Test env var values
	if config.GitHubToken != "test-token-123" {
		t.Errorf("Expected GitHubToken 'test-token-123', got %s", config.GitHubToken)
	}
	if len(config.Organizations) != 3 {
		t.Errorf("Expected 3 organizations, got %d", len(config.Organizations))
	}
	expectedOrgs := []string{"org1", "org2", "org3"}
	for i, org := range config.Organizations {
		if org != expectedOrgs[i] {
			t.Errorf("Expected org %s, got %s", expectedOrgs[i], org)
		}
	}
	if config.BaseURL != "https://github.enterprise.com" {
		t.Errorf("Expected BaseURL 'https://github.enterprise.com', got %s", config.BaseURL)
	}
}

func TestValidateAnalysisConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: Config{
				MaxGoroutines:    10,
				CloneConcurrency: 5,
				GitHubToken:      "test-token",
				Organizations:    []string{"test-org"},
			},
			expectError: false,
		},
		{
			name: "invalid max goroutines - zero",
			config: Config{
				MaxGoroutines:    0,
				CloneConcurrency: 5,
				GitHubToken:      "test-token",
				Organizations:    []string{"test-org"},
			},
			expectError: true,
			errorMsg:    "MaxGoroutines must be positive, got 0",
		},
		{
			name: "invalid max goroutines - negative",
			config: Config{
				MaxGoroutines:    -1,
				CloneConcurrency: 5,
				GitHubToken:      "test-token",
				Organizations:    []string{"test-org"},
			},
			expectError: true,
			errorMsg:    "MaxGoroutines must be positive, got -1",
		},
		{
			name: "invalid clone concurrency - zero",
			config: Config{
				MaxGoroutines:    10,
				CloneConcurrency: 0,
				GitHubToken:      "test-token",
				Organizations:    []string{"test-org"},
			},
			expectError: true,
			errorMsg:    "CloneConcurrency must be positive, got 0",
		},
		{
			name: "missing github token",
			config: Config{
				MaxGoroutines:    10,
				CloneConcurrency: 5,
				GitHubToken:      "",
				Organizations:    []string{"test-org"},
			},
			expectError: true,
			errorMsg:    "GITHUB_TOKEN is required",
		},
		{
			name: "missing organizations",
			config: Config{
				MaxGoroutines:    10,
				CloneConcurrency: 5,
				GitHubToken:      "test-token",
				Organizations:    []string{},
			},
			expectError: true,
			errorMsg:    "at least one organization must be specified in GITHUB_ORGS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAnalysisConfiguration(tt.config)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestFilterRepositoryDirs(t *testing.T) {
	// Create temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "test-filter-repos")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test directories and files
	dirs := []string{"repo1", "repo2", "repo3"}
	files := []string{"file1.txt", "file2.md"}

	for _, dir := range dirs {
		if err := os.Mkdir(filepath.Join(tempDir, dir), 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}

	for _, file := range files {
		if err := os.WriteFile(filepath.Join(tempDir, file), []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", file, err)
		}
	}

	// Read directory entries
	entries, err := readDirectory(tempDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	// Filter repository directories
	repoNames := filterRepositoryDirs(entries)

	// Verify results
	if len(repoNames) != len(dirs) {
		t.Errorf("Expected %d repository directories, got %d", len(dirs), len(repoNames))
	}

	// Check that all expected directories are present
	expectedMap := make(map[string]bool)
	for _, dir := range dirs {
		expectedMap[dir] = true
	}

	for _, repoName := range repoNames {
		if !expectedMap[repoName] {
			t.Errorf("Unexpected repository name: %s", repoName)
		}
		delete(expectedMap, repoName)
	}

	if len(expectedMap) > 0 {
		t.Errorf("Missing expected repository directories: %v", expectedMap)
	}
}

func TestCreateRepository(t *testing.T) {
	name := "test-repo"
	orgPath := "/tmp/test-org"
	organization := "test-org"

	repo := createRepository(name, orgPath, organization)

	if repo.Name != name {
		t.Errorf("Expected name %s, got %s", name, repo.Name)
	}
	if repo.Organization != organization {
		t.Errorf("Expected organization %s, got %s", organization, repo.Organization)
	}
	expectedPath := filepath.Join(orgPath, name)
	if repo.Path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, repo.Path)
	}
}

func TestGetEnvironmentVariables(t *testing.T) {
	// Set test environment variables
	testVars := map[string]string{
		"GITHUB_TOKEN":    "test-token-123",
		"GITHUB_ORGS":     "test-org1,test-org2",
		"GITHUB_BASE_URL": "https://api.github.test",
	}

	// Set environment variables
	for key, value := range testVars {
		os.Setenv(key, value)
		defer os.Unsetenv(key)
	}

	envVars := getEnvironmentVariables()

	for key, expectedValue := range testVars {
		if envVars[key] != expectedValue {
			t.Errorf("Expected %s=%s, got %s", key, expectedValue, envVars[key])
		}
	}
}

func TestCreateProcessingContext(t *testing.T) {
	validConfig := Config{
		MaxGoroutines:    10,
		CloneConcurrency: 5,
		GitHubToken:      "test-token",
		Organizations:    []string{"test-org"},
	}

	ctx, err := createProcessingContext(validConfig)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if ctx.Config.MaxGoroutines != validConfig.MaxGoroutines {
		t.Errorf("Expected MaxGoroutines %d, got %d", validConfig.MaxGoroutines, ctx.Config.MaxGoroutines)
	}

	if ctx.Pool == nil {
		t.Error("Expected pool to be created")
	}

	// Test cleanup
	releaseProcessingContext(ctx)

	// Test with invalid config
	invalidConfig := Config{
		MaxGoroutines: 0, // Invalid
	}

	_, err = createProcessingContext(invalidConfig)
	if err == nil {
		t.Error("Expected error with invalid config")
	}
}

func TestCreateResultChannel(t *testing.T) {
	repositories := []Repository{
		{Name: "repo1", Path: "/path/repo1", Organization: "org1"},
		{Name: "repo2", Path: "/path/repo2", Organization: "org1"},
		{Name: "repo3", Path: "/path/repo3", Organization: "org1"},
	}

	ch := createResultChannel(repositories)

	// Test that channel has correct capacity
	if cap(ch) != len(repositories) {
		t.Errorf("Expected channel capacity %d, got %d", len(repositories), cap(ch))
	}

	// Test that channel is not closed
	select {
	case <-ch:
		t.Error("Channel should not be closed initially")
	default:
		// Expected behavior
	}
}

func TestCreateTimeoutContext(t *testing.T) {
	timeout := 5 * time.Second
	ctx, cancel := createTimeoutContext(timeout)
	defer cancel()

	// Test that context has timeout
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("Context should have a deadline")
	}

	// Test that deadline is approximately correct (within 1 second)
	expectedDeadline := time.Now().Add(timeout)
	if deadline.Before(expectedDeadline.Add(-time.Second)) || deadline.After(expectedDeadline.Add(time.Second)) {
		t.Errorf("Deadline %v is not close to expected %v", deadline, expectedDeadline)
	}

	// Test context cancellation
	cancel()
	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be cancelled")
	}
}

func TestCalculateStats(t *testing.T) {
	// Create test results
	results := []AnalysisResult{
		{
			RepoName:     "repo1",
			Organization: "org1",
			Analysis: RepositoryAnalysis{
				ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 10},
			},
			Error: nil,
		},
		{
			RepoName:     "repo2",
			Organization: "org1",
			Analysis: RepositoryAnalysis{
				ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 15},
			},
			Error: nil,
		},
		{
			RepoName:     "repo3",
			Organization: "org1",
			Error:        fmt.Errorf("test error"),
		},
	}

	duration := 30 * time.Second
	stats := calculateStats(results, duration)

	if stats.TotalOrgs != 1 {
		t.Errorf("Expected TotalOrgs 1, got %d", stats.TotalOrgs)
	}
	if stats.TotalRepos != 3 {
		t.Errorf("Expected TotalRepos 3, got %d", stats.TotalRepos)
	}
	if stats.ProcessedRepos != 2 {
		t.Errorf("Expected ProcessedRepos 2, got %d", stats.ProcessedRepos)
	}
	if stats.FailedRepos != 1 {
		t.Errorf("Expected FailedRepos 1, got %d", stats.FailedRepos)
	}
	if stats.TotalFiles != 25 {
		t.Errorf("Expected TotalFiles 25, got %d", stats.TotalFiles)
	}
	if stats.Duration != duration {
		t.Errorf("Expected Duration %v, got %v", duration, stats.Duration)
	}
}

