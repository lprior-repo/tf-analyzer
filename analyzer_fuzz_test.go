package main

import (
	"testing"
)

// Fuzz tests for input parsing functions
// These tests discover security vulnerabilities and robustness issues

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