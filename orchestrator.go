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

// Configuration constants
const (
	DefaultMaxGoroutines    = 100
	DefaultCloneConcurrency = 100
	DefaultProcessTimeout   = 30 * time.Minute
	DefaultRetryDelay       = 100 * time.Millisecond // Fast for tests
	ProductionRetryDelay    = 1 * time.Second        // Production default
	MaxSafeMaxGoroutines    = 10000
	MaxSafeCloneConcurrency = 100
)

type Config struct {
	MaxGoroutines    int
	CloneConcurrency int
	ProcessTimeout   time.Duration
	RetryDelay       time.Duration
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

// Parameter structures to reduce function parameter counts
type ProgressUpdate struct {
	Repo, Org, Phase string
	Completed, Total, RepoCount, TotalRepos int
}

type OrgProcessContext struct {
	Ctx           context.Context
	Org           string
	ProcessingCtx ProcessingContext
	Reporter      *Reporter
	Logger        *slog.Logger
}

type MultiOrgContext struct {
	Ctx           context.Context
	ProcessingCtx ProcessingContext
	Reporter      *Reporter
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


func validateAnalysisConfiguration(config Config) error {
	if config.MaxGoroutines <= 0 {
		return fmt.Errorf("MaxGoroutines must be positive, got %d", config.MaxGoroutines)
	}
	
	// Security: Prevent resource exhaustion attacks
	if config.MaxGoroutines > MaxSafeMaxGoroutines {
		return fmt.Errorf("MaxGoroutines too high (max %d for safety), got %d", MaxSafeMaxGoroutines, config.MaxGoroutines)
	}
	
	if config.CloneConcurrency <= 0 {
		return fmt.Errorf("CloneConcurrency must be positive, got %d", config.CloneConcurrency)
	}
	
	// Security: Prevent excessive clone concurrency
	if config.CloneConcurrency > MaxSafeCloneConcurrency {
		return fmt.Errorf("CloneConcurrency too high (max %d for safety), got %d", MaxSafeCloneConcurrency, config.CloneConcurrency)
	}
	
	if config.GitHubToken == "" {
		return fmt.Errorf("GitHubToken is required")
	}
	
	if len(config.Organizations) == 0 {
		return fmt.Errorf("at least one organization must be specified")
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

func loadDotEnvFile(filePath string) error {
	return godotenv.Load(filePath)
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


type JobSubmissionContext struct {
	Repositories []Repository
	Ctx          context.Context
	Pool         *pool.Pool
	AntsPool     *ants.Pool
	Results      chan AnalysisResult
	Logger       *slog.Logger
}

func submitRepositoryJobsWithTimeout(jobCtx JobSubmissionContext) {
	jobSubmitter := createJobSubmitterWithTimeoutRecovery(jobCtx.AntsPool, jobCtx.Logger)

	for _, repo := range jobCtx.Repositories {
		repo := repo
		jobCtx.Pool.Go(func() {
			defer func() {
				if r := recover(); r != nil {
					jobCtx.Logger.Error("Repository processing panic recovered", 
						"repository", repo.Name, 
						"organization", repo.Organization, 
						"panic", r)
					jobCtx.Results <- AnalysisResult{
						RepoName:     repo.Name,
						Organization: repo.Organization,
						Error:        fmt.Errorf("panic during processing: %v", r),
					}
				}
			}()
			
			// Check context before processing
			select {
			case <-jobCtx.Ctx.Done():
				jobCtx.Results <- AnalysisResult{
					RepoName:     repo.Name,
					Organization: repo.Organization,
					Error:        fmt.Errorf("processing cancelled due to timeout: %v", jobCtx.Ctx.Err()),
				}
				return
			default:
			}
			
			result := jobSubmitter(repo)
			jobCtx.Results <- result
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


func collectResults(results chan AnalysisResult, logger *slog.Logger, totalRepos int) []AnalysisResult {
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


func processRepositoriesConcurrently(repositories []Repository, ctx context.Context, processingCtx ProcessingContext, logger *slog.Logger) []AnalysisResult {
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

	jobCtx := JobSubmissionContext{
		Repositories: repositories,
		Ctx:          ctx,
		Pool:         p,
		AntsPool:     processingCtx.Pool,
		Results:      results,
		Logger:       logger,
	}
	submitRepositoryJobsWithTimeout(jobCtx)
	waitAndCloseChannel(p, results)

	allResults := collectResults(results, logger, len(repositories))
	finalizeProcessing(allResults, startTime)

	return allResults
}

func cloneAndAnalyzeMultipleOrgs(ctx context.Context, processingCtx ProcessingContext, reporter *Reporter) error {
	multiCtx := MultiOrgContext{
		Ctx:           ctx,
		ProcessingCtx: processingCtx,
		Reporter:      reporter,
	}
	
	return processMultipleOrganizations(multiCtx)
}

func processMultipleOrganizations(multiCtx MultiOrgContext) error {
	startTime := time.Now()
	stats := initializeProcessingStats(multiCtx.ProcessingCtx.Config.Organizations)
	logProcessingStart(stats, multiCtx.ProcessingCtx.Config)
	
	for i, org := range multiCtx.ProcessingCtx.Config.Organizations {
		orgCtx := createOrgProcessContext(multiCtx, org, i, stats.TotalOrgs)
		repoCount, err := processOrganizationSafely(orgCtx)
		
		updateProcessingStats(&stats, repoCount, err != nil)
		
		logOrganizationCompletion(orgCtx.Logger, repoCount, stats)
	}
	
	return finalizeMutliOrgProcessing(startTime, stats, multiCtx.ProcessingCtx.Config.Organizations)
}

type MultiOrgStats struct {
	TotalOrgs            int
	SuccessfulOrgs       int
	FailedOrgs           int
	TotalReposAcrossOrgs int
	ProcessedRepos       int
}

func initializeProcessingStats(orgs []string) MultiOrgStats {
	return MultiOrgStats{
		TotalOrgs: len(orgs),
	}
}

func logProcessingStart(stats MultiOrgStats, config Config) {
	slog.Info("Starting multi-organization analysis", 
		"total_organizations", stats.TotalOrgs,
		"max_goroutines", config.MaxGoroutines,
		"clone_concurrency", config.CloneConcurrency)
}

func createOrgProcessContext(multiCtx MultiOrgContext, org string, index, totalOrgs int) OrgProcessContext {
	orgLogger := slog.With("organization", org, "progress", fmt.Sprintf("%d/%d", index+1, totalOrgs))
	orgLogger.Info("Processing organization")
	
	return OrgProcessContext{
		Ctx:           multiCtx.Ctx,
		Org:           org,
		ProcessingCtx: multiCtx.ProcessingCtx,
		Reporter:      multiCtx.Reporter,
		Logger:        orgLogger,
	}
}

func processOrganizationSafely(orgCtx OrgProcessContext) (int, error) {
	orgCtx.Logger.Info("Starting organization processing", "org", orgCtx.Org)
	
	return processOrganization(orgCtx.Ctx, orgCtx.Org, orgCtx.ProcessingCtx, orgCtx.Reporter, orgCtx.Logger)
}

func updateProcessingStats(stats *MultiOrgStats, repoCount int, failed bool) {
	if failed {
		stats.FailedOrgs++
	} else {
		stats.TotalReposAcrossOrgs += repoCount
		stats.ProcessedRepos += repoCount
		stats.SuccessfulOrgs++
	}
}


func logOrganizationCompletion(logger *slog.Logger, repoCount int, stats MultiOrgStats) {
	if repoCount > 0 {
		logger.Info("Organization processing completed", 
			"repositories_found", repoCount, 
			"total_repos_processed", stats.ProcessedRepos,
			"total_repos_discovered", stats.TotalReposAcrossOrgs)
	}
}

func finalizeMutliOrgProcessing(startTime time.Time, stats MultiOrgStats, orgs []string) error {
	duration := time.Since(startTime)
	slog.Info("Multi-organization processing complete", 
		"duration", duration,
		"successful_orgs", stats.SuccessfulOrgs,
		"failed_orgs", stats.FailedOrgs,
		"total_orgs", len(orgs))

	if stats.FailedOrgs > 0 {
		return fmt.Errorf("processing completed with %d/%d organizations failed", stats.FailedOrgs, len(orgs))
	}

	return nil
}

func processOrganization(ctx context.Context, org string, processingCtx ProcessingContext, reporter *Reporter, logger *slog.Logger) (int, error) {
	orgCtx := OrgProcessContext{
		Ctx:           ctx,
		Org:           org,
		ProcessingCtx: processingCtx,
		Reporter:      reporter,
		Logger:        logger,
	}
	
	return processOrganizationWorkflow(orgCtx)
}

func processOrganizationWorkflow(orgCtx OrgProcessContext) (int, error) {
	tempDir, cleanup, err := setupWorkspaceWithRetry(orgCtx.Logger, orgCtx.ProcessingCtx.Config.RetryDelay)
	if err != nil {
		return 0, err
	}
	defer cleanupWorkspace(cleanup, tempDir, orgCtx.Org, orgCtx.Logger)
	
	operation := createCloneOperation(orgCtx.Org, tempDir, orgCtx.ProcessingCtx.Config)
	if err := executeCloneWithoutRetry(orgCtx.Ctx, operation, orgCtx.Logger, orgCtx.ProcessingCtx.Config.RetryDelay); err != nil {
		return 0, err
	}
	
	repositories, err := discoverRepositoriesWrapper(tempDir, orgCtx.Org)
	if err != nil {
		return 0, err
	}
	
	results := analyzeRepositoriesConcurrently(orgCtx, repositories)
	orgCtx.Reporter.AddResults(results)
	
	return len(repositories), nil
}

func setupWorkspaceWithRetry(logger *slog.Logger, retryDelay time.Duration) (string, func(), error) {
	var tempDir string
	var cleanup func()
	var setupErr error

	for attempt := 1; attempt <= 3; attempt++ {
		tempDir, cleanup, setupErr = setupWorkspaceWithRecovery(logger)
		if setupErr == nil {
			break
		}
		logger.Warn("Workspace setup failed, retrying", "attempt", attempt, "error", setupErr)
		time.Sleep(time.Duration(attempt) * retryDelay)
	}
	
	if setupErr != nil {
		return "", nil, fmt.Errorf("failed to setup workspace after 3 attempts: %w", setupErr)
	}
	
	return tempDir, cleanup, nil
}

func cleanupWorkspace(cleanup func(), tempDir, org string, logger *slog.Logger) {
	logger.Info("Starting cleanup of repositories", "temp_dir", tempDir, "organization", org)
	cleanup()
	logger.Info("Cleanup completed", "temp_dir", tempDir, "organization", org)
}

func executeCloneWithoutRetry(ctx context.Context, operation CloneOperation, logger *slog.Logger, retryDelay time.Duration) error {
	var cloneErr error
	for attempt := 1; attempt <= 3; attempt++ {
		cloneErr = executeClonePhase(ctx, operation, logger)
		if cloneErr == nil {
			break
		}
		logger.Warn("Clone failed, retrying", "attempt", attempt, "error", cloneErr)
		time.Sleep(time.Duration(attempt*2) * retryDelay)
	}
	
	if cloneErr != nil {
		return fmt.Errorf("failed to clone after 3 attempts: %w", cloneErr)
	}
	
	return nil
}

func discoverRepositoriesWrapper(tempDir, org string) ([]Repository, error) {
	repositories, discoveryErr := discoverRepositories(tempDir, org)
	if discoveryErr != nil {
		return nil, fmt.Errorf("failed to discover repositories: %w", discoveryErr)
	}
	
	return repositories, nil
}

func analyzeRepositoriesConcurrently(orgCtx OrgProcessContext, repositories []Repository) []AnalysisResult {
	orgCtx.Logger.Info("Starting analysis", "org", orgCtx.Org, "repositories_count", len(repositories))

	analysisCtx, analysisCancel := context.WithTimeout(orgCtx.Ctx, orgCtx.ProcessingCtx.Config.ProcessTimeout)
	defer analysisCancel()
	
	repoLogger := slog.With("organization", orgCtx.Org)
	return processRepositoriesConcurrently(repositories, analysisCtx, orgCtx.ProcessingCtx, repoLogger)
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

// Test-only functions to maintain backward compatibility with existing tests
func createConfigFromEnv(envVars map[string]string) Config {
	getEnvOrDefault := func(key, defaultValue string) string {
		if value, exists := envVars[key]; exists && value != "" {
			return value
		}
		return defaultValue
	}

	// Use faster retry delay for tests, production delay for production
	retryDelay := DefaultRetryDelay
	if getEnvOrDefault("ENVIRONMENT", "test") == "production" {
		retryDelay = ProductionRetryDelay
	}

	return Config{
		MaxGoroutines:    DefaultMaxGoroutines,
		CloneConcurrency: DefaultCloneConcurrency,
		ProcessTimeout:   DefaultProcessTimeout,
		RetryDelay:       retryDelay,
		SkipArchived:     true,
		SkipForks:        false,
		GitHubToken:      getEnvOrDefault("GITHUB_TOKEN", ""),
		Organizations:    parseOrganizations(getEnvOrDefault("GITHUB_ORGS", "")),
		BaseURL:          getEnvOrDefault("GITHUB_BASE_URL", ""),
	}
}

func getEnvironmentVariables() map[string]string {
	return map[string]string{
		"GITHUB_TOKEN":    os.Getenv("GITHUB_TOKEN"),
		"GITHUB_ORGS":     os.Getenv("GITHUB_ORGS"),
		"GITHUB_BASE_URL": os.Getenv("GITHUB_BASE_URL"),
	}
}

func loadEnvironmentConfig(envFile string) (Config, error) {
	if envFile != "" {
		loadErr := loadDotEnvFile(envFile)
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

func main() {
	Execute()
}
