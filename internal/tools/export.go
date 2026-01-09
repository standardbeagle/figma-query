package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/standardbeagle/figma-query/internal/figma"
)

// ExportAssetsArgs contains arguments for the export_assets tool.
type ExportAssetsArgs struct {
	FileKey   string    `json:"file_key" jsonschema:"Figma file key"`
	NodeIDs   []string  `json:"node_ids" jsonschema:"Node IDs to export"`
	OutputDir string    `json:"output_dir" jsonschema:"Directory to save assets"`
	Formats   []string  `json:"formats,omitempty" jsonschema:"Image formats: png svg pdf jpg (default: svg)"`
	Scales    []float64 `json:"scales,omitempty" jsonschema:"Export scales: 1 2 3 for @1x @2x @3x"`
	Naming    string    `json:"naming,omitempty" jsonschema:"Naming strategy: id, name (default), or path"`
	Format    string    `json:"format,omitempty" jsonschema:"Response format: text (default) or json"`
}

// ExportAssetsResult contains the result of export_assets.
type ExportAssetsResult struct {
	Exported []string          `json:"exported"`
	Failed   []string          `json:"failed,omitempty"`
	Manifest map[string]string `json:"manifest"` // node_id -> file path
}

func registerExportAssetsTool(server *mcp.Server, r *Registry) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "export_assets",
		Description: "Export images/icons for specific nodes.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ExportAssetsArgs) (*mcp.CallToolResult, *ExportAssetsResult, error) {
		if args.FileKey == "" {
			return nil, nil, fmt.Errorf("file_key is required")
		}
		if len(args.NodeIDs) == 0 {
			return nil, nil, fmt.Errorf("node_ids is required")
		}
		if args.OutputDir == "" {
			return nil, nil, fmt.Errorf("output_dir is required")
		}

		if !r.HasClient() {
			return nil, nil, fmt.Errorf("Figma API not configured")
		}

		// Set defaults
		formats := args.Formats
		if len(formats) == 0 {
			formats = []string{"svg"}
		}
		scales := args.Scales
		if len(scales) == 0 {
			scales = []float64{1}
		}
		naming := args.Naming
		if naming == "" {
			naming = "name"
		}

		// Create output directory
		if err := os.MkdirAll(args.OutputDir, 0755); err != nil {
			return nil, nil, fmt.Errorf("creating output directory: %w", err)
		}

		// Get node names for naming
		nodeNames := make(map[string]string)
		if naming == "name" {
			nodes, err := r.Client().GetFileNodes(ctx, args.FileKey, args.NodeIDs, nil)
			if err == nil {
				for id, wrapper := range nodes.Nodes {
					if wrapper.Document != nil {
						nodeNames[id] = wrapper.Document.Name
					}
				}
			}
		}

		result := &ExportAssetsResult{
			Exported: make([]string, 0),
			Manifest: make(map[string]string),
		}

		// Export each format and scale combination
		for _, format := range formats {
			for _, scale := range scales {
				images, err := r.Client().GetImages(ctx, args.FileKey, args.NodeIDs, &figma.ImageExportOptions{
					Format: format,
					Scale:  scale,
				})
				if err != nil {
					result.Failed = append(result.Failed, fmt.Sprintf("export error: %v", err))
					continue
				}

				// Download each image
				for id, imageURL := range images.Images {
					if imageURL == "" {
						result.Failed = append(result.Failed, fmt.Sprintf("no image for %s", id))
						continue
					}

					// Build filename
					var filename string
					switch naming {
					case "name":
						name := nodeNames[id]
						if name == "" {
							name = id
						}
						filename = sanitizeName(name)
					case "path":
						filename = strings.ReplaceAll(id, ":", "-")
					default: // "id"
						filename = strings.ReplaceAll(id, ":", "-")
					}

					// Add scale suffix
					if scale != 1 {
						filename = fmt.Sprintf("%s@%dx", filename, int(scale))
					}
					filename = fmt.Sprintf("%s.%s", filename, format)

					filePath := filepath.Join(args.OutputDir, filename)

					// Download
					data, err := r.Client().DownloadImage(ctx, imageURL)
					if err != nil {
						result.Failed = append(result.Failed, fmt.Sprintf("download %s: %v", id, err))
						continue
					}

					// Write file
					if err := os.WriteFile(filePath, data, 0644); err != nil {
						result.Failed = append(result.Failed, fmt.Sprintf("write %s: %v", id, err))
						continue
					}

					result.Exported = append(result.Exported, filePath)
					result.Manifest[id] = filePath
				}
			}
		}

		// Format output
		var textOutput string
		if args.Format == "json" {
			b, _ := json.MarshalIndent(result, "", "  ")
			textOutput = string(b)
		} else {
			textOutput = formatExportResult(result)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: textOutput},
			},
		}, result, nil
	})
}

