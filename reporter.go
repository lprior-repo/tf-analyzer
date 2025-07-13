package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bitfield/script"
	"github.com/samber/lo"
)

// ============================================================================
// REPORTER - Unified reporting functionality
// ============================================================================

type BackendConfigSummary struct {
	Type   string `json:"type"`
	Region string `json:"region"`
	Count  int    `json:"count"`
}

type GlobalBackendSummary struct {
	UniqueBackendConfigCount int                     `json:"unique_backend_config_count"`
	BackendConfigs           []BackendConfigSummary  `json:"backend_configs"`
}

type GlobalSummary struct {
	TotalReposScanned    int                  `json:"total_repos_scanned"`
	GlobalBackendSummary GlobalBackendSummary `json:"global_backend_summary"`
}

type ComprehensiveReport struct {
	Repositories  []RepositoryAnalysis `json:"repositories"`
	GlobalSummary GlobalSummary        `json:"global_summary"`
}

type Reporter struct {
	results []AnalysisResult
}

func NewReporter() *Reporter {
	return &Reporter{
		results: make([]AnalysisResult, 0),
	}
}

func (r *Reporter) AddResults(results []AnalysisResult) {
	r.results = append(r.results, results...)
}

func (r *Reporter) generateGlobalSummary() GlobalSummary {
	successfulResults := r.getSuccessfulResults()
	backendSummary := r.aggregateBackends(successfulResults)

	return GlobalSummary{
		TotalReposScanned:    len(successfulResults),
		GlobalBackendSummary: backendSummary,
	}
}

func (r *Reporter) getSuccessfulResults() []AnalysisResult {
	return lo.Filter(r.results, func(result AnalysisResult, _ int) bool {
		return result.Error == nil
	})
}

func (r *Reporter) aggregateBackends(results []AnalysisResult) GlobalBackendSummary {
	backendMap := make(map[string]int)

	for _, result := range results {
		backendType := getBackendType(result.Analysis.BackendConfig)
		backendRegion := getBackendRegion(result.Analysis.BackendConfig)
		key := fmt.Sprintf("%s:%s", backendType, backendRegion)
		backendMap[key]++
	}

	backendConfigs := lo.MapToSlice(backendMap, func(key string, count int) BackendConfigSummary {
		parts := strings.Split(key, ":")
		return BackendConfigSummary{
			Type:   parts[0],
			Region: parts[1],
			Count:  count,
		}
	})

	return GlobalBackendSummary{
		UniqueBackendConfigCount: len(backendConfigs),
		BackendConfigs:           backendConfigs,
	}
}

func getBackendType(config *BackendConfig) string {
	if config == nil || config.Type == nil {
		return "none"
	}
	return *config.Type
}

func getBackendRegion(config *BackendConfig) string {
	if config == nil || config.Region == nil {
		return "none"
	}
	return *config.Region
}

func (r *Reporter) GenerateReport() ComprehensiveReport {
	successfulResults := r.getSuccessfulResults()
	
	repositories := lo.Map(successfulResults, func(result AnalysisResult, _ int) RepositoryAnalysis {
		return result.Analysis
	})

	globalSummary := r.generateGlobalSummary()

	return ComprehensiveReport{
		Repositories:  repositories,
		GlobalSummary: globalSummary,
	}
}

func (r *Reporter) PrintSummaryReport() error {
	report := r.GenerateReport()
	
	printReportHeader()
	printOverallStats(report)
	printBackendSummary(report.GlobalSummary.GlobalBackendSummary)
	printRepositorySummaries(report.Repositories)
	printReportFooter()

	return nil
}

func printReportHeader() {
	header := "\n" + strings.Repeat("=", 80)
	title := "                    TERRAFORM ANALYZER - COMPREHENSIVE REPORT"
	footer := strings.Repeat("=", 80)
	slog.Info(header)
	slog.Info(title)
	slog.Info(footer)
}

func printOverallStats(report ComprehensiveReport) {
	totalProviders := calculateTotalProviders(report.Repositories)
	totalModules := calculateTotalModules(report.Repositories)
	totalResources := calculateTotalResources(report.Repositories)
	totalVariables := calculateTotalVariables(report.Repositories)
	totalOutputs := calculateTotalOutputs(report.Repositories)

	slog.Info("Overall Statistics",
		"total_repositories", report.GlobalSummary.TotalReposScanned,
		"total_providers", totalProviders,
		"total_modules", totalModules,
		"total_resources", totalResources,
		"total_variables", totalVariables,
		"total_outputs", totalOutputs)
}

