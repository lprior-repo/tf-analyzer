package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestParseBackend(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected *BackendConfig
	}{
		{
			name: "s3 backend with region",
			content: `
terraform {
  backend "s3" {
    bucket = "my-bucket"
    key    = "terraform.tfstate"
    region = "us-west-2"
  }
}`,
			expected: &BackendConfig{
				Type:   stringPtr("s3"),
				Region: stringPtr("us-west-2"),
			},
		},
		{
			name: "backend without region",
			content: `
terraform {
  backend "local" {
    path = "terraform.tfstate"
  }
}`,
			expected: &BackendConfig{
				Type:   stringPtr("local"),
				Region: nil,
			},
		},
		{
			name:     "no backend returns nil",
			content:  `resource "aws_instance" "example" {}`,
			expected: nil,
		},
		{
			name:     "invalid HCL returns nil",
			content:  `terraform { backend "s3" { invalid syntax`,
			expected: nil,
		},
		{
			name:     "empty content returns nil",
			content:  "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: Terraform configuration content
			// When: parseBackend is called
			result := parseBackend(tt.content, "test.tf")
			
			// Then: result should match expected backend configuration
			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil result for no backend, got %+v", result)
				}
				return
			}
			
			if result == nil {
				t.Fatalf("Expected backend config %+v, got nil", tt.expected)
			}
			
			if !stringPtrEqual(tt.expected.Type, result.Type) {
				t.Errorf("Expected type %v, got %v", derefString(tt.expected.Type), derefString(result.Type))
			}
			if !stringPtrEqual(tt.expected.Region, result.Region) {
				t.Errorf("Expected region %v, got %v", derefString(tt.expected.Region), derefString(result.Region))
			}
		})
	}
}

