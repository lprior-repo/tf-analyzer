package main

import (
	"testing"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/table"
)

func TestNewTUIProgressChannel(t *testing.T) {
	totalRepos := 100
	tuiProgress := NewTUIProgressChannel(totalRepos)

	if tuiProgress == nil {
		t.Fatal("NewTUIProgressChannel should not return nil")
	}

	// Test that the progress channel is properly initialized
	if tuiProgress.progressChan == nil {
		t.Error("Expected progressChan to be initialized")
	}

	if tuiProgress.completeChan == nil {
		t.Error("Expected completeChan to be initialized")
	}

	if tuiProgress.program == nil {
		t.Error("Expected program to be initialized")
	}
}

func TestProgressModelInit(t *testing.T) {
	model := ProgressModel{
		progress:  progress.New(progress.WithDefaultGradient()),
		total:     100,
		completed: 0,
	}

	if model.total != 100 {
		t.Errorf("Expected total 100, got %d", model.total)
	}

	if model.completed != 0 {
		t.Errorf("Expected completed 0, got %d", model.completed)
	}

	if model.done {
		t.Error("Expected done to be false initially")
	}
}

func TestProgressMsg(t *testing.T) {
	msg := ProgressMsg{
		Repo:         "test-repo",
		Organization: "test-org",
		Completed:    50,
		Total:        100,
	}

	if msg.Repo != "test-repo" {
		t.Errorf("Expected repo 'test-repo', got %s", msg.Repo)
	}

	if msg.Organization != "test-org" {
		t.Errorf("Expected organization 'test-org', got %s", msg.Organization)
	}

	if msg.Completed != 50 {
		t.Errorf("Expected completed 50, got %d", msg.Completed)
	}

	if msg.Total != 100 {
		t.Errorf("Expected total 100, got %d", msg.Total)
	}
}

func TestCompletedMsg(t *testing.T) {
	results := []AnalysisResult{
		{
			RepoName:     "repo1",
			Organization: "org1",
			Analysis: RepositoryAnalysis{
				ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 10},
			},
		},
		{
			RepoName:     "repo2",
			Organization: "org1",
			Analysis: RepositoryAnalysis{
				ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 15},
			},
		},
	}

	msg := CompletedMsg{
		Results: results,
	}

	if len(msg.Results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(msg.Results))
	}

	if msg.Results[0].RepoName != "repo1" {
		t.Errorf("Expected first repo name 'repo1', got %s", msg.Results[0].RepoName)
	}
}

func TestTUIModelInit(t *testing.T) {
	model := TUIModel{
		mode: "progress",
		progress: ProgressModel{
			progress:  progress.New(progress.WithDefaultGradient()),
			total:     100,
			completed: 0,
		},
		width:  80,
		height: 24,
	}

	if model.mode != "progress" {
		t.Errorf("Expected mode 'progress', got %s", model.mode)
	}

	if model.width != 80 {
		t.Errorf("Expected width 80, got %d", model.width)
	}

	if model.height != 24 {
		t.Errorf("Expected height 24, got %d", model.height)
	}
}

func TestResultsModelInit(t *testing.T) {
	columns := []table.Column{
		{Title: "Repository", Width: 30},
		{Title: "Organization", Width: 20},
		{Title: "Resources", Width: 15},
		{Title: "Providers", Width: 15},
	}

	rows := []table.Row{
		{"test-repo", "test-org", "10", "2"},
	}

	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	model := ResultsModel{
		table:       tbl,
		results:     []AnalysisResult{},
		summary:     "Test Summary",
		showDetails: false,
	}

	if model.summary != "Test Summary" {
		t.Errorf("Expected summary 'Test Summary', got %s", model.summary)
	}

	if model.showDetails {
		t.Error("Expected showDetails to be false initially")
	}

	if len(model.results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(model.results))
	}
}

func TestTUIProgressChannelMethods(t *testing.T) {
	// Test that TUIProgressChannel methods exist and can be called
	// without causing compilation errors
	tuiProgress := NewTUIProgressChannel(100)

	// Test UpdateProgress method signature
	tuiProgress.UpdateProgress("test-repo", "test-org", 50, 100)

	// Test Complete method signature
	results := []AnalysisResult{
		{
			RepoName:     "test-repo",
			Organization: "test-org",
		},
	}
	tuiProgress.Complete(results)

	// These tests mainly ensure the methods exist and have the correct signatures
	// Full functionality testing would require integration with the actual TUI system
}

func TestProgressCalculation(t *testing.T) {
	tests := []struct {
		name       string
		completed  int
		total      int
		expectedPct float64
	}{
		{
			name:       "zero progress",
			completed:  0,
			total:      100,
			expectedPct: 0.0,
		},
		{
			name:       "half progress",
			completed:  50,
			total:      100,
			expectedPct: 0.5,
		},
		{
			name:       "full progress",
			completed:  100,
			total:      100,
			expectedPct: 1.0,
		},
		{
			name:       "over progress",
			completed:  150,
			total:      100,
			expectedPct: 1.0, // Should be capped at 1.0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pct float64
			if tt.total > 0 {
				pct = float64(tt.completed) / float64(tt.total)
				if pct > 1.0 {
					pct = 1.0
				}
			}

			if pct != tt.expectedPct {
				t.Errorf("Expected progress %f, got %f", tt.expectedPct, pct)
			}
		})
	}
}

func TestTUIModelModes(t *testing.T) {
	validModes := []string{"progress", "results"}

	for _, mode := range validModes {
		t.Run("mode_"+mode, func(t *testing.T) {
			model := TUIModel{
				mode: mode,
			}

			if model.mode != mode {
				t.Errorf("Expected mode %s, got %s", mode, model.mode)
			}
		})
	}
}

func TestResultsSummaryFormat(t *testing.T) {
	// Test summary string formatting
	results := []AnalysisResult{
		{
			RepoName:     "repo1",
			Organization: "org1",
			Analysis: RepositoryAnalysis{
				ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 10},
				Providers: ProvidersAnalysis{UniqueProviderCount: 2},
			},
		},
		{
			RepoName:     "repo2",
			Organization: "org1",
			Analysis: RepositoryAnalysis{
				ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 15},
				Providers: ProvidersAnalysis{UniqueProviderCount: 3},
			},
		},
	}

	// Test that we can create a summary from results
	totalRepos := len(results)
	totalResources := 0
	totalProviders := 0

	for _, result := range results {
		totalResources += result.Analysis.ResourceAnalysis.TotalResourceCount
		totalProviders += result.Analysis.Providers.UniqueProviderCount
	}

	if totalRepos != 2 {
		t.Errorf("Expected 2 repos, got %d", totalRepos)
	}

	if totalResources != 25 {
		t.Errorf("Expected 25 total resources, got %d", totalResources)
	}

	if totalProviders != 5 {
		t.Errorf("Expected 5 total providers, got %d", totalProviders)
	}
}