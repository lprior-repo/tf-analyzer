package main

import (
	"strings"
	"testing"
)

// ============================================================================
// ENHANCED UNIT TESTS - Core Business Logic & Edge Cases
// ============================================================================
// Following Martin Fowler's unit testing principles: focus on core logic,
// edge cases, and input validation. Small, fast, and independent tests.

// TestExtractRepoNameEdgeCases tests the repository name extraction logic
// with comprehensive edge cases that could break in production.
func TestExtractRepoNameEdgeCases(t *testing.T) {
	testCases := []struct {
		name           string
		repoPath       string
		expectedResult string
		description    string
	}{
		{
			name:           "normal_repository_path",
			repoPath:       "path/to/my-repo",
			expectedResult: "my-repo",
			description:    "Standard case with unix-style path",
		},
		{
			name:           "windows_style_path",
			repoPath:       "C:\\Users\\dev\\repos\\my-repo",
			expectedResult: "my-repo",
			description:    "Windows-style path with backslashes",
		},
		{
			name:           "path_with_trailing_slash",
			repoPath:       "path/to/my-repo/",
			expectedResult: "my-repo",
			description:    "Path ending with slash should be handled",
		},
		{
			name:           "path_with_multiple_trailing_slashes",
			repoPath:       "path/to/my-repo///",
			expectedResult: "my-repo",
			description:    "Multiple trailing slashes should be cleaned",
		},
		{
			name:           "single_directory_name",
			repoPath:       "repo-name",
			expectedResult: "repo-name",
			description:    "Just a directory name without path",
		},
		{
			name:           "empty_string",
			repoPath:       "",
			expectedResult: "",
			description:    "Empty string should return empty",
		},
		{
			name:           "only_slashes",
			repoPath:       "///",
			expectedResult: "",
			description:    "String with only slashes should return empty",
		},
		{
			name:           "path_with_spaces",
			repoPath:       "path/to/my repo name",
			expectedResult: "my repo name",
			description:    "Repository names with spaces should work",
		},
		{
			name:           "path_with_special_characters",
			repoPath:       "path/to/repo-name_v2.0",
			expectedResult: "repo-name_v2.0",
			description:    "Special characters in repo names should be preserved",
		},
		{
			name:           "very_deep_path",
			repoPath:       "very/deeply/nested/path/structure/with/many/levels/final-repo",
			expectedResult: "final-repo",
			description:    "Very deep paths should extract correctly",
		},
		{
			name:           "path_with_dots",
			repoPath:       "path/to/../repo/../final-repo",
			expectedResult: "final-repo",
			description:    "Path with relative references",
		},
		{
			name:           "unicode_characters",
			repoPath:       "path/to/测试仓库",
			expectedResult: "测试仓库",
			description:    "Unicode characters should be handled",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// WHEN: extractRepoName is called with the test path
			result := extractRepoName(tc.repoPath)
			
			// THEN: result should match expected value
			if result != tc.expectedResult {
				t.Errorf("extractRepoName(%q) = %q, expected %q\nDescription: %s",
					tc.repoPath, result, tc.expectedResult, tc.description)
			}
			
			// AND: result should be idempotent (calling again yields same result)
			secondResult := extractRepoName(tc.repoPath)
			if result != secondResult {
				t.Errorf("extractRepoName is not idempotent: first=%q, second=%q", result, secondResult)
			}
		})
	}
}