func printBackendSummary(summary GlobalBackendSummary) {
	if summary.UniqueBackendConfigCount == 0 {
		return
	}

	slog.Info("Backend Configurations", "unique_configs", summary.UniqueBackendConfigCount)
	
	for _, config := range summary.BackendConfigs {
		slog.Info("Backend config", 
			"type", config.Type,
			"region", config.Region,
			"repository_count", config.Count)
	}
}

func printRepositorySummaries(repositories []RepositoryAnalysis) {
	if len(repositories) == 0 {
		return
	}

	slog.Info("Repository Details", "total_repositories", len(repositories))
	for _, repo := range repositories {
		repoName := extractRepoName(repo.RepositoryPath)
		slog.Info("Repository summary",
			"name", repoName,
			"providers", repo.Providers.UniqueProviderCount,
			"modules", repo.Modules.UniqueModuleCount,
			"resources", repo.ResourceAnalysis.TotalResourceCount,
			"variables", len(repo.VariableAnalysis.DefinedVariables),
			"outputs", repo.OutputAnalysis.OutputCount)
	}
}

func printReportFooter() {
	header := "\n" + strings.Repeat("=", 80)
	slog.Info(header)
	slog.Info("Report completed successfully")
	slog.Info(strings.Repeat("=", 80))
}

func extractRepoName(path string) string {
	if path == "" {
		return ""
	}
	
	// Clean trailing slashes
	cleanPath := strings.TrimRight(path, "/\\")
	if cleanPath == "" {
		return ""
	}
	
	// Handle both Unix-style (/) and Windows-style (\) path separators
	// Split on both and take the last non-empty part
	var parts []string
	
	// First split on forward slashes
	unixParts := strings.Split(cleanPath, "/")
	for _, part := range unixParts {
		// Then split each part on backslashes (for Windows paths)
		winParts := strings.Split(part, "\\")
		parts = append(parts, winParts...)
	}
	
	// Find the last non-empty part
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.TrimSpace(parts[i]) != "" {
			return parts[i]
		}
	}
	
	return ""
}

func calculateTotalProviders(repositories []RepositoryAnalysis) int {
	return lo.Reduce(repositories, func(acc int, repo RepositoryAnalysis, _ int) int {
		return acc + repo.Providers.UniqueProviderCount
	}, 0)
}

func calculateTotalModules(repositories []RepositoryAnalysis) int {
	return lo.Reduce(repositories, func(acc int, repo RepositoryAnalysis, _ int) int {
		return acc + repo.Modules.TotalModuleCalls
	}, 0)
}

func calculateTotalResources(repositories []RepositoryAnalysis) int {
	return lo.Reduce(repositories, func(acc int, repo RepositoryAnalysis, _ int) int {
		return acc + repo.ResourceAnalysis.TotalResourceCount
	}, 0)
}

func calculateTotalVariables(repositories []RepositoryAnalysis) int {
	return lo.Reduce(repositories, func(acc int, repo RepositoryAnalysis, _ int) int {
		return acc + len(repo.VariableAnalysis.DefinedVariables)
	}, 0)
}

func calculateTotalOutputs(repositories []RepositoryAnalysis) int {
	return lo.Reduce(repositories, func(acc int, repo RepositoryAnalysis, _ int) int {
		return acc + repo.OutputAnalysis.OutputCount
	}, 0)
}

func (r *Reporter) ExportJSON(filename string) error {
	report := r.GenerateReport()
	
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	_, err = script.Echo(string(jsonData)).WriteFile(filename)
	if err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	slog.Info("Comprehensive report exported", "file", filename, "type", "JSON")
	return nil
}