func formatExportResult(r *ExportAssetsResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Exported %d assets\n\n", len(r.Exported)))

	for _, path := range r.Exported {
		sb.WriteString(fmt.Sprintf("  %s\n", path))
	}

	if len(r.Failed) > 0 {
		sb.WriteString(fmt.Sprintf("\nFailed: %d\n", len(r.Failed)))
		for _, f := range r.Failed {
			sb.WriteString(fmt.Sprintf("  - %s\n", f))
		}
	}

	return sb.String()
}

// ExportTokensArgs contains arguments for the export_tokens tool.
type ExportTokensArgs struct {
	FileKey     string   `json:"file_key" jsonschema:"Figma file key"`
	OutputPath  string   `json:"output_path" jsonschema:"Output file path"`
	Format      string   `json:"format" jsonschema:"Export format: css, scss, json, js, ts, or tailwind"`
	Collections []string `json:"collections,omitempty" jsonschema:"Specific collections to export (default: all)"`
	Modes       []string `json:"modes,omitempty" jsonschema:"Specific modes to export (default: all)"`
	Prefix      string   `json:"prefix,omitempty" jsonschema:"Prefix for variable names"`
}

// ExportTokensResult contains the result of export_tokens.
type ExportTokensResult struct {
	Path        string   `json:"path"`
	TokensCount int      `json:"tokens_count"`
	Collections []string `json:"collections"`
}

func registerExportTokensTool(server *mcp.Server, r *Registry) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "export_tokens",
		Description: "Export design tokens/variables to various formats.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ExportTokensArgs) (*mcp.CallToolResult, *ExportTokensResult, error) {
		if args.FileKey == "" {
			return nil, nil, fmt.Errorf("file_key is required")
		}
		if args.OutputPath == "" {
			return nil, nil, fmt.Errorf("output_path is required")
		}
		if args.Format == "" {
			args.Format = "css"
		}

		if !r.HasClient() {
			return nil, nil, fmt.Errorf("Figma API not configured")
		}

		// Fetch variables
		vars, err := r.Client().GetLocalVariables(ctx, args.FileKey)
		if err != nil {
			return nil, nil, fmt.Errorf("fetching variables: %w", err)
		}

		if vars.Meta == nil {
			return nil, nil, fmt.Errorf("no variables found in file")
		}

		// Filter collections
		collections := make(map[string]*figma.VariableCollection)
		variables := make(map[string]*figma.Variable)

		for id, coll := range vars.Meta.VariableCollections {
			if len(args.Collections) > 0 && !containsString(args.Collections, coll.Name) {
				continue
			}
			collections[id] = coll
		}

		for id, v := range vars.Meta.Variables {
			if _, ok := collections[v.VariableCollectionID]; ok {
				variables[id] = v
			}
		}

		// Generate output
		var content string
		switch args.Format {
		case "css":
			content = generateCSSTokens(variables, collections, args.Prefix, args.Modes)
		case "scss":
			content = generateSCSSTokens(variables, collections, args.Prefix, args.Modes)
		case "json":
			content = generateJSONTokens(variables, collections, args.Modes)
		case "js", "ts":
			content = generateJSTokens(variables, collections, args.Prefix, args.Modes, args.Format == "ts")
		case "tailwind":
			content = generateTailwindTokens(variables, collections, args.Modes)
		default:
			return nil, nil, fmt.Errorf("unsupported format: %s", args.Format)
		}

		// Write file
		dir := filepath.Dir(args.OutputPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, nil, fmt.Errorf("creating directory: %w", err)
		}

		if err := os.WriteFile(args.OutputPath, []byte(content), 0644); err != nil {
			return nil, nil, fmt.Errorf("writing file: %w", err)
		}

		// Build result
		collectionNames := make([]string, 0, len(collections))
		for _, coll := range collections {
			collectionNames = append(collectionNames, coll.Name)
		}

		result := &ExportTokensResult{
			Path:        args.OutputPath,
			TokensCount: len(variables),
			Collections: collectionNames,
		}

		textOutput := fmt.Sprintf("Exported %d tokens to %s\nCollections: %s",
			result.TokensCount, result.Path, strings.Join(result.Collections, ", "))

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: textOutput},
			},
		}, result, nil
	})
}

