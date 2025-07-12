package main

import (
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
			name:     "no backend",
			content:  `resource "aws_instance" "example" {}`,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseBackend(tt.content)
			
			if tt.expected == nil && result != nil {
				t.Errorf("Expected nil, got %+v", result)
				return
			}
			
			if tt.expected != nil && result == nil {
				t.Errorf("Expected %+v, got nil", tt.expected)
				return
			}
			
			if tt.expected != nil && result != nil {
				if !stringPtrEqual(tt.expected.Type, result.Type) {
					t.Errorf("Expected type %v, got %v", tt.expected.Type, result.Type)
				}
				if !stringPtrEqual(tt.expected.Region, result.Region) {
					t.Errorf("Expected region %v, got %v", tt.expected.Region, result.Region)
				}
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
	
	providers := parseProviders(content)
	
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
	
	modules := parseModules(content)
	
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
	
	variables := parseVariables(content)
	
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
	
	outputs := parseOutputs(content)
	
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
	
	resourceTypes, untaggedResources := parseResources(content)
	
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