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
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ============================================================================
// CLI - Professional Command Line Interface using Cobra and Fang
// ============================================================================

var (
	cfgFile         string
	envFile         string
	organizations   []string
	githubToken     string
	maxGoroutines   int
	cloneConcurrency int
	timeout         time.Duration
	outputFormat    string
	outputDir       string
	verbose         bool
	markdownStyle   string
	rawMarkdown     bool
	// Repository targeting flags
	targetRepos     []string
	targetReposFile string
	matchRegex      string
	matchPrefix     []string
	excludeRegex    string
	excludePrefix   []string
)

// loadRequiredEnvFile loads a .env file and returns an error if it doesn't exist
func loadRequiredEnvFile(filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf(".env file not found in current directory: %s", filePath)
	}
	
	if err := godotenv.Load(filePath); err != nil {
		return fmt.Errorf("failed to load .env file: %w", err)
	}
	
	return nil
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "tf-analyzer",
	Short: "Analyze Terraform configurations across GitHub organizations",
	Long: `
# TF-Analyzer

A high-performance Terraform configuration analyzer that concurrently clones and analyzes
multiple GitHub organizations to provide comprehensive insights into your infrastructure as code.

## Features

• **Multi-Organization Analysis**: Clone and analyze repositories from multiple GitHub organizations
• **Concurrent Processing**: High-performance concurrent analysis with configurable limits  
• **Multiple Export Formats**: JSON, CSV, and Markdown report generation
• **Provider Analysis**: Detect and analyze Terraform providers and their versions
• **Resource Analysis**: Count and categorize Terraform resources and modules
• **Backend Configuration**: Analyze Terraform backend configurations
• **Tagging Compliance**: Check for mandatory resource tags

## Quick Start

{{.}}

	# Set your GitHub token
	export GITHUB_TOKEN=your_token_here
	
	# Analyze organizations (comma-separated)
	tf-analyzer analyze --orgs "hashicorp,terraform-providers"
	
	# Analyze organizations with space-separated list (no quotes needed)
	tf-analyzer analyze --orgs hashicorp terraform-providers aws-samples
	
	# Export to custom directory  
	tf-analyzer analyze --orgs "my-org" --output-dir ./reports

For more information, visit: https://github.com/your-repo/tf-analyzer
	`,
	Version: "1.0.0",
}

// analyzeCmd represents the analyze command
var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze Terraform configurations in GitHub organizations",
	Long: `
# Analyze Command

The analyze command clones repositories from specified GitHub organizations and performs
comprehensive Terraform configuration analysis.

## Examples

	# Analyze single organization
	tf-analyzer analyze --orgs hashicorp
	
	# Analyze multiple organizations (space-separated, no quotes needed)
	tf-analyzer analyze --orgs hashicorp terraform-providers aws-samples
	
	# Analyze multiple organizations with custom settings (comma-separated)
	tf-analyzer analyze --orgs "org1,org2,org3" --max-goroutines 50 --timeout 45m
	
	# Export reports to specific directory
	tf-analyzer analyze --orgs "my-org" --output-dir ./custom-reports
	
	# Verbose logging for debugging
	tf-analyzer analyze --orgs "test-org" --verbose

## Configuration

You can specify configuration via:
• Command line flags (highest priority)
• Environment variables 
• Configuration file (.tf-analyzer.yaml)

Environment variables:
• GITHUB_TOKEN: GitHub API token (required)
• GITHUB_ORGS: Comma-separated list of organizations
• MAX_GOROUTINES: Maximum concurrent goroutines (default: ` + fmt.Sprintf("%d", DefaultMaxGoroutines) + `)
• CLONE_CONCURRENCY: Clone concurrency limit (default: ` + fmt.Sprintf("%d", DefaultCloneConcurrency) + `)
	`,
	RunE: runAnalyze,
}

// configCmd represents the config command  
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage tf-analyzer configuration",
	Long: `
# Configuration Management

Manage tf-analyzer configuration files and settings.

## Examples

	# Show current configuration
	tf-analyzer config show
	
	# Initialize configuration file
	tf-analyzer config init
	
	# Validate configuration
	tf-analyzer config validate
	`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE:  showConfig,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration file",
	RunE:  initConfig,
}

var configValidateCmd = &cobra.Command{
	Use:   "validate", 
	Short: "Validate configuration",
	RunE:  validateConfig,
}

func init() {
	cobra.OnInitialize(initializeConfig)
	initializeGlobalFlags()
	initializeAnalyzeFlags()
	bindViperFlags()
	setupCommands()
}