// TestFindMissingTagsCriticalLogic tests the core business logic for
// mandatory tag validation with edge cases that could cause security issues.
func TestFindMissingTagsCriticalLogic(t *testing.T) {
	// Define mandatory tags that should always be present for compliance
	mandatoryTags := []string{"Environment", "Owner", "Project", "CostCenter"}
	
	testCases := []struct {
		name           string
		tags           map[string]string
		expectedMissing []string
		description    string
	}{
		{
			name: "all_mandatory_tags_present",
			tags: map[string]string{
				"Environment": "production",
				"Owner":       "devops-team",
				"Project":     "web-app",
				"CostCenter":  "engineering",
				"Optional":    "extra-tag",
			},
			expectedMissing: []string{},
			description:     "Complete compliant resource",
		},
		{
			name: "missing_critical_environment_tag",
			tags: map[string]string{
				"Owner":      "devops-team",
				"Project":    "web-app", 
				"CostCenter": "engineering",
			},
			expectedMissing: []string{"Environment"},
			description:     "Missing Environment tag (security risk)",
		},
		{
			name: "missing_multiple_tags",
			tags: map[string]string{
				"Environment": "production",
				"Optional":    "some-value",
			},
			expectedMissing: []string{"Owner", "Project", "CostCenter"},
			description:     "Multiple missing tags (compliance violation)",
		},
		{
			name:            "completely_empty_tags",
			tags:            map[string]string{},
			expectedMissing: mandatoryTags,
			description:     "No tags at all (major compliance issue)",
		},
		{
			name:            "nil_tags_map",
			tags:            nil,
			expectedMissing: mandatoryTags,
			description:     "Nil map should be handled gracefully",
		},
		{
			name: "case_sensitive_tag_names",
			tags: map[string]string{
				"environment": "production", // lowercase
				"OWNER":       "team",       // uppercase
				"Project":     "app",        // correct case
				"costcenter":  "eng",        // lowercase
			},
			expectedMissing: []string{"Environment", "Owner", "CostCenter"},
			description:     "Tag names are case-sensitive",
		},
		{
			name: "empty_tag_values", 
			tags: map[string]string{
				"Environment": "",  // Empty value
				"Owner":       "team",
				"Project":     "app",
				"CostCenter":  "eng",
			},
			expectedMissing: []string{"Environment"},
			description:     "Empty tag values should be considered missing",
		},
		{
			name: "whitespace_only_values",
			tags: map[string]string{
				"Environment": "   ",  // Whitespace only
				"Owner":       "team",
				"Project":     "app", 
				"CostCenter":  "eng",
			},
			expectedMissing: []string{"Environment"},
			description:     "Whitespace-only values should be considered missing",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// WHEN: findMissingTags is called
			missing := findMissingTags(tc.tags)
			
			// THEN: result should match expected missing tags
			if len(missing) != len(tc.expectedMissing) {
				t.Errorf("Expected %d missing tags, got %d\nExpected: %v\nActual: %v\nDescription: %s",
					len(tc.expectedMissing), len(missing), tc.expectedMissing, missing, tc.description)
				return
			}
			
			// AND: all expected missing tags should be present in result
			for _, expectedTag := range tc.expectedMissing {
				found := false
				for _, actualTag := range missing {
					if actualTag == expectedTag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected missing tag '%s' not found in result %v\nDescription: %s",
						expectedTag, missing, tc.description)
				}
			}
			
			// AND: no unexpected tags should be in result
			for _, actualTag := range missing {
				found := false
				for _, expectedTag := range tc.expectedMissing {
					if actualTag == expectedTag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Unexpected missing tag '%s' found in result %v\nDescription: %s",
						actualTag, missing, tc.description)
				}
			}
		})
	}
}

