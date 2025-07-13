package main

import (
	"testing"
)

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


