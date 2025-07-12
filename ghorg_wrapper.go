package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/panjf2000/ants/v2"
	"github.com/samber/lo"
	"github.com/sourcegraph/conc/pool"
)

// ============================================================================
// 1. DATA (Pure data structures)
// ============================================================================

type Config struct {
	MaxGoroutines    int
	CloneConcurrency int
	ProcessTimeout   time.Duration
	SkipArchived     bool
	SkipForks        bool
	GitHubToken      string
	Organizations    []string
	BaseURL          string // For GitHub Enterprise
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

type AnalysisResult struct {
	RepoName       string
	Organization   string
	Files          []FileContent
	TerraformFiles int
	MarkdownFiles  int
	TotalSize      int
	Error          error
}

type CloneOperation struct {
	Org       string
	TempDir   string
	Config    Config
	StartTime time.Time
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

// ============================================================================
// 2. CALCULATIONS (Pure functions - the business logic core)
// ============================================================================

// Environment and configuration functions
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

// File filtering and validation
func isRelevantFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".tf") ||
		strings.HasSuffix(lower, ".tfvars") ||
		strings.HasSuffix(lower, ".hcl") ||
		strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".markdown")
}

func shouldSkipPath(path string) bool {
	return strings.Contains(path, "/.git/")
}

// Data transformation functions
func createFileContent(path, repoPath string, content []byte) FileContent {
	relPath, _ := filepath.Rel(repoPath, path)
	return FileContent{
		RelativePath: relPath,
		Content:      content,
		Size:         len(content),
	}
}

func categorizeFiles(files []FileContent) (int, int) {
	terraformFiles := lo.CountBy(files, func(f FileContent) bool {
		lower := strings.ToLower(f.RelativePath)
		return strings.HasSuffix(lower, ".tf") || 
			   strings.HasSuffix(lower, ".tfvars") || 
			   strings.HasSuffix(lower, ".hcl")
	})
	
	markdownFiles := lo.CountBy(files, func(f FileContent) bool {
		lower := strings.ToLower(f.RelativePath)
		return strings.HasSuffix(lower, ".md") || 
			   strings.HasSuffix(lower, ".markdown")
	})
	
	return terraformFiles, markdownFiles
}

func calculateTotalSize(files []FileContent) int {
	return lo.Reduce(files, func(acc int, file FileContent, _ int) int {
		return acc + file.Size
	}, 0)
}

func createAnalysisResult(repoName, org string, files []FileContent, err error) AnalysisResult {
	if err != nil {
		return AnalysisResult{
			RepoName:     repoName,
			Organization: org,
			Error:        err,
		}
	}
	
	tfCount, mdCount := categorizeFiles(files)
	
	return AnalysisResult{
		RepoName:       repoName,
		Organization:   org,
		Files:          files,
		TerraformFiles: tfCount,
		MarkdownFiles:  mdCount,
		TotalSize:      calculateTotalSize(files),
	}
}

// Repository filtering and processing
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

// Command building with authentication
func buildGhorgCommand(ctx context.Context, op CloneOperation) *exec.Cmd {
	args := []string{"clone", op.Org, "--path", op.TempDir}
	
	if op.Config.SkipArchived {
		args = append(args, "--skip-archived")
	}
	if op.Config.SkipForks {
		args = append(args, "--skip-forks")
	}
	if op.Config.BaseURL != "" {
		args = append(args, "--base-url", op.Config.BaseURL)
	}
	
	args = append(args, "--concurrency", fmt.Sprintf("%d", op.Config.CloneConcurrency))
	args = append(args, "--git-filter", "blob:none")
	
	cmd := exec.CommandContext(ctx, "ghorg", args...)
	
	// Set GitHub token as environment variable
	if op.Config.GitHubToken != "" {
		cmd.Env = append(os.Environ(), "GHORG_GITHUB_TOKEN="+op.Config.GitHubToken)
	}
	
	return cmd
}


// Validation functions
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

// ============================================================================
// 3. ACTIONS (Impure functions - the I/O shell)
// ============================================================================

// Environment file operations
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

// File system operations
func loadFileContent(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func walkRepositoryFiles(repoPath, org string) ([]FileContent, error) {
	var files []FileContent
	
	err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || shouldSkipPath(path) {
			return err
		}
		
		if !isRelevantFile(path) {
			return nil
		}
		
		content, readErr := loadFileContent(path)
		if readErr != nil {
			return nil // Skip unreadable files
		}
		
		files = append(files, createFileContent(path, repoPath, content))
		return nil
	})
	
	return files, err
}