// TestIsRelevantFileSecurityValidation tests file filtering logic for
// security implications - ensuring we only process safe file types.
func TestIsRelevantFileSecurityValidation(t *testing.T) {
	testCases := []struct {
		name           string
		filename       string
		expectedResult bool
		securityNote   string
	}{
		// Safe files that should be processed
		{
			name:           "terraform_main_file",
			filename:       "main.tf",
			expectedResult: true,
			securityNote:   "Standard Terraform file",
		},
		{
			name:           "terraform_variables",
			filename:       "variables.tf",
			expectedResult: true,
			securityNote:   "Safe Terraform configuration",
		},
		{
			name:           "tfvars_file",
			filename:       "terraform.tfvars",
			expectedResult: true,
			securityNote:   "Variable definitions file",
		},
		{
			name:           "hcl_configuration",
			filename:       "config.hcl",
			expectedResult: true,
			securityNote:   "HashiCorp configuration file",
		},
		
		// Potentially dangerous files that should be rejected
		{
			name:           "executable_file",
			filename:       "malicious.exe",
			expectedResult: false,
			securityNote:   "SECURITY: Executable files should never be processed",
		},
		{
			name:           "shell_script",
			filename:       "deploy.sh",
			expectedResult: false,
			securityNote:   "SECURITY: Shell scripts could contain malicious code",
		},
		{
			name:           "python_script",
			filename:       "script.py",
			expectedResult: false,
			securityNote:   "SECURITY: Python scripts should not be analyzed",
		},
		{
			name:           "binary_file",
			filename:       "data.bin",
			expectedResult: false,
			securityNote:   "SECURITY: Binary files should be ignored",
		},
		{
			name:           "hidden_file",
			filename:       ".secret",
			expectedResult: false,
			securityNote:   "SECURITY: Hidden files might contain secrets",
		},
		{
			name:           "config_with_secrets",
			filename:       ".env",
			expectedResult: false,
			securityNote:   "SECURITY: Environment files often contain secrets",
		},
		
		// Edge cases for security testing
		{
			name:           "file_with_no_extension",
			filename:       "README",
			expectedResult: false,
			securityNote:   "Files without extensions should be excluded by default",
		},
		{
			name:           "case_insensitive_terraform",
			filename:       "Main.TF",
			expectedResult: true,
			securityNote:   "Case should not affect Terraform file detection",
		},
		{
			name:           "deeply_nested_terraform",
			filename:       "modules/vpc/networking/main.tf",
			expectedResult: true,
			securityNote:   "Path depth should not affect detection",
		},
		{
			name:           "terraform_with_suspicious_name",
			filename:       "backdoor.tf",
			expectedResult: true,
			securityNote:   "Terraform files are safe regardless of name",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// WHEN: isRelevantFile is called
			result := isRelevantFile(tc.filename)
			
			// THEN: result should match security expectations
			if result != tc.expectedResult {
				t.Errorf("isRelevantFile(%q) = %v, expected %v\nSecurity Note: %s",
					tc.filename, result, tc.expectedResult, tc.securityNote)
			}
			
			// AND: if file is deemed relevant, it should be safe to process
			if result && tc.expectedResult {
				// Additional security check: ensure it's actually a terraform-related file
				lowerFilename := strings.ToLower(tc.filename)
				if !strings.HasSuffix(lowerFilename, ".tf") &&
				   !strings.HasSuffix(lowerFilename, ".tfvars") &&
				   !strings.HasSuffix(lowerFilename, ".hcl") {
					t.Errorf("File marked as relevant but may not be safe: %q", tc.filename)
				}
			}
		})
	}
}

// TestShouldSkipPathSecurityRules tests path filtering for security-sensitive
// directories that should never be processed.
func TestShouldSkipPathSecurityRules(t *testing.T) {
	testCases := []struct {
		name         string
		path         string
		shouldSkip   bool
		securityNote string
	}{
		// Paths that should be skipped for security
		{
			name:         "git_directory",
			path:         ".git/config", 
			shouldSkip:   true,
			securityNote: "SECURITY: .git contains sensitive repository data",
		},
		{
			name:         "nested_git_hooks",
			path:         "repo/.git/hooks/pre-commit",
			shouldSkip:   true,
			securityNote: "SECURITY: Git hooks can contain executable code",
		},
		{
			name:         "node_modules",
			path:         "frontend/node_modules/package/index.js",
			shouldSkip:   true,
			securityNote: "SECURITY: Node modules may contain malicious packages",
		},
		{
			name:         "vendor_directory",
			path:         "go/vendor/github.com/package/file.go",
			shouldSkip:   true,
			securityNote: "SECURITY: Vendor directories contain external code",
		},
		{
			name:         "python_cache",
			path:         "__pycache__/module.pyc",
			shouldSkip:   true,
			securityNote: "SECURITY: Python cache files are compiled bytecode",
		},
		{
			name:         "temporary_files",
			path:         "tmp/temp_file.txt",
			shouldSkip:   true,
			securityNote: "SECURITY: Temporary files may contain sensitive data",
		},
		
		// Paths that should be processed (safe)
		{
			name:         "terraform_modules",
			path:         "modules/vpc/main.tf",
			shouldSkip:   false,
			securityNote: "Safe: Terraform modules are configuration only",
		},
		{
			name:         "documentation",
			path:         "docs/README.md",
			shouldSkip:   false,
			securityNote: "Safe: Documentation files are harmless",
		},
		{
			name:         "terraform_in_subdirectory",
			path:         "environments/prod/main.tf",
			shouldSkip:   false,
			securityNote: "Safe: Terraform files in any subdirectory",
		},
		{
			name:         "root_terraform_file",
			path:         "main.tf",
			shouldSkip:   false,
			securityNote: "Safe: Root level Terraform files",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// WHEN: shouldSkipPath is called
			result := shouldSkipPath(tc.path)
			
			// THEN: result should match security expectations
			if result != tc.shouldSkip {
				t.Errorf("shouldSkipPath(%q) = %v, expected %v\nSecurity Note: %s",
					tc.path, result, tc.shouldSkip, tc.securityNote)
			}
			
			// AND: ensure security-sensitive paths are always skipped
			if strings.Contains(tc.path, ".git/") && !result {
				t.Errorf("SECURITY VIOLATION: .git path should always be skipped: %q", tc.path)
			}
			
			if strings.Contains(tc.path, "node_modules/") && !result {
				t.Errorf("SECURITY VIOLATION: node_modules should always be skipped: %q", tc.path)
			}
		})
	}
}