func generateCSSTokens(variables map[string]*figma.Variable, collections map[string]*figma.VariableCollection, prefix string, modes []string) string {
	var sb strings.Builder

	sb.WriteString("/* Design Tokens - Generated by figma-query */\n\n")
	sb.WriteString(":root {\n")

	for _, v := range variables {
		coll := collections[v.VariableCollectionID]
		if coll == nil {
			continue
		}

		// Use default mode or first matching mode
		modeID := coll.DefaultModeID
		if len(modes) > 0 {
			for _, m := range coll.Modes {
				if containsString(modes, m.Name) {
					modeID = m.ModeID
					break
				}
			}
		}

		value := v.ValuesByMode[modeID]
		cssValue := formatTokenValue(v.ResolvedType, value)
		varName := formatVarName(v.Name, prefix)

		sb.WriteString(fmt.Sprintf("  --%s: %s;\n", varName, cssValue))
	}

	sb.WriteString("}\n")
	return sb.String()
}

func generateSCSSTokens(variables map[string]*figma.Variable, collections map[string]*figma.VariableCollection, prefix string, modes []string) string {
	var sb strings.Builder

	sb.WriteString("// Design Tokens - Generated by figma-query\n\n")

	for _, v := range variables {
		coll := collections[v.VariableCollectionID]
		if coll == nil {
			continue
		}

		modeID := coll.DefaultModeID
		value := v.ValuesByMode[modeID]
		cssValue := formatTokenValue(v.ResolvedType, value)
		varName := formatVarName(v.Name, prefix)

		sb.WriteString(fmt.Sprintf("$%s: %s;\n", varName, cssValue))
	}

	return sb.String()
}

func generateJSONTokens(variables map[string]*figma.Variable, collections map[string]*figma.VariableCollection, modes []string) string {
	tokens := make(map[string]interface{})

	for _, v := range variables {
		coll := collections[v.VariableCollectionID]
		if coll == nil {
			continue
		}

		modeID := coll.DefaultModeID
		value := v.ValuesByMode[modeID]

		tokens[v.Name] = map[string]interface{}{
			"value":    value,
			"type":     v.ResolvedType,
			"collection": coll.Name,
		}
	}

	b, _ := json.MarshalIndent(tokens, "", "  ")
	return string(b)
}

func generateJSTokens(variables map[string]*figma.Variable, collections map[string]*figma.VariableCollection, prefix string, modes []string, typescript bool) string {
	var sb strings.Builder

	sb.WriteString("// Design Tokens - Generated by figma-query\n\n")

	if typescript {
		sb.WriteString("export const tokens = {\n")
	} else {
		sb.WriteString("export const tokens = {\n")
	}

	for _, v := range variables {
		coll := collections[v.VariableCollectionID]
		if coll == nil {
			continue
		}

		modeID := coll.DefaultModeID
		value := v.ValuesByMode[modeID]
		cssValue := formatTokenValue(v.ResolvedType, value)
		varName := formatJSVarName(v.Name)

		sb.WriteString(fmt.Sprintf("  %s: '%s',\n", varName, cssValue))
	}

	if typescript {
		sb.WriteString("} as const;\n")
	} else {
		sb.WriteString("};\n")
	}

	return sb.String()
}

