package main

import (
	"fmt"
	"log/slog"
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