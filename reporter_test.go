package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewReporter tests reporter creation
func TestNewReporter(t *testing.T) {
	t.Run("creates new reporter", func(t *testing.T) {
		// Given: nothing
		// When: NewReporter is called
		reporter := NewReporter()

		// Then: should create reporter with empty results
		if reporter == nil {
			t.Fatal("Expected non-nil reporter")
		}
		if len(reporter.results) != 0 {
			t.Errorf("Expected empty results, got %d", len(reporter.results))
		}
	})
}

// TestAddResults tests adding results to reporter
func TestAddResults(t *testing.T) {
	t.Run("adds results correctly", func(t *testing.T) {
		// Given: a reporter and results
		reporter := NewReporter()
		results := []AnalysisResult{
			{
				RepoName:     "repo1",
				Organization: "org1",
				Analysis: RepositoryAnalysis{
					ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 5},
				},
			},
			{
				RepoName:     "repo2",
				Organization: "org1",
				Analysis: RepositoryAnalysis{
					ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 3},
				},
			},
		}

		// When: AddResults is called
		reporter.AddResults(results)

		// Then: should add results to reporter
		if len(reporter.results) != len(results) {
			t.Errorf("Expected %d results, got %d", len(results), len(reporter.results))
		}
	})
}

// TestGetBackendType tests backend type extraction
func TestGetBackendType(t *testing.T) {
	tests := []struct {
		name     string
		backend  *BackendConfig
		expected string
	}{
		{
			name:     "s3 backend",
			backend:  &BackendConfig{Type: stringPtr("s3")},
			expected: "s3",
		},
		{
			name:     "local backend",
			backend:  &BackendConfig{Type: stringPtr("local")},
			expected: "local",
		},
		{
			name:     "nil backend",
			backend:  nil,
			expected: "none",
		},
		{
			name:     "backend with nil type",
			backend:  &BackendConfig{Type: nil},
			expected: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: a backend config
			// When: getBackendType is called
			result := getBackendType(tt.backend)

			// Then: should return correct type
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestGetBackendRegion tests backend region extraction
func TestGetBackendRegion(t *testing.T) {
	tests := []struct {
		name     string
		backend  *BackendConfig
		expected string
	}{
		{
			name:     "backend with region",
			backend:  &BackendConfig{Region: stringPtr("us-west-2")},
			expected: "us-west-2",
		},
		{
			name:     "backend without region",
			backend:  &BackendConfig{Region: nil},
			expected: "none",
		},
		{
			name:     "nil backend",
			backend:  nil,
			expected: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: a backend config
			// When: getBackendRegion is called
			result := getBackendRegion(tt.backend)

			// Then: should return correct region
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestExtractRepoName tests repository name extraction
func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		name     string
		repoPath string
		expected string
	}{
		{
			name:     "simple repo name",
			repoPath: "/path/to/my-repo",
			expected: "my-repo",
		},
		{
			name:     "nested repo path",
			repoPath: "/very/deep/path/to/complex-repo-name",
			expected: "complex-repo-name",
		},
		{
			name:     "single directory",
			repoPath: "single-repo",
			expected: "single-repo",
		},
		{
			name:     "empty path",
			repoPath: "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: a repository path
			// When: extractRepoName is called
			result := extractRepoName(tt.repoPath)

			// Then: should extract correct name
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestCalculateTotalProviders tests provider count calculation
func TestCalculateTotalProviders(t *testing.T) {
	t.Run("calculates total providers correctly", func(t *testing.T) {
		// Given: analysis results with providers
		results := []AnalysisResult{
			{
				Analysis: RepositoryAnalysis{
					Providers: ProvidersAnalysis{UniqueProviderCount: 3},
				},
			},
			{
				Analysis: RepositoryAnalysis{
					Providers: ProvidersAnalysis{UniqueProviderCount: 2},
				},
			},
			{
				Analysis: RepositoryAnalysis{
					Providers: ProvidersAnalysis{UniqueProviderCount: 1},
				},
			},
		}

		// When: calculateTotalProviders is called
		analyses := make([]RepositoryAnalysis, len(results))
		for i, result := range results {
			analyses[i] = result.Analysis
		}
		total := calculateTotalProviders(analyses)

		// Then: should return sum of providers
		expected := 6 // 3 + 2 + 1
		if total != expected {
			t.Errorf("Expected %d, got %d", expected, total)
		}
	})

	t.Run("handles empty results", func(t *testing.T) {
		// Given: empty results
		results := []AnalysisResult{}

		// When: calculateTotalProviders is called
		analyses := make([]RepositoryAnalysis, len(results))
		for i, result := range results {
			analyses[i] = result.Analysis
		}
		total := calculateTotalProviders(analyses)

		// Then: should return zero
		if total != 0 {
			t.Errorf("Expected 0, got %d", total)
		}
	})
}

// TestCalculateTotalModules tests module count calculation
func TestCalculateTotalModules(t *testing.T) {
	t.Run("calculates total modules correctly", func(t *testing.T) {
		// Given: analysis results with modules
		results := []AnalysisResult{
			{
				Analysis: RepositoryAnalysis{
					Modules: ModulesAnalysis{TotalModuleCalls: 5},
				},
			},
			{
				Analysis: RepositoryAnalysis{
					Modules: ModulesAnalysis{TotalModuleCalls: 3},
				},
			},
		}

		// When: calculateTotalModules is called
		analyses := make([]RepositoryAnalysis, len(results))
		for i, result := range results {
			analyses[i] = result.Analysis
		}
		total := calculateTotalModules(analyses)

		// Then: should return sum of modules
		expected := 8 // 5 + 3
		if total != expected {
			t.Errorf("Expected %d, got %d", expected, total)
		}
	})
}

// TestCalculateTotalResources tests resource count calculation
func TestCalculateTotalResources(t *testing.T) {
	t.Run("calculates total resources correctly", func(t *testing.T) {
		// Given: analysis results with resources
		results := []AnalysisResult{
			{
				Analysis: RepositoryAnalysis{
					ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 10},
				},
			},
			{
				Analysis: RepositoryAnalysis{
					ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 7},
				},
			},
		}

		// When: calculateTotalResources is called
		analyses := make([]RepositoryAnalysis, len(results))
		for i, result := range results {
			analyses[i] = result.Analysis
		}
		total := calculateTotalResources(analyses)

		// Then: should return sum of resources
		expected := 17 // 10 + 7
		if total != expected {
			t.Errorf("Expected %d, got %d", expected, total)
		}
	})
}

// TestCalculateTotalVariables tests variable count calculation
func TestCalculateTotalVariables(t *testing.T) {
	t.Run("calculates total variables correctly", func(t *testing.T) {
		// Given: analysis results with variables
		results := []AnalysisResult{
			{
				Analysis: RepositoryAnalysis{
					VariableAnalysis: VariableAnalysis{
						DefinedVariables: []VariableDefinition{
							{Name: "var1"}, {Name: "var2"},
						},
					},
				},
			},
			{
				Analysis: RepositoryAnalysis{
					VariableAnalysis: VariableAnalysis{
						DefinedVariables: []VariableDefinition{
							{Name: "var3"},
						},
					},
				},
			},
		}

		// When: calculateTotalVariables is called
		analyses := make([]RepositoryAnalysis, len(results))
		for i, result := range results {
			analyses[i] = result.Analysis
		}
		total := calculateTotalVariables(analyses)

		// Then: should return sum of variables
		expected := 3 // 2 + 1
		if total != expected {
			t.Errorf("Expected %d, got %d", expected, total)
		}
	})
}

// TestCalculateTotalOutputs tests output count calculation
func TestCalculateTotalOutputs(t *testing.T) {
	t.Run("calculates total outputs correctly", func(t *testing.T) {
		// Given: analysis results with outputs
		results := []AnalysisResult{
			{
				Analysis: RepositoryAnalysis{
					OutputAnalysis: OutputAnalysis{OutputCount: 4},
				},
			},
			{
				Analysis: RepositoryAnalysis{
					OutputAnalysis: OutputAnalysis{OutputCount: 2},
				},
			},
		}

		// When: calculateTotalOutputs is called
		analyses := make([]RepositoryAnalysis, len(results))
		for i, result := range results {
			analyses[i] = result.Analysis
		}
		total := calculateTotalOutputs(analyses)

		// Then: should return sum of outputs
		expected := 6 // 4 + 2
		if total != expected {
			t.Errorf("Expected %d, got %d", expected, total)
		}
	})
}

// TestExportJSON tests JSON export functionality
func TestExportJSON(t *testing.T) {
	t.Run("exports JSON successfully", func(t *testing.T) {
		// Given: a reporter with results
		reporter := &Reporter{
			results: []AnalysisResult{
				{
					RepoName:     "test-repo",
					Organization: "test-org",
					Analysis: RepositoryAnalysis{
						RepositoryPath:   "path/to/test-repo",
						ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 5},
					},
				},
			},
		}

		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "test-report.json")

		// When: ExportJSON is called
		err := reporter.ExportJSON(filename)

		// Then: should create JSON file
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify file exists and has content
		content, err := os.ReadFile(filename)
		if err != nil {
			t.Fatalf("Failed to read exported file: %v", err)
		}

		if len(content) == 0 {
			t.Error("Expected non-empty JSON file")
		}

		// Should contain expected data
		jsonStr := string(content)
		if !strings.Contains(jsonStr, "test-repo") {
			t.Errorf("JSON should contain repository name 'test-repo', but got: %s", jsonStr)
		}
	})
}

// TestExportCSV tests CSV export functionality
func TestExportCSV(t *testing.T) {
	t.Run("exports CSV successfully", func(t *testing.T) {
		// Given: a reporter with results
		reporter := &Reporter{
			results: []AnalysisResult{
				{
					RepoName:     "test-repo",
					Organization: "test-org",
					Analysis: RepositoryAnalysis{
						RepositoryPath:   "path/to/test-repo",
						ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 5},
					},
				},
			},
		}

		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "test-report.csv")

		// When: ExportCSV is called
		err := reporter.ExportCSV(filename)

		// Then: should create CSV file
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify file exists and has content
		content, err := os.ReadFile(filename)
		if err != nil {
			t.Fatalf("Failed to read exported file: %v", err)
		}

		if len(content) == 0 {
			t.Error("Expected non-empty CSV file")
		}

		// Should contain CSV headers and data
		csvStr := string(content)
		if !strings.Contains(csvStr, "Repository") {
			t.Error("CSV should contain headers")
		}
		if !strings.Contains(csvStr, "test-repo") {
			t.Errorf("CSV should contain repository name 'test-repo', but got: %s", csvStr)
		}
	})
}

// TestExportMarkdown tests Markdown export functionality
func TestExportMarkdown(t *testing.T) {
	t.Run("exports Markdown successfully", func(t *testing.T) {
		// Given: a reporter with results
		reporter := &Reporter{
			results: []AnalysisResult{
				{
					RepoName:     "test-repo",
					Organization: "test-org",
					Analysis: RepositoryAnalysis{
						RepositoryPath:   "path/to/test-repo",
						ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 5},
					},
				},
			},
		}

		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "test-report.md")

		// When: ExportMarkdown is called
		err := reporter.ExportMarkdown(filename)

		// Then: should create Markdown file
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify file exists and has content
		content, err := os.ReadFile(filename)
		if err != nil {
			t.Fatalf("Failed to read exported file: %v", err)
		}

		if len(content) == 0 {
			t.Error("Expected non-empty Markdown file")
		}

		// Should contain Markdown formatting
		mdStr := string(content)
		if !strings.Contains(mdStr, "#") {
			t.Error("Markdown should contain headers")
		}
		if !strings.Contains(mdStr, "test-repo") {
			t.Errorf("Markdown should contain repository name 'test-repo', but got: %s", mdStr)
		}
	})
}

