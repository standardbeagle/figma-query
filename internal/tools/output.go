package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OutputConfig contains configuration for output control.
type OutputConfig struct {
	// MaxOutputSize is the maximum size in bytes before forcing file output.
	// Default: 30000 (roughly 30KB, ~7500 tokens)
	MaxOutputSize int

	// OutputFile is an optional file path to write output to.
	// If set, output is always written to file regardless of size.
	OutputFile string

	// OutputDir is the base directory for auto-generated output files.
	OutputDir string

	// ToolName is used for generating output file names.
	ToolName string

	// FileKey is the Figma file key, used in output file names.
	FileKey string
}

// OutputResult contains the result of output processing.
type OutputResult struct {
	// Text is the output text (either full output or truncated with file reference).
	Text string

	// FilePath is set if output was written to a file.
	FilePath string

	// WasWrittenToFile indicates if the output was written to a file.
	WasWrittenToFile bool

	// OriginalSize is the size of the original output.
	OriginalSize int
}

// DefaultMaxOutputSize is the default maximum output size before file output.
const DefaultMaxOutputSize = 30000

// ProcessOutput handles output size control and optional file writing.
// It returns a potentially modified output string and metadata about what happened.
func ProcessOutput(output string, data any, cfg OutputConfig) (*OutputResult, error) {
	if cfg.MaxOutputSize == 0 {
		cfg.MaxOutputSize = DefaultMaxOutputSize
	}

	result := &OutputResult{
		Text:         output,
		OriginalSize: len(output),
	}

	// Check if we should write to file
	shouldWriteToFile := cfg.OutputFile != "" || len(output) > cfg.MaxOutputSize

	if !shouldWriteToFile {
		return result, nil
	}

	// Determine output file path
	filePath := cfg.OutputFile
	if filePath == "" {
		filePath = generateOutputPath(cfg)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	// Write to file
	var writeErr error
	if data != nil {
		// Write structured data as JSON
		jsonBytes, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshaling output: %w", err)
		}
		writeErr = os.WriteFile(filePath, jsonBytes, 0644)
	} else {
		// Write text output
		writeErr = os.WriteFile(filePath, []byte(output), 0644)
	}

	if writeErr != nil {
		return nil, fmt.Errorf("writing output file: %w", writeErr)
	}

	result.FilePath = filePath
	result.WasWrittenToFile = true

	// Create summary text with file reference
	result.Text = formatFileOutputSummary(output, filePath, cfg)

	return result, nil
}

// generateOutputPath generates a path for output files.
func generateOutputPath(cfg OutputConfig) string {
	outputDir := cfg.OutputDir
	if outputDir == "" {
		outputDir = "./figma-export/.tool-results"
	}

	// Create filename from tool name and file key
	timestamp := time.Now().UnixMilli()
	filename := fmt.Sprintf("%s-%s-%d.json", cfg.ToolName, sanitizeForPath(cfg.FileKey), timestamp)

	return filepath.Join(outputDir, filename)
}

// sanitizeForPath sanitizes a string for use in file paths.
func sanitizeForPath(s string) string {
	s = strings.ReplaceAll(s, ":", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	if len(s) > 30 {
		s = s[:30]
	}
	return s
}

// formatFileOutputSummary creates a summary message when output is written to file.
func formatFileOutputSummary(output, filePath string, cfg OutputConfig) string {
	var sb strings.Builder

	// Show first ~2000 chars as preview
	previewSize := 2000
	preview := output
	if len(preview) > previewSize {
		preview = preview[:previewSize]
		// Try to break at a newline
		if lastNewline := strings.LastIndex(preview, "\n"); lastNewline > previewSize/2 {
			preview = preview[:lastNewline]
		}
	}

	sb.WriteString(preview)
	sb.WriteString("\n\n")
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("Output truncated (%d bytes > %d max). Full output written to:\n", len(output), cfg.MaxOutputSize))
	sb.WriteString(fmt.Sprintf("  %s\n\n", filePath))
	sb.WriteString("Suggestions to reduce output:\n")
	sb.WriteString(getSuggestions(cfg.ToolName))

	return sb.String()
}

// getSuggestions returns tool-specific suggestions for reducing output size.
func getSuggestions(toolName string) string {
	suggestions := map[string]string{
		"get_tree": `  - Use root_node_id to focus on a specific section
  - Reduce depth (default: 3)
  - Use node_types filter to show only specific types
  - Use query tool with filters for targeted results`,

		"list_components": `  - Use limit parameter to paginate results
  - Use query tool with filters for specific components`,

		"list_styles": `  - Use types filter (color, text, effect, grid)
  - Use limit parameter to paginate results`,

		"wireframe": `  - Reduce depth (default: 2)
  - Use a more specific node_id
  - Export to file with output_path parameter`,

		"get_node": `  - Use select parameter instead of @all
  - Reduce depth to exclude children
  - Use specific projections: @structure, @bounds, @css`,

		"search": `  - Add node_types filter
  - Use more specific pattern
  - Reduce limit parameter`,

		"query": `  - Add filters in where clause
  - Use specific select fields instead of @all
  - Reduce limit parameter`,
	}

	if s, ok := suggestions[toolName]; ok {
		return s
	}
	return "  - Use more specific filters\n  - Request fewer results\n"
}

// TruncationInfo contains information about truncated results.
type TruncationInfo struct {
	Total     int
	Returned  int
	Truncated bool
	Message   string
}

// FormatTruncationWarning formats a warning message about truncated results.
func FormatTruncationWarning(total, returned int, toolName string) string {
	if returned >= total {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n[Results truncated: showing %d of %d]\n", returned, total))
	sb.WriteString(getSuggestions(toolName))
	return sb.String()
}

// Pagination contains pagination parameters.
type Pagination struct {
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// DefaultLimit returns a sensible default limit for a tool.
func DefaultLimit(toolName string) int {
	defaults := map[string]int{
		"list_components": 100,
		"list_styles":     100,
		"get_tree":        500,
		"search":          50,
		"query":           50,
	}
	if limit, ok := defaults[toolName]; ok {
		return limit
	}
	return 50
}

// Paginate applies pagination to a slice and returns pagination info.
func Paginate[T any](items []T, offset, limit int) ([]T, *TruncationInfo) {
	total := len(items)

	if offset >= total {
		return []T{}, &TruncationInfo{
			Total:     total,
			Returned:  0,
			Truncated: offset > 0,
			Message:   fmt.Sprintf("Offset %d exceeds total %d items", offset, total),
		}
	}

	end := offset + limit
	if end > total {
		end = total
	}

	result := items[offset:end]

	return result, &TruncationInfo{
		Total:     total,
		Returned:  len(result),
		Truncated: end < total,
		Message:   "",
	}
}