func TestParseProviders(t *testing.T) {
	content := `
provider "aws" {
  region = "us-west-2"
}

required_providers {
  aws = {
    source  = "hashicorp/aws"
    version = "~> 5.0"
  }
}
`
	
	providers := parseProviders(content, "test.tf")
	
	if len(providers) == 0 {
		t.Error("Expected providers to be found")
		return
	}
	
	// Should find at least one provider
	found := false
	for _, provider := range providers {
		if provider.Source == "hashicorp/aws" && provider.Version == "~> 5.0" {
			found = true
			break
		}
		// Also accept if we find the provider config separately
		if provider.Source == "aws" && len(provider.Regions) > 0 {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("Expected to find AWS provider configuration")
	}
}

func TestParseModules(t *testing.T) {
	content := `
module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
  version = "~> 3.0"
}

module "security_group" {
  source = "./modules/security-group"
}

module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
  version = "~> 3.0"
}
`
	
	modules := parseModules(content, "test.tf")
	
	if len(modules) != 2 {
		t.Errorf("Expected 2 unique modules, got %d", len(modules))
	}
	
	// Check that vpc module appears twice
	for _, module := range modules {
		if module.Source == "terraform-aws-modules/vpc/aws" {
			if module.Count != 2 {
				t.Errorf("Expected VPC module count to be 2, got %d", module.Count)
			}
		}
	}
}

func TestParseVariables(t *testing.T) {
	content := `
variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-west-2"
}

variable "instance_count" {
  description = "Number of instances"
  type        = number
}
`
	
	variables := parseVariables(content, "test.tf")
	
	if len(variables) != 2 {
		t.Errorf("Expected 2 variables, got %d", len(variables))
	}
	
	expectedVariables := map[string]bool{
		"region":         true,  // has default
		"instance_count": false, // no default
	}
	
	for _, variable := range variables {
		expectedDefault, exists := expectedVariables[variable.Name]
		if !exists {
			t.Errorf("Unexpected variable: %s", variable.Name)
		}
		if variable.HasDefault != expectedDefault {
			t.Errorf("Expected HasDefault %t for variable %s, got %t", expectedDefault, variable.Name, variable.HasDefault)
		}
	}
}

func TestParseOutputs(t *testing.T) {
	content := `
output "vpc_id" {
  description = "VPC ID"
  value       = aws_vpc.main.id
}

output "subnet_ids" {
  description = "Subnet IDs"
  value       = aws_subnet.main[*].id
}
`
	
	outputs := parseOutputs(content, "test.tf")
	
	if len(outputs) != 2 {
		t.Errorf("Expected 2 outputs, got %d", len(outputs))
	}
	
	expectedOutputs := []string{"vpc_id", "subnet_ids"}
	
	for i, output := range outputs {
		if output != expectedOutputs[i] {
			t.Errorf("Expected output %s, got %s", expectedOutputs[i], output)
		}
	}
}

func TestParseResources(t *testing.T) {
	content := `
resource "aws_instance" "web" {
  ami           = "ami-12345678"
  instance_type = "t3.micro"
  
  tags = {
    Name        = "web-server"
    Environment = "production"
    Owner       = "team-a"
  }
}

resource "aws_s3_bucket" "data" {
  bucket = "my-data-bucket"
  
  tags = {
    Name = "data-bucket"
  }
}

resource "aws_instance" "api" {
  ami           = "ami-87654321"
  instance_type = "t3.small"
}
`
	
	resourceTypes, untaggedResources := parseResources(content, "test.tf")
	
	// Should have 2 resource types
	if len(resourceTypes) != 2 {
		t.Errorf("Expected 2 resource types, got %d", len(resourceTypes))
	}
	
	// Check resource type counts
	typeMap := make(map[string]int)
	for _, rt := range resourceTypes {
		typeMap[rt.Type] = rt.Count
	}
	
	if typeMap["aws_instance"] != 2 {
		t.Errorf("Expected 2 aws_instance resources, got %d", typeMap["aws_instance"])
	}
	
	if typeMap["aws_s3_bucket"] != 1 {
		t.Errorf("Expected 1 aws_s3_bucket resource, got %d", typeMap["aws_s3_bucket"])
	}
	
	// Should have untagged resources (missing mandatory tags)
	if len(untaggedResources) == 0 {
		t.Error("Expected to find untagged resources")
	}
	
	// Check that api instance is missing tags
	foundApiInstance := false
	for _, untagged := range untaggedResources {
		if untagged.ResourceType == "aws_instance" && untagged.Name == "api" {
			foundApiInstance = true
			if len(untagged.MissingTags) < 2 { // Should be missing Environment, Owner, Project
				t.Errorf("Expected API instance to be missing multiple tags, got %v", untagged.MissingTags)
			}
		}
	}
	
	if !foundApiInstance {
		t.Error("Expected to find API instance in untagged resources")
	}
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func stringPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func derefString(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

// TestIsRelevantFile tests file extension filtering logic
func TestIsRelevantFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "terraform file is relevant",
			path:     "main.tf",
			expected: true,
		},
		{
			name:     "tfvars file is relevant",
			path:     "terraform.tfvars",
			expected: true,
		},
		{
			name:     "hcl file is relevant",
			path:     "config.hcl",
			expected: true,
		},
		{
			name:     "uppercase extension is relevant",
			path:     "main.TF",
			expected: true,
		},
		{
			name:     "go file is not relevant",
			path:     "main.go",
			expected: false,
		},
		{
			name:     "json file is not relevant",
			path:     "config.json",
			expected: false,
		},
		{
			name:     "file without extension is not relevant",
			path:     "README",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: a file path
			// When: isRelevantFile is called
			result := isRelevantFile(tt.path)
			
			// Then: should return correct relevance
			if result != tt.expected {
				t.Errorf("Expected %t for path %s, got %t", tt.expected, tt.path, result)
			}
		})
	}
}

// TestShouldSkipPath tests path filtering logic
func TestShouldSkipPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "git directory should be skipped",
			path:     "/home/user/repo/.git/config",
			expected: true,
		},
		{
			name:     "nested git directory should be skipped",
			path:     "/home/user/repo/submodule/.git/refs/heads/main",
			expected: true,
		},
		{
			name:     "regular path should not be skipped",
			path:     "/home/user/repo/main.tf",
			expected: false,
		},
		{
			name:     "path with git in name but not directory should not be skipped",
			path:     "/home/user/repo/gitops.tf",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: a file path
			// When: shouldSkipPath is called
			result := shouldSkipPath(tt.path)
			
			// Then: should return correct skip decision
			if result != tt.expected {
				t.Errorf("Expected %t for path %s, got %t", tt.expected, tt.path, result)
			}
		})
	}
}

