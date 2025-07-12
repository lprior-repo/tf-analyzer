package main

import (
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bitfield/script"
	"github.com/samber/lo"
)

// ============================================================================
// ANALYZER - Unified Terraform analysis functionality
// ============================================================================

type BackendConfig struct {
	Type   *string `json:"type"`
	Region *string `json:"region"`
}

type ProviderDetail struct {
	Source  string   `json:"source"`
	Version string   `json:"version"`
	Regions []string `json:"regions"`
}

type ProvidersAnalysis struct {
	UniqueProviderCount int              `json:"unique_provider_count"`
	ProviderDetails     []ProviderDetail `json:"provider_details"`
}

type ModuleDetail struct {
	Source string `json:"source"`
	Count  int    `json:"count"`
}

type ModulesAnalysis struct {
	TotalModuleCalls  int            `json:"total_module_calls"`
	UniqueModuleCount int            `json:"unique_module_count"`
	UniqueModules     []ModuleDetail `json:"unique_modules"`
}

type ResourceType struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

type UntaggedResource struct {
	ResourceType string   `json:"resource_type"`
	Name         string   `json:"name"`
	MissingTags  []string `json:"missing_tags"`
}

type ResourceAnalysis struct {
	TotalResourceCount      int                `json:"total_resource_count"`
	UniqueResourceTypeCount int                `json:"unique_resource_type_count"`
	ResourceTypes           []ResourceType     `json:"resource_types"`
	UntaggedResources       []UntaggedResource `json:"untagged_resources"`
}

type VariableDefinition struct {
	Name       string `json:"name"`
	HasDefault bool   `json:"has_default"`
}

type VariableAnalysis struct {
	DefinedVariables []VariableDefinition `json:"defined_variables"`
}

type OutputAnalysis struct {
	OutputCount int      `json:"output_count"`
	Outputs     []string `json:"outputs"`
}

type RepositoryAnalysis struct {
	RepositoryPath   string            `json:"repository_path"`
	BackendConfig    *BackendConfig    `json:"backend_config"`
	Providers        ProvidersAnalysis `json:"providers"`
	Modules          ModulesAnalysis   `json:"modules"`
	ResourceAnalysis ResourceAnalysis  `json:"resource_analysis"`
	VariableAnalysis VariableAnalysis  `json:"variable_analysis"`
	OutputAnalysis   OutputAnalysis    `json:"output_analysis"`
}

type AnalysisResult struct {
	RepoName     string
	Organization string
	Analysis     RepositoryAnalysis
	Error        error
}

var mandatoryTags = []string{"Environment", "Owner", "Project"}

func isRelevantFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".tf") ||
		strings.HasSuffix(lower, ".tfvars") ||
		strings.HasSuffix(lower, ".hcl")
}

func shouldSkipPath(path string) bool {
	return strings.Contains(path, "/.git/")
}

func parseBackend(content string) *BackendConfig {
	backendRegex := regexp.MustCompile(`(?s)backend\s+"([^"]+)"\s*\{([^}]*)\}`)
	match := backendRegex.FindStringSubmatch(content)

	if len(match) < 3 {
		return nil
	}

	backendType := match[1]
	backendBody := match[2]

	var region *string
	regionRegex := regexp.MustCompile(`region\s*=\s*"([^"]+)"`)
	if regionMatch := regionRegex.FindStringSubmatch(backendBody); len(regionMatch) >= 2 {
		regionVal := regionMatch[1]
		region = &regionVal
	}

	return &BackendConfig{
		Type:   &backendType,
		Region: region,
	}
}