func (r *Reporter) ExportCSV(filename string) error {
	successfulResults := r.getSuccessfulResults()
	
	csvLines := []string{
		"Repository,Path,BackendType,BackendRegion,Providers,Modules,Resources,Variables,Outputs,UntaggedResources",
	}

	for _, result := range successfulResults {
		analysis := result.Analysis
		repoName := extractRepoName(analysis.RepositoryPath)
		
		csvLines = append(csvLines, fmt.Sprintf("%s,%s,%s,%s,%d,%d,%d,%d,%d,%d",
			repoName,
			analysis.RepositoryPath,
			getBackendType(analysis.BackendConfig),
			getBackendRegion(analysis.BackendConfig),
			analysis.Providers.UniqueProviderCount,
			analysis.Modules.TotalModuleCalls,
			analysis.ResourceAnalysis.TotalResourceCount,
			len(analysis.VariableAnalysis.DefinedVariables),
			analysis.OutputAnalysis.OutputCount,
			len(analysis.ResourceAnalysis.UntaggedResources),
		))
	}

	csvContent := strings.Join(csvLines, "\n")
	_, err := script.Echo(csvContent).WriteFile(filename)
	if err != nil {
		return fmt.Errorf("failed to write CSV file: %w", err)
	}

	slog.Info("CSV report exported", "file", filename, "type", "CSV")
	return nil
}

func (r *Reporter) ExportMarkdown(filename string) error {
	markdownContent := r.generateMarkdownContent()
	
	_, err := script.Echo(markdownContent).WriteFile(filename)
	if err != nil {
		return fmt.Errorf("failed to write markdown file: %w", err)
	}
	
	slog.Info("Markdown report exported", "file", filename, "type", "Markdown")
	return nil
}

func (r *Reporter) PrintMarkdownToScreen() {
	markdownContent := r.generateMarkdownContent()
	fmt.Print(markdownContent)
}

func (r *Reporter) generateMarkdownContent() string {
	report := r.GenerateReport()
	skippedRepos := r.getSkippedRepositories()
	
	var markdownBuilder strings.Builder
	
	r.appendReportHeader(&markdownBuilder)
	r.appendExecutiveSummary(&markdownBuilder, &report, skippedRepos)
	r.appendBackendSummary(&markdownBuilder, &report)
	r.appendRepositoryDetails(&markdownBuilder, &report)
	r.appendSkippedRepositories(&markdownBuilder, skippedRepos)
	r.appendProviderDetails(&markdownBuilder, &report)
	r.appendUntaggedResourcesSummary(&markdownBuilder, &report)
	r.appendReportFooter(&markdownBuilder)
	
	return markdownBuilder.String()
}

func (r *Reporter) appendReportHeader(builder *strings.Builder) {
	builder.WriteString("# Terraform Analysis Report\n\n")
	fmt.Fprintf(builder, "Generated on: %s\n\n", 
		time.Now().Format("2006-01-02 15:04:05 UTC"))
}

func (r *Reporter) appendExecutiveSummary(builder *strings.Builder, report *ComprehensiveReport, skippedRepos []string) {
	builder.WriteString("## Executive Summary\n\n")
	fmt.Fprintf(builder, "- **Total repositories scanned**: %d\n", 
		report.GlobalSummary.TotalReposScanned)
	fmt.Fprintf(builder, "- **Repositories with content**: %d\n", 
		len(report.Repositories))
	fmt.Fprintf(builder, "- **Repositories skipped (no relevant content)**: %d\n", 
		len(skippedRepos))
	fmt.Fprintf(builder, "- **Total providers found**: %d\n", 
		calculateTotalProviders(report.Repositories))
	fmt.Fprintf(builder, "- **Total modules found**: %d\n", 
		calculateTotalModules(report.Repositories))
	fmt.Fprintf(builder, "- **Total resources found**: %d\n", 
		calculateTotalResources(report.Repositories))
	fmt.Fprintf(builder, "- **Total variables found**: %d\n", 
		calculateTotalVariables(report.Repositories))
	fmt.Fprintf(builder, "- **Total outputs found**: %d\n", 
		calculateTotalOutputs(report.Repositories))
	builder.WriteString("\n")
}

func (r *Reporter) appendBackendSummary(builder *strings.Builder, report *ComprehensiveReport) {
	if report.GlobalSummary.GlobalBackendSummary.UniqueBackendConfigCount == 0 {
		return
	}
	
	builder.WriteString("## Backend Configuration Summary\n\n")
	fmt.Fprintf(builder, "Found **%d** unique backend configurations:\n\n", 
		report.GlobalSummary.GlobalBackendSummary.UniqueBackendConfigCount)
	
	builder.WriteString("| Backend Type | Region | Repository Count |\n")
	builder.WriteString("|-------------|--------|------------------|\n")
	
	for _, config := range report.GlobalSummary.GlobalBackendSummary.BackendConfigs {
		fmt.Fprintf(builder, "| %s | %s | %d |\n",
			config.Type, config.Region, config.Count)
	}
	builder.WriteString("\n")
}

