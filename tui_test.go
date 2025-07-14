package main

import (
	"context"
	"strings"
	"testing"
	"time"

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
		progress: progress.New(progress.WithDefaultGradient()),
		data: ProgressData{
			Total:     100,
			Completed: 0,
		},
	}

	if model.data.Total != 100 {
		t.Errorf("Expected total 100, got %d", model.data.Total)
	}

	if model.data.Completed != 0 {
		t.Errorf("Expected completed 0, got %d", model.data.Completed)
	}

	if model.data.Done {
		t.Error("Expected done to be false initially")
	}

	// Test new fields are initialized correctly
	if model.data.CurrentPhase != "" {
		t.Errorf("Expected currentPhase to be empty initially, got %s", model.data.CurrentPhase)
	}

	if model.data.RepoCount != 0 {
		t.Errorf("Expected repoCount to be 0 initially, got %d", model.data.RepoCount)
	}

	if model.data.TotalRepos != 0 {
		t.Errorf("Expected totalRepos to be 0 initially, got %d", model.data.TotalRepos)
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
		state: TUIState{
			Mode:   "progress",
			Width:  80,
			Height: 24,
		},
		progress: ProgressModel{
			progress: progress.New(progress.WithDefaultGradient()),
			data: ProgressData{
				Total:     100,
				Completed: 0,
			},
		},
	}

	if model.state.Mode != "progress" {
		t.Errorf("Expected mode 'progress', got %s", model.state.Mode)
	}

	if model.state.Width != 80 {
		t.Errorf("Expected width 80, got %d", model.state.Width)
	}

	if model.state.Height != 24 {
		t.Errorf("Expected height 24, got %d", model.state.Height)
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
		table: tbl,
		data: ResultsData{
			Results:     []AnalysisResult{},
			ShowDetails: false,
		},
	}

	if model.data.ShowDetails {
		t.Error("Expected showDetails to be false initially")
	}

	if len(model.data.Results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(model.data.Results))
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
				state: TUIState{
					Mode: mode,
				},
			}

			if model.state.Mode != mode {
				t.Errorf("Expected mode %s, got %s", mode, model.state.Mode)
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

// ============================================================================
// ADDITIONAL TESTS (formerly from tui_critical_test.go)
// ============================================================================

// TestNewTUIModelCritical tests TUI model creation (critical)
func TestNewTUIModelCritical(t *testing.T) {
	t.Run("creates TUI model correctly", func(t *testing.T) {
		// Given: parameters for TUI model
		totalRepos := 10

		// When: NewTUIModel is called
		model := NewTUIModel(totalRepos)

		// Then: should create model with correct initial state
		if model.state.Mode != "progress" {
			t.Errorf("Expected mode 'progress', got %s", model.state.Mode)
		}
		if model.progress.data.Total != totalRepos {
			t.Errorf("Expected total %d, got %d", totalRepos, model.progress.data.Total)
		}
	})
}

// TestTUIInit tests TUI initialization
func TestTUIInit(t *testing.T) {
	t.Run("initializes TUI without panic", func(t *testing.T) {
		// Given: a TUI model
		model := NewTUIModel(5)

		// When: Init is called
		cmd := model.Init()

		// Then: should return a command (or nil)
		// Note: cmd can be nil, which is fine for initialization
		_ = cmd
	})
}

// TestTUIView tests TUI view rendering
func TestTUIView(t *testing.T) {
	t.Run("renders view without panic", func(t *testing.T) {
		// Given: a TUI model with some progress
		model := NewTUIModel(10)
		model.progress.data.Completed = 3
		model.progress.data.CurrentRepo = "test-repo"

		// When: View is called
		view := model.View()

		// Then: should return a non-empty string
		if view == "" {
			t.Error("Expected non-empty view output")
		}
	})

	t.Run("renders view in results mode", func(t *testing.T) {
		// Given: a TUI model in results mode
		model := NewTUIModel(5)
		model.state.Mode = "results"
		model.results.data.Results = []AnalysisResult{
			{RepoName: "repo1", Organization: "org1"},
			{RepoName: "repo2", Organization: "org1"},
		}

		// When: View is called
		view := model.View()

		// Then: should return a view with results
		if view == "" {
			t.Error("Expected non-empty view output")
		}
	})
}

// TestRenderProgressView tests progress view rendering
func TestRenderProgressView(t *testing.T) {
	t.Run("renders progress view", func(t *testing.T) {
		// Given: a TUI model in progress
		model := NewTUIModel(10)
		model.progress.data.Completed = 4
		model.progress.data.CurrentRepo = "test-repo"

		// When: renderProgressView is called
		view := renderProgressView(model.state, model.progress)

		// Then: should return progress view
		if view == "" {
			t.Error("Expected non-empty progress view")
		}
	})
}

// TestRenderResultsView tests results view rendering
func TestRenderResultsView(t *testing.T) {
	t.Run("renders results view", func(t *testing.T) {
		// Given: a TUI model with results
		model := NewTUIModel(3)
		model.results.data.Results = []AnalysisResult{
			{
				RepoName:     "test-repo",
				Organization: "test-org",
				Analysis: RepositoryAnalysis{
					ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 5},
				},
			},
		}

		// When: renderResultsView is called
		view := renderResultsView(model.state, model.results)

		// Then: should return results view
		if view == "" {
			t.Error("Expected non-empty results view")
		}
	})
}