func parseProviders(content string) []ProviderDetail {
	providerMap := make(map[string]ProviderDetail)

	// Parse provider blocks
	providerRegex := regexp.MustCompile(`(?s)provider\s+"([^"]+)"\s*\{([^}]*)\}`)
	matches := providerRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			providerName := match[1]
			providerBody := match[2]

			var regions []string
			regionRegex := regexp.MustCompile(`region\s*=\s*"([^"]+)"`)
			if regionMatch := regionRegex.FindStringSubmatch(providerBody); len(regionMatch) >= 2 {
				regions = append(regions, regionMatch[1])
			}

			key := fmt.Sprintf("%s@", providerName)
			if _, exists := providerMap[key]; !exists {
				providerMap[key] = ProviderDetail{
					Source:  providerName,
					Version: "",
					Regions: regions,
				}
			} else {
				existing := providerMap[key]
				existing.Regions = lo.Union(existing.Regions, regions)
				providerMap[key] = existing
			}
		}
	}

	// Parse required_providers
	requiredRegex := regexp.MustCompile(`(?s)required_providers\s*\{([^}]*(?:\{[^}]*\}[^}]*)*)\}`)
	requiredMatches := requiredRegex.FindAllStringSubmatch(content, -1)
	for _, match := range requiredMatches {
		if len(match) >= 2 {
			requiredBody := match[1]

			providerDefRegex := regexp.MustCompile(`(\w+)\s*=\s*\{([^}]*)\}`)
			providerDefs := providerDefRegex.FindAllStringSubmatch(requiredBody, -1)

			for _, def := range providerDefs {
				if len(def) >= 3 {
					_ = def[1] // providerName not used
					providerConfig := def[2]

					sourceRegex := regexp.MustCompile(`source\s*=\s*"([^"]+)"`)
					versionRegex := regexp.MustCompile(`version\s*=\s*"([^"]+)"`)

					var source, version string
					if sourceMatch := sourceRegex.FindStringSubmatch(providerConfig); len(sourceMatch) >= 2 {
						source = sourceMatch[1]
					}
					if versionMatch := versionRegex.FindStringSubmatch(providerConfig); len(versionMatch) >= 2 {
						version = versionMatch[1]
					}

					key := fmt.Sprintf("%s@%s", source, version)
					if existing, exists := providerMap[key]; exists {
						existing.Regions = lo.Union(existing.Regions, []string{})
						providerMap[key] = existing
					} else {
						providerMap[key] = ProviderDetail{
							Source:  source,
							Version: version,
							Regions: []string{},
						}
					}
				}
			}
		}
	}

	return lo.Values(providerMap)
}

func parseModules(content string) []ModuleDetail {
	moduleMap := make(map[string]int)

	moduleRegex := regexp.MustCompile(`(?s)module\s+"([^"]+)"\s*\{([^}]*)\}`)
	matches := moduleRegex.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			moduleBody := match[2]

			sourceRegex := regexp.MustCompile(`source\s*=\s*"([^"]+)"`)
			if sourceMatch := sourceRegex.FindStringSubmatch(moduleBody); len(sourceMatch) >= 2 {
				moduleMap[sourceMatch[1]]++
			}
		}
	}

	return lo.MapToSlice(moduleMap, func(source string, count int) ModuleDetail {
		return ModuleDetail{Source: source, Count: count}
	})
}

func parseResources(content string) ([]ResourceType, []UntaggedResource) {
	resourceRegex := regexp.MustCompile(`(?s)resource\s+"([^"]+)"\s+"([^"]+)"\s*\{([^}]*(?:\{[^}]*\}[^}]*)*)\}`)
	matches := resourceRegex.FindAllStringSubmatch(content, -1)

	resourceTypeMap := make(map[string]int)
	var untaggedResources []UntaggedResource

	for _, match := range matches {
		if len(match) >= 4 {
			resourceType := match[1]
			resourceName := match[2]
			resourceBody := match[3]

			resourceTypeMap[resourceType]++

			tags := parseResourceTags(resourceBody)
			var missingTags []string
			for _, requiredTag := range mandatoryTags {
				if _, exists := tags[requiredTag]; !exists {
					missingTags = append(missingTags, requiredTag)
				}
			}

			if len(missingTags) > 0 {
				untaggedResources = append(untaggedResources, UntaggedResource{
					ResourceType: resourceType,
					Name:         resourceName,
					MissingTags:  missingTags,
				})
			}
		}
	}

	resourceTypes := lo.MapToSlice(resourceTypeMap, func(resType string, count int) ResourceType {
		return ResourceType{Type: resType, Count: count}
	})

	return resourceTypes, untaggedResources
}

