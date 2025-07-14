package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// TUI - Terminal User Interface using Bubble Tea
// Data Structures (Pure data following CLAUDE.md principles)
// ============================================================================

// RepoProgressData holds pure repository progress data
type RepoProgressData struct {
	Name      string
	Org       string
	Status    string
	Percent   float64
	StartTime time.Time
	Elapsed   time.Duration
}

// ProgressData holds pure progress state data
type ProgressData struct {
	Total          int
	Completed      int
	CurrentRepo    string
	CurrentOrg     string
	CurrentPhase   string
	RepoCount      int
	TotalRepos     int
	Done           bool
	Results        []AnalysisResult
	ActiveRepos    []string
	CompletedRepos []string
	QueuedRepos    []string
	PageOffset     int
	PageSize       int
}

// ResultsData holds pure results state data  
type ResultsData struct {
	Results     []AnalysisResult
	ShowDetails bool
	CurrentPage int
	TotalPages  int
	PageSize    int
}

// TUIState holds pure TUI state data
type TUIState struct {
	Mode     string // "progress" or "results"
	Progress ProgressData
	Results  ResultsData
	Width    int
	Height   int
}

// Models contain impure components (progress bars, tables) 
type RepoProgress struct {
	data     RepoProgressData
	progress progress.Model
}

type ProgressModel struct {
	data         ProgressData
	progress     progress.Model
	repoProgress map[string]*RepoProgress
}

type ResultsModel struct {
	data  ResultsData
	table table.Model
}

type TUIModel struct {
	state    TUIState
	progress ProgressModel
	results  ResultsModel
}

type ProgressMsg struct {
	Repo         string
	Organization string
	Completed    int
	Total        int
	Phase        string
	RepoCount    int
	TotalRepos   int
	Percent      float64
	Status       string
}

type RepoProgressMsg struct {
	Repo    string
	Org     string
	Percent float64
	Status  string
	Phase   string
}

type RepoQueueMsg struct {
	Queued    []string
	Active    []string
	Completed []string
}

type TickMsg time.Time

type UpdateTotalReposMsg struct {
	TotalRepos int
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
		state: TUIState{
			Mode: "progress",
			Progress: ProgressData{
				Total:    total,
				PageSize: 5,
			},
			Results: ResultsData{
				PageSize: 10,
			},
		},
		progress: ProgressModel{
			data: ProgressData{
				Total:        total,
				PageOffset:   0,
				PageSize:     5,
			},
			progress:     p,
			repoProgress: make(map[string]*RepoProgress),
		},
		results: ResultsModel{
			data: ResultsData{
				PageSize:    10,
				CurrentPage: 0,
			},
		},
	}
}

func (m TUIModel) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.progress.progress.Init(),
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
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
	case UpdateTotalReposMsg:
		return m.handleTotalReposUpdate(msg), nil
	case CompletedMsg:
		return m.handleAnalysisCompletion(msg)
	case RepoProgressMsg:
		return m.handleRepoProgress(msg)
	case RepoQueueMsg:
		return m.handleRepoQueue(msg)
	case TickMsg:
		return m.handleTick(msg)
	case progress.FrameMsg:
		return m.handleProgressFrame(msg)
	}

	return m.handleResultsModeUpdate(msg)
}

func (m TUIModel) handleKeyInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	
	if cmd := handleExitKeys(key); cmd != nil {
		return m, cmd
	}
	
	if shouldExitOnKey(m, key) {
		return m, tea.Batch(tea.ExitAltScreen, tea.Quit)
	}
	
	if isToggleKey(key) {
		return handleToggleKeys(m, key), nil
	}
	
	return m, nil
}

func (m TUIModel) handleWindowResize(msg tea.WindowSizeMsg) TUIModel {
	m.state.Width = msg.Width
	m.state.Height = msg.Height
	m.progress.progress.Width = msg.Width - 20
	if m.state.Mode == "results" {
		m.results.table.SetHeight(msg.Height - 10)
		m.results.table.SetWidth(msg.Width - 4)
	}
	return m
}

func (m TUIModel) handleCustomWindowResize(msg WindowSizeMsg) TUIModel {
	m.state.Width = msg.Width
	m.state.Height = msg.Height
	m.progress.progress.Width = msg.Width - 20
	return m
}

func (m TUIModel) handleProgressUpdate(msg ProgressMsg) (tea.Model, tea.Cmd) {
	m.progress.data.CurrentRepo = msg.Repo
	m.progress.data.CurrentOrg = msg.Organization
	m.progress.data.CurrentPhase = msg.Phase
	m.progress.data.RepoCount = msg.RepoCount
	m.progress.data.TotalRepos = msg.TotalRepos
	m.progress.data.Completed = msg.Completed
	m.progress.data.Total = msg.Total
	
	var percent float64
	if msg.Total > 0 {
		percent = float64(msg.Completed) / float64(msg.Total)
	}
	
	return m, m.progress.progress.SetPercent(percent)
}

