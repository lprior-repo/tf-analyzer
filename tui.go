package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// TUI - Terminal User Interface using Bubble Tea
// ============================================================================

type ProgressModel struct {
	progress     progress.Model
	total        int
	completed    int
	currentRepo  string
	currentOrg   string
	currentPhase string
	repoCount    int
	totalRepos   int
	done         bool
	results      []AnalysisResult
}

type ResultsModel struct {
	table       table.Model
	results     []AnalysisResult
	summary     string
	showDetails bool
}

type TUIModel struct {
	mode        string // "progress" or "results"
	progress    ProgressModel
	results     ResultsModel
	width       int
	height      int
}

type ProgressMsg struct {
	Repo         string
	Organization string
	Completed    int
	Total        int
	Phase        string
	RepoCount    int
	TotalRepos   int
}

type CompletedMsg struct {
	Results []AnalysisResult
}

type WindowSizeMsg struct {
	Width  int
	Height int
}

// Styles
var (
	titleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		Bold(true)

	statusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C7C7C"))

	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Italic(true)

	successStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#04B575")).
		Bold(true)

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF0000")).
		Bold(true)

	highlightStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFA500")).
		Bold(true)
	
	infoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00BFFF"))
)

func NewTUIModel(total int) TUIModel {
	p := progress.New(progress.WithDefaultGradient())
	p.Width = 40

	return TUIModel{
		mode: "progress",
		progress: ProgressModel{
			progress: p,
			total:    total,
		},
	}
}

func (m TUIModel) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		func() tea.Msg {
			return WindowSizeMsg{Width: 80, Height: 24}
		},
	)
}

func (m TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyInput(msg)
	case tea.WindowSizeMsg:
		return m.handleWindowResize(msg), nil
	case WindowSizeMsg:
		return m.handleCustomWindowResize(msg), nil
	case ProgressMsg:
		return m.handleProgressUpdate(msg)
	case CompletedMsg:
		return m.handleAnalysisCompletion(msg), nil
	case progress.FrameMsg:
		return m.handleProgressFrame(msg)
	}

	return m.handleResultsModeUpdate(msg)
}

func (m TUIModel) handleKeyInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "tab":
		if m.mode == "results" {
			m.results.showDetails = !m.results.showDetails
		}
	case "r":
		if m.mode == "results" {
			return m, tea.ExitAltScreen
		}
	}
	return m, nil
}

func (m TUIModel) handleWindowResize(msg tea.WindowSizeMsg) TUIModel {
	m.width = msg.Width
	m.height = msg.Height
	m.progress.progress.Width = msg.Width - 20
	if m.mode == "results" {
		m.results.table.SetHeight(msg.Height - 10)
		m.results.table.SetWidth(msg.Width - 4)
	}
	return m
}

func (m TUIModel) handleCustomWindowResize(msg WindowSizeMsg) TUIModel {
	m.width = msg.Width
	m.height = msg.Height
	m.progress.progress.Width = msg.Width - 20
	return m
}

func (m TUIModel) handleProgressUpdate(msg ProgressMsg) (tea.Model, tea.Cmd) {
	m.progress.currentRepo = msg.Repo
	m.progress.currentOrg = msg.Organization
	m.progress.currentPhase = msg.Phase
	m.progress.repoCount = msg.RepoCount
	m.progress.totalRepos = msg.TotalRepos
	m.progress.completed = msg.Completed
	m.progress.total = msg.Total
	
	return m, m.progress.progress.SetPercent(float64(msg.Completed) / float64(msg.Total))
}

func (m TUIModel) handleAnalysisCompletion(msg CompletedMsg) TUIModel {
	m.mode = "results"
	m.progress.done = true
	m.progress.results = msg.Results
	
	m.results = createResultsModel(msg.Results)
	m.results.table.SetHeight(m.height - 10)
	m.results.table.SetWidth(m.width - 4)
	
	return m
}

func (m TUIModel) handleProgressFrame(msg progress.FrameMsg) (tea.Model, tea.Cmd) {
	progressModel, cmd := m.progress.progress.Update(msg)
	if pm, ok := progressModel.(progress.Model); ok {
		m.progress.progress = pm
	}
	return m, cmd
}

