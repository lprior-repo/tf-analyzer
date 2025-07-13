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
	noTUI          bool
	verbose         bool
	markdownStyle   string
	rawMarkdown     bool
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
• **Interactive TUI**: Beautiful terminal interface with real-time progress tracking
• **Multiple Export Formats**: JSON, CSV, and Markdown report generation
• **Provider Analysis**: Detect and analyze Terraform providers and their versions
• **Resource Analysis**: Count and categorize Terraform resources and modules
• **Backend Configuration**: Analyze Terraform backend configurations
• **Tagging Compliance**: Check for mandatory resource tags

## Quick Start

{{.}}

	# Set your GitHub token
	export GITHUB_TOKEN=your_token_here
	
	# Analyze organizations with interactive TUI
	tf-analyzer analyze --orgs "hashicorp,terraform-providers"
	
	# Run without TUI and export to custom directory  
	tf-analyzer analyze --orgs "my-org" --no-tui --output-dir ./reports

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
	tf-analyzer analyze --orgs "hashicorp"
	
	# Analyze multiple organizations with custom settings
	tf-analyzer analyze --orgs "org1,org2,org3" --max-goroutines 50 --timeout 45m
	
	# Export reports to specific directory without TUI
	tf-analyzer analyze --orgs "my-org" --no-tui --output-dir ./custom-reports
	
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
• MAX_GOROUTINES: Maximum concurrent goroutines (default: 100)
• CLONE_CONCURRENCY: Clone concurrency limit (default: 100)
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

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.tf-analyzer.yaml)")
	rootCmd.PersistentFlags().StringVar(&envFile, "env-file", ".env", "environment file path (default is .env in current directory)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Analyze command flags
	analyzeCmd.Flags().StringSliceVarP(&organizations, "orgs", "o", []string{}, "GitHub organizations to analyze (comma-separated)")
	analyzeCmd.Flags().StringVarP(&githubToken, "token", "t", "", "GitHub API token")
	analyzeCmd.Flags().IntVar(&maxGoroutines, "max-goroutines", 100, "maximum concurrent goroutines")
	analyzeCmd.Flags().IntVar(&cloneConcurrency, "clone-concurrency", 100, "clone concurrency limit")
	analyzeCmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "processing timeout")
	analyzeCmd.Flags().StringVar(&outputFormat, "format", "all", "output format: json, csv, markdown, or all")
	analyzeCmd.Flags().StringVar(&outputDir, "output-dir", ".", "output directory for reports")
	analyzeCmd.Flags().BoolVar(&noTUI, "no-tui", false, "disable interactive TUI")
	analyzeCmd.Flags().StringVar(&markdownStyle, "markdown-style", "auto", "markdown rendering style: auto, dark, light, notty")
	analyzeCmd.Flags().BoolVar(&rawMarkdown, "raw-markdown", false, "print raw markdown without glamour rendering")

	// Mark required flags
	analyzeCmd.MarkFlagRequired("orgs")

	// Bind flags to viper
	viper.BindPFlag("organizations", analyzeCmd.Flags().Lookup("orgs"))
	viper.BindPFlag("github.token", analyzeCmd.Flags().Lookup("token"))
	viper.BindPFlag("processing.max_goroutines", analyzeCmd.Flags().Lookup("max-goroutines"))
	viper.BindPFlag("processing.clone_concurrency", analyzeCmd.Flags().Lookup("clone-concurrency"))
	viper.BindPFlag("processing.timeout", analyzeCmd.Flags().Lookup("timeout"))
	viper.BindPFlag("output.format", analyzeCmd.Flags().Lookup("format"))
	viper.BindPFlag("output.directory", analyzeCmd.Flags().Lookup("output-dir"))
	viper.BindPFlag("ui.no_tui", analyzeCmd.Flags().Lookup("no-tui"))
	viper.BindPFlag("ui.markdown_style", analyzeCmd.Flags().Lookup("markdown-style"))
	viper.BindPFlag("ui.raw_markdown", analyzeCmd.Flags().Lookup("raw-markdown"))

	// Add subcommands
	configCmd.AddCommand(configShowCmd, configInitCmd, configValidateCmd)
	rootCmd.AddCommand(analyzeCmd, configCmd)
}