// TestConfigurationValidationBoundaryValues tests configuration validation
// with boundary values that could cause system instability.
func TestConfigurationValidationBoundaryValues(t *testing.T) {
	testCases := []struct {
		name          string
		config        Config
		expectedError bool
		errorContains string
		riskLevel     string
	}{
		{
			name: "maximum_safe_goroutines",
			config: Config{
				Organizations:    []string{"test-org"},
				GitHubToken:      "valid-token",
				MaxGoroutines:    1000, // High but reasonable
				CloneConcurrency: 50,
			},
			expectedError: false,
			riskLevel:     "LOW",
		},
		{
			name: "extremely_high_goroutines",
			config: Config{
				Organizations:    []string{"test-org"},
				GitHubToken:      "valid-token",
				MaxGoroutines:    100000, // Could exhaust system resources
				CloneConcurrency: 50,
			},
			expectedError: true,
			errorContains: "MaxGoroutines",
			riskLevel:     "HIGH - Resource exhaustion risk",
		},
		{
			name: "zero_goroutines",
			config: Config{
				Organizations:    []string{"test-org"},
				GitHubToken:      "valid-token",
				MaxGoroutines:    0, // Invalid
				CloneConcurrency: 50,
			},
			expectedError: true,
			errorContains: "MaxGoroutines",
			riskLevel:     "MEDIUM - Application would hang",
		},
		{
			name: "negative_goroutines",
			config: Config{
				Organizations:    []string{"test-org"},
				GitHubToken:      "valid-token",
				MaxGoroutines:    -1, // Invalid
				CloneConcurrency: 50,
			},
			expectedError: true,
			errorContains: "MaxGoroutines",
			riskLevel:     "MEDIUM - Invalid configuration",
		},
		{
			name: "empty_organizations_list",
			config: Config{
				Organizations:    []string{}, // Invalid
				GitHubToken:      "valid-token",
				MaxGoroutines:    10,
				CloneConcurrency: 5,
			},
			expectedError: true,
			errorContains: "organization",
			riskLevel:     "LOW - User configuration error",
		},
		{
			name: "extremely_long_organization_name",
			config: Config{
				Organizations:    []string{strings.Repeat("a", 1000)}, // Very long name
				GitHubToken:      "valid-token", 
				MaxGoroutines:    10,
				CloneConcurrency: 5,
			},
			expectedError: false, // Should be handled by GitHub API
			riskLevel:     "LOW - GitHub will reject invalid org names",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// WHEN: configuration is validated
			err := validateAnalysisConfiguration(tc.config)
			
			// THEN: validation should match expectations
			if tc.expectedError {
				if err == nil {
					t.Errorf("Expected validation error for %s (Risk: %s), but got nil",
						tc.name, tc.riskLevel)
				} else if tc.errorContains != "" && !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v (Risk: %s)",
						tc.errorContains, err, tc.riskLevel)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for %s (Risk: %s), got: %v",
						tc.name, tc.riskLevel, err)
				}
			}
			
			// Log risk assessment for security review
			t.Logf("Risk Assessment for %s: %s", tc.name, tc.riskLevel)
		})
	}
}