func (m TUIModel) handleResultsModeUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.mode == "results" {
		var cmd tea.Cmd
		m.results.table, cmd = m.results.table.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m TUIModel) View() string {
	if m.mode == "progress" {
		return m.renderProgressView()
	}
	return m.renderResultsView()
}

func (m TUIModel) renderProgressView() string {
	var b strings.Builder

	// Title with organization info
	title := "TF-ANALYZER"
	if m.progress.currentOrg != "" {
		title = fmt.Sprintf("TF-ANALYZER • %s", m.progress.currentOrg)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Current phase and progress info
	if m.progress.currentPhase != "" {
		phaseEmoji := getPhaseEmoji(m.progress.currentPhase)
		b.WriteString(fmt.Sprintf("%s %s\n", phaseEmoji, statusStyle.Render(m.progress.currentPhase)))
	}
	
	if m.progress.currentRepo != "" {
		b.WriteString(fmt.Sprintf("📁 Repository: %s\n", successStyle.Render(m.progress.currentRepo)))
	}
	
	// Repository count info
	if m.progress.totalRepos > 0 {
		b.WriteString(fmt.Sprintf("📊 Repository Progress: %d/%d\n", 
			m.progress.repoCount, m.progress.totalRepos))
	}
	b.WriteString("\n")

	// Progress bar
	b.WriteString(m.progress.progress.View())
	b.WriteString("\n\n")

	// Overall stats
	percentage := float64(0)
	if m.progress.total > 0 {
		percentage = float64(m.progress.completed) / float64(m.progress.total) * 100
	}
	b.WriteString(fmt.Sprintf("🎯 Overall Progress: %d/%d files (%.1f%%)\n", 
		m.progress.completed, m.progress.total, percentage))

	if m.progress.done {
		b.WriteString("\n")
		b.WriteString(successStyle.Render("✅ Analysis complete!"))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("Press any key to view results..."))
	} else {
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("Press 'q' to quit • Live progress updates"))
	}

	return b.String()
}

func getPhaseEmoji(phase string) string {
	switch strings.ToLower(phase) {
	case "cloning", "clone":
		return "📥"
	case "analyzing", "analysis":
		return "🔍"
	case "processing":
		return "⚡"
	case "fetching":
		return "🌐"
	case "complete", "completed":
		return "✅"
	default:
		return "🔄"
	}
}

func (m TUIModel) renderResultsView() string {
	var b strings.Builder

	// Title with completion indicator
	b.WriteString(titleStyle.Render("✅ ANALYSIS RESULTS"))
	b.WriteString("\n\n")

	// Summary with better styling
	successful := 0
	failed := 0
	totalProviders := 0
	totalModules := 0
	totalResources := 0
	
	for _, result := range m.progress.results {
		if result.Error != nil {
			failed++
		} else {
			successful++
			totalProviders += result.Analysis.Providers.UniqueProviderCount
			totalModules += result.Analysis.Modules.TotalModuleCalls
			totalResources += result.Analysis.ResourceAnalysis.TotalResourceCount
		}
	}

	// Repository summary
	b.WriteString(fmt.Sprintf("📊 %s | ", infoStyle.Render(fmt.Sprintf("Total: %d repos", len(m.progress.results)))))
	b.WriteString(successStyle.Render(fmt.Sprintf("✅ Success: %d", successful)))
	b.WriteString(" | ")
	if failed > 0 {
		b.WriteString(errorStyle.Render(fmt.Sprintf("❌ Failed: %d", failed)))
	} else {
		b.WriteString(fmt.Sprintf("❌ Failed: %d", failed))
	}
	b.WriteString("\n")

	// Infrastructure summary
	b.WriteString(fmt.Sprintf("🔧 %s | ", highlightStyle.Render(fmt.Sprintf("Providers: %d", totalProviders))))
	b.WriteString(fmt.Sprintf("📦 %s | ", highlightStyle.Render(fmt.Sprintf("Modules: %d", totalModules))))
	b.WriteString(fmt.Sprintf("🏗️  %s", highlightStyle.Render(fmt.Sprintf("Resources: %d", totalResources))))
	b.WriteString("\n\n")

	// Table
	viewType := "Summary View"
	if m.results.showDetails {
		viewType = "Detailed View"
		b.WriteString(fmt.Sprintf("📋 %s:\n", infoStyle.Render(viewType)))
		b.WriteString(m.results.table.View())
	} else {
		b.WriteString(fmt.Sprintf("📋 %s:\n", infoStyle.Render(viewType)))
		b.WriteString(m.createSummaryTable())
	}

	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("💡 Controls: [tab] toggle view | [r] export reports | [q] quit"))

	return b.String()
}