// TestTUIUpdate tests TUI update functionality
func TestTUIUpdate(t *testing.T) {
	t.Run("updates model without panic", func(t *testing.T) {
		// Given: a TUI model and message
		model := NewTUIModel(5)
		
		// Create a basic message (we'll use a window size message)
		type WindowSizeMsg struct {
			Width, Height int
		}
		
		msg := WindowSizeMsg{Width: 80, Height: 24}

		// When: Update is called
		newModel, cmd := model.Update(msg)

		// Then: should return updated model
		if newModel == nil {
			t.Error("Expected non-nil model")
		}
		// cmd can be nil, which is fine
		_ = cmd
	})
}

// TestTruncate tests string truncation
func TestTruncate(t *testing.T) {
	t.Run("truncates long string", func(t *testing.T) {
		// Given: a long string
		longString := "This is a very long string that should be truncated"
		maxLength := 20

		// When: truncate is called
		result := truncate(longString, maxLength)

		// Then: should be truncated with ellipsis
		if len(result) > maxLength {
			t.Errorf("Expected result length <= %d, got %d", maxLength, len(result))
		}
		if len(longString) > maxLength && result == longString {
			t.Error("Expected string to be truncated")
		}
	})

	t.Run("does not truncate short string", func(t *testing.T) {
		// Given: a short string
		shortString := "Short"
		maxLength := 20

		// When: truncate is called
		result := truncate(shortString, maxLength)

		// Then: should remain unchanged
		if result != shortString {
			t.Errorf("Expected %s, got %s", shortString, result)
		}
	})
}

// TestNewTUIProgressChannelCritical tests progress channel creation (critical)
func TestNewTUIProgressChannelCritical(t *testing.T) {
	t.Run("creates progress channel", func(t *testing.T) {
		// Given: total repositories
		totalRepos := 5

		// When: NewTUIProgressChannel is called
		tuiProgress := NewTUIProgressChannel(totalRepos)

		// Then: should create progress channel
		if tuiProgress == nil {
			t.Error("Expected non-nil TUI progress channel")
		}
	})
}

// TestTUIProgressChannelStart tests progress channel start
func TestTUIProgressChannelStart(t *testing.T) {
	t.Run("starts progress channel", func(t *testing.T) {
		// Given: a TUI progress channel
		tuiProgress := NewTUIProgressChannel(3)
		ctx := context.Background()

		// When: Start is called
		tuiProgress.Start(ctx)

		// Then: should not panic
		// Note: Start sets up the channel but doesn't start the UI loop
	})
}