// ============================================================================
// ADDITIONAL TESTS (formerly from reporter_critical_test.go)
// ============================================================================

// TestPrintSummaryReport tests summary report printing
func TestPrintSummaryReport(t *testing.T) {
	t.Run("prints summary report without panic", func(t *testing.T) {
		// Given: a reporter with results
		reporter := &Reporter{
			results: []AnalysisResult{
				{
					RepoName:     "test-repo",
					Organization: "test-org",
					Analysis: RepositoryAnalysis{
						RepositoryPath:   "path/to/test-repo",
						ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 5},
						Providers:        ProvidersAnalysis{UniqueProviderCount: 2},
						Modules:          ModulesAnalysis{TotalModuleCalls: 3},
					},
				},
			},
		}

		// When: PrintSummaryReport is called
		err := reporter.PrintSummaryReport()

		// Then: should not error
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("handles empty results", func(t *testing.T) {
		// Given: a reporter with no results
		reporter := &Reporter{
			results: []AnalysisResult{},
		}

		// When: PrintSummaryReport is called
		err := reporter.PrintSummaryReport()

		// Then: should not error
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})
}

// TestGenerateReport tests full report generation
func TestGenerateReport(t *testing.T) {
	t.Run("generates report without panic", func(t *testing.T) {
		// Given: a reporter with results
		reporter := &Reporter{
			results: []AnalysisResult{
				{
					RepoName:     "test-repo",
					Organization: "test-org",
					Analysis: RepositoryAnalysis{
						ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 5},
						Providers:        ProvidersAnalysis{UniqueProviderCount: 2},
						BackendConfig: &BackendConfig{
							Type:   stringPtr("s3"),
							Region: stringPtr("us-west-2"),
						},
					},
				},
			},
		}

		// When: GenerateReport is called
		report := reporter.GenerateReport()

		// Then: should return a report
		if len(report.Repositories) == 0 {
			t.Error("Expected non-empty repositories in report")
		}
	})
}

