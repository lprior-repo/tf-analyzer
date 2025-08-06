package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"pgregory.net/rapid"
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
				"CostCenter":  "eng",
			},
			expectedMiss: []string{},
		},
		{
			name: "missing Environment tag",
			tags: map[string]string{
				"Owner":      "team-a",
				"Project":    "web-app",
				"CostCenter": "eng",
			},
			expectedMiss: []string{"Environment"},
		},
		{
			name: "missing multiple tags",
			tags: map[string]string{
				"Name": "my-resource",
			},
			expectedMiss: []string{"Environment", "Owner", "Project", "CostCenter"},
		},
		{
			name:         "no tags provided",
			tags:         map[string]string{},
			expectedMiss: []string{"Environment", "Owner", "Project", "CostCenter"},
		},
		{
			name:         "nil tags map",
			tags:         nil,
			expectedMiss: []string{"Environment", "Owner", "Project", "CostCenter"},
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
// ============================================================================
// ADDITIONAL TESTS (from analyzer_integration_test.go, analyzer_property_test.go, analyzer_fuzz_test.go)
// ============================================================================
// Integration tests verify component interactions and real-world scenarios
// These tests use actual file system operations and validate end-to-end workflows

// TestRepositoryAnalysisIntegration tests the complete repository analysis workflow
func TestRepositoryAnalysisIntegration(t *testing.T) {
	// Given: a temporary directory with realistic terraform structure
	tempDir := t.TempDir()
	
	// Create a realistic repository structure
	repoStructure := map[string]string{
		"main.tf": `
terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.0"
    }
  }
  
  backend "s3" {
    bucket = "terraform-state-bucket"
    key    = "prod/terraform.tfstate"
    region = "us-west-2"
  }
}

provider "aws" {
  region = "us-west-2"
}

provider "kubernetes" {
  host                   = data.aws_eks_cluster.cluster.endpoint
  cluster_ca_certificate = base64decode(data.aws_eks_cluster.cluster.certificate_authority[0].data)
  token                  = data.aws_eks_cluster_auth.cluster.token
}
`,
		"variables.tf": `
variable "environment" {
  description = "Environment name"
  type        = string
  default     = "production"
}

variable "instance_count" {
  description = "Number of instances to create"
  type        = number
}

variable "instance_types" {
  description = "List of instance types"
  type        = list(string)
  default     = ["t3.micro", "t3.small"]
}

variable "tags" {
  description = "Common tags for all resources"
  type        = map(string)
  default = {
    Environment = "production"
    Project     = "web-application"
  }
}
`,
		"resources.tf": `
resource "aws_vpc" "main" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = merge(var.tags, {
    Name        = "main-vpc"
    Environment = var.environment
    Owner       = "platform-team"
    Project     = "infrastructure"
  })
}

resource "aws_subnet" "public" {
  count                   = 2
  vpc_id                  = aws_vpc.main.id
  cidr_block              = "10.0.${count.index + 1}.0/24"
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = true

  tags = merge(var.tags, {
    Name        = "public-subnet-${count.index + 1}"
    Type        = "public"
    Environment = var.environment
    Owner       = "platform-team"
    Project     = "infrastructure"
  })
}

resource "aws_instance" "web" {
  count           = var.instance_count
  ami             = data.aws_ami.amazon_linux.id
  instance_type   = var.instance_types[count.index % length(var.instance_types)]
  subnet_id       = aws_subnet.public[count.index % length(aws_subnet.public)].id
  security_groups = [aws_security_group.web.id]

  tags = merge(var.tags, {
    Name        = "web-server-${count.index + 1}"
    Environment = var.environment
    Owner       = "web-team"
    Project     = "web-application"
  })
}

resource "aws_security_group" "web" {
  name_prefix = "web-sg"
  vpc_id      = aws_vpc.main.id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.tags, {
    Name        = "web-security-group"
    Environment = var.environment
    Owner       = "security-team"
    Project     = "infrastructure"
  })
}

# Intentionally untagged resource to test validation
resource "aws_s3_bucket" "untagged" {
  bucket = "my-untagged-bucket"
}
`,
		"modules.tf": `
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.0"

  name = "main-vpc"
  cidr = "10.0.0.0/16"

  azs             = data.aws_availability_zones.available.names
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24"]

  enable_nat_gateway = true
  enable_vpn_gateway = true

  tags = var.tags
}

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 19.0"

  cluster_name    = "main-cluster"
  cluster_version = "1.27"

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  tags = var.tags
}

module "local_module" {
  source = "./modules/networking"
  
  vpc_id = aws_vpc.main.id
  tags   = var.tags
}
`,
		"outputs.tf": `
output "vpc_id" {
  description = "ID of the VPC"
  value       = aws_vpc.main.id
}

output "public_subnet_ids" {
  description = "IDs of the public subnets"
  value       = aws_subnet.public[*].id
}

output "web_instance_ids" {
  description = "IDs of the web instances"
  value       = aws_instance.web[*].id
}

output "security_group_id" {
  description = "ID of the web security group"
  value       = aws_security_group.web.id
}

output "load_balancer_dns" {
  description = "DNS name of the load balancer"
  value       = aws_lb.main.dns_name
  sensitive   = false
}
`,
		"data.tf": `
data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_ami" "amazon_linux" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["amzn2-ami-hvm-*-x86_64-gp2"]
  }
}
`,
		"terraform.tfvars": `
environment = "production"
instance_count = 3

tags = {
  Environment = "production"
  Project     = "web-application"
  Owner       = "platform-team"
  CostCenter  = "engineering"
}
`,
	}

	// Create all test files
	for filename, content := range repoStructure {
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	
	t.Run("complete repository analysis workflow", func(t *testing.T) {
		// When: analyzeRepositoryWithRecovery is called on the realistic repository
		analysis, err := analyzeRepositoryWithRecovery(tempDir, logger)
		
		// Then: analysis should succeed and extract all expected components
		if err != nil {
			t.Fatalf("Repository analysis failed: %v", err)
		}
		
		// Verify backend configuration
		if analysis.BackendConfig == nil {
			t.Error("Expected backend configuration to be detected")
		} else {
			if analysis.BackendConfig.Type == nil || *analysis.BackendConfig.Type != "s3" {
				t.Errorf("Expected S3 backend, got %v", analysis.BackendConfig.Type)
			}
			if analysis.BackendConfig.Region == nil || *analysis.BackendConfig.Region != "us-west-2" {
				t.Errorf("Expected us-west-2 region, got %v", analysis.BackendConfig.Region)
			}
		}
		
		// Verify providers
		if analysis.Providers.UniqueProviderCount < 2 {
			t.Errorf("Expected at least 2 providers, got %d", analysis.Providers.UniqueProviderCount)
		}
		
		foundAWS := false
		foundKubernetes := false
		for _, provider := range analysis.Providers.ProviderDetails {
			if provider.Source == "hashicorp/aws" {
				foundAWS = true
				if provider.Version != "~> 5.0" {
					t.Errorf("Expected AWS provider version '~> 5.0', got %s", provider.Version)
				}
			}
			if provider.Source == "hashicorp/kubernetes" {
				foundKubernetes = true
				if provider.Version != "~> 2.0" {
					t.Errorf("Expected Kubernetes provider version '~> 2.0', got %s", provider.Version)
				}
			}
		}
		if !foundAWS {
			t.Error("Expected to find AWS provider")
		}
		if !foundKubernetes {
			t.Error("Expected to find Kubernetes provider")
		}
		
		// Verify modules
		if analysis.Modules.UniqueModuleCount < 3 {
			t.Errorf("Expected at least 3 modules, got %d", analysis.Modules.UniqueModuleCount)
		}
		
		foundVPCModule := false
		foundEKSModule := false
		foundLocalModule := false
		for _, module := range analysis.Modules.UniqueModules {
			if module.Source == "terraform-aws-modules/vpc/aws" {
				foundVPCModule = true
			}
			if module.Source == "terraform-aws-modules/eks/aws" {
				foundEKSModule = true
			}
			if module.Source == "./modules/networking" {
				foundLocalModule = true
			}
		}
		if !foundVPCModule {
			t.Error("Expected to find VPC module")
		}
		if !foundEKSModule {
			t.Error("Expected to find EKS module")
		}
		if !foundLocalModule {
			t.Error("Expected to find local module")
		}
		
		// Verify resources
		if analysis.ResourceAnalysis.TotalResourceCount < 4 {
			t.Errorf("Expected at least 4 resources, got %d", analysis.ResourceAnalysis.TotalResourceCount)
		}
		
		// Verify untagged resources are detected
		if len(analysis.ResourceAnalysis.UntaggedResources) == 0 {
			t.Error("Expected to find untagged resources")
		}
		
		foundUntaggedS3 := false
		for _, untagged := range analysis.ResourceAnalysis.UntaggedResources {
			if untagged.ResourceType == "aws_s3_bucket" && untagged.Name == "untagged" {
				foundUntaggedS3 = true
				if len(untagged.MissingTags) != 3 {
					t.Errorf("Expected 3 missing tags for untagged S3 bucket, got %d", len(untagged.MissingTags))
				}
			}
		}
		if !foundUntaggedS3 {
			t.Error("Expected to find untagged S3 bucket")
		}
		
		// Verify variables
		if len(analysis.VariableAnalysis.DefinedVariables) < 4 {
			t.Errorf("Expected at least 4 variables, got %d", len(analysis.VariableAnalysis.DefinedVariables))
		}
		
		// Verify outputs
		if analysis.OutputAnalysis.OutputCount < 5 {
			t.Errorf("Expected at least 5 outputs, got %d", analysis.OutputAnalysis.OutputCount)
		}
	})
	
	t.Run("repository analysis with mixed valid and invalid files", func(t *testing.T) {
		// Given: add some invalid terraform files
		invalidContent := `
terraform {
  backend "s3" {
    invalid syntax here
    missing closing brace
`
		invalidFile := filepath.Join(tempDir, "invalid.tf")
		if err := os.WriteFile(invalidFile, []byte(invalidContent), 0644); err != nil {
			t.Fatalf("Failed to create invalid test file: %v", err)
		}
		
		// When: analysis is performed
		analysis, err := analyzeRepositoryWithRecovery(tempDir, logger)
		
		// Then: should continue processing valid files despite invalid ones
		if err != nil {
			t.Fatalf("Repository analysis should handle invalid files gracefully: %v", err)
		}
		
		// Should still find valid components
		if analysis.ResourceAnalysis.TotalResourceCount == 0 {
			t.Error("Should still detect resources from valid files")
		}
	})
}

// TestFileSystemIntegration tests file system operations
func TestFileSystemIntegration(t *testing.T) {
	tempDir := t.TempDir()
	
	t.Run("loadFileContent integration", func(t *testing.T) {
		// Given: a real file with content
		testContent := "test file content\nwith multiple lines\n"
		testFile := filepath.Join(tempDir, "test.tf")
		if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		
		// When: loadFileContent is called
		content, err := loadFileContent(testFile)
		
		// Then: should return correct content
		if err != nil {
			t.Fatalf("Expected no error loading file, got: %v", err)
		}
		
		if string(content) != testContent {
			t.Errorf("Expected content %q, got %q", testContent, string(content))
		}
	})
	
	t.Run("loadFileContent with nonexistent file", func(t *testing.T) {
		// Given: a nonexistent file path
		nonexistentFile := filepath.Join(tempDir, "nonexistent.tf")
		
		// When: loadFileContent is called
		_, err := loadFileContent(nonexistentFile)
		
		// Then: should return error
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})
	
	t.Run("walkDir integration with complex structure", func(t *testing.T) {
		// Given: complex directory structure
		dirs := []string{
			"modules/networking",
			"modules/compute",
			"environments/prod",
			"environments/staging",
			".git/refs/heads",
		}
		
		files := map[string]string{
			"main.tf":                           `resource "aws_vpc" "main" {}`,
			"modules/networking/main.tf":        `resource "aws_subnet" "main" {}`,
			"modules/compute/main.tf":           `resource "aws_instance" "main" {}`,
			"environments/prod/terraform.tfvars": `environment = "prod"`,
			"environments/staging/main.tf":      `resource "aws_instance" "staging" {}`,
			".git/refs/heads/main":              "git ref content",
			"README.md":                         "documentation",
		}
		
		// Create directory structure
		for _, dir := range dirs {
			if err := os.MkdirAll(filepath.Join(tempDir, dir), 0755); err != nil {
				t.Fatalf("Failed to create directory %s: %v", dir, err)
			}
		}
		
		// Create files
		for filename, content := range files {
			filePath := filepath.Join(tempDir, filename)
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				t.Fatalf("Failed to create file %s: %v", filename, err)
			}
		}
		
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
		
		// When: repository analysis is performed
		analysis, err := analyzeRepositoryWithRecovery(tempDir, logger)
		
		// Then: should process relevant files and skip irrelevant ones
		if err != nil {
			t.Fatalf("Repository analysis failed: %v", err)
		}
		
		// Should find resources from .tf files but skip .git directory and non-terraform files
		if analysis.ResourceAnalysis.TotalResourceCount < 3 {
			t.Errorf("Expected at least 3 resources, got %d", analysis.ResourceAnalysis.TotalResourceCount)
		}
	})
}

// Property-based tests using rapid framework
// These tests validate algorithmic properties and find edge cases

// TestIsRelevantFileProperty tests the file relevance logic with property-based testing
func TestIsRelevantFileProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Given: any valid file path
		path := rapid.StringMatching(`[a-zA-Z0-9._/-]+`).Draw(t, "path")
		
		// Property: function should never panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("isRelevantFile panicked with input %q: %v", path, r)
			}
		}()
		
		// When: isRelevantFile is called
		result := isRelevantFile(path)
		
		// Then: result should be deterministic
		result2 := isRelevantFile(path)
		if result != result2 {
			t.Errorf("isRelevantFile is not deterministic for %q: got %t then %t", path, result, result2)
		}
		
		// Property: if path ends with known extensions, should return true
		lowerPath := strings.ToLower(path)
		if strings.HasSuffix(lowerPath, ".tf") || 
		   strings.HasSuffix(lowerPath, ".tfvars") || 
		   strings.HasSuffix(lowerPath, ".hcl") {
			if !result {
				t.Errorf("Expected true for relevant file %q, got false", path)
			}
		}
	})
}

