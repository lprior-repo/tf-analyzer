# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

tf-analyzer is a high-performance Go application designed to concurrently clone multiple GitHub organizations and analyze their files in real-time. The program focuses on parsing markdown, HCL, Terraform (.tf), and tfvars files with maximum speed while maintaining extremely DRY and simple code following functional programming principles.

## Core Architecture Principles

- **Simplicity**: Code must be instantly clear - readable by an intern with minimal explanation
- **Pure Functions**: Use focused functions with no side effects for predictability
- **Modularity**: Design loosely coupled, composable components
- **Domain-based Structure**: Each file has a specific purpose, avoiding folders when possible
- **90%+ Test Coverage**: All code must be thoroughly tested (currently achieved 90%+ coverage)
- **Zero Linting Issues**: All code passes golangci-lint with strict error checking (errcheck + staticcheck)
- **AI Compatibility**: Structure code to be easily parsed and enhanced by AI tools

## Code Standards

### Function Guidelines

- Limit functions to 10-25 lines, performing one task only
- Cap cyclomatic complexity at 8 (enforced by golangci-lint)
- Limit nesting to 2 levels maximum
- Restrict to 3 parameters max (use objects for additional data)
- Use verb+noun naming (e.g., `calculateTotal`)

### File Organization

- Limit files to 300 lines for navigability
- Export minimally - only expose essentials
- Use descriptive variable names (e.g., `userProfiles`)
- Avoid abbreviations unless universal

### Quality Requirements

- Use assertive programming throughout the codebase
- Enforce type safety to catch errors early
- Validate inputs and handle errors safely
- Follow CUPID principles: Composable, Unix-like, Predictable, Idiomatic, Domain-based
- **Zero tolerance for linting issues**: All errcheck and staticcheck issues must be resolved
- **Comprehensive error handling**: All function return values must be properly checked

## Development Commands

### Essential Commands

- `task build` - Build the application
- `task test` - Run all tests
- `task lint` - Run golangci-lint with complexity checks
- `task run` - Run the application
- `task ci` - Run full CI pipeline (lint + test)

### Development Setup

- `task init` - Initialize project and install tools
- `task install-tools` - Install golangci-lint and development tools
- `task deps` - Download dependencies

### Code Quality

- `task lint-fix` - Auto-fix linting issues
- `task coverage` - Generate test coverage report (target: 90%+)
- `task sec` - Run security checks

### Performance

- `task bench` - Run benchmarks
- `task profile-cpu` - CPU profiling
- `task profile-mem` - Memory profiling

## Project Configuration

### Environment Variables (.env)

- `GITHUB_TOKEN` - GitHub API token for repository access
- `MAX_CONCURRENT_CLONES` - Concurrent repository cloning limit
- `MAX_CONCURRENT_ANALYZERS` - Concurrent file analysis limit
- `ANALYZE_EXTENSIONS` - File types to analyze (.md,.tf,.tfvars,.hcl)
- `GHORG_RECLONE_CONFIG` - Path to ghorg reclone configuration file (default: ~/.config/ghorg/reclone.yaml)
- `GHORG_CRON_TIMER_MINUTES` - Interval for automated reclone operations (default: 60 minutes)
- `GHORG_BASE_DIRECTORY` - Base directory for cloned repositories
- `GHORG_SCM_TYPE` - Source control management type (github, gitlab, gitea, etc.)

### Code Quality Enforcement

- golangci-lint configured with cyclomatic complexity limit of 8
- Function length limited to 25 lines
- Maximum 3 parameters per function
- Comprehensive linting rules for performance and security
- **Zero linting issues policy**: All errcheck and staticcheck violations resolved
- **90%+ test coverage achieved** with comprehensive unit, integration, and property-based tests

### Test Organization

- **One test file per source file**: Each `xyz.go` has corresponding `xyz_test.go`
- **Comprehensive test types**: Unit tests, integration tests, property-based tests, and fuzz tests
- **Test consolidation**: All test types (critical, additional, integration) consolidated into single files
- **Error path coverage**: All error handling paths tested with proper cleanup

The project emphasizes avoiding OOP patterns in favor of simple, stateless functions and values adaptability and easy refactoring for dynamic development workflows.

## Functional Programming in Go

Modern Go development often relies on the `github.com/samber/lo` library for common functional patterns like `Map`, `Filter`, and `Reduce`.

