# Tiered Mutation Testing Strategy

This document outlines the comprehensive tiered mutation testing strategy implemented for tf-analyzer, designed to provide fast feedback for developers while maintaining high code quality standards.

## Overview

The mutation testing strategy follows a **tiered performance-optimized approach** that addresses the key challenges:

- ⚡ **Speed**: Different tiers for different use cases (15s to 180s)
- 🎯 **Focus**: Target critical business logic first
- 🚀 **CI/CD Ready**: Optimized configurations for automated pipelines
- 🔄 **Developer Friendly**: Quick feedback loops for development

## Configuration Files

### 1. `.go-mutesting-quick.yml` - Development Feedback (<15s)
```yaml
# Ultra-fast mutation testing for development iteration
- Target: analyzer.go only
- Mutators: 3 essential (conditional-negation, boolean-inversion, binary-operator)
- Threshold: 70% (development feedback)
- Max mutations: 10
- Use Case: Quick validation after code changes
```

### 2. `.go-mutesting-tier1.yml` - Critical Business Logic (<60s)
```yaml
# Comprehensive testing of critical components
- Target: analyzer.go, orchestrator.go, cmd.go
- Mutators: 6 comprehensive mutators
- Threshold: 95% (high quality for critical code)
- Max mutations: 50
- Use Case: Pull request validation, critical path testing
```

### 3. `.go-mutesting-tier2.yml` - Full Codebase (<180s)
```yaml
# Complete mutation testing across all files
- Target: All application files
- Mutators: 8 complete mutator set
- Threshold: 85% (balanced for full coverage)
- Use Case: Release validation, comprehensive quality gates
```

### 4. `.go-mutesting-ci.yml` - CI/CD Optimized (<45s)
```yaml
# Optimized for automated pipeline execution
- Target: analyzer.go, orchestrator.go (core logic)
- Mutators: 4 essential mutators
- Threshold: 90% (high quality for CI gates)
- Max mutations: 30
- Use Case: Automated quality gates, CI/CD pipelines
```

## Taskfile Commands

### Core Mutation Testing Tasks

| Command | Purpose | Duration | Files Tested |
|---------|---------|----------|--------------|
| `task mutation-quick` | Development feedback | <15s | analyzer.go |
| `task mutation-tier1` | Critical business logic | <60s | analyzer.go, orchestrator.go, cmd.go |
| `task mutation-tier2` | Full codebase analysis | <180s | All application files |
| `task mutation-ci` | CI/CD optimization | <45s | analyzer.go, orchestrator.go |

### Workflow-Specific Tasks

| Command | Use Case | Description |
|---------|----------|-------------|
| `task mutation-dev` | Development | Quick test + mutation-quick |
| `task mutation-pr` | Pull Request | Full test + mutation-tier1 |
| `task mutation-release` | Release | Race test + coverage + mutation-tier2 |
| `task mutation-file -- file.go` | Single file | Targeted mutation testing |

### CI/CD Integration Tasks

| Command | Pipeline Stage | Description |
|---------|----------------|-------------|
| `task ci-dev` | Development CI | Lint + short tests + quick mutation |
| `task ci-mutation` | Standard CI | Lint + tests + CI-optimized mutation |
| `task ci-full` | Release CI | Lint + race tests + coverage + tier1 mutation |

## File Prioritization Strategy

### Tier 1: Critical Business Logic (95% threshold)
- **analyzer.go** - Core Terraform parsing and analysis
- **orchestrator.go** - Workflow coordination and error handling
- **cmd.go** - CLI interface and configuration

### Tier 2: Supporting Logic (85% threshold)
- **cloner.go** - Repository management
- **reporter.go** - Report generation
- **markdown.go** - Markdown processing

## Performance Optimizations

### Quick Configuration (15s target)
- Single worker process
- 3 essential mutators only
- Maximum 10 mutations
- Short timeout (5s per test)
- Development feedback focus