func parseResourceTags(resourceBody string) map[string]string {
	tags := make(map[string]string)

	tagsRegex := regexp.MustCompile(`(?s)tags\s*=\s*\{([^}]*(?:\{[^}]*\}[^}]*)*)\}`)
	tagsMatch := tagsRegex.FindStringSubmatch(resourceBody)

	if len(tagsMatch) >= 2 {
		tagsContent := tagsMatch[1]

		tagRegex := regexp.MustCompile(`"([^"]+)"\s*=\s*"([^"]*)"`)
		tagMatches := tagRegex.FindAllStringSubmatch(tagsContent, -1)

		for _, tagMatch := range tagMatches {
			if len(tagMatch) >= 3 {
				tags[tagMatch[1]] = tagMatch[2]
			}
		}
	}

	return tags
}

func parseVariables(content string) []VariableDefinition {
	variableRegex := regexp.MustCompile(`(?s)variable\s+"([^"]+)"\s*\{([^}]*)\}`)
	matches := variableRegex.FindAllStringSubmatch(content, -1)

	return lo.Map(matches, func(match []string, _ int) VariableDefinition {
		variableName := match[1]
		variableBody := match[2]
		hasDefault := strings.Contains(variableBody, "default")

		return VariableDefinition{
			Name:       variableName,
			HasDefault: hasDefault,
		}
	})
}

func parseOutputs(content string) []string {
	outputRegex := regexp.MustCompile(`(?s)output\s+"([^"]+)"\s*\{([^}]*)\}`)
	matches := outputRegex.FindAllStringSubmatch(content, -1)

	return lo.Map(matches, func(match []string, _ int) string {
		return match[1]
	})
}

func loadFileContent(path string) ([]byte, error) {
	return script.File(path).Bytes()
}


func analyzeRepositoryWithRecovery(repoPath string, logger *slog.Logger) (RepositoryAnalysis, error) {
	var allProviders []ProviderDetail
	var allModules []ModuleDetail
	var allResourceTypes []ResourceType
	var allUntaggedResources []UntaggedResource
	var allVariables []VariableDefinition
	var allOutputs []string
	var backend *BackendConfig

	filesProcessed := 0
	filesSkipped := 0
	filesErrored := 0

	err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logger.Debug("Error accessing path", "path", path, "error", err)
			return err
		}
		
		if d.IsDir() || shouldSkipPath(path) {
			return nil
		}
		
		if !isRelevantFile(path) {
			filesSkipped++
			return nil
		}

		content, readErr := loadFileContent(path)
		if readErr != nil {
			logger.Debug("Failed to read file, skipping", "path", path, "error", readErr)
			filesErrored++
			return nil // Continue processing other files
		}

		contentStr := string(content)
		
		// Parse with error recovery for each section
		if backend == nil {
			if parsedBackend := parseBackendSafely(contentStr, logger); parsedBackend != nil {
				backend = parsedBackend
			}
		}

		if providers := parseProvidersSafely(contentStr, logger); len(providers) > 0 {
			allProviders = append(allProviders, providers...)
		}
		
		if modules := parseModulesSafely(contentStr, logger); len(modules) > 0 {
			allModules = append(allModules, modules...)
		}

		resourceTypes, untaggedResources := parseResourcesSafely(contentStr, logger)
		allResourceTypes = append(allResourceTypes, resourceTypes...)
		allUntaggedResources = append(allUntaggedResources, untaggedResources...)

		if variables := parseVariablesSafely(contentStr, logger); len(variables) > 0 {
			allVariables = append(allVariables, variables...)
		}
		
		if outputs := parseOutputsSafely(contentStr, logger); len(outputs) > 0 {
			allOutputs = append(allOutputs, outputs...)
		}

		filesProcessed++
		return nil
	})

	logger.Debug("Repository analysis stats", 
		"files_processed", filesProcessed,
		"files_skipped", filesSkipped,
		"files_errored", filesErrored)

	if err != nil {
		return RepositoryAnalysis{RepositoryPath: repoPath}, fmt.Errorf("failed to walk directory: %w", err)
	}

	// Aggregate and deduplicate
	uniqueProviders := lo.UniqBy(allProviders, func(p ProviderDetail) string {
		return fmt.Sprintf("%s@%s", p.Source, p.Version)
	})

	moduleCountMap := make(map[string]int)
	totalModuleCalls := 0
	for _, module := range allModules {
		moduleCountMap[module.Source] += module.Count
		totalModuleCalls += module.Count
	}
	uniqueModules := lo.MapToSlice(moduleCountMap, func(source string, count int) ModuleDetail {
		return ModuleDetail{Source: source, Count: count}
	})

	resourceTypeCountMap := make(map[string]int)
	for _, resourceType := range allResourceTypes {
		resourceTypeCountMap[resourceType.Type] += resourceType.Count
	}
	aggregatedResourceTypes := lo.MapToSlice(resourceTypeCountMap, func(resType string, count int) ResourceType {
		return ResourceType{Type: resType, Count: count}
	})
	totalResourceCount := lo.Reduce(aggregatedResourceTypes, func(acc int, rt ResourceType, _ int) int {
		return acc + rt.Count
	}, 0)

	return RepositoryAnalysis{
		RepositoryPath: repoPath,
		BackendConfig:  backend,
		Providers: ProvidersAnalysis{
			UniqueProviderCount: len(uniqueProviders),
			ProviderDetails:     uniqueProviders,
		},
		Modules: ModulesAnalysis{
			TotalModuleCalls:  totalModuleCalls,
			UniqueModuleCount: len(uniqueModules),
			UniqueModules:     uniqueModules,
		},
		ResourceAnalysis: ResourceAnalysis{
			TotalResourceCount:      totalResourceCount,
			UniqueResourceTypeCount: len(aggregatedResourceTypes),
			ResourceTypes:           aggregatedResourceTypes,
			UntaggedResources:       allUntaggedResources,
		},
		VariableAnalysis: VariableAnalysis{
			DefinedVariables: allVariables,
		},
		OutputAnalysis: OutputAnalysis{
			OutputCount: len(allOutputs),
			Outputs:     allOutputs,
		},
	}, nil
}

