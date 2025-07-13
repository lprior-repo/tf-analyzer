package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"pgregory.net/rapid"
)

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