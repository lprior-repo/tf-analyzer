package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/panjf2000/ants/v2"
	"github.com/samber/lo"
	"github.com/sourcegraph/conc/pool"
)

// ============================================================================
// ORCHESTRATOR - Main workflow coordination and data structures
// ============================================================================

type Config struct {
	MaxGoroutines    int
	CloneConcurrency int
	ProcessTimeout   time.Duration
	SkipArchived     bool
	SkipForks        bool
	GitHubToken      string
	Organizations    []string
	BaseURL          string
}

type Repository struct {
	Name         string
	Path         string
	Organization string
}

type FileContent struct {
	RelativePath string
	Content      []byte
	Size         int
}

type ProcessingStats struct {
	TotalOrgs      int
	TotalRepos     int
	ProcessedRepos int
	FailedRepos    int
	TotalFiles     int
	Duration       time.Duration
}

type OrganizationResult struct {
	Organization string
	Results      []AnalysisResult
	Duration     time.Duration
	Error        error
}

type ProcessingContext struct {
	Config Config
	Pool   *ants.Pool
}

func parseOrganizations(orgString string) []string {
	if orgString == "" {
		return []string{}
	}

	orgs := strings.Split(orgString, ",")
	return lo.Map(orgs, func(org string, _ int) string {
		return strings.TrimSpace(org)
	})
}

func createConfigFromEnv(envVars map[string]string) Config {
	getEnvOrDefault := func(key, defaultValue string) string {
		if value, exists := envVars[key]; exists && value != "" {
			return value
		}
		return defaultValue
	}

	return Config{
		MaxGoroutines:    100,
		CloneConcurrency: 100,
		ProcessTimeout:   30 * time.Minute,
		SkipArchived:     true,
		SkipForks:        false,
		GitHubToken:      getEnvOrDefault("GITHUB_TOKEN", ""),
		Organizations:    parseOrganizations(getEnvOrDefault("GITHUB_ORGS", "")),
		BaseURL:          getEnvOrDefault("GITHUB_BASE_URL", ""),
	}
}

func validateConfig(config Config) error {
	if config.MaxGoroutines <= 0 {
		return fmt.Errorf("MaxGoroutines must be positive, got %d", config.MaxGoroutines)
	}
	if config.CloneConcurrency <= 0 {
		return fmt.Errorf("CloneConcurrency must be positive, got %d", config.CloneConcurrency)
	}
	if config.GitHubToken == "" {
		return fmt.Errorf("GITHUB_TOKEN is required")
	}
	if len(config.Organizations) == 0 {
		return fmt.Errorf("at least one organization must be specified in GITHUB_ORGS")
	}
	return nil
}

func filterRepositoryDirs(entries []os.DirEntry) []string {
	return lo.FilterMap(entries, func(entry os.DirEntry, _ int) (string, bool) {
		return entry.Name(), entry.IsDir()
	})
}

func createRepository(name, orgPath, organization string) Repository {
	return Repository{
		Name:         name,
		Path:         filepath.Join(orgPath, name),
		Organization: organization,
	}
}

func loadEnvironmentFile(filePath string) error {
	return godotenv.Load(filePath)
}

func getEnvironmentVariables() map[string]string {
	return map[string]string{
		"GITHUB_TOKEN":    os.Getenv("GITHUB_TOKEN"),
		"GITHUB_ORGS":     os.Getenv("GITHUB_ORGS"),
		"GITHUB_BASE_URL": os.Getenv("GITHUB_BASE_URL"),
	}
}