For system operations, file handling, and command execution, this project uses the `github.com/bitfield/script` library, which provides a shell-like pipeline API for Go programs.

### Architectural Pattern: Data, Calculations, and Actions

Structure functional programs using the **"Functional Core, Imperative Shell"** pattern:

1. **Data**: Inert data structures (structs, basic types) that hold information without behavior
2. **Calculations**: Pure functions containing business logic that transform data
3. **Actions**: Impure functions that interact with the outside world and orchestrate workflow

### Pure Functions

Core principle: Functions must be deterministic (same inputs → same outputs) and have no side effects (no I/O, global state modification, or external dependencies).

### Core Functional Utilities

#### Filter: Selecting Data

```go
longWords := lo.Filter(words, func(s string, _ int) bool {
    return len(s) > 6
})
```

#### Map: Transforming Data

```go
names := lo.Map(users, func(u User, _ int) string {
    return strings.ToUpper(u.Name)
})
```

#### Reduce: Aggregating Data

```go
total := lo.Reduce(items, func(acc float64, item OrderItem, _ int) float64 {
    return acc + (item.Price * float64(item.Quantity))
}, 0.0)
```

### Advanced Patterns

- **Function Composition**: Combining functions where output of one becomes input of next
- **Closures**: Functions that remember their creation environment for encapsulated state
- **Partial Application**: Creating specialized functions by pre-filling arguments

### Handling Impurity

Contain side effects by pushing impure operations to program edges. Create wrapper Actions that perform impure work and pass clean data to the pure core.

### Workflow Pattern

1. Actions get data from external world
2. Pass data to pure Calculations
3. Use results in final Actions for I/O operations

## ghorg Integration

This project integrates with [ghorg](https://github.com/gabrie30/ghorg) for efficient repository cloning and management.

### ghorg reclone Command

The `ghorg reclone` command provides centralized configuration management for multiple repository cloning operations:

#### Key Features

- **Configuration-driven**: Uses `reclone.yaml` stored in `$HOME/.config/ghorg`
- **Batch operations**: Clone multiple organizations/repositories with a single command
- **Post-execution scripts**: Run custom scripts after successful/failed clones
- **Selective execution**: Target specific reclone configurations by name

#### Usage Examples

```bash
# Clone all configured entries
ghorg reclone

# Clone specific entries only
ghorg reclone kubernetes-sig-staging kubernetes-sig

# List all configured commands
ghorg reclone --list

# Start HTTP server for remote triggering
ghorg reclone-server

# Set up automated cloning with cron
ghorg reclone-cron
```

#### Configuration Format

```yaml
# ~/.config/ghorg/reclone.yaml
gitlab-examples:
  cmd: "ghorg clone gitlab-examples --scm=gitlab --token=XXXXXXX"
  description: "Clone GitLab example repositories"
  post_exec_script: "/path/to/notify.sh"

kubernetes-sig:
  cmd: "ghorg clone kubernetes-sigs --scm=github --token=$GITHUB_TOKEN"
  description: "Clone Kubernetes SIG repositories"
```

#### Integration Benefits

- **Consistency**: Standardized cloning across different SCM providers
- **Automation**: Scheduled repository updates via cron integration
- **Monitoring**: Post-execution scripts for logging and notifications
- **Flexibility**: Support for GitHub, GitLab, Gitea, and other SCM platforms

## Required Dependencies

This project requires the following Go libraries:

### Core Libraries

- `github.com/samber/lo` - Functional programming utilities (Map, Filter, Reduce)
- `github.com/bitfield/script` - Shell-like operations and file handling
- `github.com/panjf2000/ants/v2` - High-performance goroutine pool
- `github.com/sourcegraph/conc` - Structured concurrency utilities
- `github.com/joho/godotenv` - Environment variable loading

### Script Library Usage

The `script` library replaces standard library operations for:

- File reading/writing: `script.File(path).Bytes()` instead of `os.ReadFile`
- Command execution: `script.Exec(cmd)` instead of `exec.Command`
- Text processing: Built-in pipeline operations like `Match`, `Filter`, `Replace`
- HTTP requests: `script.Get(url)` and `script.Post(url)` for web operations

### Installation

```bash
go get github.com/bitfield/script
go get github.com/samber/lo
go get github.com/panjf2000/ants/v2
go get github.com/sourcegraph/conc
go get github.com/joho/godotenv
```

All libraries should be imported and used consistently across the codebase to maintain compatibility and leverage their performance optimizations.
