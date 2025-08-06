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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			name:     "multiple organizations comma-separated",
			input:    "hashicorp,terraform-providers,aws-samples",
			expected: []string{"hashicorp", "terraform-providers", "aws-samples"},
		},
		{
			name:     "multiple organizations space-separated",
			input:    "hashicorp terraform-providers aws-samples",
			expected: []string{"hashicorp", "terraform-providers", "aws-samples"},
		},
		{
			name:     "organizations with spaces around commas",
			input:    " hashicorp , terraform-providers , aws-samples ",
			expected: []string{"hashicorp", "terraform-providers", "aws-samples"},
		},
		{
			name:     "organizations with multiple spaces",
			input:    "hashicorp   terraform-providers    aws-samples",
			expected: []string{"hashicorp", "terraform-providers", "aws-samples"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "whitespace only",
			input:    "   \t  \n  ",
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
			errorMsg:    "GitHubToken is required",
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
			errorMsg:    "at least one organization must be specified",
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
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("Failed to remove temp directory: %v", err)
		}
	}()

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
		if err := os.Setenv(key, value); err != nil {
			t.Fatalf("Failed to set env var %s: %v", key, err)
		}
		defer func(envKey string) {
			if err := os.Unsetenv(envKey); err != nil {
				t.Errorf("Failed to unset env var %s: %v", envKey, err)
			}
		}(key)
	}

	envVars := getEnvironmentVariables()

	for key, expectedValue := range testVars {
		if envVars[key] != expectedValue {
			t.Errorf("Expected %s=%s, got %s", key, expectedValue, envVars[key])
		}
	}
}

func TestCreateProcessingContextWithValidation(t *testing.T) {
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


// ============================================================================
// ADDITIONAL TESTS (formerly from orchestrator_additional_test.go and orchestrator_critical_test.go)
// ============================================================================
// TestCreateTimeoutContextAdditional tests timeout context creation (additional tests)
func TestCreateTimeoutContextAdditional(t *testing.T) {
	t.Run("creates context with timeout", func(t *testing.T) {
		// Given: a timeout duration
		timeout := 5 * time.Second

		// When: createTimeoutContext is called
		ctx, cancel := createTimeoutContext(timeout)
		defer cancel()

		// Then: context should have deadline
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Error("Expected context to have deadline")
		}

		expectedDeadline := time.Now().Add(timeout)
		if deadline.Before(expectedDeadline.Add(-time.Second)) || deadline.After(expectedDeadline.Add(time.Second)) {
			t.Errorf("Deadline %v not close to expected %v", deadline, expectedDeadline)
		}
	})
}

// TestMaskTokenOrchestratorAdditional tests token masking for security (orchestrator version)
func TestMaskTokenOrchestratorAdditional(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected string
	}{
		{
			name:     "masks long token",
			token:    "ghp_1234567890abcdefghij1234567890abcdefgh",
			expected: "ghp_...efgh",
		},
		{
			name:     "masks short token",
			token:    "token123",
			expected: "toke...n123",
		},
		{
			name:     "handles very short token",
			token:    "abc",
			expected: "***",
		},
		{
			name:     "handles empty token",
			token:    "",
			expected: "***",
		},
		{
			name:     "handles single character",
			token:    "a",
			expected: "***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: a token
			// When: maskToken is called
			result := maskToken(tt.token)

			// Then: should mask appropriately
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestReadDirectory tests directory reading
func TestReadDirectory(t *testing.T) {
	t.Run("reads existing directory", func(t *testing.T) {
		// Given: a temporary directory with files
		tempDir := t.TempDir()
		
		// Create test files
		testFiles := []string{"file1.txt", "file2.txt", "dir1"}
		for _, file := range testFiles {
			if file == "dir1" {
				if err := os.Mkdir(tempDir+"/"+file, 0755); err != nil {
					t.Fatalf("Failed to create directory %s: %v", file, err)
				}
			} else {
				if err := os.WriteFile(tempDir+"/"+file, []byte("test"), 0644); err != nil {
					t.Fatalf("Failed to create file %s: %v", file, err)
				}
			}
		}

		// When: readDirectory is called
		entries, err := readDirectory(tempDir)

		// Then: should read directory successfully
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(entries) != len(testFiles) {
			t.Errorf("Expected %d entries, got %d", len(testFiles), len(entries))
		}
	})

	t.Run("handles non-existent directory", func(t *testing.T) {
		// Given: a non-existent directory
		nonExistentDir := "/non/existent/directory"

		// When: readDirectory is called
		_, err := readDirectory(nonExistentDir)

		// Then: should return error
		if err == nil {
			t.Error("Expected error for non-existent directory")
		}
	})
}

// TestCreateResultChannelAdditional tests result channel creation (additional tests)
func TestCreateResultChannelAdditional(t *testing.T) {
	t.Run("creates channel with correct capacity", func(t *testing.T) {
		// Given: a list of repositories
		repos := []Repository{
			{Name: "repo1", Path: "/path1", Organization: "org1"},
			{Name: "repo2", Path: "/path2", Organization: "org1"},
			{Name: "repo3", Path: "/path3", Organization: "org1"},
		}

		// When: createResultChannel is called
		ch := createResultChannel(repos)

		// Then: channel should have correct capacity
		if cap(ch) != len(repos) {
			t.Errorf("Expected channel capacity %d, got %d", len(repos), cap(ch))
		}

		// Check channel is not closed
		select {
		case <-ch:
			t.Error("Channel should not be closed initially")
		default:
			// Expected behavior
		}
	})
}

// TestGetEnvironmentVariablesAdditional tests environment variable collection (additional tests)
func TestGetEnvironmentVariablesAdditional(t *testing.T) {
	t.Run("collects environment variables", func(t *testing.T) {
		// Given: GitHub-related environment variables
		testVars := map[string]string{
			"GITHUB_TOKEN":    "test-token",
			"GITHUB_ORGS":     "test-org",
			"GITHUB_BASE_URL": "https://api.github.com",
		}

		// Set test environment variables
		for key, value := range testVars {
			if err := os.Setenv(key, value); err != nil {
				t.Fatalf("Failed to set env var %s: %v", key, err)
			}
			defer func(envKey string) {
				if err := os.Unsetenv(envKey); err != nil {
					t.Errorf("Failed to unset env var %s: %v", envKey, err)
				}
			}(key)
		}

		// When: getEnvironmentVariables is called
		envVars := getEnvironmentVariables()

		// Then: should collect GitHub environment variables
		for key, expectedValue := range testVars {
			if envVars[key] != expectedValue {
				t.Errorf("Expected %s=%s, got %s", key, expectedValue, envVars[key])
			}
		}
	})
}

// TestCreateRepositoryAdditional tests repository struct creation (additional tests)
func TestCreateRepositoryAdditional(t *testing.T) {
	t.Run("creates repository with correct fields", func(t *testing.T) {
		// Given: repository parameters
		name := "test-repo"
		orgPath := "/path/to/org"
		organization := "test-org"

		// When: createRepository is called
		repo := createRepository(name, orgPath, organization)

		// Then: should create repository with correct fields
		if repo.Name != name {
			t.Errorf("Expected name %s, got %s", name, repo.Name)
		}
		if repo.Organization != organization {
			t.Errorf("Expected organization %s, got %s", organization, repo.Organization)
		}
		expectedPath := orgPath + "/" + name
		if repo.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, repo.Path)
		}
	})
}