func (r *Reporter) appendRepositoryDetails(builder *strings.Builder, report *ComprehensiveReport) {
	if len(report.Repositories) == 0 {
		return
	}
	
	builder.WriteString("## Repository Analysis Details\n\n")
	builder.WriteString("| Repository | Providers | Modules | Resources | Variables | Outputs | Backend | Region |\n")
	builder.WriteString("|------------|-----------|---------|-----------|-----------|---------|---------|--------|\n")
	
	for _, repo := range report.Repositories {
		r.appendRepositoryRow(builder, repo)
	}
	builder.WriteString("\n")
}

func (r *Reporter) appendRepositoryRow(builder *strings.Builder, repo RepositoryAnalysis) {
	repoName := extractRepoName(repo.RepositoryPath)
	backendType := getBackendType(repo.BackendConfig)
	backendRegion := getBackendRegion(repo.BackendConfig)
	
	fmt.Fprintf(builder, "| %s | %d | %d | %d | %d | %d | %s | %s |\n",
		repoName,
		repo.Providers.UniqueProviderCount,
		repo.Modules.TotalModuleCalls,
		repo.ResourceAnalysis.TotalResourceCount,
		len(repo.VariableAnalysis.DefinedVariables),
		repo.OutputAnalysis.OutputCount,
		backendType,
		backendRegion)
}

func (r *Reporter) appendSkippedRepositories(builder *strings.Builder, skippedRepos []string) {
	if len(skippedRepos) == 0 {
		return
	}
	
	builder.WriteString("## Repositories with No Relevant Content\n\n")
	builder.WriteString("The following repositories were scanned but contained no Terraform files (.tf, .tfvars, .hcl):\n\n")
	
	for _, repoName := range skippedRepos {
		fmt.Fprintf(builder, "- %s\n", repoName)
	}
	builder.WriteString("\n")
}

func (r *Reporter) appendProviderDetails(builder *strings.Builder, report *ComprehensiveReport) {
	if len(report.Repositories) == 0 {
		return
	}
	
	builder.WriteString("## Provider Usage Details\n\n")
	providerUsage := r.aggregateProviderUsage(report.Repositories)
	
	if len(providerUsage) > 0 {
		builder.WriteString("| Provider | Version | Repository Count |\n")
		builder.WriteString("|----------|---------|------------------|\n")
		
		for _, provider := range providerUsage {
			fmt.Fprintf(builder, "| %s | %s | %d |\n",
				provider.Source, provider.Version, provider.Count)
		}
		builder.WriteString("\n")
	}
}

func (r *Reporter) appendUntaggedResourcesSummary(builder *strings.Builder, report *ComprehensiveReport) {
	untaggedResourcesCount := r.calculateTotalUntaggedResources(report.Repositories)
	if untaggedResourcesCount == 0 {
		return
	}
	
	builder.WriteString("## Resource Tagging Compliance\n\n")
	fmt.Fprintf(builder, "Found **%d** resources missing mandatory tags (%s).\n\n",
		untaggedResourcesCount, strings.Join(mandatoryTags, ", "))
	
	builder.WriteString("### Repositories with Untagged Resources\n\n")
	builder.WriteString("| Repository | Untagged Resources |\n")
	builder.WriteString("|------------|--------------------|\n")
	
	for _, repo := range report.Repositories {
		if len(repo.ResourceAnalysis.UntaggedResources) > 0 {
			repoName := extractRepoName(repo.RepositoryPath)
			fmt.Fprintf(builder, "| %s | %d |\n",
				repoName, len(repo.ResourceAnalysis.UntaggedResources))
		}
	}
	builder.WriteString("\n")
}

func (r *Reporter) appendReportFooter(builder *strings.Builder) {
	builder.WriteString("---\n")
	builder.WriteString("*Report generated by tf-analyzer*\n")
}

