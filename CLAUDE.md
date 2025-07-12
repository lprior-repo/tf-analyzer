# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

tf-analyzer is a high-performance Go application designed to concurrently clone multiple GitHub organizations and analyze their files in real-time. The program focuses on parsing markdown, HCL, Terraform (.tf), and tfvars files with maximum speed while maintaining extremely DRY and simple code following functional programming principles.

## Core Architecture Principles

- **Simplicity**: Code must be instantly clear - readable by an intern with minimal explanation
- **Pure Functions**: Use focused functions with no side effects for predictability
- **Modularity**: Design loosely coupled, composable components
- **Domain-based Structure**: Each file has a specific purpose, avoiding folders when possible
- **100% Test Coverage**: All code must be thoroughly tested
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

## Development Commands

### Essential Commands
- `make build` - Build the application
- `make test` - Run all tests
- `make lint` - Run golangci-lint with complexity checks
- `make run` - Run the application
- `make ci` - Run full CI pipeline (lint + test)

### Development Setup
- `make init` - Initialize project and install tools
- `make install-tools` - Install golangci-lint and development tools
- `make deps` - Download dependencies

### Code Quality
- `make lint-fix` - Auto-fix linting issues
- `make coverage` - Generate test coverage report
- `make sec` - Run security checks

### Performance
- `make bench` - Run benchmarks
- `make profile-cpu` - CPU profiling
- `make profile-mem` - Memory profiling

## Project Configuration

### Environment Variables (.env)
- `GITHUB_TOKEN` - GitHub API token for repository access
- `MAX_CONCURRENT_CLONES` - Concurrent repository cloning limit
- `MAX_CONCURRENT_ANALYZERS` - Concurrent file analysis limit
- `ANALYZE_EXTENSIONS` - File types to analyze (.md,.tf,.tfvars,.hcl)

### Code Quality Enforcement
- golangci-lint configured with cyclomatic complexity limit of 8
- Function length limited to 25 lines
- Maximum 3 parameters per function
- Comprehensive linting rules for performance and security

The project emphasizes avoiding OOP patterns in favor of simple, stateless functions and values adaptability and easy refactoring for dynamic development workflows.

## Functional Programming in Go

This guide explores functional programming (FP) principles in Go, focusing on practical application over theoretical purity. The goal is to leverage FP concepts to write clear, maintainable, and highly testable Go code by treating functions as first-class citizens and creating clean separation between logic and side effects.

Modern Go development often relies on the `github.com/samber/lo` library for common functional patterns like `Map`, `Filter`, and `Reduce`.

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