// TestPrintMarkdownToScreen tests markdown printing
func TestPrintMarkdownToScreen(t *testing.T) {
	t.Run("prints markdown without panic", func(t *testing.T) {
		// Given: a reporter with results
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

		// When: PrintMarkdownToScreen is called
		// Then: should not panic
		reporter.PrintMarkdownToScreen()
	})
}

// TestConvertMarkdownToTerminal tests markdown conversion
func TestConvertMarkdownToTerminal(t *testing.T) {
	t.Run("converts markdown to terminal format", func(t *testing.T) {
		// Given: markdown content
		markdown := "# Test Header\n\n**Bold text** and *italic text*\n\n- List item 1\n- List item 2"

		// When: convertMarkdownToTerminal is called
		result := convertMarkdownToTerminal(markdown)

		// Then: should return formatted text
		if result == "" {
			t.Error("Expected non-empty result")
		}
		if result == markdown {
			t.Error("Expected text to be modified from original markdown")
		}
	})

	t.Run("handles empty markdown", func(t *testing.T) {
		// Given: empty markdown
		markdown := ""

		// When: convertMarkdownToTerminal is called
		result := convertMarkdownToTerminal(markdown)

		// Then: should return empty string
		if result != "" {
			t.Errorf("Expected empty result, got %s", result)
		}
	})
}

