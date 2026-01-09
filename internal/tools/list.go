package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/standardbeagle/figma-query/internal/figma"
)

// ListComponentsArgs contains arguments for the list_components tool.
type ListComponentsArgs struct {
	FileKey         string   `json:"file_key" jsonschema:"Figma file key"`
	IncludeVariants bool     `json:"include_variants,omitempty" jsonschema:"Include variant info (default: true)"`
	IncludeUsage    bool     `json:"include_usage,omitempty" jsonschema:"Include instance count and locations"`
	Select          []string `json:"select,omitempty" jsonschema:"Properties to return"`
	Limit           int      `json:"limit,omitempty" jsonschema:"Max results to return (default: 100, max: 500)"`
	Offset          int      `json:"offset,omitempty" jsonschema:"Pagination offset"`
	Format          string   `json:"format,omitempty" jsonschema:"Response format: text (default) or json"`
	OutputFile      string   `json:"output_file,omitempty" jsonschema:"Write full output to file path"`
}

// ComponentInfo represents component information.
type ComponentInfo struct {
	ID          string   `json:"id"`
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Variants    int      `json:"variants,omitempty"`
	Instances   int      `json:"instances,omitempty"`
}

// ListComponentsResult contains the result of list_components.
type ListComponentsResult struct {
	Components []ComponentInfo     `json:"components"`
	Total      int                 `json:"total"`
	Returned   int                 `json:"returned"`
	HasMore    bool                `json:"has_more"`
	Offset     int                 `json:"offset,omitempty"`
	ByCategory map[string][]string `json:"by_category,omitempty"`
	FilePath   string              `json:"file_path,omitempty"`
}

func registerListComponentsTool(server *mcp.Server, r *Registry) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_components",
		Description: "List all components with usage statistics.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListComponentsArgs) (*mcp.CallToolResult, *ListComponentsResult, error) {
		if args.FileKey == "" {
			return nil, nil, fmt.Errorf("file_key is required")
		}

		if !r.HasClient() {
			return nil, nil, fmt.Errorf("Figma API not configured")
		}

		// Set defaults
		limit := args.Limit
		if limit == 0 {
			limit = 100
		}
		if limit > 500 {
			limit = 500
		}

		// Fetch file
		file, err := r.Client().GetFile(ctx, args.FileKey, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("fetching file: %w", err)
		}

		// Count instances if requested
		instanceCounts := make(map[string]int)
		if args.IncludeUsage && file.Document != nil {
			countInstances(file.Document, instanceCounts)
		}

		// Build component list
		components := make([]ComponentInfo, 0, len(file.Components))
		categories := make(map[string][]string)

		for id, comp := range file.Components {
			info := ComponentInfo{
				ID:          id,
				Key:         comp.Key,
				Name:        comp.Name,
				Description: comp.Description,
			}

			if args.IncludeUsage {
				info.Instances = instanceCounts[comp.Key]
			}

			components = append(components, info)

			// Categorize by prefix (e.g., "Button/Primary" -> "Button")
			parts := strings.SplitN(comp.Name, "/", 2)
			if len(parts) > 1 {
				categories[parts[0]] = append(categories[parts[0]], comp.Name)
			}
		}

		// Count variants from component sets
		if args.IncludeVariants {
			for _, comp := range components {
				// Find component set for this component
				for _, cs := range file.ComponentSets {
					if strings.HasPrefix(comp.Name, cs.Name) {
						// Count variants in this set
						variantCount := 0
						for _, c := range file.Components {
							if c.ComponentSetID != "" {
								variantCount++
							}
						}
						comp.Variants = variantCount
						break
					}
				}
			}
		}

		// Sort by name
		sort.Slice(components, func(i, j int) bool {
			return components[i].Name < components[j].Name
		})

		// Apply pagination
		total := len(components)
		paginatedComponents, truncInfo := Paginate(components, args.Offset, limit)

		result := &ListComponentsResult{
			Components: paginatedComponents,
			Total:      total,
			Returned:   truncInfo.Returned,
			HasMore:    truncInfo.Truncated,
			Offset:     args.Offset,
			ByCategory: categories,
		}

		// Format output
		var textOutput string
		if args.Format == "json" {
			b, _ := json.MarshalIndent(result, "", "  ")
			textOutput = string(b)
		} else {
			textOutput = formatComponentList(result, args.IncludeUsage)
			if truncInfo.Truncated {
				textOutput += FormatTruncationWarning(total, truncInfo.Returned, "list_components")
			}
		}

		// Handle large output / file writing
		outputCfg := OutputConfig{
			MaxOutputSize: DefaultMaxOutputSize,
			OutputFile:    args.OutputFile,
			OutputDir:     r.ExportDir(),
			ToolName:      "list_components",
			FileKey:       args.FileKey,
		}

		outputResult, err := ProcessOutput(textOutput, result, outputCfg)
		if err != nil {
			return nil, nil, fmt.Errorf("processing output: %w", err)
		}

		if outputResult.WasWrittenToFile {
			result.FilePath = outputResult.FilePath
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: outputResult.Text},
			},
		}, result, nil
	})
}