// Pure function to expand home directory in path
func expandHomePath(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}
	
	return filepath.Join(currentUser.HomeDir, path[2:]), nil
}

// Pure function to create absolute path
func createAbsolutePath(path string) (string, error) {
	expandedPath, err := expandHomePath(path)
	if err != nil {
		return "", err
	}
	
	if filepath.IsAbs(expandedPath) {
		return expandedPath, nil
	}
	
	return filepath.Abs(expandedPath)
}

func createTempDirectory() (string, error) {
	// Use absolute path ~/src/gh-repos-clone instead of system temp
	baseDir := "~/src/gh-repos-clone"
	
	expandedPath, err := createAbsolutePath(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to expand path %s: %w", baseDir, err)
	}
	
	// Ensure parent directory exists
	if err := os.MkdirAll(expandedPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create base directory %s: %w", expandedPath, err)
	}
	
	// Create timestamped subdirectory for this run
	timestamp := time.Now().Format("20060102-150405")
	tempDir := filepath.Join(expandedPath, fmt.Sprintf("ghorg-%s", timestamp))
	
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory %s: %w", tempDir, err)
	}
	
	return tempDir, nil
}

func removeTempDirectory(path string) error {
	if path == "" {
		return nil
	}
	return os.RemoveAll(path)
}