func (m TUIModel) handleTotalReposUpdate(msg UpdateTotalReposMsg) TUIModel {
	m.progress.data.TotalRepos = msg.TotalRepos
	m.progress.data.Total = msg.TotalRepos
	return m
}

func (m TUIModel) handleAnalysisCompletion(msg CompletedMsg) (tea.Model, tea.Cmd) {
	m.state.Mode = "results"
	m.progress.data.Done = true
	m.progress.data.Results = msg.Results
	
	m.results = createResultsModel(msg.Results)
	m.results.table.SetHeight(m.state.Height - 10)
	m.results.table.SetWidth(m.state.Width - 4)
	m = updateResultsPagination(m)
	
	return m, nil
}

func (m TUIModel) handleProgressFrame(msg progress.FrameMsg) (tea.Model, tea.Cmd) {
	if m.state.Mode == "progress" {
		progressModel, cmd := m.progress.progress.Update(msg)
		if pm, ok := progressModel.(progress.Model); ok {
			m.progress.progress = pm
		}
		return m, cmd
	}
	return m, nil
}


func (m TUIModel) handleRepoProgress(msg RepoProgressMsg) (tea.Model, tea.Cmd) {
	repoKey := createRepoKey(msg.Org, msg.Repo)
	
	if !repoProgressExists(m.progress.repoProgress, repoKey) {
		m.progress.repoProgress[repoKey] = createNewRepoProgress(msg)
	}
	
	return updateExistingRepoProgress(m, repoKey, msg)
}

func (m TUIModel) handleRepoQueue(msg RepoQueueMsg) (tea.Model, tea.Cmd) {
	m.progress.data.QueuedRepos = msg.Queued
	m.progress.data.ActiveRepos = msg.Active
	m.progress.data.CompletedRepos = msg.Completed
	return m, nil
}

func (m TUIModel) handleTick(msg TickMsg) (tea.Model, tea.Cmd) {
	for _, repo := range m.progress.repoProgress {
		repo.data.Elapsed = time.Since(repo.data.StartTime)
	}
	return m, tea.Batch(tickCmd())
}

func (m TUIModel) handleResultsModeUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.state.Mode == "results" {
		var cmd tea.Cmd
		m.results.table, cmd = m.results.table.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m TUIModel) View() string {
	if m.state.Mode == "progress" {
		return renderProgressView(m.state, m.progress)
	}
	return renderResultsView(m.state, m.results)
}