type ProviderUsage struct {
	Source  string
	Version string
	Count   int
}

func (r *Reporter) getSkippedRepositories() []string {
	allResults := r.results
	failedResults := lo.Filter(allResults, func(result AnalysisResult, _ int) bool {
		return result.Error != nil
	})
	
	return lo.Map(failedResults, func(result AnalysisResult, _ int) string {
		return result.RepoName
	})
}

func (r *Reporter) aggregateProviderUsage(repositories []RepositoryAnalysis) []ProviderUsage {
	providerMap := make(map[string]int)
	
	for _, repo := range repositories {
		for _, provider := range repo.Providers.ProviderDetails {
			key := fmt.Sprintf("%s@%s", provider.Source, provider.Version)
			providerMap[key]++
		}
	}
	
	return lo.MapToSlice(providerMap, func(key string, count int) ProviderUsage {
		parts := strings.Split(key, "@")
		source := parts[0]
		version := ""
		if len(parts) > 1 {
			version = parts[1]
		}
		return ProviderUsage{
			Source:  source,
			Version: version,
			Count:   count,
		}
	})
}

func (r *Reporter) calculateTotalUntaggedResources(repositories []RepositoryAnalysis) int {
	return lo.Reduce(repositories, func(acc int, repo RepositoryAnalysis, _ int) int {
		return acc + len(repo.ResourceAnalysis.UntaggedResources)
	}, 0)
}

func convertMarkdownToTerminal(markdown string) string {
	lines := strings.Split(markdown, "\n")
	var terminalLines []string
	
	for _, line := range lines {
		converted := convertLineToTerminal(line)
		terminalLines = append(terminalLines, converted)
	}
	
	return strings.Join(terminalLines, "\n")
}

func convertLineToTerminal(line string) string {
	// Convert headers
	if strings.HasPrefix(line, "# ") {
		return fmt.Sprintf("\n%s\n%s", 
			strings.ToUpper(strings.TrimPrefix(line, "# ")),
			strings.Repeat("=", len(strings.TrimPrefix(line, "# "))))
	}
	if strings.HasPrefix(line, "## ") {
		return fmt.Sprintf("\n%s\n%s", 
			strings.TrimPrefix(line, "## "),
			strings.Repeat("-", len(strings.TrimPrefix(line, "## "))))
	}
	if strings.HasPrefix(line, "### ") {
		return fmt.Sprintf("\n%s:", strings.TrimPrefix(line, "### "))
	}
	
	// Convert bullet points
	if strings.HasPrefix(line, "- ") {
		return fmt.Sprintf("  • %s", strings.TrimPrefix(line, "- "))
	}
	
	// Convert bold text
	line = convertBoldText(line)
	
	// Convert tables to aligned format
	if strings.Contains(line, "|") && !strings.Contains(line, "---") {
		return formatTableRow(line)
	}
	
	// Skip table separator lines
	if strings.Contains(line, "---") && strings.Contains(line, "|") {
		return ""
	}
	
	// Convert horizontal rules
	if strings.TrimSpace(line) == "---" {
		return strings.Repeat("-", 60)
	}
	
	return line
}

func convertBoldText(line string) string {
	// Convert **text** to uppercase
	for strings.Contains(line, "**") {
		start := strings.Index(line, "**")
		if start == -1 {
			break
		}
		end := strings.Index(line[start+2:], "**")
		if end == -1 {
			break
		}
		end += start + 2
		
		boldText := line[start+2 : end]
		line = line[:start] + strings.ToUpper(boldText) + line[end+2:]
	}
	return line
}

func formatTableRow(line string) string {
	if !strings.Contains(line, "|") {
		return line
	}
	
	cells := strings.Split(line, "|")
	if len(cells) < 3 {
		return line
	}
	
	// Remove empty first/last cells from markdown table format
	if strings.TrimSpace(cells[0]) == "" {
		cells = cells[1:]
	}
	if len(cells) > 0 && strings.TrimSpace(cells[len(cells)-1]) == "" {
		cells = cells[:len(cells)-1]
	}
	
	// Trim and format each cell
	formattedCells := lo.Map(cells, func(cell string, _ int) string {
		return fmt.Sprintf("%-20s", strings.TrimSpace(cell))
	})
	
	return "  " + strings.Join(formattedCells, " ")
}

