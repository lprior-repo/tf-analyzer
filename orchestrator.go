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
	
	// Split by both comma and space to support flexible input
	var orgs []string
	if strings.Contains(orgString, ",") {
		orgs = strings.Split(orgString, ",")
	} else {
		orgs = strings.Fields(orgString)
	}
	
	return lo.Filter(lo.Map(orgs, func(org string, _ int) string {
		return strings.TrimSpace(org)
	}), func(org string, _ int) bool {
		return org != ""
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

func validateAnalysisConfiguration(config Config) error {
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



func createProcessingContext(config Config) (ProcessingContext, error) {
	validationErr := validateAnalysisConfiguration(config)
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

	slog.Info("Repositories discovered", 
		"repository_count", len(repositories), 
		"organization", org)
	return repositories, nil
}


func createResultChannel(repositories []Repository) chan AnalysisResult {
	return make(chan AnalysisResult, len(repositories))
}

func configureWaitGroup(maxGoroutines int) *pool.Pool {
	return pool.New().WithMaxGoroutines(maxGoroutines)
}


func submitRepositoryJobsWithTimeout(repositories []Repository, ctx context.Context, p *pool.Pool, antsPool *ants.Pool, results chan AnalysisResult, logger *slog.Logger) {
	jobSubmitter := createJobSubmitterWithTimeoutRecovery(antsPool, logger)

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
			
			// Check context before processing
			select {
			case <-ctx.Done():
				results <- AnalysisResult{
					RepoName:     repo.Name,
					Organization: repo.Organization,
					Error:        fmt.Errorf("processing cancelled due to timeout: %v", ctx.Err()),
				}
				return
			default:
			}
			
			result := jobSubmitter(repo)
			results <- result
		})
	}
}

func createJobSubmitterWithTimeoutRecovery(pool *ants.Pool, logger *slog.Logger) func(Repository) AnalysisResult {
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


func waitAndCloseChannel(p *pool.Pool, results chan AnalysisResult) {
	p.Wait()
	close(results)
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


func collectResultsWithRecovery(results chan AnalysisResult, logger *slog.Logger, tuiProgress *TUIProgressChannel, totalRepos int) []AnalysisResult {
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
		
		// Update TUI progress
		if tuiProgress != nil {
			tuiProgress.UpdateProgress(result.RepoName, result.Organization, len(allResults), totalRepos)
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
	slog.Info("Processing statistics",
		"total_repositories", stats.TotalRepos,
		"successfully_processed", stats.ProcessedRepos,
		"failed", stats.FailedRepos,
		"total_files_extracted", stats.TotalFiles,
		"duration", stats.Duration)
}

func finalizeProcessing(allResults []AnalysisResult, startTime time.Time) {
	stats := calculateStats(allResults, time.Since(startTime))
	logStats(stats)
}


func processRepositoriesConcurrentlyWithTimeout(repositories []Repository, ctx context.Context, processingCtx ProcessingContext, logger *slog.Logger, tuiProgress *TUIProgressChannel) []AnalysisResult {
	startTime := time.Now()

	logger.Info("Starting concurrent repository processing with timeout", 
		"repository_count", len(repositories),
		"timeout", processingCtx.Config.ProcessTimeout)

	p := configureWaitGroup(processingCtx.Config.MaxGoroutines)
	results := createResultChannel(repositories)

	// Monitor context cancellation
	go func() {
		<-ctx.Done()
		if ctx.Err() == context.DeadlineExceeded {
			logger.Warn("Repository processing timeout reached", 
				"timeout", processingCtx.Config.ProcessTimeout,
				"elapsed", time.Since(startTime))
		}
	}()

	submitRepositoryJobsWithTimeout(repositories, ctx, p, processingCtx.Pool, results, logger)
	waitAndCloseChannel(p, results)

	allResults := collectResultsWithRecovery(results, logger, tuiProgress, len(repositories))
	finalizeProcessing(allResults, startTime)

	return allResults
}


func cloneAndAnalyzeMultipleOrgs(ctx context.Context, processingCtx ProcessingContext, reporter *Reporter, tuiProgress *TUIProgressChannel) error {
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
		if err := processOrganizationWithRecovery(ctx, org, processingCtx, reporter, orgLogger, tuiProgress); err != nil {
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

func processOrganizationWithRecovery(ctx context.Context, org string, processingCtx ProcessingContext, reporter *Reporter, logger *slog.Logger, tuiProgress *TUIProgressChannel) error {
	var tempDir string
	var cleanup func()
	var setupErr error

	// Workspace setup with retry
	for attempt := 1; attempt <= 3; attempt++ {
		tempDir, cleanup, setupErr = setupWorkspaceWithRecovery(logger)
		if setupErr == nil {
			break
		}
		logger.Warn("Workspace setup failed, retrying", "attempt", attempt, "error", setupErr)
		time.Sleep(time.Duration(attempt) * time.Second)
	}
	if setupErr != nil {
		return fmt.Errorf("failed to setup workspace after 3 attempts: %w", setupErr)
	}
	defer func() {
		logger.Info("Starting cleanup of repositories", "temp_dir", tempDir, "organization", org)
		cleanup()
		logger.Info("Cleanup completed", "temp_dir", tempDir, "organization", org)
	}()

	// Clone with retry
	operation := createCloneOperation(org, tempDir, processingCtx.Config)
	var cloneErr error
	for attempt := 1; attempt <= 3; attempt++ {
		cloneErr = executeClonePhaseWithRecovery(ctx, operation, logger)
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

	// Process repositories with error recovery and context timeout
	analysisCtx, analysisCancel := context.WithTimeout(ctx, processingCtx.Config.ProcessTimeout)
	defer analysisCancel()
	
	results := processRepositoriesConcurrentlyWithTimeout(repositories, analysisCtx, processingCtx, logger, tuiProgress)
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
	slog.Error("Application error", "error", err)
}

func logConfiguration(config Config) {
	slog.Info("Configuration loaded",
		"organizations", config.Organizations,
		"max_goroutines", config.MaxGoroutines,
		"clone_concurrency", config.CloneConcurrency,
		"github_token", maskToken(config.GitHubToken),
		"base_url", config.BaseURL)
}

func maskToken(token string) string {
	if len(token) < 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

func main() {
	Execute()
}