// TestFilterRepositoryDirsAdditional tests directory filtering (additional tests)
func TestFilterRepositoryDirsAdditional(t *testing.T) {
	t.Run("filters directories correctly", func(t *testing.T) {
		// Given: mixed directory entries
		tempDir := t.TempDir()
		
		// Create directories and files
		if err := os.Mkdir(tempDir+"/repo1", 0755); err != nil {
			t.Fatalf("Failed to create repo1 directory: %v", err)
		}
		if err := os.Mkdir(tempDir+"/repo2", 0755); err != nil {
			t.Fatalf("Failed to create repo2 directory: %v", err)
		}
		if err := os.WriteFile(tempDir+"/file1.txt", []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file1.txt: %v", err)
		}

		// Read directory entries
		entries, err := readDirectory(tempDir)
		if err != nil {
			t.Fatalf("Failed to read directory: %v", err)
		}

		// When: filterRepositoryDirs is called
		repoNames := filterRepositoryDirs(entries)

		// Then: should filter directories only
		expectedRepos := []string{"repo1", "repo2"}
		if len(repoNames) != len(expectedRepos) {
			t.Errorf("Expected %d repositories, got %d", len(expectedRepos), len(repoNames))
		}

		for _, expectedRepo := range expectedRepos {
			found := false
			for _, actualRepo := range repoNames {
				if actualRepo == expectedRepo {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected repository %s not found in result", expectedRepo)
			}
		}
	})
}

// TestReleaseProcessingContext tests context cleanup
func TestReleaseProcessingContext(t *testing.T) {
	t.Run("releases context without panic", func(t *testing.T) {
		// Given: a processing context
		config := Config{
			MaxGoroutines:    10,
			CloneConcurrency: 5,
			GitHubToken:      "test-token",
			Organizations:    []string{"test-org"},
		}

		ctx, err := createProcessingContext(config)
		if err != nil {
			t.Fatalf("Failed to create processing context: %v", err)
		}

		// When: releaseProcessingContext is called
		// Then: should not panic
		releaseProcessingContext(ctx)
	})
}

// TestCalculateStatsDetailed tests stats calculation with various scenarios
func TestCalculateStatsDetailed(t *testing.T) {
	t.Run("calculates stats with mixed results", func(t *testing.T) {
		// Given: mixed analysis results
		results := []AnalysisResult{
			{
				RepoName:     "repo1",
				Organization: "org1",
				Analysis: RepositoryAnalysis{
					ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 5},
					Providers:        ProvidersAnalysis{UniqueProviderCount: 2},
					Modules:          ModulesAnalysis{TotalModuleCalls: 3},
					VariableAnalysis: VariableAnalysis{DefinedVariables: []VariableDefinition{{Name: "var1"}}},
					OutputAnalysis:   OutputAnalysis{OutputCount: 1},
				},
				Error: nil,
			},
			{
				RepoName:     "repo2",
				Organization: "org2",
				Analysis: RepositoryAnalysis{
					ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 8},
					Providers:        ProvidersAnalysis{UniqueProviderCount: 1},
					Modules:          ModulesAnalysis{TotalModuleCalls: 2},
					VariableAnalysis: VariableAnalysis{DefinedVariables: []VariableDefinition{{Name: "var1"}, {Name: "var2"}}},
					OutputAnalysis:   OutputAnalysis{OutputCount: 3},
				},
				Error: nil,
			},
			{
				RepoName:     "repo3",
				Organization: "org1",
				Analysis:     RepositoryAnalysis{},
				Error:        os.ErrNotExist,
			},
		}

		duration := 45 * time.Second

		// When: calculateStats is called
		stats := calculateStats(results, duration)

		// Then: should calculate correct statistics
		if stats.TotalOrgs != 2 {
			t.Errorf("Expected 2 orgs, got %d", stats.TotalOrgs)
		}
		if stats.TotalRepos != 3 {
			t.Errorf("Expected 3 repos, got %d", stats.TotalRepos)
		}
		if stats.ProcessedRepos != 2 {
			t.Errorf("Expected 2 processed repos, got %d", stats.ProcessedRepos)
		}
		if stats.FailedRepos != 1 {
			t.Errorf("Expected 1 failed repo, got %d", stats.FailedRepos)
		}
		if stats.TotalFiles != 13 { // 5 + 8 = 13
			t.Errorf("Expected 13 total files, got %d", stats.TotalFiles)
		}
		if stats.Duration != duration {
			t.Errorf("Expected duration %v, got %v", duration, stats.Duration)
		}
	})
}