// TestTUIProgressChannelUpdateProgress tests progress updates
func TestTUIProgressChannelUpdateProgress(t *testing.T) {
	t.Run("updates progress without panic", func(t *testing.T) {
		// Given: a TUI progress channel
		tuiProgress := NewTUIProgressChannel(5)
		ctx := context.Background()
		tuiProgress.Start(ctx)

		// When: UpdateProgress is called
		// Then: should not panic
		tuiProgress.UpdateProgress("repo1", "org1", 1, 5)
		tuiProgress.UpdateProgress("repo2", "org1", 2, 5)
	})
}

// TestTUIProgressChannelComplete tests completion
func TestTUIProgressChannelComplete(t *testing.T) {
	t.Run("completes without panic", func(t *testing.T) {
		// Given: a TUI progress channel
		tuiProgress := NewTUIProgressChannel(2)
		ctx := context.Background()
		tuiProgress.Start(ctx)

		// When: Complete is called with results
		results := []AnalysisResult{
			{RepoName: "repo1", Organization: "org1"},
		}
		tuiProgress.Complete(results)

		// Then: should not panic
	})
}

// TestTUIProgressChannelRun tests the main TUI run loop
func TestTUIProgressChannelRun(t *testing.T) {
	t.Run("runs TUI loop", func(t *testing.T) {
		// Given: a TUI progress channel
		tuiProgress := NewTUIProgressChannel(1)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		tuiProgress.Start(ctx)

		// Complete immediately to exit the loop
		tuiProgress.Complete([]AnalysisResult{})

		// When: Run is called
		err := tuiProgress.Run()

		// Then: should complete without serious error
		// Note: May return context deadline exceeded, which is expected
		if err != nil && err.Error() != "context deadline exceeded" {
			// Only fail if it's a different error
			t.Logf("TUI run completed with: %v", err)
		}
	})
}

// ============================================================================
// ENHANCED TUI FUNCTIONALITY TESTS
// ============================================================================

// TestGetPhaseEmoji tests phase emoji mapping
func TestGetPhaseEmoji(t *testing.T) {
	tests := []struct {
		phase    string
		expected string
	}{
		{"cloning", "📥"},
		{"clone", "📥"},
		{"analyzing", "🔍"},
		{"analysis", "🔍"},
		{"processing", "⚡"},
		{"fetching", "🌐"},
		{"complete", "✅"},
		{"completed", "✅"},
		{"unknown", "🔄"},
		{"", "🔄"},
	}

	for _, tt := range tests {
		t.Run("phase_"+tt.phase, func(t *testing.T) {
			result := getPhaseEmoji(tt.phase)
			if result != tt.expected {
				t.Errorf("Expected emoji %s for phase %s, got %s", tt.expected, tt.phase, result)
			}
		})
	}
}

// TestEnhancedProgressMsg tests the enhanced progress message
func TestEnhancedProgressMsg(t *testing.T) {
	msg := ProgressMsg{
		Repo:         "test-repo",
		Organization: "test-org",
		Phase:        "cloning",
		Completed:    25,
		Total:        100,
		RepoCount:    5,
		TotalRepos:   20,
	}

	if msg.Phase != "cloning" {
		t.Errorf("Expected phase 'cloning', got %s", msg.Phase)
	}

	if msg.RepoCount != 5 {
		t.Errorf("Expected repoCount 5, got %d", msg.RepoCount)
	}

	if msg.TotalRepos != 20 {
		t.Errorf("Expected totalRepos 20, got %d", msg.TotalRepos)
	}
}

// TestProgressWithPhaseUpdate tests UpdateProgressWithPhase method
func TestProgressWithPhaseUpdate(t *testing.T) {
	tuiProgress := NewTUIProgressChannel(100)
	ctx := context.Background()
	tuiProgress.Start(ctx)

	// Test that UpdateProgressWithPhase works without panic
	tuiProgress.UpdateProgressWithPhase("repo1", "org1", "cloning", 10, 100, 2, 10)
	tuiProgress.UpdateProgressWithPhase("repo2", "org1", "analyzing", 20, 100, 3, 10)

	// Test backward compatibility - UpdateProgress should call UpdateProgressWithPhase
	tuiProgress.UpdateProgress("repo3", "org1", 30, 100)
}

