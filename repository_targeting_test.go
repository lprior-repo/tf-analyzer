package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// REPOSITORY TARGETING TESTS - TDD for ghorg targeting functionality
// ============================================================================

// TestTargetingConfigStruct tests the extended Config struct
func TestTargetingConfigStruct(t *testing.T) {
	t.Run("it should have repository targeting fields", func(t *testing.T) {
		// Given: a config with repository targeting options
		config := Config{
			Organizations:     []string{"test-org"},
			GitHubToken:       "test-token",
			MaxGoroutines:     10,
			CloneConcurrency:  5,
			TargetRepos:       []string{"repo1", "repo2"},
			TargetReposFile:   "/path/to/repos.txt",
			MatchRegex:        "^terraform-.*",
			MatchPrefix:       []string{"tf-", "aws-"},
			ExcludeRegex:      ".*-deprecated$",
			ExcludePrefix:     []string{"test-", "demo-"},
		}

		// Then: config should contain targeting fields
		assert.Equal(t, []string{"repo1", "repo2"}, config.TargetRepos)
		assert.Equal(t, "/path/to/repos.txt", config.TargetReposFile)
		assert.Equal(t, "^terraform-.*", config.MatchRegex)
		assert.Equal(t, []string{"tf-", "aws-"}, config.MatchPrefix)
		assert.Equal(t, ".*-deprecated$", config.ExcludeRegex)
		assert.Equal(t, []string{"test-", "demo-"}, config.ExcludePrefix)
	})
}