func readDirectory(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

// Console output actions
func logProgress(message string, args ...interface{}) {
	fmt.Printf(message+"\n", args...)
}

func logError(err error) {
	fmt.Printf("ERROR: %v\n", err)
}




// Repository processing actions
func findOrgDirectory(tempDir string) (string, error) {
	entries, err := readDirectory(tempDir)
	if err != nil {
		return "", err
	}
	
	orgDirs := lo.FilterMap(entries, func(entry os.DirEntry, _ int) (string, bool) {
		if entry.IsDir() {
			return filepath.Join(tempDir, entry.Name()), true
		}
		return "", false
	})
	
	if len(orgDirs) == 0 {
		return "", fmt.Errorf("no org directory found in %s", tempDir)
	}
	
	return orgDirs[0], nil
}

func executeCloneCommand(ctx context.Context, op CloneOperation) error {
	cmd := buildGhorgCommand(ctx, op)
	return cmd.Run()
}

func processRepositoryFiles(repo Repository) AnalysisResult {
	files, err := walkRepositoryFiles(repo.Path, repo.Organization)
	return createAnalysisResult(repo.Name, repo.Organization, files, err)
}

// ============================================================================
// 4. ORCHESTRATION (Pure functional workflow)
// ============================================================================

// Context holds all dependencies needed for processing
type ProcessingContext struct {
	Config Config
	Pool   *ants.Pool
}

// Pure function to create processing context
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

// Action to cleanup processing context
func releaseProcessingContext(ctx ProcessingContext) {
	releasePool := func(pool *ants.Pool) {
		pool.Release()
	}
	
	if ctx.Pool != nil {
		releasePool(ctx.Pool)
	}
}

// Pure function to create clone operation
func createCloneOperation(org, tempDir string, config Config) CloneOperation {
	return CloneOperation{
		Org:       org,
		TempDir:   tempDir,
		Config:    config,
		StartTime: time.Now(),
	}
}

// Pure function to setup temporary workspace
func setupWorkspace() (string, func(), error) {
	tempDir, err := createTempDirectory()
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	
	cleanup := func() {
		_ = removeTempDirectory(tempDir)
	}
	
	return tempDir, cleanup, nil
}

// Action to execute clone phase
func executeClonePhase(ctx context.Context, operation CloneOperation) error {
	logProgress("Cloning organization: %s", operation.Org)
	
	cloneErr := executeCloneCommand(ctx, operation)
	if cloneErr != nil {
		return fmt.Errorf("ghorg clone failed: %w", cloneErr)
	}
	
	logProgress("Clone completed successfully")
	return nil
}

// Action to discover repositories with organization context
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

// Pure function to create job submission function
func createJobSubmitter(pool *ants.Pool) func(Repository) AnalysisResult {
	return func(repo Repository) AnalysisResult {
		submitErr := pool.Submit(func() {
			// This will be handled by the result channel
		})
		
		if submitErr != nil {
			return AnalysisResult{
				RepoName: repo.Name,
				Error:    fmt.Errorf("failed to submit job: %w", submitErr),
			}
		}
		
		return processRepositoryFiles(repo)
	}
}

// Pure function to create result channel
func createResultChannel(repositories []Repository) chan AnalysisResult {
	return make(chan AnalysisResult, len(repositories))
}

// Pure function to configure wait group with concurrency limit
func configureWaitGroup(maxGoroutines int) *pool.Pool {
	return pool.New().WithMaxGoroutines(maxGoroutines)
}

// Action to submit repository processing jobs
func submitRepositoryJobs(repositories []Repository, p *pool.Pool, antsPool *ants.Pool, results chan AnalysisResult) {
	jobSubmitter := createJobSubmitter(antsPool)
	
	for _, repo := range repositories {
		repo := repo // Capture for closure
		p.Go(func() {
			result := jobSubmitter(repo)
			results <- result
		})
	}
}

// Action to wait for jobs and close channel
func waitAndCloseChannel(p *pool.Pool, results chan AnalysisResult) {
	p.Wait()
	close(results)
}


// Action to handle single result
func handleSingleResult(result AnalysisResult, analyzeFunc func(AnalysisResult) error) {
	if result.Error != nil {
		logError(fmt.Errorf("repository %s: %w", result.RepoName, result.Error))
		return
	}
	
	if len(result.Files) == 0 {
		return // Skip repos with no relevant files
	}
	
	analysisErr := analyzeFunc(result)
	if analysisErr != nil {
		logError(fmt.Errorf("analysis failed for %s: %w", result.RepoName, analysisErr))
	}
}

// Pure function to check if progress should be logged
func shouldLogProgress(currentCount int) bool {
	return currentCount%50 == 0
}

// Action to log progress if needed
func logProgressIfNeeded(currentCount int) {
	if shouldLogProgress(currentCount) {
		logProgress("Processed %d repositories...", currentCount)
	}
}

// Action to collect and process all results
func collectAndProcessResults(results chan AnalysisResult, analyzeFunc func(AnalysisResult) error) []AnalysisResult {
	var allResults []AnalysisResult
	
	for result := range results {
		allResults = append(allResults, result)
		handleSingleResult(result, analyzeFunc)
		logProgressIfNeeded(len(allResults))
	}
	
	return allResults
}

// Pure function to calculate processing statistics
func calculateStats(allResults []AnalysisResult, duration time.Duration) ProcessingStats {
	successful := lo.Filter(allResults, func(r AnalysisResult, _ int) bool {
		return r.Error == nil
	})
	
	failed := lo.Filter(allResults, func(r AnalysisResult, _ int) bool {
		return r.Error != nil
	})
	
	totalFiles := lo.Reduce(successful, func(acc int, result AnalysisResult, _ int) int {
		return acc + len(result.Files)
	}, 0)
	
	return ProcessingStats{
		TotalOrgs:      1, // Single org processing
		TotalRepos:     len(allResults),
		ProcessedRepos: len(successful),
		FailedRepos:    len(failed),
		TotalFiles:     totalFiles,
		Duration:       duration,
	}
}

// Action to log processing statistics
func logStats(stats ProcessingStats) {
	logProgress("\n=== Processing Complete ===")
	logProgress("Total repositories: %d", stats.TotalRepos)
	logProgress("Successfully processed: %d", stats.ProcessedRepos)
	logProgress("Failed: %d", stats.FailedRepos)
	logProgress("Total files extracted: %d", stats.TotalFiles)
	logProgress("Total duration: %v", stats.Duration)
}

// Action to finalize processing with statistics
func finalizeProcessing(allResults []AnalysisResult, startTime time.Time) {
	stats := calculateStats(allResults, time.Since(startTime))
	logStats(stats)
}

// Main processing workflow function
func processRepositoriesConcurrently(repositories []Repository, ctx ProcessingContext, analyzeFunc func(AnalysisResult) error) error {
	startTime := time.Now()
	
	p := configureWaitGroup(ctx.Config.MaxGoroutines)
	results := createResultChannel(repositories)
	
	submitRepositoryJobs(repositories, p, ctx.Pool, results)
	waitAndCloseChannel(p, results)
	
	allResults := collectAndProcessResults(results, analyzeFunc)
	finalizeProcessing(allResults, startTime)
	
	return nil
}


// Main orchestration function for multiple organizations
func cloneAndAnalyzeMultipleOrgs(ctx context.Context, processingCtx ProcessingContext, analyzeFunc func(AnalysisResult) error) error {
	startTime := time.Now()
	
	logProgress("Starting analysis of %d organizations", len(processingCtx.Config.Organizations))
	
	for i, org := range processingCtx.Config.Organizations {
		logProgress("[%d/%d] Processing organization: %s", i+1, len(processingCtx.Config.Organizations), org)
		
		tempDir, cleanup, setupErr := setupWorkspace()
		if setupErr != nil {
			logError(fmt.Errorf("failed to setup workspace for %s: %w", org, setupErr))
			continue
		}
		
		operation := createCloneOperation(org, tempDir, processingCtx.Config)
		cloneErr := executeClonePhase(ctx, operation)
		if cloneErr != nil {
			logError(fmt.Errorf("failed to clone %s: %w", org, cloneErr))
			cleanup()
			continue
		}
		
		repositories, discoveryErr := discoverRepositories(tempDir, org)
		if discoveryErr != nil {
			logError(fmt.Errorf("failed to discover repositories for %s: %w", org, discoveryErr))
			cleanup()
			continue
		}
		
		_ = processRepositoriesConcurrently(repositories, processingCtx, analyzeFunc)
		cleanup()
	}
	
	duration := time.Since(startTime)
	logProgress("\n=== Multi-Organization Processing Complete ===")
	logProgress("Total duration: %v", duration)
	
	return nil
}

// ============================================================================
// 5. EXAMPLE USAGE
// ============================================================================

// Action to load environment configuration
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

// Pure function to create context with timeout
func createTimeoutContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// Pure function to check if file contains AWS resources
func containsAWSResources(file FileContent) bool {
	return strings.HasSuffix(strings.ToLower(file.RelativePath), ".tf") &&
		   strings.Contains(string(file.Content), "resource \"aws_")
}

// Pure function to check if file is Terraform documentation
func isTerraformDocumentation(file FileContent) bool {
	return strings.HasSuffix(strings.ToLower(file.RelativePath), ".md") &&
		   strings.Contains(strings.ToLower(string(file.Content)), "terraform")
}

// Pure function to filter AWS resource files
func filterAWSFiles(files []FileContent) []FileContent {
	return lo.Filter(files, func(file FileContent, _ int) bool {
		return containsAWSResources(file)
	})
}

// Pure function to filter Terraform documentation files
func filterTerraformDocs(files []FileContent) []FileContent {
	return lo.Filter(files, func(file FileContent, _ int) bool {
		return isTerraformDocumentation(file)
	})
}

// Action to log AWS files found
func logAWSFilesFound(awsFiles []FileContent) {
	fileCount := len(awsFiles)
	if fileCount > 0 {
		logProgress("  -> Found %d files with AWS resources", fileCount)
	}
}

// Action to log Terraform documentation found
func logTerraformDocsFound(terraformDocs []FileContent) {
	docCount := len(terraformDocs)
	if docCount > 0 {
		logProgress("  -> Found %d Terraform documentation files", docCount)
	}
}

// Action to log repository analysis summary
func logRepositoryAnalysis(result AnalysisResult) {
	logProgress("[%s] Repository: %s (%d files, %d TF, %d MD, %d bytes)", 
		result.Organization,
		result.RepoName,
		len(result.Files),
		result.TerraformFiles,
		result.MarkdownFiles,
		result.TotalSize)
}

// Pure analysis function
func analyzeRepository(result AnalysisResult) error {
	logRepositoryAnalysis(result)
	
	awsFiles := filterAWSFiles(result.Files)
	logAWSFilesFound(awsFiles)
	
	terraformDocs := filterTerraformDocs(result.Files)
	logTerraformDocsFound(terraformDocs)
	
	return nil
}

// Action to handle application errors
func handleApplicationError(err error) {
	logError(err)
}

// Action to log configuration
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

// Pure function to mask GitHub token for logging
func maskToken(token string) string {
	if len(token) < 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

// Main application workflow
func runApplication() {
	// Load environment configuration
	config, configErr := loadEnvironmentConfig(".env")
	if configErr != nil {
		handleApplicationError(fmt.Errorf("failed to load configuration: %w", configErr))
		return
	}
	
	logConfiguration(config)
	
	// Create processing context
	processingCtx, ctxErr := createProcessingContext(config)
	if ctxErr != nil {
		handleApplicationError(fmt.Errorf("failed to create processing context: %w", ctxErr))
		return
	}
	defer releaseProcessingContext(processingCtx)
	
	// Create timeout context
	ctx, cancel := createTimeoutContext(config.ProcessTimeout)
	defer cancel()
	
	// Execute multi-organization analysis
	analysisErr := cloneAndAnalyzeMultipleOrgs(ctx, processingCtx, analyzeRepository)
	if analysisErr != nil {
		handleApplicationError(analysisErr)
	}
}

func main() {
	runApplication()
}