### Tier 1 Configuration (60s target)
- 2 parallel workers
- 6 comprehensive mutators
- Maximum 50 mutations
- Moderate timeout (30s per test)
- Critical path coverage

### CI Configuration (45s target)
- 2 optimized workers
- 4 essential mutators
- Maximum 30 mutations
- Short timeout (20s per test)
- Automated pipeline focus

## Usage Examples

### Development Workflow
```bash
# After making code changes
task mutation-dev
# Output: Quick feedback in <30s total
```

### Pull Request Validation
```bash
# Before submitting PR
task mutation-pr
# Output: Comprehensive validation in <90s total
```

### Release Preparation
```bash
# Before release
task mutation-release  
# Output: Full quality validation in <300s total
```

### CI/CD Pipeline Integration
```bash
# In CI pipeline
task ci-mutation
# Output: Automated quality gate in <60s total
```

## Mutation Strategies

### Critical Function Patterns
```yaml
critical_functions:
  - "parseHCL"
  - "extractProviders" 
  - "extractResources"
  - "parseBackend"
  - "validateTerraform"
```

### Error Handling Patterns
```yaml
error_patterns:
  - "*Error"
  - "handleErr*"
  - "validateErr*"
  - "processErr*"
```

### Performance-Critical Paths
```yaml
performance_paths:
  - "concurrent_processing"
  - "file_operations" 
  - "data_transformation"
```

## Quality Gates

### Development Quality Gate
- Quick mutation score: ≥70%
- Focus: Rapid feedback for code changes
- Action: Continue development with awareness

### Pull Request Quality Gate
- Tier 1 mutation score: ≥95%
- Focus: Critical business logic validation
- Action: Block PR merge if failing

### Release Quality Gate
- Tier 2 mutation score: ≥85%
- Focus: Comprehensive codebase validation
- Action: Block release if failing

### CI/CD Quality Gate
- CI mutation score: ≥90%
- Focus: Automated quality assurance
- Action: Fail build if not meeting standards

## Integration with Existing Quality Standards

### Test Coverage Integration
```bash
# Combined coverage and mutation testing
task mutation-report
# Generates: Coverage report + Tier 1 mutation analysis
```

### Linting Integration
```bash
# Full quality pipeline
task ci-full
# Runs: golangci-lint + race tests + coverage + mutation testing
```

### Security Integration
```bash
# Security + mutation testing
task sec && task mutation-tier1
# Validates: Security issues + logic correctness
```

## Troubleshooting

### Performance Issues
- **Too slow**: Use `mutation-quick` for development
- **Timeouts**: Check file-specific configurations
- **High CPU**: Reduce `max_workers` in config

### Quality Issues  
- **Low scores**: Focus on error handling and edge cases
- **Failing mutations**: Review test coverage for specific functions
- **CI failures**: Use `mutation-ci` for pipeline optimization

### Configuration Issues
- **YAML errors**: Validate syntax with `task --list`
- **Tool missing**: Run `task mutation-install`
- **Test failures**: Verify with `task test` first

## Benefits Achieved

### Performance Benefits
- ✅ **15-second feedback** for development iteration
- ✅ **45-second CI validation** for automated pipelines  
- ✅ **60-second critical path** validation for PRs
- ✅ **180-second comprehensive** analysis for releases

### Quality Benefits
- ✅ **95% mutation score** for critical business logic
- ✅ **90% mutation score** for CI quality gates
- ✅ **85% mutation score** for full codebase coverage
- ✅ **Tiered approach** matching development workflow

### Developer Benefits
- ✅ **Fast feedback loops** during development
- ✅ **Targeted testing** for different scenarios
- ✅ **Clear quality gates** for different stages
- ✅ **Easy integration** with existing tools

This tiered mutation testing strategy provides a practical, performance-optimized approach to maintaining high code quality while supporting efficient development workflows.