func countInstances(doc *figma.DocumentNode, counts map[string]int) {
	var walk func(*figma.Node)
	walk = func(n *figma.Node) {
		if n.Type == figma.NodeTypeInstance && n.ComponentID != "" {
			counts[n.ComponentID]++
		}
		for _, child := range n.Children {
			walk(child)
		}
	}

	for _, page := range doc.Children {
		walk(page)
	}
}

func formatComponentList(r *ListComponentsResult, showUsage bool) string {
	var sb strings.Builder

	if r.HasMore {
		sb.WriteString(fmt.Sprintf("Components: %d of %d (offset %d)\n\n", r.Returned, r.Total, r.Offset))
	} else {
		sb.WriteString(fmt.Sprintf("Found %d components\n\n", r.Total))
	}

	if showUsage {
		sb.WriteString("ID       | Name                           | Instances\n")
		sb.WriteString("-------- | ------------------------------ | ---------\n")
	} else {
		sb.WriteString("ID       | Name                           | Description\n")
		sb.WriteString("-------- | ------------------------------ | -----------\n")
	}

	for _, c := range r.Components {
		name := c.Name
		if len(name) > 30 {
			name = name[:27] + "..."
		}

		if showUsage {
			sb.WriteString(fmt.Sprintf("%-8s | %-30s | %d\n", c.ID, name, c.Instances))
		} else {
			desc := c.Description
			if len(desc) > 30 {
				desc = desc[:27] + "..."
			}
			sb.WriteString(fmt.Sprintf("%-8s | %-30s | %s\n", c.ID, name, desc))
		}
	}

	if len(r.ByCategory) > 0 {
		sb.WriteString("\nCategories:\n")
		for cat, items := range r.ByCategory {
			sb.WriteString(fmt.Sprintf("  %s: %d items\n", cat, len(items)))
		}
	}

	if r.HasMore {
		nextOffset := r.Offset + r.Returned
		sb.WriteString(fmt.Sprintf("\n[Use offset=%d to see next page]\n", nextOffset))
	}

	return sb.String()
}

// ListStylesArgs contains arguments for the list_styles tool.
type ListStylesArgs struct {
	FileKey       string   `json:"file_key" jsonschema:"Figma file key"`
	Types         []string `json:"types,omitempty" jsonschema:"Filter by type: color text effect grid"`
	IncludeValues bool     `json:"include_values,omitempty" jsonschema:"Include resolved style values (default: true)"`
	Limit         int      `json:"limit,omitempty" jsonschema:"Max results to return (default: 100, max: 500)"`
	Offset        int      `json:"offset,omitempty" jsonschema:"Pagination offset"`
	Format        string   `json:"format,omitempty" jsonschema:"Response format: text (default) or json"`
	OutputFile    string   `json:"output_file,omitempty" jsonschema:"Write full output to file path"`
}

// StyleInfo represents style information.
type StyleInfo struct {
	ID          string         `json:"id"`
	Key         string         `json:"key"`
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Description string         `json:"description,omitempty"`
	Value       map[string]any `json:"value,omitempty"`
}

// ListStylesResult contains the result of list_styles.
type ListStylesResult struct {
	Styles   map[string][]StyleInfo `json:"styles"`
	Total    int                    `json:"total"`
	Returned int                    `json:"returned"`
	HasMore  bool                   `json:"has_more"`
	Offset   int                    `json:"offset,omitempty"`
	FilePath string                 `json:"file_path,omitempty"`
}