// Safe parsing functions with error recovery
func parseBackendSafely(content string, logger *slog.Logger) *BackendConfig {
	defer func() {
		if r := recover(); r != nil {
			logger.Debug("Backend parsing panic recovered", "panic", r)
		}
	}()
	return parseBackend(content)
}

func parseProvidersSafely(content string, logger *slog.Logger) []ProviderDetail {
	defer func() {
		if r := recover(); r != nil {
			logger.Debug("Provider parsing panic recovered", "panic", r)
		}
	}()
	return parseProviders(content)
}

func parseModulesSafely(content string, logger *slog.Logger) []ModuleDetail {
	defer func() {
		if r := recover(); r != nil {
			logger.Debug("Module parsing panic recovered", "panic", r)
		}
	}()
	return parseModules(content)
}

func parseResourcesSafely(content string, logger *slog.Logger) ([]ResourceType, []UntaggedResource) {
	defer func() {
		if r := recover(); r != nil {
			logger.Debug("Resource parsing panic recovered", "panic", r)
		}
	}()
	return parseResources(content)
}

func parseVariablesSafely(content string, logger *slog.Logger) []VariableDefinition {
	defer func() {
		if r := recover(); r != nil {
			logger.Debug("Variable parsing panic recovered", "panic", r)
		}
	}()
	return parseVariables(content)
}

func parseOutputsSafely(content string, logger *slog.Logger) []string {
	defer func() {
		if r := recover(); r != nil {
			logger.Debug("Output parsing panic recovered", "panic", r)
		}
	}()
	return parseOutputs(content)
}


func processRepositoryFilesWithRecovery(repo Repository, logger *slog.Logger) AnalysisResult {
	repoLogger := logger.With("repository", repo.Name, "organization", repo.Organization)
	
	defer func() {
		if r := recover(); r != nil {
			repoLogger.Error("Repository analysis panic recovered", "panic", r)
		}
	}()

	analysis, err := analyzeRepositoryWithRecovery(repo.Path, repoLogger)
	if err != nil {
		return AnalysisResult{
			RepoName:     repo.Name,
			Organization: repo.Organization,
			Error:        err,
		}
	}

	return AnalysisResult{
		RepoName:     repo.Name,
		Organization: repo.Organization,
		Analysis:     analysis,
	}
}