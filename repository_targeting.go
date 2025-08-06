package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/samber/lo"
)

// ============================================================================
// REPOSITORY TARGETING - Functions for ghorg repository targeting
// ============================================================================

// parseTargetRepos parses a comma-separated string of repository names
func parseTargetRepos(input string) []string {
	if input == "" {
		return []string{}
	}
	
	repos := strings.Split(input, ",")
	return lo.Filter(lo.Map(repos, func(repo string, _ int) string {
		return strings.TrimSpace(repo)
	}), func(repo string, _ int) bool {
		return repo != ""
	})
}

// readTargetReposFromFile reads repository names from a file
func readTargetReposFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read target repos file %s: %w", filePath, err)
	}
	defer func() {
		_ = file.Close() // Ignore close error for read-only operations
	}()
	
	var repos []string
	scanner := bufio.NewScanner(file)
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line != "" && !strings.HasPrefix(line, "#") {
			repos = append(repos, line)
		}
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan target repos file %s: %w", filePath, err)
	}
	
	return repos, nil
}

// validateRegexPattern validates a regex pattern
func validateRegexPattern(pattern string) error {
	if pattern == "" {
		return nil // Empty pattern is valid
	}
	
	_, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern '%s': %w", pattern, err)
	}
	
	return nil
}

// parsePrefixes parses a comma-separated string of prefixes
func parsePrefixes(input string) []string {
	if input == "" {
		return []string{}
	}
	
	prefixes := strings.Split(input, ",")
	return lo.Filter(lo.Map(prefixes, func(prefix string, _ int) string {
		return strings.TrimSpace(prefix)
	}), func(prefix string, _ int) bool {
		return prefix != ""
	})
}

// validateTargetingConfiguration validates repository targeting configuration
func validateTargetingConfiguration(config Config) error {
	// Validate conflicting target options
	if len(config.TargetRepos) > 0 && config.TargetReposFile != "" {
		return fmt.Errorf("cannot specify both --target-repos and --target-repos-file")
	}
	
	// Validate conflicting match options
	if config.MatchRegex != "" && len(config.MatchPrefix) > 0 {
		return fmt.Errorf("cannot specify both --match-regex and --match-prefix")
	}
	
	// Validate conflicting exclude options
	if config.ExcludeRegex != "" && len(config.ExcludePrefix) > 0 {
		return fmt.Errorf("cannot specify both --exclude-regex and --exclude-prefix")
	}
	
	// Validate regex patterns
	if err := validateRegexPattern(config.MatchRegex); err != nil {
		return fmt.Errorf("invalid match regex: %w", err)
	}
	
	if err := validateRegexPattern(config.ExcludeRegex); err != nil {
		return fmt.Errorf("invalid exclude regex: %w", err)
	}
	
	return nil
}