func registerListStylesTool(server *mcp.Server, r *Registry) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_styles",
		Description: "List all styles (color, text, effect, grid).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListStylesArgs) (*mcp.CallToolResult, *ListStylesResult, error) {
		if args.FileKey == "" {
			return nil, nil, fmt.Errorf("file_key is required")
		}

		if !r.HasClient() {
			return nil, nil, fmt.Errorf("Figma API not configured")
		}

		// Set defaults
		limit := args.Limit
		if limit == 0 {
			limit = 100
		}
		if limit > 500 {
			limit = 500
		}

		types := args.Types
		if len(types) == 0 {
			types = []string{"color", "text", "effect", "grid"}
		}

		// Fetch file
		file, err := r.Client().GetFile(ctx, args.FileKey, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("fetching file: %w", err)
		}

		// Collect all styles first
		allStyles := make(map[string][]StyleInfo)
		totalCount := 0

		typeMap := map[figma.StyleType]string{
			figma.StyleTypeFill:   "color",
			figma.StyleTypeText:   "text",
			figma.StyleTypeEffect: "effect",
			figma.StyleTypeGrid:   "grid",
		}

		for id, style := range file.Styles {
			typeName := typeMap[style.StyleType]

			// Filter by type
			if !containsString(types, typeName) {
				continue
			}

			info := StyleInfo{
				ID:          id,
				Key:         style.Key,
				Name:        style.Name,
				Type:        typeName,
				Description: style.Description,
			}

			allStyles[typeName] = append(allStyles[typeName], info)
			totalCount++
		}

		// Sort each category
		for _, styles := range allStyles {
			sort.Slice(styles, func(i, j int) bool {
				return styles[i].Name < styles[j].Name
			})
		}

		// Apply pagination across all styles
		// Flatten styles for pagination
		var flatStyles []StyleInfo
		for _, typeName := range []string{"color", "text", "effect", "grid"} {
			flatStyles = append(flatStyles, allStyles[typeName]...)
		}

		paginatedStyles, truncInfo := Paginate(flatStyles, args.Offset, limit)

		// Rebuild grouped styles from paginated results
		result := &ListStylesResult{
			Styles:   make(map[string][]StyleInfo),
			Total:    totalCount,
			Returned: truncInfo.Returned,
			HasMore:  truncInfo.Truncated,
			Offset:   args.Offset,
		}

		for _, s := range paginatedStyles {
			result.Styles[s.Type] = append(result.Styles[s.Type], s)
		}

		// Format output
		var textOutput string
		if args.Format == "json" {
			b, _ := json.MarshalIndent(result, "", "  ")
			textOutput = string(b)
		} else {
			textOutput = formatStyleList(result)
			if truncInfo.Truncated {
				textOutput += FormatTruncationWarning(totalCount, truncInfo.Returned, "list_styles")
			}
		}

		// Handle large output / file writing
		outputCfg := OutputConfig{
			MaxOutputSize: DefaultMaxOutputSize,
			OutputFile:    args.OutputFile,
			OutputDir:     r.ExportDir(),
			ToolName:      "list_styles",
			FileKey:       args.FileKey,
		}

		outputResult, err := ProcessOutput(textOutput, result, outputCfg)
		if err != nil {
			return nil, nil, fmt.Errorf("processing output: %w", err)
		}

		if outputResult.WasWrittenToFile {
			result.FilePath = outputResult.FilePath
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: outputResult.Text},
			},
		}, result, nil
	})
}

func formatStyleList(r *ListStylesResult) string {
	var sb strings.Builder

	if r.HasMore {
		sb.WriteString(fmt.Sprintf("Styles: %d of %d (offset %d)\n\n", r.Returned, r.Total, r.Offset))
	} else {
		sb.WriteString(fmt.Sprintf("Found %d styles\n\n", r.Total))
	}

	for typeName, styles := range r.Styles {
		sb.WriteString(fmt.Sprintf("%s Styles (%d)\n", strings.Title(typeName), len(styles)))
		sb.WriteString(strings.Repeat("-", 40) + "\n")

		for _, s := range styles {
			sb.WriteString(fmt.Sprintf("  [%s] %s\n", s.ID, s.Name))
			if s.Description != "" {
				sb.WriteString(fmt.Sprintf("         %s\n", s.Description))
			}
		}
		sb.WriteString("\n")
	}

	if r.HasMore {
		nextOffset := r.Offset + r.Returned
		sb.WriteString(fmt.Sprintf("[Use offset=%d to see next page]\n", nextOffset))
	}

	return sb.String()
}