func renderProgressView(state TUIState, progress ProgressModel) string {
	var b strings.Builder

	// Title with organization info
	title := "TF-ANALYZER"
	if progress.data.CurrentOrg != "" {
		title = fmt.Sprintf("TF-ANALYZER • %s", progress.data.CurrentOrg)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Current phase and progress info
	if progress.data.CurrentPhase != "" {
		phaseEmoji := getPhaseEmoji(progress.data.CurrentPhase)
		phaseText := progress.data.CurrentPhase
		if progress.data.CurrentRepo != "" {
			phaseText = fmt.Sprintf("%s: %s", progress.data.CurrentPhase, progress.data.CurrentRepo)
		}
		b.WriteString(fmt.Sprintf("%s %s\n", phaseEmoji, statusStyle.Render(phaseText)))
	}
	
	// Repository count info
	if progress.data.TotalRepos > 0 {
		repoPercentage := float64(progress.data.RepoCount) / float64(progress.data.TotalRepos) * 100
		b.WriteString(fmt.Sprintf("📊 Repository Progress: %d/%d (%.1f%%)\n", 
			progress.data.RepoCount, progress.data.TotalRepos, repoPercentage))
	}

	// Overall progress bar
	b.WriteString("\n")
	b.WriteString(progress.progress.View())
	b.WriteString("\n\n")

	// Repository status overview
	b.WriteString(renderRepoStatusOverview(progress.data))

	// Individual repository progress (paginated)
	b.WriteString(renderPaginatedRepoProgress(progress.data, progress.repoProgress))

	// Overall stats
	percentage := float64(0)
	if progress.data.Total > 0 {
		percentage = float64(progress.data.Completed) / float64(progress.data.Total) * 100
	}
	b.WriteString(fmt.Sprintf("🎯 Overall Progress: %d/%d files (%.1f%%)\n", 
		progress.data.Completed, progress.data.Total, percentage))

	if progress.data.Done {
		b.WriteString("\n")
		b.WriteString(successStyle.Render("✅ Analysis complete!"))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("Press 'q' to quit • 'r' to view results"))
	} else {
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("🔑 Controls: [↑↓] navigate • [q] quit • Live updates"))
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

func renderResultsView(state TUIState, results ResultsModel) string {
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
	
	for _, result := range results.data.Results {
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
	b.WriteString(fmt.Sprintf("📊 %s | ", infoStyle.Render(fmt.Sprintf("Total: %d repos", len(results.data.Results)))))
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
	if results.data.ShowDetails {
		viewType = "Detailed View"
		b.WriteString(fmt.Sprintf("📋 %s:\n", infoStyle.Render(viewType)))
		b.WriteString(results.table.View())
	} else {
		b.WriteString(fmt.Sprintf("📋 %s:\n", infoStyle.Render(viewType)))
		b.WriteString(createSummaryTable(results.data.Results))
	}

	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("💡 Controls: [tab] toggle view | [r] export reports | [q] quit"))

	return b.String()
}

func createSummaryTable(results []AnalysisResult) string {
	var rows [][]string
	
	for _, result := range results {
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
		data: ResultsData{
			Results:  results,
			PageSize: 10, // Initialize with default page size to prevent divide by zero
		},
		table: t,
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
	program := tea.NewProgram(model)
	
	return &TUIProgressChannel{
		progressChan: make(chan ProgressMsg, 100),
		completeChan: make(chan CompletedMsg, 1),
		program:      program,
	}
}

func (t *TUIProgressChannel) Start(ctx context.Context) {
	go func() {
		defer func() {
			close(t.progressChan)
			close(t.completeChan)
		}()
		
		for {
			select {
			case progress := <-t.progressChan:
				t.program.Send(progress)
			case complete := <-t.completeChan:
				t.program.Send(complete)
				return
			case <-ctx.Done():
				t.program.Send(tea.Batch(tea.ExitAltScreen, tea.Quit))
				return
			}
		}
	}()
}

func (t *TUIProgressChannel) UpdateProgress(repo, org string, completed, total int) {
	t.UpdateProgressWithPhase(repo, org, "", completed, total, 0, 0)
}

// UpdateProgressWithUpdate uses the ProgressUpdate struct to reduce parameters
func (t *TUIProgressChannel) UpdateProgressWithUpdate(update ProgressUpdate) {
	select {
	case t.progressChan <- ProgressMsg{
		Repo:         update.Repo,
		Organization: update.Org,
		Phase:        update.Phase,
		Completed:    update.Completed,
		Total:        update.Total,
		RepoCount:    update.RepoCount,
		TotalRepos:   update.TotalRepos,
	}:
	default:
	}
}

func (t *TUIProgressChannel) UpdateProgressWithPhase(repo, org, phase string, completed, total, repoCount, totalRepos int) {
	t.UpdateProgressWithUpdate(ProgressUpdate{
		Repo: repo, Org: org, Phase: phase,
		Completed: completed, Total: total, RepoCount: repoCount, TotalRepos: totalRepos,
	})
}

func (t *TUIProgressChannel) UpdateTotalRepos(totalRepos int) {
	t.program.Send(UpdateTotalReposMsg{TotalRepos: totalRepos})
}

func (t *TUIProgressChannel) Complete(results []AnalysisResult) {
	select {
	case t.completeChan <- CompletedMsg{Results: results}:
	default:
	}
}

func (t *TUIProgressChannel) Run() error {
	_, err := t.program.Run()
	return err
}

func (t *TUIProgressChannel) Cleanup() {
	t.program.Send(tea.Batch(tea.ExitAltScreen, tea.Quit))
}

// ============================================================================
// Pure Calculation Functions (following CLAUDE.md functional principles)
// ============================================================================


func updateResultsPagination(model TUIModel) TUIModel {
	totalResults := len(model.results.data.Results)
	
	// Defensive programming: ensure PageSize is valid to prevent divide by zero
	if model.results.data.PageSize <= 0 {
		model.results.data.PageSize = 10 // Default page size
	}
	
	// Ensure CurrentPage is never negative before calculations
	if model.results.data.CurrentPage < 0 {
		model.results.data.CurrentPage = 0
	}
	
	if totalResults == 0 {
		model.results.data.TotalPages = 1
		model.results.data.CurrentPage = 0 // Always reset to 0 for empty results
		return model
	}
	
	model.results.data.TotalPages = (totalResults + model.results.data.PageSize - 1) / model.results.data.PageSize
	
	// Ensure TotalPages is at least 1
	if model.results.data.TotalPages < 1 {
		model.results.data.TotalPages = 1
	}
	
	// Ensure CurrentPage is within valid bounds
	if model.results.data.CurrentPage >= model.results.data.TotalPages {
		model.results.data.CurrentPage = model.results.data.TotalPages - 1
	}
	if model.results.data.CurrentPage < 0 {
		model.results.data.CurrentPage = 0
	}
	
	return model
}

func renderRepoStatusOverview(progress ProgressData) string {
	var b strings.Builder
	
	// Queue status
	queuedCount := len(progress.QueuedRepos)
	activeCount := len(progress.ActiveRepos)
	completedCount := len(progress.CompletedRepos)
	
	b.WriteString("📊 Repository Status:\n")
	b.WriteString(fmt.Sprintf("  🔄 Active: %d | ⏳ Queued: %d | ✅ Completed: %d\n\n", 
		activeCount, queuedCount, completedCount))
	
	return b.String()
}

func renderPaginatedRepoProgress(progress ProgressData, repoProgress map[string]*RepoProgress) string {
	var b strings.Builder
	
	activeRepos := progress.ActiveRepos
	if len(activeRepos) == 0 {
		return ""
	}
	
	startIdx := progress.PageOffset
	endIdx := startIdx + progress.PageSize
	if endIdx > len(activeRepos) {
		endIdx = len(activeRepos)
	}
	
	if startIdx >= len(activeRepos) {
		return ""
	}
	
	b.WriteString("🔍 Currently Processing:\n")
	
	for i := startIdx; i < endIdx; i++ {
		repoKey := activeRepos[i]
		if repo, exists := repoProgress[repoKey]; exists {
			statusIcon := getRepoStatusIcon(repo.data.Status)
			elapsed := formatDuration(repo.data.Elapsed)
			
			b.WriteString(fmt.Sprintf("  %s %s/%s (%s) [%s]\n", 
				statusIcon, repo.data.Org, repo.data.Name, elapsed, repo.data.Status))
			b.WriteString(fmt.Sprintf("    %s\n", repo.progress.View()))
		}
	}
	
	// Pagination controls
	if len(activeRepos) > progress.PageSize {
		totalPages := (len(activeRepos) + progress.PageSize - 1) / progress.PageSize
		currentPage := progress.PageOffset/progress.PageSize + 1
		b.WriteString(fmt.Sprintf("\n📄 Page %d/%d (↑↓ to navigate)\n", currentPage, totalPages))
	}
	
	b.WriteString("\n")
	return b.String()
}

func getRepoStatusIcon(status string) string {
	switch strings.ToLower(status) {
	case "cloning":
		return "📥"
	case "analyzing":
		return "🔍"
	case "processing":
		return "⚡"
	case "complete":
		return "✅"
	case "failed":
		return "❌"
	default:
		return "🔄"
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

// ============================================================================
// Repo Progress Management Functions (Pure calculations and focused actions)
// ============================================================================

func createRepoKey(org, repo string) string {
	return fmt.Sprintf("%s/%s", org, repo)
}

func repoProgressExists(repoProgress map[string]*RepoProgress, repoKey string) bool {
	_, exists := repoProgress[repoKey]
	return exists
}

func createNewRepoProgress(msg RepoProgressMsg) *RepoProgress {
	p := progress.New(progress.WithDefaultGradient())
	p.Width = 30
	return &RepoProgress{
		data: RepoProgressData{
			Name:      msg.Repo,
			Org:       msg.Org,
			Status:    msg.Status,
			StartTime: time.Now(),
		},
		progress: p,
	}
}

func updateExistingRepoProgress(model TUIModel, repoKey string, msg RepoProgressMsg) (tea.Model, tea.Cmd) {
	repo := model.progress.repoProgress[repoKey]
	repo.data.Percent = msg.Percent
	repo.data.Status = msg.Status
	repo.data.Elapsed = time.Since(repo.data.StartTime)
	
	cmd := repo.progress.SetPercent(msg.Percent)
	return model, cmd
}

// ============================================================================
// Key Input Handling Functions (Pure calculations and focused actions)
// ============================================================================

func handleExitKeys(key string) tea.Cmd {
	if key == "q" || key == "ctrl+c" {
		return tea.Batch(tea.ExitAltScreen, tea.Quit)
	}
	return nil
}

func shouldExitOnKey(model TUIModel, key string) bool {
	return key == "r" && model.state.Mode == "results"
}

func isToggleKey(key string) bool {
	return key == "tab"
}

func handleToggleKeys(model TUIModel, key string) TUIModel {
	if key == "tab" && model.state.Mode == "results" {
		model.results.data.ShowDetails = !model.results.data.ShowDetails
	}
	return model
}

