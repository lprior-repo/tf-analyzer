package main

import (
	"os"
	"testing"
)

// Test the core helper functions to ensure they work correctly
func TestTestHelpers(t *testing.T) {
	// Test configuration builders
	t.Run("newValidConfig", func(t *testing.T) {
		config := newValidConfig()
		assertConfigValid(t, config)
		
		if len(config.Organizations) != 1 {
			t.Errorf("Expected 1 organization, got %d", len(config.Organizations))
		}
		if config.Organizations[0] != "test-org" {
			t.Errorf("Expected 'test-org', got %s", config.Organizations[0])
		}
		if config.GitHubToken != "valid-token" {
			t.Errorf("Expected 'valid-token', got %s", config.GitHubToken)
		}
	})

	t.Run("newInvalidConfig", func(t *testing.T) {
		config := newInvalidConfig()
		
		// This should have empty/zero values
		if len(config.Organizations) != 0 {
			t.Errorf("Expected 0 organizations, got %d", len(config.Organizations))
		}
		if config.GitHubToken != "" {
			t.Errorf("Expected empty token, got %s", config.GitHubToken)
		}
		if config.CloneConcurrency != 0 {
			t.Errorf("Expected 0 clone concurrency, got %d", config.CloneConcurrency)
		}
	})

	t.Run("newTargetingConfig", func(t *testing.T) {
		targetRepos := []string{"repo1", "repo2"}
		config := newTargetingConfig(targetRepos)
		
		assertConfigValid(t, config)
		if len(config.TargetRepos) != 2 {
			t.Errorf("Expected 2 target repos, got %d", len(config.TargetRepos))
		}
	})
}

func TestFileSystemHelpers(t *testing.T) {
	// Test temporary file creation
	t.Run("createTempTerraformFile", func(t *testing.T) {
		content := `resource "aws_s3_bucket" "test" {
			bucket = "test-bucket"
		}`
		
		filePath := createTempTerraformFile(t, content)
		assertFileExists(t, filePath)
		assertFileContains(t, filePath, "aws_s3_bucket")
		assertFileContains(t, filePath, "test-bucket")
	})

	// Test repository structure creation
	t.Run("createTempTerraformRepo", func(t *testing.T) {
		files := map[string]string{
			"main.tf":      createMockTerraformContent("aws_vpc", "main"),
			"variables.tf": createMockVariableContent("vpc_cidr", "string", "VPC CIDR"),
		}
		
		repoDir := createTempTerraformRepo(t, files)
		assertFileExists(t, repoDir+"/main.tf")
		assertFileExists(t, repoDir+"/variables.tf")
		
		assertFileContains(t, repoDir+"/main.tf", "aws_vpc")
		assertFileContains(t, repoDir+"/variables.tf", "vpc_cidr")
	})
}

func TestDataBuilders(t *testing.T) {
	// Test analysis result creation
	t.Run("newTestAnalysisResult", func(t *testing.T) {
		result := newTestAnalysisResult("test-repo", "test-org")
		
		if result.RepoName != "test-repo" {
			t.Errorf("Expected repo name 'test-repo', got %s", result.RepoName)
		}
		if result.Organization != "test-org" {
			t.Errorf("Expected organization 'test-org', got %s", result.Organization)
		}
		if result.Error != nil {
			t.Errorf("Expected no error, got %v", result.Error)
		}
	})

	// Test reporter creation
	t.Run("newTestReporter", func(t *testing.T) {
		reporter := newTestReporter()
		if reporter == nil {
			t.Error("Expected non-nil reporter")
		}
	})
}

func TestAssertionHelpers(t *testing.T) {
	// Test assertNoError with no error
	t.Run("assertNoError_success", func(t *testing.T) {
		// This should not fail
		assertNoError(t, nil)
	})

	// Test assertResultsEqual
	t.Run("assertResultsEqual", func(t *testing.T) {
		result1 := newTestAnalysisResult("repo1", "org1")
		result2 := newTestAnalysisResult("repo2", "org2")
		
		expected := []AnalysisResult{result1, result2}
		actual := []AnalysisResult{result1, result2}
		
		// This should not fail
		assertResultsEqual(t, expected, actual)
	})
}

func TestFunctionalUtilities(t *testing.T) {
	// Test filter terraform files
	t.Run("filterTerraformFiles", func(t *testing.T) {
		files := []string{"main.tf", "readme.md", "variables.tfvars", "test.hcl", "script.sh"}
		terraformFiles := filterTerraformFiles(files)
		
		expectedCount := 3 // .tf, .tfvars, .hcl
		if len(terraformFiles) != expectedCount {
			t.Errorf("Expected %d terraform files, got %d", expectedCount, len(terraformFiles))
		}
	})

	// Test map file names
	t.Run("mapFileNames", func(t *testing.T) {
		paths := []string{"/path/to/main.tf", "/another/path/variables.tf"}
		names := mapFileNames(paths)
		
		expected := []string{"main.tf", "variables.tf"}
		if len(names) != len(expected) {
			t.Errorf("Expected %d names, got %d", len(expected), len(names))
		}
		if names[0] != expected[0] || names[1] != expected[1] {
			t.Errorf("Expected %v, got %v", expected, names)
		}
	})
}

func TestEnvironmentHelpers(t *testing.T) {
	// Test environment variable management
	t.Run("withEnvVars", func(t *testing.T) {
		envVars := map[string]string{
			"TEST_VAR1": "value1",
			"TEST_VAR2": "value2",
		}
		
		withEnvVars(t, envVars)
		
		// Variables should be set
		if os.Getenv("TEST_VAR1") != "value1" {
			t.Error("Expected TEST_VAR1 to be set to 'value1'")
		}
		if os.Getenv("TEST_VAR2") != "value2" {
			t.Error("Expected TEST_VAR2 to be set to 'value2'")
		}
	})
}

func TestHigherOrderHelpers(t *testing.T) {
	// Test withTempRepo
	t.Run("withTempRepo", func(t *testing.T) {
		files := map[string]string{
			"test.tf": createMockTerraformContent("aws_instance", "test"),
		}
		
		withTempRepo(t, files, func(repoPath string) {
			assertFileExists(t, repoPath+"/test.tf")
			assertFileContains(t, repoPath+"/test.tf", "aws_instance")
		})
	})

	// Test withMockConfig
	t.Run("withMockConfig", func(t *testing.T) {
		withMockConfig(t, func(config *Config) {
			config.MaxGoroutines = 100
		}, func(config Config) {
			if config.MaxGoroutines != 100 {
				t.Errorf("Expected MaxGoroutines 100, got %d", config.MaxGoroutines)
			}
		})
	})
}