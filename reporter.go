package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

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
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
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

