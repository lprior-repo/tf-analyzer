package main

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

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
			expected: "ghp_****...fgh",
		},
		{
			name:     "masks short token",
			token:    "token123",
			expected: "tok***23",
		},
		{
			name:     "handles very short token",
			token:    "abc",
			expected: "a**",
		},
		{
			name:     "handles empty token",
			token:    "",
			expected: "",
		},
		{
			name:     "handles single character",
			token:    "a",
			expected: "*",
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
				os.Mkdir(tempDir+"/"+file, 0755)
			} else {
				os.WriteFile(tempDir+"/"+file, []byte("test"), 0644)
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
		// Given: some environment variables
		testVars := map[string]string{
			"TEST_VAR1": "value1",
			"TEST_VAR2": "value2",
		}

		// Set test environment variables
		for key, value := range testVars {
			os.Setenv(key, value)
			defer os.Unsetenv(key)
		}

		// When: getEnvironmentVariables is called
		envVars := getEnvironmentVariables()

		// Then: should collect all environment variables
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
		os.Mkdir(tempDir+"/repo1", 0755)
		os.Mkdir(tempDir+"/repo2", 0755)
		os.WriteFile(tempDir+"/file1.txt", []byte("test"), 0644)

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