func initializeConfig() {
	// Load .env file - required by default
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

	// Environment variable bindings
	viper.SetEnvPrefix("TF_ANALYZER")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Specific environment variable mappings
	viper.BindEnv("github.token", "GITHUB_TOKEN")
	viper.BindEnv("organizations", "GITHUB_ORGS")
	viper.BindEnv("processing.max_goroutines", "MAX_GOROUTINES")
	viper.BindEnv("processing.clone_concurrency", "CLONE_CONCURRENCY")

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

	processingCtx, err := setupProcessingResources(config)
	if err != nil {
		return err
	}
	defer releaseProcessingContext(processingCtx)

	tuiProgress := setupTUIIfEnabled(config, logger)
	
	ctx, cancel := context.WithTimeout(context.Background(), config.ProcessTimeout)
	defer cancel()

	reporter, analysisErr := executeAnalysisWorkflow(ctx, processingCtx, tuiProgress)
	
	completeTUIAndWait(tuiProgress, reporter)
	
	if analysisErr != nil {
		logger.Error("Analysis completed with errors", "error", analysisErr)
	}

	if err := generateReports(reporter, config); err != nil {
		return fmt.Errorf("failed to generate reports: %w", err)
	}

	handleNoTUIOutput(reporter, logger)
	
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

func setupProcessingResources(config Config) (ProcessingContext, error) {
	processingCtx, err := createProcessingContext(config)
	if err != nil {
		return ProcessingContext{}, fmt.Errorf("failed to create processing context: %w", err)
	}
	return processingCtx, nil
}

func setupTUIIfEnabled(config Config, logger *slog.Logger) *TUIProgressChannel {
	if noTUI {
		return nil
	}

	totalRepos := estimateRepositoryCount(config)
	tuiProgress := NewTUIProgressChannel(totalRepos)
	ctx := context.Background()
	tuiProgress.Start(ctx)

	go func() {
		if err := tuiProgress.Run(); err != nil {
			logger.Error("TUI error", "error", err)
		}
	}()

	return tuiProgress
}

func executeAnalysisWorkflow(ctx context.Context, processingCtx ProcessingContext, tuiProgress *TUIProgressChannel) (*Reporter, error) {
	reporter := NewReporter()
	analysisErr := cloneAndAnalyzeMultipleOrgs(ctx, processingCtx, reporter, tuiProgress)
	return reporter, analysisErr
}

func completeTUIAndWait(tuiProgress *TUIProgressChannel, reporter *Reporter) {
	if tuiProgress != nil {
		tuiProgress.Complete(reporter.results)
		time.Sleep(2 * time.Second)
	}
}

func handleNoTUIOutput(reporter *Reporter, logger *slog.Logger) {
	if noTUI {
		if err := reporter.PrintSummaryReport(); err != nil {
			logger.Error("Failed to print summary report", "error", err)
		}
		
		printMarkdownReport(reporter, logger)
	}
}

func printMarkdownReport(reporter *Reporter, logger *slog.Logger) {
	if viper.GetBool("ui.raw_markdown") {
		reporter.PrintMarkdownToScreen()
		return
	}

	style := viper.GetString("ui.markdown_style")
	if style == "auto" {
		style = detectTerminalCapabilities()
	}
	
	if err := reporter.PrintMarkdownToScreenWithStyle(style); err != nil {
		logger.Error("Failed to render markdown", "error", err)
		reporter.PrintMarkdownToScreen()
	}
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

	return Config{
		Organizations:    orgs,
		GitHubToken:      viper.GetString("github.token"),
		MaxGoroutines:    viper.GetInt("processing.max_goroutines"),
		CloneConcurrency: viper.GetInt("processing.clone_concurrency"),
		ProcessTimeout:   viper.GetDuration("processing.timeout"),
		SkipArchived:     viper.GetBool("github.skip_archived"),
		SkipForks:        viper.GetBool("github.skip_forks"),
		BaseURL:          viper.GetString("github.base_url"),
	}, nil
}

func validateCLIAnalysisConfig(config Config) error {
	if len(config.Organizations) == 0 {
		return fmt.Errorf("at least one organization must be specified")
	}
	if config.GitHubToken == "" {
		return fmt.Errorf("GitHub token is required (set GITHUB_TOKEN or use --token)")
	}
	return validateAnalysisConfiguration(config)
}

func estimateRepositoryCount(config Config) int {
	// Conservative estimate: 50 repos per organization
	return len(config.Organizations) * 50
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
	fmt.Printf("Output Format: %s\n", viper.GetString("output.format"))
	fmt.Printf("Output Directory: %s\n", viper.GetString("output.directory"))
	fmt.Printf("No TUI: %t\n", viper.GetBool("ui.no_tui"))
	fmt.Printf("Markdown Style: %s\n", viper.GetString("ui.markdown_style"))
	fmt.Printf("Raw Markdown: %t\n", viper.GetBool("ui.raw_markdown"))

	return nil
}

func initConfig(cmd *cobra.Command, args []string) error {
	configTemplate := `# TF-Analyzer Configuration File

# GitHub Configuration
github:
  token: "${GITHUB_TOKEN}"  # Set via environment variable
  base_url: ""              # For GitHub Enterprise (optional)
  skip_archived: true       # Skip archived repositories
  skip_forks: false        # Skip forked repositories

# Organizations to analyze
organizations:
  - "hashicorp"
  - "terraform-providers"

# Processing Configuration  
processing:
  max_goroutines: 100       # Maximum concurrent goroutines
  clone_concurrency: 100    # Clone concurrency limit
  timeout: "30m"           # Processing timeout

# Output Configuration
output:
  format: "all"            # json, csv, markdown, or all
  directory: "."           # Output directory for reports

# UI Configuration
ui:
  no_tui: false           # Disable interactive TUI
  markdown_style: "auto"  # Markdown rendering style: auto, dark, light, notty
  raw_markdown: false     # Print raw markdown without glamour rendering
`

	configPath := ".tf-analyzer.yaml"
	if cfgFile != "" {
		configPath = cfgFile
	}

	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("configuration file already exists: %s", configPath)
	}

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