// TestFindMissingTags tests tag validation logic
func TestFindMissingTags(t *testing.T) {
	tests := []struct {
		name         string
		tags         map[string]string
		expectedMiss []string
	}{
		{
			name: "all mandatory tags present",
			tags: map[string]string{
				"Environment": "prod",
				"Owner":       "team-a",
				"Project":     "web-app",
			},
			expectedMiss: []string{},
		},
		{
			name: "missing Environment tag",
			tags: map[string]string{
				"Owner":   "team-a",
				"Project": "web-app",
			},
			expectedMiss: []string{"Environment"},
		},
		{
			name: "missing multiple tags",
			tags: map[string]string{
				"Name": "my-resource",
			},
			expectedMiss: []string{"Environment", "Owner", "Project"},
		},
		{
			name:         "no tags provided",
			tags:         map[string]string{},
			expectedMiss: []string{"Environment", "Owner", "Project"},
		},
		{
			name:         "nil tags map",
			tags:         nil,
			expectedMiss: []string{"Environment", "Owner", "Project"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: a set of resource tags
			// When: findMissingTags is called
			result := findMissingTags(tt.tags)
			
			// Then: should return correct missing tags
			if len(result) != len(tt.expectedMiss) {
				t.Errorf("Expected %d missing tags, got %d", len(tt.expectedMiss), len(result))
			}
			
			for _, expectedTag := range tt.expectedMiss {
				found := false
				for _, actualTag := range result {
					if actualTag == expectedTag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected missing tag %s not found in result %v", expectedTag, result)
				}
			}
		})
	}
}

// TestProcessRepositoryFilesWithRecovery tests the main analysis function with error recovery
func TestProcessRepositoryFilesWithRecovery(t *testing.T) {
	// Create temporary test directory structure
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "test-repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create test repo directory: %v", err)
	}

	// Create test terraform file
	tfContent := `
terraform {
  backend "s3" {
    bucket = "test-bucket"
    region = "us-west-2"
  }
}

provider "aws" {
  region = "us-west-2"
}

resource "aws_instance" "web" {
  ami           = "ami-12345"
  instance_type = "t3.micro"
  
  tags = {
    Name        = "web-server"
    Environment = "test"
    Owner       = "test-team"
    Project     = "test-project"
  }
}

variable "instance_count" {
  description = "Number of instances"
  type        = number
  default     = 1
}

output "instance_id" {
  value = aws_instance.web.id
}
`
	tfFile := filepath.Join(repoDir, "main.tf")
	if err := os.WriteFile(tfFile, []byte(tfContent), 0644); err != nil {
		t.Fatalf("Failed to write test terraform file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	repo := Repository{
		Name:         "test-repo",
		Organization: "test-org",
		Path:         repoDir,
	}

	t.Run("successful repository analysis", func(t *testing.T) {
		// Given: a repository with valid terraform files
		// When: processRepositoryFilesWithRecovery is called
		result := processRepositoryFilesWithRecovery(repo, logger)
		
		// Then: analysis should succeed with expected data
		if result.Error != nil {
			t.Fatalf("Expected no error, got %v", result.Error)
		}
		
		if result.RepoName != "test-repo" {
			t.Errorf("Expected repo name 'test-repo', got %s", result.RepoName)
		}
		
		if result.Organization != "test-org" {
			t.Errorf("Expected organization 'test-org', got %s", result.Organization)
		}
		
		// Verify backend was parsed
		if result.Analysis.BackendConfig == nil {
			t.Error("Expected backend config to be parsed")
		} else {
			if result.Analysis.BackendConfig.Type == nil || *result.Analysis.BackendConfig.Type != "s3" {
				t.Errorf("Expected backend type 's3', got %v", result.Analysis.BackendConfig.Type)
			}
		}
		
		// Verify resources were counted
		if result.Analysis.ResourceAnalysis.TotalResourceCount != 1 {
			t.Errorf("Expected 1 resource, got %d", result.Analysis.ResourceAnalysis.TotalResourceCount)
		}
	})

	t.Run("repository analysis with invalid path", func(t *testing.T) {
		// Given: a repository with invalid path
		invalidRepo := Repository{
			Name:         "invalid-repo",
			Organization: "test-org",
			Path:         "/nonexistent/path",
		}
		
		// When: processRepositoryFilesWithRecovery is called
		result := processRepositoryFilesWithRecovery(invalidRepo, logger)
		
		// Then: should return error result
		if result.Error == nil {
			t.Error("Expected error for invalid repository path")
		}
		
		if result.RepoName != "invalid-repo" {
			t.Errorf("Expected repo name 'invalid-repo', got %s", result.RepoName)
		}
	})
}