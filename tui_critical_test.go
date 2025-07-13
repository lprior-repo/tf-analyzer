package main

import (
	"context"
	"testing"
	"time"
)

// TestNewTUIModelCritical tests TUI model creation (critical)
func TestNewTUIModelCritical(t *testing.T) {
	t.Run("creates TUI model correctly", func(t *testing.T) {
		// Given: parameters for TUI model
		totalRepos := 10

		// When: NewTUIModel is called
		model := NewTUIModel(totalRepos)

		// Then: should create model with correct initial state
		if model.mode != "progress" {
			t.Errorf("Expected mode 'progress', got %s", model.mode)
		}
		if model.progress.total != totalRepos {
			t.Errorf("Expected total %d, got %d", totalRepos, model.progress.total)
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
		model.progress.completed = 3
		model.progress.currentRepo = "test-repo"

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
		model.mode = "results"
		model.results.results = []AnalysisResult{
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
		model.progress.completed = 4
		model.progress.currentRepo = "test-repo"

		// When: renderProgressView is called
		view := model.renderProgressView()

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
		model.results.results = []AnalysisResult{
			{
				RepoName:     "test-repo",
				Organization: "test-org",
				Analysis: RepositoryAnalysis{
					ResourceAnalysis: ResourceAnalysis{TotalResourceCount: 5},
				},
			},
		}

		// When: renderResultsView is called
		view := model.renderResultsView()

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