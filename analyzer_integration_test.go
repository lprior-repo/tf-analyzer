package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

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