// initializeGlobalFlags sets up persistent flags for all commands
func initializeGlobalFlags() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.tf-analyzer.yaml)")
	rootCmd.PersistentFlags().StringVar(&envFile, "env-file", ".env", "environment file path (default is .env in current directory)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}

// initializeAnalyzeFlags sets up flags specific to the analyze command
func initializeAnalyzeFlags() {
	analyzeCmd.Flags().StringSliceVarP(&organizations, "orgs", "o", []string{}, "GitHub organizations to analyze (space or comma-separated)")
	analyzeCmd.Flags().StringVarP(&githubToken, "token", "t", "", "GitHub API token")
	analyzeCmd.Flags().IntVar(&maxGoroutines, "max-goroutines", DefaultMaxGoroutines, "maximum concurrent goroutines")
	analyzeCmd.Flags().IntVar(&cloneConcurrency, "clone-concurrency", DefaultCloneConcurrency, "clone concurrency limit")
	analyzeCmd.Flags().DurationVar(&timeout, "timeout", DefaultProcessTimeout, "processing timeout")
	analyzeCmd.Flags().StringVar(&outputFormat, "format", "all", "output format: json, csv, markdown, or all")
	analyzeCmd.Flags().StringVar(&outputDir, "output-dir", ".", "output directory for reports")
	analyzeCmd.Flags().StringVar(&markdownStyle, "markdown-style", "auto", "markdown rendering style: auto, dark, light, notty")
	analyzeCmd.Flags().BoolVar(&rawMarkdown, "raw-markdown", false, "print raw markdown without glamour rendering")
	
	// Repository targeting flags for ghorg integration
	analyzeCmd.Flags().StringSliceVar(&targetRepos, "target-repos", []string{}, "comma-separated list of specific repositories to clone")
	analyzeCmd.Flags().StringVar(&targetReposFile, "target-repos-file", "", "path to file containing repository names (one per line)")
	analyzeCmd.Flags().StringVar(&matchRegex, "match-regex", "", "regex pattern to match repository names")
	analyzeCmd.Flags().StringSliceVar(&matchPrefix, "match-prefix", []string{}, "comma-separated prefixes to match repository names")
	analyzeCmd.Flags().StringVar(&excludeRegex, "exclude-regex", "", "regex pattern to exclude repository names")
	analyzeCmd.Flags().StringSliceVar(&excludePrefix, "exclude-prefix", []string{}, "comma-separated prefixes to exclude repository names")
	
	// Mark required flags
	if err := analyzeCmd.MarkFlagRequired("orgs"); err != nil {
		panic(fmt.Sprintf("Failed to mark orgs flag as required: %v", err))
	}
}

// bindViperFlags binds command flags to viper configuration
func bindViperFlags() {
	flagBindings := map[string]string{
		"orgs":             "organizations",
		"token":            "github.token",
		"max-goroutines":   "processing.max_goroutines",
		"clone-concurrency": "processing.clone_concurrency",
		"timeout":          "processing.timeout",
		"format":           "output.format",
		"output-dir":       "output.directory",
		"markdown-style":   "ui.markdown_style",
		"raw-markdown":     "ui.raw_markdown",
		// Repository targeting flags
		"target-repos":     "github.target_repos",
		"target-repos-file": "github.target_repos_file",
		"match-regex":      "github.match_regex",
		"match-prefix":     "github.match_prefix",
		"exclude-regex":    "github.exclude_regex",
		"exclude-prefix":   "github.exclude_prefix",
	}
	
	for flag, viperKey := range flagBindings {
		if err := viper.BindPFlag(viperKey, analyzeCmd.Flags().Lookup(flag)); err != nil {
			panic(fmt.Sprintf("Failed to bind %s flag: %v", flag, err))
		}
	}
}

// setupCommands adds all subcommands to the root command
func setupCommands() {
	configCmd.AddCommand(configShowCmd, configInitCmd, configValidateCmd)
	rootCmd.AddCommand(analyzeCmd, configCmd)
}

// initializeConfig loads configuration from files and environment
func initializeConfig() {
	loadEnvironmentFile()
	setupViperConfig()
	bindEnvironmentVariables()
	loadConfigFile()
}

// loadEnvironmentFile loads the .env file
func loadEnvironmentFile() {
	envFilePath := envFile
	if envFilePath == "" {
		envFilePath = ".env"
	}
	
	if err := loadRequiredEnvFile(envFilePath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Create an environment file at '%s' with required configuration.\n", envFilePath)
		fmt.Fprintf(os.Stderr, "Use --env-file flag to specify a different location.\n")
		os.Exit(1)
	}
}

// setupViperConfig configures viper settings
func setupViperConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".tf-analyzer")
	}

	viper.SetEnvPrefix("TF_ANALYZER")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}