func readDirectory(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

func logProgress(message string, args ...interface{}) {
	slog.Info(fmt.Sprintf(message, args...))
}

func logError(err error) {
	slog.Error("operation failed", "error", err)
}

func logWarning(message string, args ...interface{}) {
	slog.Warn(fmt.Sprintf(message, args...))
}

func logDebug(message string, args ...interface{}) {
	slog.Debug(fmt.Sprintf(message, args...))
}

func createProcessingContext(config Config) (ProcessingContext, error) {
	validationErr := validateConfig(config)
	if validationErr != nil {
		return ProcessingContext{}, validationErr
	}

	pool, poolErr := ants.NewPool(config.MaxGoroutines, ants.WithPreAlloc(true))
	if poolErr != nil {
		return ProcessingContext{}, fmt.Errorf("failed to create goroutine pool: %w", poolErr)
	}

	return ProcessingContext{
		Config: config,
		Pool:   pool,
	}, nil
}

func releaseProcessingContext(ctx ProcessingContext) {
	if ctx.Pool != nil {
		ctx.Pool.Release()
	}
}

func discoverRepositories(tempDir, org string) ([]Repository, error) {
	orgDir, orgErr := findOrgDirectory(tempDir)
	if orgErr != nil {
		return nil, orgErr
	}

	entries, readErr := readDirectory(orgDir)
	if readErr != nil {
		return nil, readErr
	}

	repoNames := filterRepositoryDirs(entries)
	repositories := lo.Map(repoNames, func(name string, _ int) Repository {
		return createRepository(name, orgDir, org)
	})

	logProgress("Found %d repositories to process in %s", len(repositories), org)
	return repositories, nil
}

func createJobSubmitter(pool *ants.Pool) func(Repository) AnalysisResult {
	return createJobSubmitterWithRecovery(pool, slog.Default())
}

func createJobSubmitterWithRecovery(pool *ants.Pool, logger *slog.Logger) func(Repository) AnalysisResult {
	return func(repo Repository) AnalysisResult {
		repoLogger := logger.With("repository", repo.Name, "organization", repo.Organization)
		
		submitErr := pool.Submit(func() {
			// This will be handled by the result channel
		})

		if submitErr != nil {
			repoLogger.Error("Failed to submit repository job", "error", submitErr)
			return AnalysisResult{
				RepoName:     repo.Name,
				Organization: repo.Organization,
				Error:        fmt.Errorf("failed to submit job: %w", submitErr),
			}
		}

		return processRepositoryFilesWithRecovery(repo, repoLogger)
	}
}

func createResultChannel(repositories []Repository) chan AnalysisResult {
	return make(chan AnalysisResult, len(repositories))
}

func configureWaitGroup(maxGoroutines int) *pool.Pool {
	return pool.New().WithMaxGoroutines(maxGoroutines)
}

func submitRepositoryJobs(repositories []Repository, p *pool.Pool, antsPool *ants.Pool, results chan AnalysisResult) {
	submitRepositoryJobsWithRecovery(repositories, p, antsPool, results, slog.Default())
}

func submitRepositoryJobsWithRecovery(repositories []Repository, p *pool.Pool, antsPool *ants.Pool, results chan AnalysisResult, logger *slog.Logger) {
	jobSubmitter := createJobSubmitterWithRecovery(antsPool, logger)

	for _, repo := range repositories {
		repo := repo
		p.Go(func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Repository processing panic recovered", 
						"repository", repo.Name, 
						"organization", repo.Organization, 
						"panic", r)
					results <- AnalysisResult{
						RepoName:     repo.Name,
						Organization: repo.Organization,
						Error:        fmt.Errorf("panic during processing: %v", r),
					}
				}
			}()
			result := jobSubmitter(repo)
			results <- result
		})
	}
}

func waitAndCloseChannel(p *pool.Pool, results chan AnalysisResult) {
	p.Wait()
	close(results)
}

func logRepositoryResult(result AnalysisResult) {
	logRepositoryResultStructured(result, slog.Default())
}

func logRepositoryResultStructured(result AnalysisResult, logger *slog.Logger) {
	if result.Error != nil {
		logger.Error("Repository analysis failed", 
			"repository", result.RepoName,
			"organization", result.Organization,
			"error", result.Error)
		return
	}

	analysis := result.Analysis
	logger.Info("Repository analysis completed",
		"organization", result.Organization,
		"repository", result.RepoName,
		"providers", analysis.Providers.UniqueProviderCount,
		"modules", analysis.Modules.TotalModuleCalls,
		"resources", analysis.ResourceAnalysis.TotalResourceCount,
		"variables", len(analysis.VariableAnalysis.DefinedVariables),
		"outputs", analysis.OutputAnalysis.OutputCount)
}

func shouldLogProgress(currentCount int) bool {
	return currentCount%50 == 0
}

func logProgressIfNeeded(currentCount int) {
	if shouldLogProgress(currentCount) {
		logProgress("Processed %d repositories...", currentCount)
	}
}

func collectResults(results chan AnalysisResult) []AnalysisResult {
	return collectResultsWithRecovery(results, slog.Default())
}