func (m TUIModel) createSummaryTable() string {
	var rows [][]string
	
	for _, result := range m.progress.results {
		if result.Error != nil {
			rows = append(rows, []string{
				result.RepoName,
				result.Organization,
				errorStyle.Render("FAILED"),
				result.Error.Error(),
			})
		} else {
			analysis := result.Analysis
			rows = append(rows, []string{
				result.RepoName,
				result.Organization,
				successStyle.Render("SUCCESS"),
				fmt.Sprintf("P:%d M:%d R:%d", 
					analysis.Providers.UniqueProviderCount,
					analysis.Modules.TotalModuleCalls,
					analysis.ResourceAnalysis.TotalResourceCount),
			})
		}
	}

	// Simple table formatting
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-25s %-20s %-10s %s\n", "Repository", "Organization", "Status", "Details"))
	b.WriteString(strings.Repeat("-", 80) + "\n")
	
	for _, row := range rows {
		b.WriteString(fmt.Sprintf("%-25s %-20s %-10s %s\n", 
			truncate(row[0], 24),
			truncate(row[1], 19),
			row[2],
			truncate(row[3], 35)))
	}

	return b.String()
}

func createResultsModel(results []AnalysisResult) ResultsModel {
	columns := []table.Column{
		{Title: "Repository", Width: 25},
		{Title: "Organization", Width: 20},
		{Title: "Status", Width: 10},
		{Title: "Providers", Width: 10},
		{Title: "Modules", Width: 10},
		{Title: "Resources", Width: 10},
		{Title: "Variables", Width: 10},
		{Title: "Outputs", Width: 10},
	}

	var rows []table.Row
	for _, result := range results {
		if result.Error != nil {
			rows = append(rows, table.Row{
				result.RepoName,
				result.Organization,
				"FAILED",
				"-",
				"-",
				"-",
				"-",
				"-",
			})
		} else {
			analysis := result.Analysis
			rows = append(rows, table.Row{
				result.RepoName,
				result.Organization,
				"SUCCESS",
				fmt.Sprintf("%d", analysis.Providers.UniqueProviderCount),
				fmt.Sprintf("%d", analysis.Modules.TotalModuleCalls),
				fmt.Sprintf("%d", analysis.ResourceAnalysis.TotalResourceCount),
				fmt.Sprintf("%d", len(analysis.VariableAnalysis.DefinedVariables)),
				fmt.Sprintf("%d", analysis.OutputAnalysis.OutputCount),
			})
		}
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	return ResultsModel{
		table:   t,
		results: results,
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// TUI Progress Channel for communication
type TUIProgressChannel struct {
	progressChan chan ProgressMsg
	completeChan chan CompletedMsg
	program      *tea.Program
}

func NewTUIProgressChannel(total int) *TUIProgressChannel {
	model := NewTUIModel(total)
	program := tea.NewProgram(model, tea.WithAltScreen())
	
	return &TUIProgressChannel{
		progressChan: make(chan ProgressMsg, 100),
		completeChan: make(chan CompletedMsg, 1),
		program:      program,
	}
}

func (t *TUIProgressChannel) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case progress := <-t.progressChan:
				t.program.Send(progress)
			case complete := <-t.completeChan:
				t.program.Send(complete)
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (t *TUIProgressChannel) UpdateProgress(repo, org string, completed, total int) {
	t.UpdateProgressWithPhase(repo, org, "", completed, total, 0, 0)
}

func (t *TUIProgressChannel) UpdateProgressWithPhase(repo, org, phase string, completed, total, repoCount, totalRepos int) {
	select {
	case t.progressChan <- ProgressMsg{
		Repo:         repo,
		Organization: org,
		Phase:        phase,
		Completed:    completed,
		Total:        total,
		RepoCount:    repoCount,
		TotalRepos:   totalRepos,
	}:
	default:
		// Channel full, skip update
	}
}

func (t *TUIProgressChannel) Complete(results []AnalysisResult) {
	t.completeChan <- CompletedMsg{Results: results}
}

func (t *TUIProgressChannel) Run() error {
	_, err := t.program.Run()
	return err
}