// TestTargetReposFlagParsing tests --target-repos flag parsing
func TestTargetReposFlagParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single repository",
			input:    "repo1",
			expected: []string{"repo1"},
		},
		{
			name:     "comma-separated repositories",
			input:    "repo1,repo2,repo3",
			expected: []string{"repo1", "repo2", "repo3"},
		},
		{
			name:     "repositories with spaces",
			input:    "repo1, repo2 , repo3",
			expected: []string{"repo1", "repo2", "repo3"},
		},
		{
			name:     "empty input",
			input:    "",
			expected: []string{},
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When: parseTargetRepos is called (function to be implemented)
			result := parseTargetRepos(tt.input)

			// Then: should parse correctly
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestTargetReposFileReading tests --target-repos-file functionality
func TestTargetReposFileReading(t *testing.T) {
	t.Run("it reads repositories from file", func(t *testing.T) {
		// Given: a file with repository names
		tempFile := createTempRepoFile(t, "repo1\nrepo2\nrepo3\n")
		defer func() {
			if err := os.Remove(tempFile); err != nil {
				// Test cleanup failed, but continue
				t.Logf("Failed to remove temp file: %v", err)
			}
		}()

		// When: readTargetReposFromFile is called (function to be implemented)
		repos, err := readTargetReposFromFile(tempFile)

		// Then: should read repositories successfully
		require.NoError(t, err)
		assert.Equal(t, []string{"repo1", "repo2", "repo3"}, repos)
	})

	t.Run("it handles empty lines and comments", func(t *testing.T) {
		// Given: a file with mixed content
		content := `# This is a comment
repo1
# Another comment

repo2
   repo3   
`
		tempFile := createTempRepoFile(t, content)
		defer func() {
			if err := os.Remove(tempFile); err != nil {
				// Test cleanup failed, but continue
				t.Logf("Failed to remove temp file: %v", err)
			}
		}()

		// When: readTargetReposFromFile is called
		repos, err := readTargetReposFromFile(tempFile)

		// Then: should filter out comments and empty lines
		require.NoError(t, err)
		assert.Equal(t, []string{"repo1", "repo2", "repo3"}, repos)
	})

	t.Run("it returns error for non-existent file", func(t *testing.T) {
		// Given: a non-existent file path
		nonExistentFile := "/tmp/does-not-exist.txt"

		// When: readTargetReposFromFile is called
		_, err := readTargetReposFromFile(nonExistentFile)

		// Then: should return error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read target repos file")
	})
}

// TestMatchRegexValidation tests --match-regex validation
func TestMatchRegexValidation(t *testing.T) {
	tests := []struct {
		name        string
		regex       string
		expectError bool
	}{
		{
			name:        "valid regex pattern",
			regex:       "^terraform-.*",
			expectError: false,
		},
		{
			name:        "valid complex regex",
			regex:       "^(terraform|aws)-[a-z]+-v[0-9]+$",
			expectError: false,
		},
		{
			name:        "empty regex (valid)",
			regex:       "",
			expectError: false,
		},
		{
			name:        "invalid regex pattern",
			regex:       "[unclosed",
			expectError: true,
		},
		{
			name:        "invalid regex with unmatched parenthesis",
			regex:       "^terraform-(.*",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When: validateRegexPattern is called (function to be implemented)
			err := validateRegexPattern(tt.regex)

			// Then: should validate correctly
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestExcludeRegexValidation tests --exclude-regex validation
func TestExcludeRegexValidation(t *testing.T) {
	t.Run("it validates exclude regex patterns", func(t *testing.T) {
		// Given: various regex patterns
		validRegex := ".*-deprecated$"
		invalidRegex := "[unclosed"

		// When: validateRegexPattern is called
		validErr := validateRegexPattern(validRegex)
		invalidErr := validateRegexPattern(invalidRegex)

		// Then: should validate correctly
		assert.NoError(t, validErr)
		assert.Error(t, invalidErr)
	})
}

// TestPrefixParsing tests prefix parsing for match and exclude
func TestPrefixParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single prefix",
			input:    "terraform-",
			expected: []string{"terraform-"},
		},
		{
			name:     "comma-separated prefixes",
			input:    "tf-,aws-,gcp-",
			expected: []string{"tf-", "aws-", "gcp-"},
		},
		{
			name:     "prefixes with spaces",
			input:    "tf- , aws- , gcp-",
			expected: []string{"tf-", "aws-", "gcp-"},
		},
		{
			name:     "empty input",
			input:    "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When: parsePrefixes is called (function to be implemented)
			result := parsePrefixes(tt.input)

			// Then: should parse correctly
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGhorgCommandBuildingWithTargeting tests ghorg command building with targeting options
func TestGhorgCommandBuildingWithTargeting(t *testing.T) {
	t.Run("it builds command with target repos", func(t *testing.T) {
		// Given: config with target repositories
		config := Config{
			GitHubToken:      "token123",
			CloneConcurrency: 5,
			TargetRepos:      []string{"repo1", "repo2"},
		}
		op := CloneOperation{
			Org:     "test-org",
			TempDir: "/tmp/test",
			Config:  config,
		}
		ctx := context.Background()

		// When: buildGhorgCommand is called
		cmd := buildGhorgCommand(ctx, op)

		// Then: should include target repos arguments
		cmdArgs := strings.Join(cmd.Args, " ")
		assert.Contains(t, cmdArgs, "--target-repos")
		assert.Contains(t, cmdArgs, "repo1,repo2")
	})

	t.Run("it builds command with match regex", func(t *testing.T) {
		// Given: config with match regex
		config := Config{
			GitHubToken:      "token123",
			CloneConcurrency: 5,
			MatchRegex:       "^terraform-.*",
		}
		op := CloneOperation{
			Org:     "test-org",
			TempDir: "/tmp/test",
			Config:  config,
		}
		ctx := context.Background()

		// When: buildGhorgCommand is called
		cmd := buildGhorgCommand(ctx, op)

		// Then: should include match regex arguments
		cmdArgs := strings.Join(cmd.Args, " ")
		assert.Contains(t, cmdArgs, "--match-regex")
		assert.Contains(t, cmdArgs, "^terraform-.*")
	})

	t.Run("it builds command with match prefixes", func(t *testing.T) {
		// Given: config with match prefixes
		config := Config{
			GitHubToken:      "token123",
			CloneConcurrency: 5,
			MatchPrefix:      []string{"tf-", "aws-"},
		}
		op := CloneOperation{
			Org:     "test-org",
			TempDir: "/tmp/test",
			Config:  config,
		}
		ctx := context.Background()

		// When: buildGhorgCommand is called
		cmd := buildGhorgCommand(ctx, op)

		// Then: should include match prefix arguments
		cmdArgs := strings.Join(cmd.Args, " ")
		assert.Contains(t, cmdArgs, "--match-prefix")
		assert.Contains(t, cmdArgs, "tf-,aws-")
	})

	t.Run("it builds command with exclude options", func(t *testing.T) {
		// Given: config with exclude options
		config := Config{
			GitHubToken:      "token123",
			CloneConcurrency: 5,
			ExcludeRegex:     ".*-deprecated$",
			ExcludePrefix:    []string{"test-", "demo-"},
		}
		op := CloneOperation{
			Org:     "test-org",
			TempDir: "/tmp/test",
			Config:  config,
		}
		ctx := context.Background()

		// When: buildGhorgCommand is called
		cmd := buildGhorgCommand(ctx, op)

		// Then: should include exclude arguments
		cmdArgs := strings.Join(cmd.Args, " ")
		assert.Contains(t, cmdArgs, "--exclude-regex")
		assert.Contains(t, cmdArgs, ".*-deprecated$")
		assert.Contains(t, cmdArgs, "--exclude-prefix")
		assert.Contains(t, cmdArgs, "test-,demo-")
	})

	t.Run("it builds command with target repos file", func(t *testing.T) {
		// Given: config with target repos file
		config := Config{
			GitHubToken:      "token123",
			CloneConcurrency: 5,
			TargetReposFile:  "/path/to/repos.txt",
		}
		op := CloneOperation{
			Org:     "test-org",
			TempDir: "/tmp/test",
			Config:  config,
		}
		ctx := context.Background()

		// When: buildGhorgCommand is called
		cmd := buildGhorgCommand(ctx, op)

		// Then: should include target repos file argument
		cmdArgs := strings.Join(cmd.Args, " ")
		assert.Contains(t, cmdArgs, "--target-repos-file")
		assert.Contains(t, cmdArgs, "/path/to/repos.txt")
	})
}

// TestTargetingConfigValidation tests validation of targeting configurations
func TestTargetingConfigValidation(t *testing.T) {
	t.Run("it allows orgs with targeting options", func(t *testing.T) {
		// Given: config with organizations and targeting options
		config := Config{
			Organizations:    []string{"test-org"},
			GitHubToken:      "test-token",
			MaxGoroutines:    10,
			CloneConcurrency: 5,
			TargetRepos:      []string{"repo1", "repo2"},
		}

		// When: validateTargetingConfiguration is called (function to be implemented)
		err := validateTargetingConfiguration(config)

		// Then: should be valid
		assert.NoError(t, err)
	})

	t.Run("it rejects conflicting targeting options", func(t *testing.T) {
		// Given: config with conflicting targeting options
		config := Config{
			Organizations:    []string{"test-org"},
			GitHubToken:      "test-token",
			MaxGoroutines:    10,
			CloneConcurrency: 5,
			TargetRepos:      []string{"repo1"},
			TargetReposFile:  "/path/to/repos.txt", // Conflict with TargetRepos
		}

		// When: validateTargetingConfiguration is called
		err := validateTargetingConfiguration(config)

		// Then: should return validation error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot specify both --target-repos and --target-repos-file")
	})

	t.Run("it rejects conflicting match options", func(t *testing.T) {
		// Given: config with conflicting match options
		config := Config{
			Organizations:    []string{"test-org"},
			GitHubToken:      "test-token",
			MaxGoroutines:    10,
			CloneConcurrency: 5,
			MatchRegex:       "^terraform-.*",
			MatchPrefix:      []string{"tf-"}, // Conflict with MatchRegex
		}

		// When: validateTargetingConfiguration is called
		err := validateTargetingConfiguration(config)

		// Then: should return validation error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot specify both --match-regex and --match-prefix")
	})

	t.Run("it rejects conflicting exclude options", func(t *testing.T) {
		// Given: config with conflicting exclude options
		config := Config{
			Organizations:    []string{"test-org"},
			GitHubToken:      "test-token",
			MaxGoroutines:    10,
			CloneConcurrency: 5,
			ExcludeRegex:     ".*-deprecated$",
			ExcludePrefix:    []string{"test-"}, // Conflict with ExcludeRegex
		}

		// When: validateTargetingConfiguration is called
		err := validateTargetingConfiguration(config)

		// Then: should return validation error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot specify both --exclude-regex and --exclude-prefix")
	})

	t.Run("it validates regex patterns", func(t *testing.T) {
		// Given: config with invalid regex
		config := Config{
			Organizations:    []string{"test-org"},
			GitHubToken:      "test-token",
			MaxGoroutines:    10,
			CloneConcurrency: 5,
			MatchRegex:       "[unclosed", // Invalid regex
		}

		// When: validateTargetingConfiguration is called
		err := validateTargetingConfiguration(config)

		// Then: should return validation error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid match regex")
	})
}

// TestViperBindingForTargeting tests viper configuration binding
func TestViperBindingForTargeting(t *testing.T) {
	t.Run("it binds targeting flags to viper", func(t *testing.T) {
		// Given: viper reset and targeting configuration
		viper.Reset()
		viper.Set("github.target_repos", []string{"repo1", "repo2"})
		viper.Set("github.target_repos_file", "/path/to/repos.txt")
		viper.Set("github.match_regex", "^terraform-.*")
		viper.Set("github.match_prefix", []string{"tf-", "aws-"})
		viper.Set("github.exclude_regex", ".*-deprecated$")
		viper.Set("github.exclude_prefix", []string{"test-", "demo-"})

		// When: createConfigFromViper is called
		config, err := createConfigFromViper()

		// Then: targeting options should be loaded
		require.NoError(t, err)
		assert.Equal(t, []string{"repo1", "repo2"}, config.TargetRepos)
		assert.Equal(t, "/path/to/repos.txt", config.TargetReposFile)
		assert.Equal(t, "^terraform-.*", config.MatchRegex)
		assert.Equal(t, []string{"tf-", "aws-"}, config.MatchPrefix)
		assert.Equal(t, ".*-deprecated$", config.ExcludeRegex)
		assert.Equal(t, []string{"test-", "demo-"}, config.ExcludePrefix)
	})

	t.Run("it handles string input for target repos", func(t *testing.T) {
		// Given: viper with string target repos
		viper.Reset()
		viper.Set("github.target_repos", "repo1,repo2,repo3")

		// When: createConfigFromViper is called
		config, err := createConfigFromViper()

		// Then: should parse string to slice
		require.NoError(t, err)
		assert.Equal(t, []string{"repo1", "repo2", "repo3"}, config.TargetRepos)
	})
}

// Helper function to create temporary repo file for testing
func createTempRepoFile(t *testing.T, content string) string {
	t.Helper()
	tempFile, err := os.CreateTemp("", "repos-*.txt")
	require.NoError(t, err)
	
	_, err = tempFile.WriteString(content)
	require.NoError(t, err)
	
	err = tempFile.Close()
	require.NoError(t, err)
	
	return tempFile.Name()
}

// The following functions need to be implemented to make tests pass:
// - parseTargetRepos(string) []string
// - readTargetReposFromFile(string) ([]string, error)
// - validateRegexPattern(string) error
// - parsePrefixes(string) []string
// - validateTargetingConfiguration(Config) error
// - Extended Config struct with targeting fields
// - Modified buildGhorgCommand to include targeting options
// - Modified createConfigFromViper to load targeting options