func collectResultsWithRecovery(results chan AnalysisResult, logger *slog.Logger) []AnalysisResult {
	var allResults []AnalysisResult
	successful := 0
	failed := 0

	for result := range results {
		allResults = append(allResults, result)
		if result.Error != nil {
			failed++
			logger.Error("Repository processing failed", 
				"repository", result.RepoName,
				"organization", result.Organization,
				"error", result.Error)
		} else {
			successful++
			logRepositoryResultStructured(result, logger)
		}
		
		if len(allResults)%50 == 0 {
			logger.Info("Processing progress", 
				"processed", len(allResults),
				"successful", successful,
				"failed", failed)
		}
	}

	logger.Info("Repository processing complete", 
		"total_processed", len(allResults),
		"successful", successful,
		"failed", failed)

	return allResults
}

func calculateStats(allResults []AnalysisResult, duration time.Duration) ProcessingStats {
	successful := lo.Filter(allResults, func(r AnalysisResult, _ int) bool {
		return r.Error == nil
	})

	failed := lo.Filter(allResults, func(r AnalysisResult, _ int) bool {
		return r.Error != nil
	})

	totalFiles := lo.Reduce(successful, func(acc int, result AnalysisResult, _ int) int {
		return acc + result.Analysis.ResourceAnalysis.TotalResourceCount
	}, 0)

	return ProcessingStats{
		TotalOrgs:      1,
		TotalRepos:     len(allResults),
		ProcessedRepos: len(successful),
		FailedRepos:    len(failed),
		TotalFiles:     totalFiles,
		Duration:       duration,
	}
}

func logStats(stats ProcessingStats) {
	logProgress("\n=== Processing Complete ===")
	logProgress("Total repositories: %d", stats.TotalRepos)
	logProgress("Successfully processed: %d", stats.ProcessedRepos)
	logProgress("Failed: %d", stats.FailedRepos)
	logProgress("Total files extracted: %d", stats.TotalFiles)
	logProgress("Total duration: %v", stats.Duration)
}

func finalizeProcessing(allResults []AnalysisResult, startTime time.Time) {
	stats := calculateStats(allResults, time.Since(startTime))
	logStats(stats)
}

func processRepositoriesConcurrently(repositories []Repository, ctx ProcessingContext) []AnalysisResult {
	return processRepositoriesConcurrentlyWithRecovery(repositories, ctx, slog.Default())
}

func processRepositoriesConcurrentlyWithRecovery(repositories []Repository, ctx ProcessingContext, logger *slog.Logger) []AnalysisResult {
	startTime := time.Now()

	logger.Info("Starting concurrent repository processing", "repository_count", len(repositories))

	p := configureWaitGroup(ctx.Config.MaxGoroutines)
	results := createResultChannel(repositories)

	submitRepositoryJobsWithRecovery(repositories, p, ctx.Pool, results, logger)
	waitAndCloseChannel(p, results)

	allResults := collectResultsWithRecovery(results, logger)
	finalizeProcessing(allResults, startTime)

	return allResults
}

func cloneAndAnalyzeMultipleOrgs(ctx context.Context, processingCtx ProcessingContext, reporter *Reporter) error {
	startTime := time.Now()
	successfulOrgs := 0
	failedOrgs := 0

	slog.Info("Starting multi-organization analysis", 
		"total_organizations", len(processingCtx.Config.Organizations),
		"max_goroutines", processingCtx.Config.MaxGoroutines,
		"clone_concurrency", processingCtx.Config.CloneConcurrency)

	for i, org := range processingCtx.Config.Organizations {
		orgLogger := slog.With("organization", org, "progress", fmt.Sprintf("%d/%d", i+1, len(processingCtx.Config.Organizations)))
		orgLogger.Info("Processing organization")

		// Attempt organization processing with recovery
		if err := processOrganizationWithRecovery(ctx, org, processingCtx, reporter, orgLogger); err != nil {
			orgLogger.Error("Organization processing failed completely", "error", err)
			failedOrgs++
			continue
		}
		successfulOrgs++
	}

	duration := time.Since(startTime)
	slog.Info("Multi-organization processing complete", 
		"duration", duration,
		"successful_orgs", successfulOrgs,
		"failed_orgs", failedOrgs,
		"total_orgs", len(processingCtx.Config.Organizations))

	if failedOrgs > 0 {
		return fmt.Errorf("processing completed with %d/%d organizations failed", failedOrgs, len(processingCtx.Config.Organizations))
	}

	return nil
}