// TestConvertLineToTerminal tests line conversion
func TestConvertLineToTerminal(t *testing.T) {
	t.Run("converts line with formatting", func(t *testing.T) {
		// Given: a line with markdown formatting
		line := "**Bold text** and *italic text*"

		// When: convertLineToTerminal is called
		result := convertLineToTerminal(line)

		// Then: should convert formatting
		if result == line {
			t.Error("Expected line to be modified")
		}
	})

	t.Run("handles plain text", func(t *testing.T) {
		// Given: plain text line
		line := "This is plain text"

		// When: convertLineToTerminal is called
		result := convertLineToTerminal(line)

		// Then: should return same text
		if result != line {
			t.Errorf("Expected %s, got %s", line, result)
		}
	})
}

// TestConvertBoldText tests bold text conversion
func TestConvertBoldText(t *testing.T) {
	t.Run("converts bold text", func(t *testing.T) {
		// Given: text with bold formatting
		text := "This is **bold text** in a sentence"

		// When: convertBoldText is called
		result := convertBoldText(text)

		// Then: should convert bold markers
		if result == text {
			t.Error("Expected text to be modified")
		}
		// The exact formatting may vary, but it should be different
	})

	t.Run("handles text without bold", func(t *testing.T) {
		// Given: text without bold formatting
		text := "This is normal text"

		// When: convertBoldText is called
		result := convertBoldText(text)

		// Then: should return unchanged
		if result != text {
			t.Errorf("Expected %s, got %s", text, result)
		}
	})
}