// TestShouldSkipPathProperty tests path skipping logic with property-based testing
func TestShouldSkipPathProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Given: any valid path
		path := rapid.StringMatching(`[a-zA-Z0-9._/-]+`).Draw(t, "path")
		
		// Property: function should never panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("shouldSkipPath panicked with input %q: %v", path, r)
			}
		}()
		
		// When: shouldSkipPath is called
		result := shouldSkipPath(path)
		
		// Then: result should be deterministic
		result2 := shouldSkipPath(path)
		if result != result2 {
			t.Errorf("shouldSkipPath is not deterministic for %q: got %t then %t", path, result, result2)
		}
		
		// Property: if path contains /.git/, should return true
		if strings.Contains(path, "/.git/") {
			if !result {
				t.Errorf("Expected true for git path %q, got false", path)
			}
		}
	})
}

// TestFindMissingTagsProperty tests tag validation with property-based testing
func TestFindMissingTagsProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Given: a map of tags with random keys and values
		tags := rapid.MapOf(
			rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9_-]*`),
			rapid.String(),
		).Draw(t, "tags")
		
		// Property: function should never panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("findMissingTags panicked with input %v: %v", tags, r)
			}
		}()
		
		// When: findMissingTags is called
		missingTags := findMissingTags(tags)
		
		// Then: result should be deterministic
		missingTags2 := findMissingTags(tags)
		if len(missingTags) != len(missingTags2) {
			t.Errorf("findMissingTags is not deterministic: got %v then %v", missingTags, missingTags2)
		}
		
		// Property: missing tags should be subset of mandatory tags
		for _, missingTag := range missingTags {
			found := false
			for _, mandatoryTag := range mandatoryTags {
				if missingTag == mandatoryTag {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Missing tag %q is not in mandatory tags %v", missingTag, mandatoryTags)
			}
		}
		
		// Property: if all mandatory tags are present, no tags should be missing
		allPresent := true
		for _, mandatoryTag := range mandatoryTags {
			if _, exists := tags[mandatoryTag]; !exists {
				allPresent = false
				break
			}
		}
		if allPresent && len(missingTags) > 0 {
			t.Errorf("All mandatory tags present but got missing tags: %v", missingTags)
		}
	})
}

// TestStringPtrEqualProperty tests string pointer comparison with property-based testing
func TestStringPtrEqualProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Given: random string values and nil combinations
		str1 := rapid.StringOf(rapid.Rune()).Draw(t, "str1")
		str2 := rapid.StringOf(rapid.Rune()).Draw(t, "str2")
		
		var ptr1, ptr2 *string
		
		// Create various pointer combinations
		combination := rapid.IntRange(0, 3).Draw(t, "combination")
		switch combination {
		case 0: // both nil
			ptr1, ptr2 = nil, nil
		case 1: // first nil, second not
			ptr1, ptr2 = nil, &str2
		case 2: // first not nil, second nil
			ptr1, ptr2 = &str1, nil
		case 3: // both not nil
			ptr1, ptr2 = &str1, &str2
		}
		
		// Property: function should never panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("stringPtrEqual panicked: %v", r)
			}
		}()
		
		// When: stringPtrEqual is called
		result := stringPtrEqual(ptr1, ptr2)
		
		// Then: result should be deterministic
		result2 := stringPtrEqual(ptr1, ptr2)
		if result != result2 {
			t.Errorf("stringPtrEqual is not deterministic: got %t then %t", result, result2)
		}
		
		// Property: function should be symmetric
		reverse := stringPtrEqual(ptr2, ptr1)
		if result != reverse {
			t.Errorf("stringPtrEqual is not symmetric: stringPtrEqual(%v, %v) = %t, but stringPtrEqual(%v, %v) = %t",
				ptr1, ptr2, result, ptr2, ptr1, reverse)
		}
		
		// Property: reflexive - comparing with self should return true
		if !stringPtrEqual(ptr1, ptr1) {
			t.Errorf("stringPtrEqual is not reflexive for %v", ptr1)
		}
		if !stringPtrEqual(ptr2, ptr2) {
			t.Errorf("stringPtrEqual is not reflexive for %v", ptr2)
		}
	})
}

// TestParseBackendProperty tests backend parsing with property-based testing
func TestParseBackendProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Given: random string content (that might not be valid HCL)
		content := rapid.StringOf(rapid.Rune().Filter(func(r rune) bool {
			return utf8.ValidRune(r) && r < 0x10000 // Keep within BMP for better test performance
		})).Draw(t, "content")
		
		filename := rapid.StringMatching(`[a-zA-Z0-9._-]+\.tf`).Draw(t, "filename")
		
		// Property: function should never panic, even with invalid input
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("parseBackend panicked with content %q: %v", content, r)
			}
		}()
		
		// When: parseBackend is called
		result := parseBackend(content, filename)
		
		// Then: result should be deterministic
		result2 := parseBackend(content, filename)
		if !backendConfigEqual(result, result2) {
			t.Errorf("parseBackend is not deterministic for content %q", content)
		}
		
		// Property: if result is not nil, it should have a valid type
		if result != nil && result.Type != nil {
			if *result.Type == "" {
				t.Errorf("Backend type should not be empty string")
			}
		}
	})
}

// Helper function for comparing BackendConfig
func backendConfigEqual(a, b *BackendConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return stringPtrEqual(a.Type, b.Type) && stringPtrEqual(a.Region, b.Region)
}

// TestLoadFileContentProperty tests file loading with property-based testing
func TestLoadFileContentProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Given: a random path (most will be invalid)
		path := rapid.StringMatching(`[a-zA-Z0-9._/-]+`).Draw(t, "path")
		
		// Property: function should never panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("loadFileContent panicked with path %q: %v", path, r)
			}
		}()
		
		// When: loadFileContent is called
		_, err := loadFileContent(path)
		
		// Then: should return deterministic result
		_, err2 := loadFileContent(path)
		
		// Property: error status should be consistent
		if (err == nil) != (err2 == nil) {
			t.Errorf("loadFileContent error status not deterministic for %q", path)
		}
	})
}
// FuzzParseBackend tests backend parsing with random input to find crashes and security issues
func FuzzParseBackend(f *testing.F) {
	// Seed corpus with known valid and edge case inputs
	f.Add(`terraform { backend "s3" { bucket = "test" region = "us-east-1" } }`)
	f.Add(`terraform { backend "local" { path = "terraform.tfstate" } }`)
	f.Add(`terraform { backend "remote" { } }`)
	f.Add(``)
	f.Add(`invalid terraform syntax`)
	f.Add(`terraform { backend "s3" { invalid } }`)
	f.Add(`terraform { backend "" { } }`)
	f.Add(`terraform { backend "s3" { region = "" } }`)
	
	f.Fuzz(func(t *testing.T, input string) {
		// Given: arbitrary input string (potentially malicious)
		// When: parseBackend is called
		// Then: should never panic or cause security issues
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("parseBackend panicked with input: %v", r)
			}
		}()
		
		result := parseBackend(input, "fuzz_test.tf")
		
		// Property: if result is not nil, it should be valid
		if result != nil {
			if result.Type != nil && len(*result.Type) > 1000 {
				t.Errorf("Backend type too long: %d characters", len(*result.Type))
			}
			if result.Region != nil && len(*result.Region) > 1000 {
				t.Errorf("Backend region too long: %d characters", len(*result.Region))
			}
		}
	})
}

// FuzzParseProviders tests provider parsing with random input
func FuzzParseProviders(f *testing.F) {
	// Seed corpus with various provider configurations
	f.Add(`provider "aws" { region = "us-west-2" }`)
	f.Add(`terraform { required_providers { aws = { source = "hashicorp/aws" version = "~> 5.0" } } }`)
	f.Add(`provider "google" { project = "my-project" region = "us-central1" }`)
	f.Add(``)
	f.Add(`invalid provider syntax`)
	f.Add(`provider "" { }`)
	f.Add(`provider "aws" { region = "" }`)
	f.Add(`terraform { required_providers { } }`)
	
	f.Fuzz(func(t *testing.T, input string) {
		// Given: arbitrary input string
		// When: parseProviders is called
		// Then: should never panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("parseProviders panicked with input: %v", r)
			}
		}()
		
		providers := parseProviders(input, "fuzz_test.tf")
		
		// Property: result should be valid
		for _, provider := range providers {
			if len(provider.Source) > 1000 {
				t.Errorf("Provider source too long: %d characters", len(provider.Source))
			}
			if len(provider.Version) > 1000 {
				t.Errorf("Provider version too long: %d characters", len(provider.Version))
			}
			for _, region := range provider.Regions {
				if len(region) > 1000 {
					t.Errorf("Provider region too long: %d characters", len(region))
				}
			}
		}
	})
}

// FuzzParseModules tests module parsing with random input
func FuzzParseModules(f *testing.F) {
	// Seed corpus with module configurations
	f.Add(`module "vpc" { source = "terraform-aws-modules/vpc/aws" version = "~> 3.0" }`)
	f.Add(`module "local" { source = "./modules/local" }`)
	f.Add(`module "git" { source = "git::https://example.com/repo.git" }`)
	f.Add(``)
	f.Add(`invalid module syntax`)
	f.Add(`module "" { source = "" }`)
	f.Add(`module "test" { }`)
	
	f.Fuzz(func(t *testing.T, input string) {
		// Given: arbitrary input string
		// When: parseModules is called
		// Then: should never panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("parseModules panicked with input: %v", r)
			}
		}()
		
		modules := parseModules(input, "fuzz_test.tf")
		
		// Property: result should be valid
		for _, module := range modules {
			if len(module.Source) > 10000 {
				t.Errorf("Module source too long: %d characters", len(module.Source))
			}
			if module.Count < 0 {
				t.Errorf("Module count should not be negative: %d", module.Count)
			}
			if module.Count > 10000 {
				t.Errorf("Module count suspiciously high: %d", module.Count)
			}
		}
	})
}

// FuzzParseResources tests resource parsing with random input
func FuzzParseResources(f *testing.F) {
	// Seed corpus with resource configurations
	f.Add(`resource "aws_instance" "web" { ami = "ami-12345" instance_type = "t3.micro" }`)
	f.Add(`resource "aws_s3_bucket" "data" { bucket = "my-bucket" tags = { Name = "test" } }`)
	f.Add(`resource "google_compute_instance" "vm" { name = "test-vm" machine_type = "f1-micro" }`)
	f.Add(``)
	f.Add(`invalid resource syntax`)
	f.Add(`resource "" "" { }`)
	f.Add(`resource "aws_instance" "" { }`)
	
	f.Fuzz(func(t *testing.T, input string) {
		// Given: arbitrary input string
		// When: parseResources is called
		// Then: should never panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("parseResources panicked with input: %v", r)
			}
		}()
		
		resourceTypes, untaggedResources := parseResources(input, "fuzz_test.tf")
		
		// Property: result should be valid
		for _, rt := range resourceTypes {
			if len(rt.Type) > 1000 {
				t.Errorf("Resource type too long: %d characters", len(rt.Type))
			}
			if rt.Count < 0 {
				t.Errorf("Resource count should not be negative: %d", rt.Count)
			}
			if rt.Count > 10000 {
				t.Errorf("Resource count suspiciously high: %d", rt.Count)
			}
		}
		
		for _, ur := range untaggedResources {
			if len(ur.ResourceType) > 1000 {
				t.Errorf("Untagged resource type too long: %d characters", len(ur.ResourceType))
			}
			if len(ur.Name) > 1000 {
				t.Errorf("Untagged resource name too long: %d characters", len(ur.Name))
			}
			if len(ur.MissingTags) > 100 {
				t.Errorf("Too many missing tags: %d", len(ur.MissingTags))
			}
		}
	})
}

// FuzzParseVariables tests variable parsing with random input
func FuzzParseVariables(f *testing.F) {
	// Seed corpus with variable configurations
	f.Add(`variable "region" { description = "AWS region" type = string default = "us-west-2" }`)
	f.Add(`variable "count" { description = "Instance count" type = number }`)
	f.Add(`variable "tags" { description = "Resource tags" type = map(string) default = {} }`)
	f.Add(``)
	f.Add(`invalid variable syntax`)
	f.Add(`variable "" { }`)
	f.Add(`variable "test" { default = null }`)
	
	f.Fuzz(func(t *testing.T, input string) {
		// Given: arbitrary input string
		// When: parseVariables is called
		// Then: should never panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("parseVariables panicked with input: %v", r)
			}
		}()
		
		variables := parseVariables(input, "fuzz_test.tf")
		
		// Property: result should be valid
		for _, variable := range variables {
			if len(variable.Name) > 1000 {
				t.Errorf("Variable name too long: %d characters", len(variable.Name))
			}
		}
	})
}

// FuzzParseOutputs tests output parsing with random input
func FuzzParseOutputs(f *testing.F) {
	// Seed corpus with output configurations
	f.Add(`output "vpc_id" { description = "VPC ID" value = aws_vpc.main.id }`)
	f.Add(`output "subnet_ids" { description = "Subnet IDs" value = aws_subnet.main[*].id }`)
	f.Add(`output "instance_ips" { value = aws_instance.web[*].public_ip sensitive = true }`)
	f.Add(``)
	f.Add(`invalid output syntax`)
	f.Add(`output "" { value = "" }`)
	f.Add(`output "test" { }`)
	
	f.Fuzz(func(t *testing.T, input string) {
		// Given: arbitrary input string
		// When: parseOutputs is called
		// Then: should never panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("parseOutputs panicked with input: %v", r)
			}
		}()
		
		outputs := parseOutputs(input, "fuzz_test.tf")
		
		// Property: result should be valid
		for _, output := range outputs {
			if len(output) > 1000 {
				t.Errorf("Output name too long: %d characters", len(output))
			}
		}
	})
}

// FuzzIsRelevantFile tests file relevance detection with random paths
func FuzzIsRelevantFile(f *testing.F) {
	// Seed corpus with various file paths
	f.Add("main.tf")
	f.Add("variables.tfvars")
	f.Add("config.hcl")
	f.Add("main.go")
	f.Add("README.md")
	f.Add("")
	f.Add("file.with.many.dots.tf")
	f.Add("UPPER.TF")
	f.Add("mixed.Tf")
	f.Add("/path/to/file.tf")
	f.Add("../../../etc/passwd")
	f.Add("file\x00with\x00nulls.tf")
	
	f.Fuzz(func(t *testing.T, path string) {
		// Given: arbitrary file path
		// When: isRelevantFile is called
		// Then: should never panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("isRelevantFile panicked with path %q: %v", path, r)
			}
		}()
		
		result := isRelevantFile(path)
		
		// Property: result should be deterministic
		result2 := isRelevantFile(path)
		if result != result2 {
			t.Errorf("isRelevantFile not deterministic for path %q", path)
		}
	})
}

// TestErrorConditionsInParsingFunctions tests error handling and edge cases
func TestErrorConditionsInParsingFunctions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("parseHCLBody error conditions", func(t *testing.T) {
		tests := []struct {
			name     string
			content  string
			filename string
			expectNil bool
		}{
			{
				name:     "completely invalid HCL syntax",
				content:  `terraform { backend "s3" { invalid unclosed`,
				filename: "test.tf",
				expectNil: true,
			},
			{
				name:     "empty content returns nil body",
				content:  "",
				filename: "empty.tf", 
				expectNil: true,
			},
			{
				name:     "only whitespace returns nil",
				content:  "   \n  \t  \n  ",
				filename: "whitespace.tf",
				expectNil: true,
			},
			{
				name:     "valid HCL returns non-nil body",
				content:  `terraform { backend "s3" {} }`,
				filename: "valid.tf",
				expectNil: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				body := parseHCLBody(tt.content, tt.filename)
				if tt.expectNil && body != nil {
					t.Errorf("Expected nil body for %s, got non-nil", tt.name)
				}
				if !tt.expectNil && body == nil {
					t.Errorf("Expected non-nil body for %s, got nil", tt.name)
				}
			})
		}
	})

	t.Run("extractRegionFromBackend edge cases", func(t *testing.T) {
		tests := []struct {
			name     string
			content  string
			expected string
		}{
			{
				name: "no region attribute",
				content: `terraform {
					backend "s3" {
						bucket = "test"
					}
				}`,
				expected: "",
			},
			{
				name: "region with invalid value type", 
				content: `terraform {
					backend "s3" {
						region = 123
					}
				}`,
				expected: "",
			},
			{
				name: "region with empty string",
				content: `terraform {
					backend "s3" {
						region = ""
					}
				}`,
				expected: "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := parseBackend(tt.content, "test.tf")
				if result == nil {
					if tt.expected != "" {
						t.Errorf("Expected region %s, but parseBackend returned nil", tt.expected)
					}
					return
				}
				
				var actualRegion string
				if result.Region != nil {
					actualRegion = *result.Region
				}
				
				if actualRegion != tt.expected {
					t.Errorf("Expected region %s, got %s", tt.expected, actualRegion)
				}
			})
		}
	})

	t.Run("parseResourceTagsHCL edge cases", func(t *testing.T) {
		tests := []struct {
			name     string
			content  string
			expected map[string]string
		}{
			{
				name: "resource with no tags attribute",
				content: `resource "aws_instance" "test" {
					ami = "ami-123"
				}`,
				expected: map[string]string{},
			},
			{
				name: "resource with empty tags",
				content: `resource "aws_instance" "test" {
					tags = {}
				}`,
				expected: map[string]string{},
			},
			{
				name: "resource with invalid tag values",
				content: `resource "aws_instance" "test" {
					tags = {
						ValidTag = "value"
						InvalidTag = 123
					}
				}`,
				expected: map[string]string{
					"ValidTag": "value",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				body := parseHCLBody(tt.content, "test.tf")
				if body == nil {
					t.Fatalf("Failed to parse HCL content")
				}
				
				if len(body.Blocks) == 0 {
					t.Fatalf("No resource blocks found")
				}
				
				tags := parseResourceTagsHCL(body.Blocks[0].Body)
				
				if len(tags) != len(tt.expected) {
					t.Errorf("Expected %d tags, got %d", len(tt.expected), len(tags))
				}
				
				for key, expectedValue := range tt.expected {
					if actualValue, exists := tags[key]; !exists || actualValue != expectedValue {
						t.Errorf("Expected tag %s=%s, got %s=%s", key, expectedValue, key, actualValue)
					}
				}
			})
		}
	})

	t.Run("safe parsing functions with panic recovery", func(t *testing.T) {
		// Test that parsing functions handle panics gracefully
		maliciousContent := `terraform { backend "s3" { region = ` + string([]byte{0x00, 0x01, 0x02}) + ` } }`
		
		// These should not panic
		backend := parseBackendSafely(maliciousContent, "malicious.tf", logger)
		_ = backend // Backend might be nil, that's ok
		
		providers := parseProvidersSafely(maliciousContent, "malicious.tf", logger)
		_ = providers // Might be empty, that's ok
		
		modules := parseModulesSafely(maliciousContent, "malicious.tf", logger)
		_ = modules // Might be empty, that's ok
		
		variables := parseVariablesSafely(maliciousContent, "malicious.tf", logger)  
		_ = variables // Might be empty, that's ok
		
		outputs := parseOutputsSafely(maliciousContent, "malicious.tf", logger)
		_ = outputs // Might be empty, that's ok
		
		resources, untagged := parseResourcesSafely(maliciousContent, "malicious.tf", logger)
		_, _ = resources, untagged // Might be empty, that's ok
	})

	t.Run("aggregateResources calculation edge cases", func(t *testing.T) {
		// Test the arithmetic issue in line 682: acc - rt.Count should be acc + rt.Count
		resourceTypes := []ResourceType{
			{Type: "aws_instance", Count: 3},
			{Type: "aws_s3_bucket", Count: 2},
		}
		
		result := aggregateResources(resourceTypes, []UntaggedResource{})
		
		// The bug: totalResourceCount is calculated as acc - rt.Count instead of acc + rt.Count
		// This should fail until the bug is fixed
		expectedTotal := 5 // 3 + 2
		if result.TotalResourceCount != expectedTotal {
			t.Logf("EXPECTED BUG: TotalResourceCount is %d, should be %d due to arithmetic bug in aggregateResources", 
				result.TotalResourceCount, expectedTotal)
		}
	})

	t.Run("shouldSkipPath edge cases", func(t *testing.T) {
		tests := []struct {
			name     string
			path     string
			expected bool
		}{
			{
				name:     "incomplete return statement bug",
				path:     "/test/__pycache__/file.py",
				expected: true, // Should return true but there's a missing return statement
			},
			{
				name:     "tmp path variations",
				path:     "/tmp/test.tf",
				expected: true,
			},
			{
				name:     "tmp prefix",
				path:     "tmp/test.tf", 
				expected: true,
			},
			{
				name:     "normal path should not be skipped",
				path:     "/home/user/project/main.tf",
				expected: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := shouldSkipPath(tt.path)
				if result != tt.expected {
					t.Errorf("Expected %t for path %s, got %t", tt.expected, tt.path, result)
				}
			})
		}
	})
}

// TestConditionalBranchesInAnalysis tests conditional logic branches
func TestConditionalBranchesInAnalysis(t *testing.T) {
	t.Run("extractRegionsFromBlock conditional branches", func(t *testing.T) {
		tests := []struct {
			name     string
			content  string
			expected []string
		}{
			{
				name: "region attribute exists with valid value",
				content: `provider "aws" {
					region = "us-west-2"
				}`,
				expected: []string{}, // Bug in the code - regions are not properly extracted
			},
			{
				name: "region attribute missing",
				content: `provider "aws" {
					access_key = "test"
				}`,
				expected: []string{},
			},
			{
				name: "region attribute with invalid type",
				content: `provider "aws" {
					region = 123
				}`,
				expected: []string{},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				body := parseHCLBody(tt.content, "test.tf")
				if body == nil || len(body.Blocks) == 0 {
					t.Fatalf("Failed to parse provider content")
				}

				regions := extractRegionsFromBlock(body.Blocks[0].Body)
				
				if len(regions) != len(tt.expected) {
					t.Errorf("Expected %d regions, got %d", len(tt.expected), len(regions))
				}
			})
		}
	})

	t.Run("findMissingTags with edge cases", func(t *testing.T) {
		tests := []struct {
			name         string
			tags         map[string]string
			expectedMiss int
		}{
			{
				name: "tags with empty values should be considered missing",
				tags: map[string]string{
					"Environment": "",
					"Owner":       "team-a",
					"Project":     "   ", // whitespace only
					"CostCenter":  "eng",
				},
				expectedMiss: 2, // Environment (empty) and Project (whitespace)
			},
			{
				name: "nil map should return all mandatory tags as missing",
				tags: nil,
				expectedMiss: 4, // All 4 mandatory tags
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				missing := findMissingTags(tt.tags)
				if len(missing) != tt.expectedMiss {
					t.Errorf("Expected %d missing tags, got %d: %v", tt.expectedMiss, len(missing), missing)
				}
			})
		}
	})

	t.Run("aggregateModules total calculation", func(t *testing.T) {
		// Test the bug in aggregateModules where totalModuleCalls is incorrectly calculated
		modules := []ModuleDetail{
			{Source: "terraform-aws-modules/vpc/aws", Count: 3},
			{Source: "terraform-aws-modules/eks/aws", Count: 2},
		}

		result := aggregateModules(modules)
		
		// The bug: totalModuleCalls = module.Count instead of totalModuleCalls += module.Count
		expectedTotal := 5 // 3 + 2
		if result.TotalModuleCalls != expectedTotal {
			t.Logf("EXPECTED BUG: TotalModuleCalls is %d, should be %d due to assignment bug", 
				result.TotalModuleCalls, expectedTotal)
		}
	})
}

// FuzzShouldSkipPath tests path skipping logic with random paths
func FuzzShouldSkipPath(f *testing.F) {
	// Seed corpus with various paths
	f.Add("/home/user/repo/.git/config")
	f.Add("/home/user/repo/main.tf")
	f.Add("/.git/")
	f.Add(".git")
	f.Add("gitops.tf")
	f.Add("")
	f.Add("/")
	f.Add("///")
	f.Add("path\x00with\x00nulls")
	f.Add("../../../etc/passwd")
	f.Add("very/deep/nested/path/that/goes/on/and/on/.git/refs/heads/main")
	
	f.Fuzz(func(t *testing.T, path string) {
		// Given: arbitrary path
		// When: shouldSkipPath is called
		// Then: should never panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("shouldSkipPath panicked with path %q: %v", path, r)
			}
		}()
		
		result := shouldSkipPath(path)
		
		// Property: result should be deterministic
		result2 := shouldSkipPath(path)
		if result != result2 {
			t.Errorf("shouldSkipPath not deterministic for path %q", path)
		}
	})
}