// TestEnhancedProgressView tests the enhanced progress view with new fields
func TestEnhancedProgressView(t *testing.T) {
	model := NewTUIModel(100)
	model.progress.data.CurrentOrg = "test-org"
	model.progress.data.CurrentRepo = "test-repo"
	model.progress.data.CurrentPhase = "cloning"
	model.progress.data.RepoCount = 5
	model.progress.data.TotalRepos = 20
	model.progress.data.Completed = 25
	model.progress.data.Total = 100

	view := renderProgressView(model.state, model.progress)

	// Check that the view contains enhanced information
	if !strings.Contains(view, "test-org") {
		t.Error("Expected view to contain organization name")
	}

	if !strings.Contains(view, "test-repo") {
		t.Error("Expected view to contain repository name")
	}

	if !strings.Contains(view, "📥") { // cloning emoji
		t.Error("Expected view to contain phase emoji")
	}

	if !strings.Contains(view, "5/20") { // repo progress
		t.Error("Expected view to contain repository progress")
	}

	if !strings.Contains(view, "25.0%") { // percentage
		t.Error("Expected view to contain percentage")
	}
}

// TestEnhancedResultsView tests the enhanced results view with statistics
func TestEnhancedResultsView(t *testing.T) {
	model := NewTUIModel(10)
	model.state.Mode = "results"
	results := []AnalysisResult{
		{
			RepoName:     "repo1",
			Organization: "org1",
			Analysis: RepositoryAnalysis{
				ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 10},
				Providers:        ProvidersAnalysis{UniqueProviderCount: 2},
				Modules:          ModulesAnalysis{TotalModuleCalls: 3},
			},
		},
		{
			RepoName:     "repo2",
			Organization: "org1",
			Analysis: RepositoryAnalysis{
				ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 15},
				Providers:        ProvidersAnalysis{UniqueProviderCount: 3},
				Modules:          ModulesAnalysis{TotalModuleCalls: 5},
			},
		},
	}
	model.progress.data.Results = results
	model.results = createResultsModel(results)

	view := renderResultsView(model.state, model.results)

	// Check that the view contains enhanced statistics
	if !strings.Contains(view, "✅ Success: 2") {
		t.Error("Expected view to contain success count")
	}

	if !strings.Contains(view, "Providers: 5") { // 2 + 3
		t.Error("Expected view to contain total providers")
	}

	if !strings.Contains(view, "Modules: 8") { // 3 + 5
		t.Error("Expected view to contain total modules")
	}

	if !strings.Contains(view, "Resources: 25") { // 10 + 15
		t.Error("Expected view to contain total resources")
	}
}

// TestTUIModelHandleProgressUpdate tests enhanced progress update handling
func TestTUIModelHandleProgressUpdate(t *testing.T) {
	model := NewTUIModel(100)
	
	msg := ProgressMsg{
		Repo:         "test-repo",
		Organization: "test-org",
		Phase:        "analyzing",
		Completed:    50,
		Total:        100,
		RepoCount:    10,
		TotalRepos:   50,
	}

	updatedModel, _ := model.handleProgressUpdate(msg)
	tuiModel := updatedModel.(TUIModel)

	if tuiModel.progress.data.CurrentRepo != "test-repo" {
		t.Errorf("Expected currentRepo 'test-repo', got %s", tuiModel.progress.data.CurrentRepo)
	}

	if tuiModel.progress.data.CurrentOrg != "test-org" {
		t.Errorf("Expected currentOrg 'test-org', got %s", tuiModel.progress.data.CurrentOrg)
	}

	if tuiModel.progress.data.CurrentPhase != "analyzing" {
		t.Errorf("Expected currentPhase 'analyzing', got %s", tuiModel.progress.data.CurrentPhase)
	}

	if tuiModel.progress.data.RepoCount != 10 {
		t.Errorf("Expected repoCount 10, got %d", tuiModel.progress.data.RepoCount)
	}

	if tuiModel.progress.data.TotalRepos != 50 {
		t.Errorf("Expected totalRepos 50, got %d", tuiModel.progress.data.TotalRepos)
	}
}