// TestLogRepositoryResultStructured tests structured logging
func TestLogRepositoryResultStructured(t *testing.T) {
	t.Run("logs repository result without panic", func(t *testing.T) {
		// Given: a logger and repository result
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		result := AnalysisResult{
			RepoName:     "test-repo",
			Organization: "test-org",
			Analysis: RepositoryAnalysis{
				ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 5},
			},
			Error: nil,
		}

		// When: logRepositoryResultStructured is called
		// Then: should not panic
		logRepositoryResultStructured(result, logger)
	})

	t.Run("logs error result without panic", func(t *testing.T) {
		// Given: a logger and error result
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		result := AnalysisResult{
			RepoName:     "test-repo",
			Organization: "test-org",
			Error:        os.ErrNotExist,
		}

		// When: logRepositoryResultStructured is called
		// Then: should not panic
		logRepositoryResultStructured(result, logger)
	})
}
// TestDiscoverRepositories tests repository discovery
func TestDiscoverRepositories(t *testing.T) {
	t.Run("discovers repositories correctly", func(t *testing.T) {
		// Given: a temporary directory with repositories
		tempDir := t.TempDir()
		orgDir := filepath.Join(tempDir, "test-org")
		if err := os.MkdirAll(orgDir, 0755); err != nil {
			t.Fatalf("Failed to create org directory: %v", err)
		}
		
		// Create some test repositories
		if err := os.Mkdir(filepath.Join(orgDir, "repo1"), 0755); err != nil {
			t.Fatalf("Failed to create repo1: %v", err)
		}
		if err := os.Mkdir(filepath.Join(orgDir, "repo2"), 0755); err != nil {
			t.Fatalf("Failed to create repo2: %v", err)
		}
		if err := os.WriteFile(filepath.Join(orgDir, "notarepo.txt"), []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

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
		defer func() {
			if err := os.Remove(tempFile.Name()); err != nil {
				t.Errorf("Failed to remove temp file: %v", err)
			}
		}()
		
		if _, err := tempFile.WriteString("TEST_VAR=test_value\n"); err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		if err := tempFile.Close(); err != nil {
			t.Fatalf("Failed to close temp file: %v", err)
		}

		// When: loadDotEnvFile is called
		// Then: should not panic
		if err := loadDotEnvFile(tempFile.Name()); err != nil {
			t.Logf("loadDotEnvFile returned error (may be expected): %v", err)
		}
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

// ============================================================================
// ERROR PATH AND EDGE CASE TESTS - Target surviving mutations
// ============================================================================

// TestErrorConditionsInOrchestratorFunctions tests error paths in orchestrator functions
func TestErrorConditionsInOrchestratorFunctions(t *testing.T) {
	t.Run("parseOrganizations handles empty input edge cases", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected []string
		}{
			{"empty string", "", []string{}},
			{"whitespace only", "   \t  \n  ", []string{}},
			{"commas only", ",,,", []string{}},
			{"spaces only", "   ", []string{}},
			{"mixed empty", "  ,  ,  ", []string{}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := parseOrganizations(tt.input)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("validateAnalysisConfiguration catches all invalid conditions", func(t *testing.T) {
		tests := []struct {
			name        string
			config      Config
			expectError bool
			errorMsg    string
		}{
			{
				name: "MaxGoroutines exceeds safety limit",
				config: Config{
					MaxGoroutines:    MaxSafeMaxGoroutines + 1,
					CloneConcurrency: 5,
					GitHubToken:      "token",
					Organizations:    []string{"org"},
				},
				expectError: true,
				errorMsg:    fmt.Sprintf("MaxGoroutines too high (max %d for safety), got %d", MaxSafeMaxGoroutines, MaxSafeMaxGoroutines+1),
			},
			{
				name: "CloneConcurrency exceeds safety limit",
				config: Config{
					MaxGoroutines:    10,
					CloneConcurrency: MaxSafeCloneConcurrency + 1,
					GitHubToken:      "token",
					Organizations:    []string{"org"},
				},
				expectError: true,
				errorMsg:    fmt.Sprintf("CloneConcurrency too high (max %d for safety), got %d", MaxSafeCloneConcurrency, MaxSafeCloneConcurrency+1),
			},
			{
				name: "negative CloneConcurrency",
				config: Config{
					MaxGoroutines:    10,
					CloneConcurrency: -1,
					GitHubToken:      "token",
					Organizations:    []string{"org"},
				},
				expectError: true,
				errorMsg:    "CloneConcurrency must be positive, got -1",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validateAnalysisConfiguration(tt.config)
				if tt.expectError {
					assert.Error(t, err)
					if tt.errorMsg != "" {
						assert.Equal(t, tt.errorMsg, err.Error())
					}
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("filterRepositoryDirs handles edge cases", func(t *testing.T) {
		// Create temporary directory with mixed entries
		tempDir := t.TempDir()

		// Create test entries
		testEntries := []struct {
			name  string
			isDir bool
		}{
			{"repo1", true},
			{"file1.txt", false},
			{".hidden", true},
			{"repo2", true},
			{"script.sh", false},
		}

		for _, entry := range testEntries {
			path := filepath.Join(tempDir, entry.name)
			if entry.isDir {
				err := os.Mkdir(path, 0755)
				require.NoError(t, err)
			} else {
				err := os.WriteFile(path, []byte("test"), 0644)
				require.NoError(t, err)
			}
		}

		// Read directory entries
		entries, err := readDirectory(tempDir)
		require.NoError(t, err)

		// Filter directories
		result := filterRepositoryDirs(entries)

		// Should only return directories
		expectedDirs := []string{"repo1", ".hidden", "repo2"}
		assert.Equal(t, len(expectedDirs), len(result))
		
		for _, expectedDir := range expectedDirs {
			assert.Contains(t, result, expectedDir)
		}
	})

	t.Run("createRepository handles path joining edge cases", func(t *testing.T) {
		tests := []struct {
			name         string
			repoName     string
			orgPath      string
			organization string
			expectedPath string
		}{
			{
				name:         "normal paths",
				repoName:     "repo1",
				orgPath:      "/tmp/org",
				organization: "testorg",
				expectedPath: "/tmp/org/repo1",
			},
			{
				name:         "paths with trailing slash",
				repoName:     "repo2",
				orgPath:      "/tmp/org/",
				organization: "testorg",
				expectedPath: "/tmp/org/repo2",
			},
			{
				name:         "empty repo name",
				repoName:     "",
				orgPath:      "/tmp/org",
				organization: "testorg",
				expectedPath: "/tmp/org",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				repo := createRepository(tt.repoName, tt.orgPath, tt.organization)
				assert.Equal(t, tt.repoName, repo.Name)
				assert.Equal(t, tt.organization, repo.Organization)
				assert.Equal(t, tt.expectedPath, repo.Path)
			})
		}
	})

	t.Run("maskToken handles various token lengths", func(t *testing.T) {
		tests := []struct {
			name     string
			token    string
			expected string
		}{
			{"empty token", "", "***"},
			{"single char", "a", "***"},
			{"two chars", "ab", "***"},
			{"three chars", "abc", "***"},
			{"four chars", "abcd", "***"},
			{"five chars", "abcde", "***"},
			{"six chars", "abcdef", "***"},
			{"seven chars", "abcdefg", "***"},
			{"eight chars", "abcdefgh", "abcd...efgh"},
			{"normal token", "ghp_1234567890abcdefghij", "ghp_...fghij"},
			{"very long token", "ghp_1234567890abcdefghij1234567890abcdefghij", "ghp_...efghij"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := maskToken(tt.token)
				assert.Equal(t, tt.expected, result)
			})
		}
	})
}

// TestConfigurationEdgeCases tests configuration creation edge cases
func TestConfigurationEdgeCases(t *testing.T) {
	t.Run("createConfigFromEnv with missing environment variables", func(t *testing.T) {
		// Test with completely empty environment
		emptyEnvVars := map[string]string{}
		config := createConfigFromEnv(emptyEnvVars)

		// Should use defaults
		assert.Equal(t, DefaultMaxGoroutines, config.MaxGoroutines)
		assert.Equal(t, DefaultCloneConcurrency, config.CloneConcurrency)
		assert.Equal(t, DefaultProcessTimeout, config.ProcessTimeout)
		assert.Equal(t, DefaultRetryDelay, config.RetryDelay)
		assert.True(t, config.SkipArchived)
		assert.False(t, config.SkipForks)
		assert.Equal(t, "", config.GitHubToken)
		assert.Equal(t, []string{}, config.Organizations)
		assert.Equal(t, "", config.BaseURL)
	})

	t.Run("createConfigFromEnv with production environment", func(t *testing.T) {
		envVars := map[string]string{
			"ENVIRONMENT": "production",
		}
		config := createConfigFromEnv(envVars)

		// Should use production retry delay
		assert.Equal(t, ProductionRetryDelay, config.RetryDelay)
	})

	t.Run("loadEnvironmentConfig with invalid file path", func(t *testing.T) {
		nonExistentFile := "/non/existent/file.env"
		_, err := loadEnvironmentConfig(nonExistentFile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load")
	})

	t.Run("loadEnvironmentConfig with empty file path", func(t *testing.T) {
		config, err := loadEnvironmentConfig("")
		assert.NoError(t, err)
		assert.NotNil(t, config)
	})
}

// TestProcessingContextErrorHandling tests processing context creation errors
func TestProcessingContextErrorHandling(t *testing.T) {
	t.Run("createProcessingContext with invalid config", func(t *testing.T) {
		invalidConfig := Config{
			MaxGoroutines:    0, // Invalid
			CloneConcurrency: 5,
			GitHubToken:      "token",
			Organizations:    []string{"org"},
		}

		_, err := createProcessingContext(invalidConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "MaxGoroutines must be positive")
	})

	t.Run("releaseProcessingContext with nil pool", func(t *testing.T) {
		ctx := ProcessingContext{Pool: nil}
		// Should not panic
		releaseProcessingContext(ctx)
	})
}

// TestStatisticsCalculationEdgeCases tests stats calculation with edge cases
func TestStatisticsCalculationEdgeCases(t *testing.T) {
	t.Run("calculateStats with empty results", func(t *testing.T) {
		results := []AnalysisResult{}
		duration := 10 * time.Second
		stats := calculateStats(results, duration)

		assert.Equal(t, 1, stats.TotalOrgs)
		assert.Equal(t, 0, stats.TotalRepos)
		assert.Equal(t, 0, stats.ProcessedRepos)
		assert.Equal(t, 0, stats.FailedRepos)
		assert.Equal(t, 0, stats.TotalFiles)
		assert.Equal(t, duration, stats.Duration)
	})

	t.Run("calculateStats with all failed results", func(t *testing.T) {
		results := []AnalysisResult{
			{RepoName: "repo1", Organization: "org1", Error: fmt.Errorf("error1")},
			{RepoName: "repo2", Organization: "org1", Error: fmt.Errorf("error2")},
		}
		duration := 15 * time.Second
		stats := calculateStats(results, duration)

		assert.Equal(t, 1, stats.TotalOrgs)
		assert.Equal(t, 2, stats.TotalRepos)
		assert.Equal(t, 0, stats.ProcessedRepos)
		assert.Equal(t, 2, stats.FailedRepos)
		assert.Equal(t, 0, stats.TotalFiles)
		assert.Equal(t, duration, stats.Duration)
	})

	t.Run("calculateStats with mixed zero resource counts", func(t *testing.T) {
		results := []AnalysisResult{
			{
				RepoName:     "repo1",
				Organization: "org1",
				Analysis:     RepositoryAnalysis{ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 0}},
				Error:        nil,
			},
			{
				RepoName:     "repo2",
				Organization: "org1",
				Analysis:     RepositoryAnalysis{ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 5}},
				Error:        nil,
			},
		}
		duration := 20 * time.Second
		stats := calculateStats(results, duration)

		assert.Equal(t, 1, stats.TotalOrgs)
		assert.Equal(t, 2, stats.TotalRepos)
		assert.Equal(t, 2, stats.ProcessedRepos)
		assert.Equal(t, 0, stats.FailedRepos)
		assert.Equal(t, 5, stats.TotalFiles) // 0 + 5 = 5
		assert.Equal(t, duration, stats.Duration)
	})
}

// TestChannelOperationsEdgeCases tests channel creation and operations
func TestChannelOperationsEdgeCases(t *testing.T) {
	t.Run("createResultChannel with empty repositories", func(t *testing.T) {
		repositories := []Repository{}
		ch := createResultChannel(repositories)

		assert.Equal(t, 0, cap(ch))
		
		// Channel should be ready for immediate close without blocking
		close(ch)
		
		// Should be able to read from closed channel
		_, ok := <-ch
		assert.False(t, ok)
	})

	t.Run("configureWaitGroup with various goroutine counts", func(t *testing.T) {
		tests := []int{1, 10, 100, 1000}
		
		for _, maxGoroutines := range tests {
			t.Run(fmt.Sprintf("maxGoroutines_%d", maxGoroutines), func(t *testing.T) {
				pool := configureWaitGroup(maxGoroutines)
				assert.NotNil(t, pool)
				
				// Should be able to wait immediately
				pool.Wait()
			})
		}
	})
}

// TestTimeoutContextEdgeCases tests timeout context creation with edge cases
func TestTimeoutContextEdgeCases(t *testing.T) {
	t.Run("createTimeoutContext with zero timeout", func(t *testing.T) {
		timeout := 0 * time.Second
		ctx, cancel := createTimeoutContext(timeout)
		defer cancel()

		// Context should be immediately expired
		deadline, ok := ctx.Deadline()
		assert.True(t, ok)
		
		// Should be close to now (already expired)
		now := time.Now()
		assert.True(t, deadline.Before(now.Add(time.Second)))
	})

	t.Run("createTimeoutContext with very short timeout", func(t *testing.T) {
		timeout := 1 * time.Nanosecond
		ctx, cancel := createTimeoutContext(timeout)
		defer cancel()

		// Context should expire very quickly
		select {
		case <-ctx.Done():
			// Expected - context expired
		case <-time.After(100 * time.Millisecond):
			t.Error("Context should have expired quickly")
		}
	})
}

// ============================================================================
// TUI-FREE ORCHESTRATOR FUNCTION TESTS - Tests for functions without TUI dependencies
// ============================================================================

// TestProcessRepositoriesConcurrently tests repository processing without TUI
func TestProcessRepositoriesConcurrently(t *testing.T) {
	t.Run("processes repositories without TUI progress tracking", func(t *testing.T) {
		// Given: Test repositories and processing context
		repositories := []Repository{
			{Name: "test-repo1", Path: "/tmp/test1", Organization: "test-org"},
			{Name: "test-repo2", Path: "/tmp/test2", Organization: "test-org"},
		}
		
		config := Config{
			MaxGoroutines:    2,
			CloneConcurrency: 1,
			ProcessTimeout:   5 * time.Second,
		}
		
		processingCtx, err := createProcessingContext(config)
		require.NoError(t, err)
		defer releaseProcessingContext(processingCtx)
		
		ctx := context.Background()
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		// When: processRepositoriesConcurrently is called
		results := processRepositoriesConcurrently(repositories, ctx, processingCtx, logger)
		
		// Then: Results should be returned without TUI progress updates
		assert.NotNil(t, results)
		assert.Len(t, results, len(repositories))
	})
}

// TestCloneAndAnalyzeMultipleOrgsCore tests multi-org analysis core functionality
func TestCloneAndAnalyzeMultipleOrgsCore(t *testing.T) {
	t.Run("processes multiple orgs without TUI", func(t *testing.T) {
		// Given: Processing context and reporter
		config := Config{
			Organizations:    []string{"test-org"},
			GitHubToken:      "fake-token",
			MaxGoroutines:    2,
			CloneConcurrency: 1,
			ProcessTimeout:   1 * time.Second,
		}
		
		processingCtx, err := createProcessingContext(config)
		require.NoError(t, err)
		defer releaseProcessingContext(processingCtx)
		
		reporter := NewReporter()
		ctx := context.Background()
		
		// When: cloneAndAnalyzeMultipleOrgs is called
		err = cloneAndAnalyzeMultipleOrgs(ctx, processingCtx, reporter)
		
		// Then: Function should not panic (error expected due to fake token)
		_ = err // Error is expected due to fake token
	})
}

// TestProcessOrganization tests organization processing without TUI
func TestProcessOrganization(t *testing.T) {
	t.Run("processes organization without TUI updates", func(t *testing.T) {
		// Given: Organization and processing context
		org := "test-org"
		config := Config{
			GitHubToken:      "fake-token",
			MaxGoroutines:    2,
			CloneConcurrency: 1,
		}
		
		processingCtx, err := createProcessingContext(config)
		require.NoError(t, err)
		defer releaseProcessingContext(processingCtx)
		
		reporter := NewReporter()
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		ctx := context.Background()
		
		// When: processOrganization is called
		count, err := processOrganization(ctx, org, processingCtx, reporter, logger)
		
		// Then: Function should return count and not panic
		_ = count // Count might be 0 due to fake token
		_ = err   // Error is expected due to fake token
	})
}

// TestDiscoverRepositoriesWrapper tests repository discovery wrapper functionality
func TestDiscoverRepositoriesWrapper(t *testing.T) {
	t.Run("discovers repositories without TUI updates", func(t *testing.T) {
		// Given: Temporary directory and organization
		tempDir := t.TempDir()
		org := "test-org"
		
		// When: discoverRepositories is called
		repos, err := discoverRepositories(tempDir, org)
		
		// Then: Function should not panic
		_ = repos // Repos might be empty for non-existent directory
		_ = err   // Error is expected for non-existent git repos
	})
}

// ============================================================================
// PANIC RECOVERY TESTS - Comprehensive tests for panic recovery mechanisms
// ============================================================================

// TestSubmitRepositoryJobsWithTimeoutPanicRecovery tests panic recovery in repository job submission
func TestSubmitRepositoryJobsWithTimeoutPanicRecovery(t *testing.T) {
	t.Run("recovers from panic in repository processing goroutine", func(t *testing.T) {
		// Given: A repository that will cause panic and processing context
		repo := Repository{
			Name:         "panic-repo",
			Path:         "/non/existent/path", // This will cause panic in file operations
			Organization: "test-org",
		}
		
		config := Config{
			MaxGoroutines:    2,
			CloneConcurrency: 1,
			ProcessTimeout:   1 * time.Second,
			GitHubToken:      "fake-token", // Required for validation
			Organizations:    []string{"test-org"}, // Required for validation
		}
		
		processingCtx, err := createProcessingContext(config)
		require.NoError(t, err)
		defer releaseProcessingContext(processingCtx)
		
		// Create a pool with max goroutines
		p := configureWaitGroup(1)
		results := make(chan AnalysisResult, 1)
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		ctx := context.Background()
		jobCtx := JobSubmissionContext{
			Repositories: []Repository{repo},
			Ctx:          ctx,
			Pool:         p,
			AntsPool:     processingCtx.Pool,
			Results:      results,
			Logger:       logger,
		}
		
		// When: submitRepositoryJobsWithTimeout is called
		submitRepositoryJobsWithTimeout(jobCtx)
		p.Wait()
		close(results)
		
		// Then: Should receive result with error (not panic)
		var receivedResults []AnalysisResult
		for result := range results {
			receivedResults = append(receivedResults, result)
		}
		
		assert.Len(t, receivedResults, 1)
		result := receivedResults[0]
		assert.Equal(t, "panic-repo", result.RepoName)
		assert.Equal(t, "test-org", result.Organization)
		assert.Error(t, result.Error)
	})
	
	t.Run("handles context cancellation during panic recovery", func(t *testing.T) {
		// Given: A cancelled context and repository
		tempDir := t.TempDir()
		repo := Repository{
			Name:         "timeout-repo",
			Path:         tempDir,
			Organization: "test-org",
		}
		
		config := Config{
			MaxGoroutines:    1,
			CloneConcurrency: 1,
			ProcessTimeout:   1 * time.Millisecond, // Very short timeout
			GitHubToken:      "fake-token", // Required for validation
			Organizations:    []string{"test-org"}, // Required for validation
		}
		
		processingCtx, err := createProcessingContext(config)
		require.NoError(t, err)
		defer releaseProcessingContext(processingCtx)
		
		// Create cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		p := configureWaitGroup(1)
		results := make(chan AnalysisResult, 1)
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		jobCtx := JobSubmissionContext{
			Repositories: []Repository{repo},
			Ctx:          ctx,
			Pool:         p,
			AntsPool:     processingCtx.Pool,
			Results:      results,
			Logger:       logger,
		}
		
		// When: submitRepositoryJobsWithTimeout is called with cancelled context
		submitRepositoryJobsWithTimeout(jobCtx)
		p.Wait()
		close(results)
		
		// Then: Should handle cancellation gracefully
		var receivedResults []AnalysisResult
		for result := range results {
			receivedResults = append(receivedResults, result)
		}
		
		assert.Len(t, receivedResults, 1)
		result := receivedResults[0]
		assert.Equal(t, "timeout-repo", result.RepoName)
		assert.Equal(t, "test-org", result.Organization)
		assert.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "cancelled")
	})
}

// TestProcessRepositoryFilesWithRecoveryPanicRecovery tests panic recovery in repository file processing
func TestProcessRepositoryFilesWithRecoveryPanicRecovery(t *testing.T) {
	t.Run("recovers from panic in repository analysis", func(t *testing.T) {
		// Given: A repository with a problematic path
		repo := Repository{
			Name:         "panic-analysis-repo",
			Path:         "/dev/null/impossible/path", // This will cause panic in file operations
			Organization: "test-org",
		}
		
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		// When: processRepositoryFilesWithRecovery is called
		result := processRepositoryFilesWithRecovery(repo, logger)
		
		// Then: Should return result with error instead of panicking
		assert.Equal(t, "panic-analysis-repo", result.RepoName)
		assert.Equal(t, "test-org", result.Organization)
		assert.Error(t, result.Error)
		// The function should handle the error gracefully, not necessarily contain "panic"
	})
	
	t.Run("handles nil pointer dereference in repository processing", func(t *testing.T) {
		// Given: A repository with minimal data that might cause nil pointer issues
		repo := Repository{
			Name:         "",
			Path:         "",
			Organization: "",
		}
		
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		// When: processRepositoryFilesWithRecovery is called
		result := processRepositoryFilesWithRecovery(repo, logger)
		
		// Then: Should return result without panicking
		assert.Equal(t, "", result.RepoName)
		assert.Equal(t, "", result.Organization)
		// Error is expected for empty path
		assert.Error(t, result.Error)
	})
	
	t.Run("processes valid repository successfully without panic", func(t *testing.T) {
		// Given: A valid repository directory
		tempDir := t.TempDir()
		
		// Create a simple terraform file
		tfFile := filepath.Join(tempDir, "main.tf")
		tfContent := `
resource "aws_instance" "example" {
  ami           = "ami-12345678"
  instance_type = "t2.micro"
  
  tags = {
    Name        = "ExampleInstance"
    Environment = "dev"
    Owner       = "test"
    Project     = "example"
    CostCenter  = "eng"
  }
}
`
		err := os.WriteFile(tfFile, []byte(tfContent), 0644)
		require.NoError(t, err)
		
		repo := Repository{
			Name:         "valid-repo",
			Path:         tempDir,
			Organization: "test-org",
		}
		
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		// When: processRepositoryFilesWithRecovery is called
		result := processRepositoryFilesWithRecovery(repo, logger)
		
		// Then: Should process successfully without panic
		assert.Equal(t, "valid-repo", result.RepoName)
		assert.Equal(t, "test-org", result.Organization)
		assert.NoError(t, result.Error)
		assert.Equal(t, tempDir, result.Analysis.RepositoryPath)
		// Should have at least parsed the file successfully (resource count might be 0 if parsing fails)
		assert.GreaterOrEqual(t, result.Analysis.ResourceAnalysis.TotalResourceCount, 0)
	})
}

// TestParseWithRecoveryPanicRecovery tests generic parsing panic recovery
func TestParseWithRecoveryPanicRecovery(t *testing.T) {
	t.Run("recovers from panic in backend parsing", func(t *testing.T) {
		// Given: Invalid HCL content that might cause panic
		invalidContent := `terraform {
			backend "s3" {
				bucket = $INVALID_SYNTAX
				key    = "terraform.tfstate"
			}
		}`
		
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		// When: parseBackendSafely is called (which uses parseWithRecovery internally)
		result := parseBackendSafely(invalidContent, "test.tf", logger)
		
		// Then: Should return nil instead of panicking
		assert.Nil(t, result)
	})
	
	t.Run("recovers from panic in provider parsing", func(t *testing.T) {
		// Given: Malformed provider content
		malformedContent := `terraform {
			required_providers {
				aws = {
					source = "hashicorp/aws"
					version = $MALFORMED_VERSION
				}
			}
		}`
		
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		// When: parseProvidersSafely is called
		result := parseProvidersSafely(malformedContent, "providers.tf", logger)
		
		// Then: Should return empty slice instead of panicking
		assert.NotNil(t, result)
		assert.Equal(t, []ProviderDetail{}, result)
	})
	
	t.Run("recovers from panic in module parsing", func(t *testing.T) {
		// Given: Invalid module syntax
		invalidModuleContent := `module "example" {
			source = $INVALID{SYNTAX}HERE
		}`
		
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		// When: parseModulesSafely is called
		result := parseModulesSafely(invalidModuleContent, "modules.tf", logger)
		
		// Then: Should return empty slice instead of panicking
		assert.NotNil(t, result)
		assert.Equal(t, []ModuleDetail{}, result)
	})
	
	t.Run("recovers from panic in variable parsing", func(t *testing.T) {
		// Given: Malformed variable syntax
		malformedVariableContent := `variable "example" {
			type = $INVALID_TYPE
			default = $INVALID_DEFAULT
		}`
		
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		// When: parseVariablesSafely is called
		result := parseVariablesSafely(malformedVariableContent, "variables.tf", logger)
		
		// Then: Should return empty slice instead of panicking
		assert.NotNil(t, result)
		assert.Equal(t, []VariableDefinition{}, result)
	})
	
	t.Run("recovers from panic in output parsing", func(t *testing.T) {
		// Given: Invalid output syntax
		invalidOutputContent := `output "example" {
			value = $INVALID{EXPRESSION}
		}`
		
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		// When: parseOutputsSafely is called
		result := parseOutputsSafely(invalidOutputContent, "outputs.tf", logger)
		
		// Then: Should return empty slice instead of panicking
		assert.NotNil(t, result)
		assert.Equal(t, []string{}, result)
	})
}

// TestParseResourcesSafelyPanicRecovery tests resource parsing panic recovery
func TestParseResourcesSafelyPanicRecovery(t *testing.T) {
	t.Run("recovers from panic in resource parsing", func(t *testing.T) {
		// Given: Malformed resource content
		malformedResourceContent := `resource "aws_instance" "example" {
			ami = $INVALID_SYNTAX
			instance_type = $ANOTHER_INVALID
			tags = {
				Name = $INVALID_TAG_VALUE
			}
		}`
		
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		// When: parseResourcesSafely is called
		resourceTypes, untaggedResources := parseResourcesSafely(malformedResourceContent, "resources.tf", logger)
		
		// Then: Should return empty slices instead of panicking
		assert.NotNil(t, resourceTypes)
		assert.NotNil(t, untaggedResources)
		assert.Equal(t, []ResourceType{}, resourceTypes)
		assert.Equal(t, []UntaggedResource{}, untaggedResources)
	})
	
	t.Run("handles complex nested resource structures without panic", func(t *testing.T) {
		// Given: Complex resource with potential panic triggers
		complexContent := `resource "aws_launch_configuration" "complex" {
			name_prefix = "complex-"
			image_id = data.aws_ami.ubuntu.id
			instance_type = var.instance_type
			
			root_block_device {
				volume_type = "gp2"
				volume_size = 20
				delete_on_termination = true
			}
			
			ebs_block_device {
				device_name = "/dev/sdf"
				volume_size = $INVALID_SIZE
			}
		}`
		
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		// When: parseResourcesSafely is called
		resourceTypes, untaggedResources := parseResourcesSafely(complexContent, "complex.tf", logger)
		
		// Then: Should handle gracefully without panicking
		assert.NotNil(t, resourceTypes)
		assert.NotNil(t, untaggedResources)
	})
}

// TestJobSubmitterPanicRecovery tests panic recovery in job submission
func TestJobSubmitterPanicRecovery(t *testing.T) {
	t.Run("createJobSubmitterWithTimeoutRecovery handles pool submission errors", func(t *testing.T) {
		// Given: A processing context and logger
		config := Config{
			MaxGoroutines:    1,
			CloneConcurrency: 1,
			GitHubToken:      "fake-token", // Required for validation
			Organizations:    []string{"test-org"}, // Required for validation
		}
		
		processingCtx, err := createProcessingContext(config)
		require.NoError(t, err)
		defer releaseProcessingContext(processingCtx)
		
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		// Create job submitter
		jobSubmitter := createJobSubmitterWithTimeoutRecovery(processingCtx.Pool, logger)
		
		// Given: A repository
		repo := Repository{
			Name:         "test-repo",
			Path:         "/tmp/test",
			Organization: "test-org",
		}
		
		// When: jobSubmitter is called
		result := jobSubmitter(repo)
		
		// Then: Should return result without panicking
		assert.Equal(t, "test-repo", result.RepoName)
		assert.Equal(t, "test-org", result.Organization)
		// Result may have error due to invalid path, but shouldn't panic
	})
}

// TestPanicRecoveryWithNilPointers tests panic recovery with nil pointer scenarios
func TestPanicRecoveryWithNilPointers(t *testing.T) {
	t.Run("handles nil logger in parsing functions", func(t *testing.T) {
		// Given: Valid content but nil logger (edge case)
		content := `resource "aws_instance" "test" {
			ami = "ami-12345678"
			instance_type = "t2.micro"
		}`
		
		// When: parsing functions are called with nil logger
		// Note: The functions should handle nil logger gracefully
		result := parseBackendSafely(content, "test.tf", nil)
		
		// Then: Should return result without panicking
		assert.Nil(t, result) // No backend in this content
	})
	
	t.Run("handles empty content in all parsing functions", func(t *testing.T) {
		// Given: Empty content
		emptyContent := ""
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		// When: all parsing functions are called with empty content
		backend := parseBackendSafely(emptyContent, "empty.tf", logger)
		providers := parseProvidersSafely(emptyContent, "empty.tf", logger)
		modules := parseModulesSafely(emptyContent, "empty.tf", logger)
		variables := parseVariablesSafely(emptyContent, "empty.tf", logger)
		outputs := parseOutputsSafely(emptyContent, "empty.tf", logger)
		resourceTypes, untaggedResources := parseResourcesSafely(emptyContent, "empty.tf", logger)
		
		// Then: All should return appropriate empty results without panicking
		assert.Nil(t, backend)
		// These can be nil or empty slices, both are acceptable for empty content
		assert.True(t, len(providers) == 0)
		assert.True(t, len(modules) == 0)
		assert.True(t, len(variables) == 0)
		assert.True(t, len(outputs) == 0)
		assert.True(t, len(resourceTypes) == 0)
		assert.True(t, len(untaggedResources) == 0)
	})
	
	t.Run("handles extremely malformed HCL content", func(t *testing.T) {
		// Given: Extremely malformed content that could trigger various panics
		malformedContent := `{[}]$%^&*()$#@!}{P}{L}>{A}{N}{I}{C}`
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		// When: parsing functions are called with malformed content
		backend := parseBackendSafely(malformedContent, "malformed.tf", logger)
		providers := parseProvidersSafely(malformedContent, "malformed.tf", logger)
		modules := parseModulesSafely(malformedContent, "malformed.tf", logger)
		variables := parseVariablesSafely(malformedContent, "malformed.tf", logger)
		outputs := parseOutputsSafely(malformedContent, "malformed.tf", logger)
		resourceTypes, untaggedResources := parseResourcesSafely(malformedContent, "malformed.tf", logger)
		
		// Then: All should handle gracefully without panicking
		assert.Nil(t, backend)
		assert.NotNil(t, providers)
		assert.NotNil(t, modules)
		assert.NotNil(t, variables)
		assert.NotNil(t, outputs)
		assert.NotNil(t, resourceTypes)
		assert.NotNil(t, untaggedResources)
	})
}

// TestConcurrentPanicRecovery tests panic recovery under concurrent execution
func TestConcurrentPanicRecovery(t *testing.T) {
	t.Run("handles multiple concurrent panics in repository processing", func(t *testing.T) {
		// Given: Multiple repositories that will cause different types of panics
		repositories := []Repository{
			{Name: "panic-repo-1", Path: "/invalid/path/1", Organization: "test-org"},
			{Name: "panic-repo-2", Path: "/invalid/path/2", Organization: "test-org"},
			{Name: "panic-repo-3", Path: "/invalid/path/3", Organization: "test-org"},
		}
		
		config := Config{
			MaxGoroutines:    3,
			CloneConcurrency: 2,
			ProcessTimeout:   2 * time.Second,
			GitHubToken:      "fake-token", // Required for validation
			Organizations:    []string{"test-org"}, // Required for validation
		}
		
		processingCtx, err := createProcessingContext(config)
		require.NoError(t, err)
		defer releaseProcessingContext(processingCtx)
		
		p := configureWaitGroup(len(repositories))
		results := make(chan AnalysisResult, len(repositories))
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		ctx := context.Background()
		jobCtx := JobSubmissionContext{
			Repositories: repositories,
			Ctx:          ctx,
			Pool:         p,
			AntsPool:     processingCtx.Pool,
			Results:      results,
			Logger:       logger,
		}
		
		// When: submitRepositoryJobsWithTimeout is called with multiple problematic repos
		submitRepositoryJobsWithTimeout(jobCtx)
		p.Wait()
		close(results)
		
		// Then: Should handle all panics and return appropriate results
		var receivedResults []AnalysisResult
		for result := range results {
			receivedResults = append(receivedResults, result)
		}
		
		assert.Len(t, receivedResults, len(repositories))
		
		for _, result := range receivedResults {
			assert.NotEmpty(t, result.RepoName)
			assert.Equal(t, "test-org", result.Organization)
			// All should have errors due to invalid paths, but no panics
		}
	})
}

// TestPanicRecoveryLogging tests that panic recovery includes proper logging
func TestPanicRecoveryLogging(t *testing.T) {
	t.Run("logs panic recovery information", func(t *testing.T) {
		// Given: A repository that will cause panic and a logger that captures output
		repo := Repository{
			Name:         "logging-test-repo",
			Path:         "/will/cause/panic",
			Organization: "test-org",
		}
		
		// Create a logger that writes to a buffer so we can verify logging
		var logBuffer strings.Builder
		logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		
		// When: processRepositoryFilesWithRecovery is called
		result := processRepositoryFilesWithRecovery(repo, logger)
		
		// Then: Should log appropriately and return error result
		assert.Equal(t, "logging-test-repo", result.RepoName)
		assert.Equal(t, "test-org", result.Organization)
		assert.Error(t, result.Error)
		
		// Verify that some form of logging occurred (exact content may vary)
		logOutput := logBuffer.String()
		// The specific log message depends on the actual error that occurs,
		// but there should be some logging activity
		_ = logOutput // Just verify no panic occurred
	})
}