func generateTailwindTokens(variables map[string]*figma.Variable, collections map[string]*figma.VariableCollection, modes []string) string {
	config := map[string]interface{}{
		"theme": map[string]interface{}{
			"extend": map[string]interface{}{},
		},
	}

	colors := make(map[string]string)
	spacing := make(map[string]string)

	for _, v := range variables {
		coll := collections[v.VariableCollectionID]
		if coll == nil {
			continue
		}

		modeID := coll.DefaultModeID
		value := v.ValuesByMode[modeID]
		cssValue := formatTokenValue(v.ResolvedType, value)
		varName := formatVarName(v.Name, "")

		switch v.ResolvedType {
		case "COLOR":
			colors[varName] = cssValue
		case "FLOAT":
			spacing[varName] = cssValue
		}
	}

	extend := config["theme"].(map[string]interface{})["extend"].(map[string]interface{})
	if len(colors) > 0 {
		extend["colors"] = colors
	}
	if len(spacing) > 0 {
		extend["spacing"] = spacing
	}

	b, _ := json.MarshalIndent(config, "", "  ")
	return "// tailwind.config.js extend\nmodule.exports = " + string(b) + ";\n"
}

func formatTokenValue(resolvedType string, value json.RawMessage) string {
	switch resolvedType {
	case "COLOR":
		var color map[string]float64
		if err := json.Unmarshal(value, &color); err == nil {
			r := int(color["r"] * 255)
			g := int(color["g"] * 255)
			b := int(color["b"] * 255)
			a := color["a"]
			if a >= 1 {
				return fmt.Sprintf("#%02x%02x%02x", r, g, b)
			}
			return fmt.Sprintf("rgba(%d, %d, %d, %.2f)", r, g, b, a)
		}
	case "FLOAT":
		var f float64
		if err := json.Unmarshal(value, &f); err == nil {
			return fmt.Sprintf("%.0fpx", f)
		}
	case "STRING":
		var s string
		if err := json.Unmarshal(value, &s); err == nil {
			return s
		}
	}

	return string(value)
}

func formatVarName(name, prefix string) string {
	// Convert path separators to dashes
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ToLower(name)

	if prefix != "" {
		return prefix + "-" + name
	}
	return name
}

func formatJSVarName(name string) string {
	// Convert to camelCase
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '/' || r == '-' || r == ' '
	})

	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
		}
	}

	if len(parts) > 0 {
		parts[0] = strings.ToLower(parts[0])
	}

	return strings.Join(parts, "")
}

// DownloadImageArgs contains arguments for the download_image tool.
type DownloadImageArgs struct {
	FileKey   string   `json:"file_key" jsonschema:"Figma file key"`
	ImageRefs []string `json:"image_refs,omitempty" jsonschema:"Image reference IDs (from fills/strokes/backgrounds)"`
	NodeIDs   []string `json:"node_ids,omitempty" jsonschema:"Node IDs to render as images"`
	OutputDir string   `json:"output_dir" jsonschema:"Directory to save images"`
	Format    string   `json:"format,omitempty" jsonschema:"Image format for renders: png (default), svg, jpg, pdf"`
	Scale     float64  `json:"scale,omitempty" jsonschema:"Scale for renders: 1 (default), 2, 3, etc."`
}

// DownloadImageResult contains the result of download_image.
type DownloadImageResult struct {
	Downloaded []DownloadedImage `json:"downloaded"`
	Failed     []string          `json:"failed,omitempty"`
}

// DownloadedImage represents a downloaded image file.
type DownloadedImage struct {
	Ref      string `json:"ref,omitempty"`      // Image ref or node ID
	Path     string `json:"path"`               // File path where saved
	Size     int    `json:"size"`               // File size in bytes
	Type     string `json:"type"`               // "fill" or "render"
}

