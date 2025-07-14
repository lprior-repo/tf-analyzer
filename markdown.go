package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/glamour"
)

// ============================================================================
// MARKDOWN - Beautiful markdown rendering using Glamour
// ============================================================================

func (r *Reporter) PrintMarkdownToScreenWithGlamour() error {
	markdownContent := r.generateMarkdownContent()
	
	// Create glamour renderer with auto-detection of terminal capabilities
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(120),
	)
	if err != nil {
		// Fallback to raw markdown if glamour fails
		fmt.Print(markdownContent)
		return nil
	}

	// Render the markdown
	rendered, err := renderer.Render(markdownContent)
	if err != nil {
		// Fallback to raw markdown if rendering fails
		fmt.Print(markdownContent)
		return nil
	}

	fmt.Print(rendered)
	return nil
}

func (r *Reporter) PrintMarkdownToScreenWithStyle(style string) error {
	markdownContent := r.generateMarkdownContent()
	
	renderer, err := createGlamourRenderer(style)
	if err != nil {
		return fallbackToRawMarkdown(markdownContent)
	}

	rendered, err := renderer.Render(markdownContent)
	if err != nil {
		return fallbackToRawMarkdown(markdownContent)
	}

	fmt.Print(rendered)
	return nil
}

// createGlamourRenderer creates a renderer based on style using a map-based approach
func createGlamourRenderer(style string) (*glamour.TermRenderer, error) {
	rendererConfigs := map[string]func() (*glamour.TermRenderer, error){
		"dark": func() (*glamour.TermRenderer, error) {
			return glamour.NewTermRenderer(
				glamour.WithStylePath("dark"),
				glamour.WithWordWrap(120),
			)
		},
		"light": func() (*glamour.TermRenderer, error) {
			return glamour.NewTermRenderer(
				glamour.WithStylePath("light"),
				glamour.WithWordWrap(120),
			)
		},
		"notty": func() (*glamour.TermRenderer, error) {
			return glamour.NewTermRenderer(
				glamour.WithStylePath("notty"),
				glamour.WithWordWrap(120),
			)
		},
	}
	
	if createFunc, exists := rendererConfigs[style]; exists {
		return createFunc()
	}
	
	// Default case
	return glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(120),
	)
}

func fallbackToRawMarkdown(markdownContent string) error {
	fmt.Print(markdownContent)
	return nil
}

func detectTerminalCapabilities() string {
	// Check if we're in a TTY
	if !isTerminal() {
		return "notty"
	}
	
	// Check terminal color support
	colorTerm := os.Getenv("COLORTERM")
	term := os.Getenv("TERM")
	
	// Check for true color support
	if colorTerm == "truecolor" || colorTerm == "24bit" {
		return "dark"
	}
	
	// Check for 256 color support
	if term == "xterm-256color" || term == "screen-256color" {
		return "dark"
	}
	
	// Default to light for better compatibility
	return "light"
}

func isTerminal() bool {
	// Check if stdout is a terminal
	stat, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}