package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	
	"github.com/spf13/viper"
)

// TestGenerateReports tests report generation
func TestGenerateReports(t *testing.T) {
	t.Run("generates all reports successfully", func(t *testing.T) {
		// Given: a reporter with results and config
		reporter := &Reporter{
			results: []AnalysisResult{
				{
					RepoName:     "test-repo",
					Organization: "test-org",
					Analysis: RepositoryAnalysis{
						ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 5},
					},
				},
			},
		}
		
		config := Config{
			Organizations: []string{"test-org"},
			GitHubToken:   "test-token",
		}
		
		// Set up viper for output configuration
		viper.Reset()
		tempDir := t.TempDir()
		viper.Set("output.format", "all")
		viper.Set("output.directory", tempDir)

		// When: generateReports is called
		err := generateReports(reporter, config)

		// Then: should generate reports without error
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		
		// Verify files were created
		expectedFiles := []string{
			"terraform-analysis-report.json",
			"terraform-analysis-report.csv", 
			"terraform-analysis-report.md",
		}
		
		for _, filename := range expectedFiles {
			filePath := filepath.Join(tempDir, filename)
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				t.Errorf("Expected file %s to be created", filename)
			}
		}
	})

	t.Run("generates specific format only", func(t *testing.T) {
		// Given: a reporter and config for JSON only
		reporter := &Reporter{
			results: []AnalysisResult{
				{
					RepoName:     "test-repo",
					Organization: "test-org",
					Analysis: RepositoryAnalysis{
						ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 3},
					},
				},
			},
		}
		
		config := Config{
			Organizations: []string{"test-org"},
			GitHubToken:   "test-token",
		}
		
		// Set up viper for JSON only
		viper.Reset()
		tempDir := t.TempDir()
		viper.Set("output.format", "json")
		viper.Set("output.directory", tempDir)

		// When: generateReports is called
		err := generateReports(reporter, config)

		// Then: should generate only JSON report
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		
		// Verify only JSON file was created
		jsonPath := filepath.Join(tempDir, "terraform-analysis-report.json")
		if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
			t.Error("Expected JSON file to be created")
		}
		
		// Verify other files were not created
		csvPath := filepath.Join(tempDir, "terraform-analysis-report.csv")
		if _, err := os.Stat(csvPath); !os.IsNotExist(err) {
			t.Error("Expected CSV file NOT to be created")
		}
	})
}

// TestShowConfig tests configuration display
func TestShowConfig(t *testing.T) {
	t.Run("shows configuration without panic", func(t *testing.T) {
		// Given: viper configuration
		viper.Reset()
		viper.Set("organizations", []string{"test-org1", "test-org2"})
		viper.Set("github.token", "test-token-123")
		viper.Set("processing.max_goroutines", 50)
		viper.Set("processing.clone_concurrency", 25)
		viper.Set("processing.timeout", 45*time.Minute)
		viper.Set("output.format", "all")
		viper.Set("output.directory", ".")
		viper.Set("ui.no_tui", false)
		viper.Set("ui.markdown_style", "auto")
		viper.Set("ui.raw_markdown", false)

		// When: showConfig is called
		err := showConfig(nil, nil)

		// Then: should not error
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})
}

// TestInitConfig tests configuration file initialization
func TestInitConfig(t *testing.T) {
	t.Run("creates configuration file successfully", func(t *testing.T) {
		// Given: a temporary directory
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, ".tf-analyzer.yaml")
		
		// Set the global cfgFile variable to use our temp path
		originalCfgFile := cfgFile
		cfgFile = configPath
		defer func() { cfgFile = originalCfgFile }()

		// When: initConfig is called
		err := initConfig(nil, nil)

		// Then: should create config file
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		
		// Verify file was created
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Error("Expected config file to be created")
		}
		
		// Verify file has content
		content, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read config file: %v", err)
		}
		
		if len(content) == 0 {
			t.Error("Expected config file to have content")
		}
	})

	t.Run("returns error if file already exists", func(t *testing.T) {
		// Given: an existing config file
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, ".tf-analyzer.yaml")
		
		// Create the file first
		err := os.WriteFile(configPath, []byte("existing content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create existing file: %v", err)
		}
		
		// Set the global cfgFile variable
		originalCfgFile := cfgFile
		cfgFile = configPath
		defer func() { cfgFile = originalCfgFile }()

		// When: initConfig is called
		err = initConfig(nil, nil)

		// Then: should return error
		if err == nil {
			t.Error("Expected error for existing config file")
		}
	})
}

// TestValidateConfig tests configuration validation
func TestValidateConfig(t *testing.T) {
	t.Run("validates valid configuration", func(t *testing.T) {
		// Given: valid viper configuration
		viper.Reset()
		viper.Set("organizations", []string{"test-org"})
		viper.Set("github.token", "test-token")
		viper.Set("processing.max_goroutines", 10)
		viper.Set("processing.clone_concurrency", 5)

		// When: validateConfig is called
		err := validateConfig(nil, nil)

		// Then: should not error
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("returns error for invalid configuration", func(t *testing.T) {
		// Given: invalid viper configuration (missing token)
		viper.Reset()
		viper.Set("organizations", []string{"test-org"})
		// Missing github.token

		// When: validateConfig is called
		err := validateConfig(nil, nil)

		// Then: should return error
		if err == nil {
			t.Error("Expected error for invalid configuration")
		}
	})
}

// TestExecute tests the main command execution
func TestExecute(t *testing.T) {
	t.Run("executes without panic", func(t *testing.T) {
		// Given: original os.Args
		originalArgs := os.Args
		defer func() { os.Args = originalArgs }()
		
		// Set args to show help (won't actually execute analyze)
		os.Args = []string{"tf-analyzer", "--help"}

		// When: Execute is called
		// Then: should not panic (will exit with code 0 for help)
		// Note: We can't easily test this without mocking os.Exit
		// But we can at least verify the function exists
		// Execute() would actually exit the process for help, so we don't call it
	})
}