func registerDownloadImageTool(server *mcp.Server, r *Registry) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "download_image",
		Description: "Download images by reference ID (from fills/strokes/backgrounds) or render nodes as images.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args DownloadImageArgs) (*mcp.CallToolResult, *DownloadImageResult, error) {
		if args.FileKey == "" {
			return nil, nil, fmt.Errorf("file_key is required")
		}
		if len(args.ImageRefs) == 0 && len(args.NodeIDs) == 0 {
			return nil, nil, fmt.Errorf("either image_refs or node_ids is required")
		}
		if args.OutputDir == "" {
			return nil, nil, fmt.Errorf("output_dir is required")
		}

		if !r.HasClient() {
			return nil, nil, fmt.Errorf("Figma API not configured")
		}

		// Set defaults
		format := args.Format
		if format == "" {
			format = "png"
		}
		scale := args.Scale
		if scale == 0 {
			scale = 1
		}

		// Create output directory
		if err := os.MkdirAll(args.OutputDir, 0755); err != nil {
			return nil, nil, fmt.Errorf("creating output directory: %w", err)
		}

		result := &DownloadImageResult{
			Downloaded: make([]DownloadedImage, 0),
		}

		// Download image fills
		if len(args.ImageRefs) > 0 {
			imageFillURLs, err := r.Client().GetImageFills(ctx, args.FileKey)
			if err != nil {
				result.Failed = append(result.Failed, fmt.Sprintf("fetching image fills: %v", err))
			} else {
				for _, ref := range args.ImageRefs {
					imageURL, ok := imageFillURLs[ref]
					if !ok || imageURL == "" {
						result.Failed = append(result.Failed, fmt.Sprintf("no URL for image ref: %s", ref))
						continue
					}

					data, err := r.Client().DownloadImage(ctx, imageURL)
					if err != nil {
						result.Failed = append(result.Failed, fmt.Sprintf("downloading %s: %v", ref, err))
						continue
					}

					// Determine extension from URL
					ext := "png"
					if strings.Contains(imageURL, ".jpg") || strings.Contains(imageURL, ".jpeg") {
						ext = "jpg"
					} else if strings.Contains(imageURL, ".svg") {
						ext = "svg"
					} else if strings.Contains(imageURL, ".gif") {
						ext = "gif"
					} else if strings.Contains(imageURL, ".webp") {
						ext = "webp"
					}

					filename := fmt.Sprintf("%s.%s", strings.ReplaceAll(ref, ":", "-"), ext)
					filePath := filepath.Join(args.OutputDir, filename)

					if err := os.WriteFile(filePath, data, 0644); err != nil {
						result.Failed = append(result.Failed, fmt.Sprintf("writing %s: %v", ref, err))
						continue
					}

					result.Downloaded = append(result.Downloaded, DownloadedImage{
						Ref:  ref,
						Path: filePath,
						Size: len(data),
						Type: "fill",
					})
				}
			}
		}

		// Render nodes as images
		if len(args.NodeIDs) > 0 {
			// Get node names for filenames
			nodeNames := make(map[string]string)
			nodes, err := r.Client().GetFileNodes(ctx, args.FileKey, args.NodeIDs, nil)
			if err == nil {
				for id, wrapper := range nodes.Nodes {
					if wrapper.Document != nil {
						nodeNames[id] = wrapper.Document.Name
					}
				}
			}

			images, err := r.Client().GetImages(ctx, args.FileKey, args.NodeIDs, &figma.ImageExportOptions{
				Format: format,
				Scale:  scale,
			})
			if err != nil {
				result.Failed = append(result.Failed, fmt.Sprintf("rendering nodes: %v", err))
			} else {
				for id, imageURL := range images.Images {
					if imageURL == "" {
						result.Failed = append(result.Failed, fmt.Sprintf("no render for node: %s", id))
						continue
					}

					data, err := r.Client().DownloadImage(ctx, imageURL)
					if err != nil {
						result.Failed = append(result.Failed, fmt.Sprintf("downloading render %s: %v", id, err))
						continue
					}

					// Build filename
					name := nodeNames[id]
					if name == "" {
						name = strings.ReplaceAll(id, ":", "-")
					} else {
						name = sanitizeName(name)
					}
					if scale != 1 {
						name = fmt.Sprintf("%s@%dx", name, int(scale))
					}
					filename := fmt.Sprintf("%s.%s", name, format)
					filePath := filepath.Join(args.OutputDir, filename)

					if err := os.WriteFile(filePath, data, 0644); err != nil {
						result.Failed = append(result.Failed, fmt.Sprintf("writing render %s: %v", id, err))
						continue
					}

					result.Downloaded = append(result.Downloaded, DownloadedImage{
						Ref:  id,
						Path: filePath,
						Size: len(data),
						Type: "render",
					})
				}
			}
		}

		// Format output as JSON (this tool is primarily for programmatic use)
		b, _ := json.MarshalIndent(result, "", "  ")
		textOutput := string(b)

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: textOutput},
			},
		}, result, nil
	})
}