func processOrganizationWithRecovery(ctx context.Context, org string, processingCtx ProcessingContext, reporter *Reporter, logger *slog.Logger) error {
	var tempDir string
	var cleanup func()
	var setupErr error

	// Workspace setup with retry
	for attempt := 1; attempt <= 3; attempt++ {
		tempDir, cleanup, setupErr = setupWorkspace()
		if setupErr == nil {
			break
		}
		logger.Warn("Workspace setup failed, retrying", "attempt", attempt, "error", setupErr)
		time.Sleep(time.Duration(attempt) * time.Second)
	}
	if setupErr != nil {
		return fmt.Errorf("failed to setup workspace after 3 attempts: %w", setupErr)
	}
	defer cleanup()

	// Clone with retry
	operation := createCloneOperation(org, tempDir, processingCtx.Config)
	var cloneErr error
	for attempt := 1; attempt <= 3; attempt++ {
		cloneErr = executeClonePhase(ctx, operation)
		if cloneErr == nil {
			break
		}
		logger.Warn("Clone failed, retrying", "attempt", attempt, "error", cloneErr)
		time.Sleep(time.Duration(attempt*2) * time.Second)
	}
	if cloneErr != nil {
		return fmt.Errorf("failed to clone after 3 attempts: %w", cloneErr)
	}

	// Repository discovery
	repositories, discoveryErr := discoverRepositories(tempDir, org)
	if discoveryErr != nil {
		return fmt.Errorf("failed to discover repositories: %w", discoveryErr)
	}

	// Process repositories with error recovery
	results := processRepositoriesConcurrentlyWithRecovery(repositories, processingCtx, logger)
	reporter.AddResults(results)

	logger.Info("Organization processing completed", "repositories_found", len(repositories))
	return nil
}

func loadEnvironmentConfig(envFile string) (Config, error) {
	if envFile != "" {
		loadErr := loadEnvironmentFile(envFile)
		if loadErr != nil {
			return Config{}, fmt.Errorf("failed to load %s: %w", envFile, loadErr)
		}
	}

	envVars := getEnvironmentVariables()
	config := createConfigFromEnv(envVars)

	return config, nil
}

func createTimeoutContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

func handleApplicationError(err error) {
	logError(err)
}

func logConfiguration(config Config) {
	logProgress("Configuration loaded:")
	logProgress("  Organizations: %v", config.Organizations)
	logProgress("  Max Goroutines: %d", config.MaxGoroutines)
	logProgress("  Clone Concurrency: %d", config.CloneConcurrency)
	logProgress("  GitHub Token: %s", maskToken(config.GitHubToken))
	if config.BaseURL != "" {
		logProgress("  Base URL: %s", config.BaseURL)
	}
}

func maskToken(token string) string {
	if len(token) < 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

func runApplication() {
	// Initialize structured logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		AddSource: true,
	}))
	slog.SetDefault(logger)

	logger.Info("Starting tf-analyzer application")

	config, configErr := loadEnvironmentConfig(".env")
	if configErr != nil {
		handleApplicationError(fmt.Errorf("failed to load configuration: %w", configErr))
		return
	}

	logConfiguration(config)

	processingCtx, ctxErr := createProcessingContext(config)
	if ctxErr != nil {
		handleApplicationError(fmt.Errorf("failed to create processing context: %w", ctxErr))
		return
	}
	defer releaseProcessingContext(processingCtx)

	ctx, cancel := createTimeoutContext(config.ProcessTimeout)
	defer cancel()

	reporter := NewReporter()
	analysisErr := cloneAndAnalyzeMultipleOrgs(ctx, processingCtx, reporter)
	if analysisErr != nil {
		logger.Error("Analysis completed with errors", "error", analysisErr)
		// Don't return - still try to generate reports for successful organizations
	}

	// Generate and display comprehensive report
	if err := reporter.PrintSummaryReport(); err != nil {
		logger.Error("Failed to print summary report", "error", err)
	}
	
	// Export detailed reports
	if err := reporter.ExportJSON("terraform-analysis-report.json"); err != nil {
		logger.Error("Failed to export JSON report", "error", err)
	}
	if err := reporter.ExportCSV("terraform-analysis-report.csv"); err != nil {
		logger.Error("Failed to export CSV report", "error", err)
	}

	logger.Info("tf-analyzer application completed")
}

func main() {
	runApplication()
}