// bindEnvironmentVariables binds specific environment variables
func bindEnvironmentVariables() {
	envBindings := map[string]string{
		"github.token":               "GITHUB_TOKEN",
		"organizations":              "GITHUB_ORGS",
		"processing.max_goroutines":   "MAX_GOROUTINES",
		"processing.clone_concurrency": "CLONE_CONCURRENCY",
	}
	
	for viperKey, envVar := range envBindings {
		if err := viper.BindEnv(viperKey, envVar); err != nil {
			panic(fmt.Sprintf("Failed to bind %s env var: %v", envVar, err))
		}
	}
}

// loadConfigFile reads the configuration file if it exists
func loadConfigFile() {
	if err := viper.ReadInConfig(); err == nil && verbose {
		fmt.Printf("Using config file: %s\n", viper.ConfigFileUsed())
	}
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	logger := setupAnalysisLogger()
	config, err := prepareAnalysisConfig()
	if err != nil {
		return err
	}

	processingCtx, err := setupAnalysis(config, logger)
	if err != nil {
		return err
	}
	defer releaseProcessingContext(processingCtx)
	
	ctx, cancel := context.WithTimeout(context.Background(), config.ProcessTimeout)
	defer cancel()

	reporter, analysisErr := executeAnalysisWorkflow(ctx, processingCtx)
	
	if analysisErr != nil {
		logger.Error("Analysis completed with errors", "error", analysisErr)
	}

	if err := generateReports(reporter, config); err != nil {
		return fmt.Errorf("failed to generate reports: %w", err)
	}

	if err := handleConsoleOutput(reporter, logger); err != nil {
		logger.Error("Failed to display console output", "error", err)
	}
	
	return analysisErr
}

func setupAnalysisLogger() *slog.Logger {
	logLevel := slog.LevelInfo
	if verbose {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: verbose,
	}))
	slog.SetDefault(logger)
	return logger
}

func prepareAnalysisConfig() (Config, error) {
	config, err := createConfigFromViper()
	if err != nil {
		return Config{}, fmt.Errorf("failed to create configuration: %w", err)
	}

	if err := validateCLIAnalysisConfig(config); err != nil {
		return Config{}, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

func setupAnalysis(config Config, logger *slog.Logger) (ProcessingContext, error) {
	processingCtx, err := createProcessingContext(config)
	if err != nil {
		return ProcessingContext{}, fmt.Errorf("failed to create processing context: %w", err)
	}
	return processingCtx, nil
}

func executeAnalysisWorkflow(ctx context.Context, processingCtx ProcessingContext) (*Reporter, error) {
	reporter := NewReporter()
	analysisErr := cloneAndAnalyzeMultipleOrgs(ctx, processingCtx, reporter)
	return reporter, analysisErr
}

func handleConsoleOutput(reporter *Reporter, logger *slog.Logger) error {
	if err := reporter.PrintSummaryReport(); err != nil {
		return fmt.Errorf("failed to print summary report: %w", err)
	}
	
	if err := printMarkdownReport(reporter, logger); err != nil {
		return fmt.Errorf("failed to print markdown report: %w", err)
	}
	
	return nil
}

func printMarkdownReport(reporter *Reporter, logger *slog.Logger) error {
	if viper.GetBool("ui.raw_markdown") {
		reporter.PrintMarkdownToScreen()
		return nil
	}

	style := viper.GetString("ui.markdown_style")
	if style == "auto" {
		style = detectTerminalCapabilities()
	}
	
	if err := reporter.PrintMarkdownToScreenWithStyle(style); err != nil {
		logger.Error("Failed to render markdown", "error", err)
		reporter.PrintMarkdownToScreen()
		return err
	}
	
	return nil
}

// getStringSliceFromViper gets a string slice from viper, handling both slice and string formats
func getStringSliceFromViper(key string) []string {
	// First try to get as slice
	slice := viper.GetStringSlice(key)
	if len(slice) > 0 {
		// If we got a single item slice that contains commas, parse it
		if len(slice) == 1 && strings.Contains(slice[0], ",") {
			return parseTargetRepos(slice[0])
		}
		return slice
	}
	
	// If empty, try to get as string and parse it
	str := viper.GetString(key)
	if str != "" {
		return parseTargetRepos(str) // Reuse the parsing logic
	}
	
	return []string{}
}

func createConfigFromViper() (Config, error) {
	// Get organizations from viper
	orgs := viper.GetStringSlice("organizations")
	if len(orgs) == 0 {
		// Try comma-separated string format
		orgString := viper.GetString("organizations")
		if orgString != "" {
			orgs = parseOrganizations(orgString)
		}
	}

	// Use faster retry delay for tests, production delay for production
	retryDelay := DefaultRetryDelay
	if viper.GetString("environment") == "production" {
		retryDelay = ProductionRetryDelay
	}

	// Parse targeting options from viper
	targetRepos := getStringSliceFromViper("github.target_repos")
	matchPrefix := getStringSliceFromViper("github.match_prefix")
	excludePrefix := getStringSliceFromViper("github.exclude_prefix")

	return Config{
		Organizations:    orgs,
		GitHubToken:      viper.GetString("github.token"),
		MaxGoroutines:    viper.GetInt("processing.max_goroutines"),
		CloneConcurrency: viper.GetInt("processing.clone_concurrency"),
		ProcessTimeout:   viper.GetDuration("processing.timeout"),
		RetryDelay:       retryDelay,
		SkipArchived:     viper.GetBool("github.skip_archived"),
		SkipForks:        viper.GetBool("github.skip_forks"),
		BaseURL:          viper.GetString("github.base_url"),
		// Repository targeting options
		TargetRepos:     targetRepos,
		TargetReposFile: viper.GetString("github.target_repos_file"),
		MatchRegex:      viper.GetString("github.match_regex"),
		MatchPrefix:     matchPrefix,
		ExcludeRegex:    viper.GetString("github.exclude_regex"),
		ExcludePrefix:   excludePrefix,
	}, nil
}

func validateCLIAnalysisConfig(config Config) error {
	if len(config.Organizations) == 0 {
		return fmt.Errorf("at least one organization must be specified")
	}
	if config.GitHubToken == "" {
		return fmt.Errorf("GitHub token is required (set GITHUB_TOKEN or use --token)")
	}
	
	// Validate targeting configuration
	if err := validateTargetingConfiguration(config); err != nil {
		return err
	}
	
	return validateAnalysisConfiguration(config)
}


func generateReports(reporter *Reporter, config Config) error {
	format := viper.GetString("output.format")
	outputDir := viper.GetString("output.directory")

	if err := ensureOutputDirectory(outputDir); err != nil {
		return err
	}

	return generateReportsByFormat(reporter, format, outputDir)
}

func ensureOutputDirectory(outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	return nil
}

func generateReportsByFormat(reporter *Reporter, format, outputDir string) error {
	if shouldGenerateJSON(format) {
		if err := generateJSONReport(reporter, outputDir); err != nil {
			return err
		}
	}

	if shouldGenerateCSV(format) {
		if err := generateCSVReport(reporter, outputDir); err != nil {
			return err
		}
	}

	if shouldGenerateMarkdown(format) {
		if err := generateMarkdownReport(reporter, outputDir); err != nil {
			return err
		}
	}

	return nil
}

func shouldGenerateJSON(format string) bool {
	return format == "all" || format == "json"
}

func shouldGenerateCSV(format string) bool {
	return format == "all" || format == "csv"
}

func shouldGenerateMarkdown(format string) bool {
	return format == "all" || format == "markdown"
}

func generateJSONReport(reporter *Reporter, outputDir string) error {
	jsonPath := filepath.Join(outputDir, "terraform-analysis-report.json")
	if err := reporter.ExportJSON(jsonPath); err != nil {
		return fmt.Errorf("failed to generate JSON report: %w", err)
	}
	return nil
}

func generateCSVReport(reporter *Reporter, outputDir string) error {
	csvPath := filepath.Join(outputDir, "terraform-analysis-report.csv")
	if err := reporter.ExportCSV(csvPath); err != nil {
		return fmt.Errorf("failed to generate CSV report: %w", err)
	}
	return nil
}

func generateMarkdownReport(reporter *Reporter, outputDir string) error {
	mdPath := filepath.Join(outputDir, "terraform-analysis-report.md")
	if err := reporter.ExportMarkdown(mdPath); err != nil {
		return fmt.Errorf("failed to generate Markdown report: %w", err)
	}
	return nil
}

func showConfig(cmd *cobra.Command, args []string) error {
	fmt.Printf("Configuration File: %s\n\n", viper.ConfigFileUsed())
	
	config, err := createConfigFromViper()
	if err != nil {
		return err
	}

	fmt.Printf("Organizations: %s\n", strings.Join(config.Organizations, ", "))
	fmt.Printf("GitHub Token: %s\n", maskToken(config.GitHubToken))
	fmt.Printf("Max Goroutines: %d\n", config.MaxGoroutines)
	fmt.Printf("Clone Concurrency: %d\n", config.CloneConcurrency)
	fmt.Printf("Timeout: %v\n", config.ProcessTimeout)
	
	// Repository targeting options
	if len(config.TargetRepos) > 0 {
		fmt.Printf("Target Repos: %s\n", strings.Join(config.TargetRepos, ", "))
	}
	if config.TargetReposFile != "" {
		fmt.Printf("Target Repos File: %s\n", config.TargetReposFile)
	}
	if config.MatchRegex != "" {
		fmt.Printf("Match Regex: %s\n", config.MatchRegex)
	}
	if len(config.MatchPrefix) > 0 {
		fmt.Printf("Match Prefix: %s\n", strings.Join(config.MatchPrefix, ", "))
	}
	if config.ExcludeRegex != "" {
		fmt.Printf("Exclude Regex: %s\n", config.ExcludeRegex)
	}
	if len(config.ExcludePrefix) > 0 {
		fmt.Printf("Exclude Prefix: %s\n", strings.Join(config.ExcludePrefix, ", "))
	}
	
	fmt.Printf("Output Format: %s\n", viper.GetString("output.format"))
	fmt.Printf("Output Directory: %s\n", viper.GetString("output.directory"))
	fmt.Printf("Markdown Style: %s\n", viper.GetString("ui.markdown_style"))
	fmt.Printf("Raw Markdown: %t\n", viper.GetBool("ui.raw_markdown"))

	return nil
}

// createConfigTemplate generates the configuration file template
func createConfigTemplate() string {
	return `# TF-Analyzer Configuration File

# GitHub Configuration
github:
  token: "${GITHUB_TOKEN}"  # Set via environment variable
  base_url: ""              # For GitHub Enterprise (optional)
  skip_archived: true       # Skip archived repositories
  skip_forks: false        # Skip forked repositories
  
  # Repository targeting options (use only one approach)
  # target_repos:           # Specific repositories to clone
  #   - "terraform-aws-vpc"
  #   - "terraform-aws-s3"
  # target_repos_file: ""   # Path to file with repository names (one per line)
  # match_regex: ""         # Regex pattern to match repository names (e.g., "^terraform-.*")
  # match_prefix:           # Prefixes to match repository names
  #   - "terraform-"
  #   - "aws-"
  # exclude_regex: ""       # Regex pattern to exclude repository names (e.g., ".*-deprecated$")
  # exclude_prefix:         # Prefixes to exclude repository names
  #   - "test-"
  #   - "demo-"

# Organizations to analyze
organizations:
  - "hashicorp"
  - "terraform-providers"

# Processing Configuration  
processing:
  max_goroutines: ` + fmt.Sprintf("%d", DefaultMaxGoroutines) + `       # Maximum concurrent goroutines
  clone_concurrency: ` + fmt.Sprintf("%d", DefaultCloneConcurrency) + `    # Clone concurrency limit
  timeout: "30m"           # Processing timeout

# Output Configuration
output:
  format: "all"            # json, csv, markdown, or all
  directory: "."           # Output directory for reports

# UI Configuration
ui:
  markdown_style: "auto"  # Markdown rendering style: auto, dark, light, notty
  raw_markdown: false     # Print raw markdown without glamour rendering
`
}

func initConfig(cmd *cobra.Command, args []string) error {
	configPath := ".tf-analyzer.yaml"
	if cfgFile != "" {
		configPath = cfgFile
	}

	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("configuration file already exists: %s", configPath)
	}

	configTemplate := createConfigTemplate()
	if err := os.WriteFile(configPath, []byte(configTemplate), 0644); err != nil {
		return fmt.Errorf("failed to create configuration file: %w", err)
	}

	fmt.Printf("Configuration file created: %s\n", configPath)
	fmt.Println("Edit the file and set your GITHUB_TOKEN environment variable.")
	
	return nil
}

func validateConfig(cmd *cobra.Command, args []string) error {
	config, err := createConfigFromViper()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if err := validateCLIAnalysisConfig(config); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	fmt.Println("✓ Configuration is valid")
	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}