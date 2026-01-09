package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/standardbeagle/figma-query/internal/figma"
)

// SyncFileArgs contains the arguments for the sync_file tool.
type SyncFileArgs struct {
	FileKey     string       `json:"file_key" jsonschema:"Figma file key (from URL: figma.com/file/<KEY>/...)"`
	OutputDir   string       `json:"output_dir,omitempty" jsonschema:"Base directory for export (default: ./figma-export)"`
	Include     []string     `json:"include,omitempty" jsonschema:"What to export: pages components styles variables assets"`
	Assets      AssetOptions `json:"assets,omitempty" jsonschema:"Asset export options"`
	Incremental bool         `json:"incremental,omitempty" jsonschema:"Only update changed nodes (default: true)"`
	Format      string       `json:"format,omitempty" jsonschema:"Response format: text (default) or json"`
}

// AssetOptions contains options for asset export.
type AssetOptions struct {
	Formats []string  `json:"formats,omitempty" jsonschema:"Image formats: png svg pdf jpg"`
	Scales  []float64 `json:"scales,omitempty" jsonschema:"Export scales: 1 2 3 for @1x @2x @3x"`
	MaxSize int       `json:"max_size,omitempty" jsonschema:"Skip assets larger than N bytes"`
}

// SyncFileResult contains the result of the sync_file tool.
type SyncFileResult struct {
	ExportPath  string           `json:"export_path"`
	Stats       SyncStats        `json:"stats"`
	TreePreview string           `json:"tree_preview,omitempty"`
	Errors      []string         `json:"errors,omitempty"`
}

// SyncStats contains export statistics.
type SyncStats struct {
	Pages       int   `json:"pages"`
	Nodes       int   `json:"nodes"`
	Components  int   `json:"components"`
	Styles      int   `json:"styles"`
	Variables   int   `json:"variables"`
	Assets      int   `json:"assets"`
	ImageFills  int   `json:"image_fills"`
	DurationMS  int64 `json:"duration_ms"`
}

// ImageCollector tracks image references and nodes to export during sync.
type ImageCollector struct {
	// ImageRefs maps imageRef values to their source node IDs for context
	ImageRefs map[string][]string
	// ExportNodes contains node IDs that have explicit export settings
	ExportNodes map[string]*figma.Node
}

// NewImageCollector creates a new ImageCollector.
func NewImageCollector() *ImageCollector {
	return &ImageCollector{
		ImageRefs:   make(map[string][]string),
		ExportNodes: make(map[string]*figma.Node),
	}
}

// AddImageRef records an image reference from a node.
func (ic *ImageCollector) AddImageRef(imageRef, nodeID string) {
	if imageRef != "" {
		ic.ImageRefs[imageRef] = append(ic.ImageRefs[imageRef], nodeID)
	}
}

// AddExportNode records a node that has export settings.
func (ic *ImageCollector) AddExportNode(node *figma.Node) {
	if len(node.ExportSettings) > 0 {
		ic.ExportNodes[node.ID] = node
	}
}

func registerSyncFileTool(server *mcp.Server, r *Registry) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sync_file",
		Description: "Export entire Figma file to nested folder structure for grep/jq access. Creates local cache.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args SyncFileArgs) (*mcp.CallToolResult, *SyncFileResult, error) {
		if !r.HasClient() {
			return nil, nil, fmt.Errorf("Figma API not configured. Set FIGMA_ACCESS_TOKEN environment variable")
		}

		if args.FileKey == "" {
			return nil, nil, fmt.Errorf("file_key is required")
		}

		startTime := time.Now()

		// Set defaults
		outputDir := args.OutputDir
		if outputDir == "" {
			outputDir = r.ExportDir()
		}

		include := args.Include
		if len(include) == 0 {
			include = []string{"pages", "components", "styles", "variables", "assets"}
		}

		// Fetch the file
		file, err := r.Client().GetFile(ctx, args.FileKey, &figma.GetFileOptions{
			Geometry: "paths",
		})
		if err != nil {
			return nil, nil, fmt.Errorf("fetching file: %w", err)
		}

		// Create export directory
		exportPath := filepath.Join(outputDir, sanitizeName(file.Name))
		if err := os.MkdirAll(exportPath, 0755); err != nil {
			return nil, nil, fmt.Errorf("creating export directory: %w", err)
		}

		stats := SyncStats{}
		var errors []string
		var treeLines []string
		imageCollector := NewImageCollector()

		// Export metadata
		meta := map[string]interface{}{
			"name":          file.Name,
			"version":       file.Version,
			"lastModified":  file.LastModified,
			"exportedAt":    time.Now().UTC().Format(time.RFC3339),
			"fileKey":       args.FileKey,
			"schemaVersion": file.SchemaVersion,
		}
		if err := writeJSON(filepath.Join(exportPath, "_meta.json"), meta); err != nil {
			errors = append(errors, fmt.Sprintf("writing meta: %v", err))
		}

		// Build node index
		nodeIndex := make(map[string]string) // node_id -> path

		// Export pages
		if contains(include, "pages") && file.Document != nil {
			pagesDir := filepath.Join(exportPath, "pages")
			if err := os.MkdirAll(pagesDir, 0755); err != nil {
				errors = append(errors, fmt.Sprintf("creating pages dir: %v", err))
			}

			for _, page := range file.Document.Children {
				if page.Type == figma.NodeTypeCanvas {
					stats.Pages++
					pagePath := filepath.Join(pagesDir, sanitizeName(page.Name)+"-"+sanitizeID(page.ID))
					treeLines = append(treeLines, fmt.Sprintf("Page: %s [%s]", page.Name, page.ID))

					nodeCount, pageErrors := exportNode(ctx, page, pagePath, 1, &treeLines, nodeIndex, imageCollector)
					stats.Nodes += nodeCount
					errors = append(errors, pageErrors...)
				}
			}
		}

		// Export components
		if contains(include, "components") && len(file.Components) > 0 {
			componentsDir := filepath.Join(exportPath, "components")
			if err := os.MkdirAll(componentsDir, 0755); err != nil {
				errors = append(errors, fmt.Sprintf("creating components dir: %v", err))
			}

			componentList := make([]map[string]interface{}, 0, len(file.Components))
			for id, comp := range file.Components {
				stats.Components++
				componentList = append(componentList, map[string]interface{}{
					"id":          id,
					"key":         comp.Key,
					"name":        comp.Name,
					"description": comp.Description,
				})
			}

			if err := writeJSON(filepath.Join(componentsDir, "_components.json"), componentList); err != nil {
				errors = append(errors, fmt.Sprintf("writing components: %v", err))
			}
		}

		// Export styles
		if contains(include, "styles") && len(file.Styles) > 0 {
			stylesDir := filepath.Join(exportPath, "styles")
			if err := os.MkdirAll(stylesDir, 0755); err != nil {
				errors = append(errors, fmt.Sprintf("creating styles dir: %v", err))
			}

			// Group styles by type
			colorStyles := make([]map[string]interface{}, 0)
			textStyles := make([]map[string]interface{}, 0)
			effectStyles := make([]map[string]interface{}, 0)
			gridStyles := make([]map[string]interface{}, 0)

			for id, style := range file.Styles {
				stats.Styles++
				styleData := map[string]interface{}{
					"id":          id,
					"key":         style.Key,
					"name":        style.Name,
					"description": style.Description,
				}

				switch style.StyleType {
				case figma.StyleTypeFill:
					colorStyles = append(colorStyles, styleData)
				case figma.StyleTypeText:
					textStyles = append(textStyles, styleData)
				case figma.StyleTypeEffect:
					effectStyles = append(effectStyles, styleData)
				case figma.StyleTypeGrid:
					gridStyles = append(gridStyles, styleData)
				}
			}

			if len(colorStyles) > 0 {
				writeJSON(filepath.Join(stylesDir, "colors.json"), colorStyles)
			}
			if len(textStyles) > 0 {
				writeJSON(filepath.Join(stylesDir, "typography.json"), textStyles)
			}
			if len(effectStyles) > 0 {
				writeJSON(filepath.Join(stylesDir, "effects.json"), effectStyles)
			}
			if len(gridStyles) > 0 {
				writeJSON(filepath.Join(stylesDir, "grids.json"), gridStyles)
			}
		}

		// Export variables
		if contains(include, "variables") {
			vars, err := r.Client().GetLocalVariables(ctx, args.FileKey)
			if err == nil && vars.Meta != nil {
				varsDir := filepath.Join(exportPath, "variables")
				if err := os.MkdirAll(varsDir, 0755); err != nil {
					errors = append(errors, fmt.Sprintf("creating variables dir: %v", err))
				}

				stats.Variables = len(vars.Meta.Variables)

				// Export collections
				collectionsDir := filepath.Join(varsDir, "collections")
				os.MkdirAll(collectionsDir, 0755)

				for _, coll := range vars.Meta.VariableCollections {
					collData := map[string]interface{}{
						"id":            coll.ID,
						"name":          coll.Name,
						"key":           coll.Key,
						"modes":         coll.Modes,
						"defaultModeId": coll.DefaultModeID,
						"variableIds":   coll.VariableIDs,
					}
					writeJSON(filepath.Join(collectionsDir, sanitizeName(coll.Name)+".json"), collData)
				}

				// Export all variables
				writeJSON(filepath.Join(varsDir, "tokens.json"), vars.Meta.Variables)
			}
		}

		// Export assets (image fills and node renders)
		if contains(include, "assets") {
			assetsDir := filepath.Join(exportPath, "assets")
			if err := os.MkdirAll(assetsDir, 0755); err != nil {
				errors = append(errors, fmt.Sprintf("creating assets dir: %v", err))
			}

			// Set default formats and scales
			formats := args.Assets.Formats
			if len(formats) == 0 {
				formats = []string{"png"}
			}
			scales := args.Assets.Scales
			if len(scales) == 0 {
				scales = []float64{1}
			}

			// Export image fills (backgrounds, fill images, etc.)
			if len(imageCollector.ImageRefs) > 0 {
				imageFillsDir := filepath.Join(assetsDir, "fills")
				if err := os.MkdirAll(imageFillsDir, 0755); err != nil {
					errors = append(errors, fmt.Sprintf("creating fills dir: %v", err))
				}

				// Get image fill URLs from Figma
				imageFillURLs, err := r.Client().GetImageFills(ctx, args.FileKey)
				if err != nil {
					errors = append(errors, fmt.Sprintf("fetching image fills: %v", err))
				} else {
					// Download each image fill
					for imageRef, nodeIDs := range imageCollector.ImageRefs {
						imageURL, ok := imageFillURLs[imageRef]
						if !ok || imageURL == "" {
							errors = append(errors, fmt.Sprintf("no URL for image ref %s (used in %v)", imageRef, nodeIDs))
							continue
						}

						// Download the image
						data, err := r.Client().DownloadImage(ctx, imageURL)
						if err != nil {
							errors = append(errors, fmt.Sprintf("downloading image %s: %v", imageRef, err))
							continue
						}

						// Skip if over size limit
						if args.Assets.MaxSize > 0 && len(data) > args.Assets.MaxSize {
							continue
						}

						// Determine file extension from URL or default to png
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

						filename := fmt.Sprintf("%s.%s", sanitizeID(imageRef), ext)
						filePath := filepath.Join(imageFillsDir, filename)

						if err := os.WriteFile(filePath, data, 0644); err != nil {
							errors = append(errors, fmt.Sprintf("writing image %s: %v", imageRef, err))
							continue
						}

						stats.ImageFills++
					}
				}
			}

			// Export nodes with export settings (icons, rendered images)
			if len(imageCollector.ExportNodes) > 0 {
				rendersDir := filepath.Join(assetsDir, "renders")
				if err := os.MkdirAll(rendersDir, 0755); err != nil {
					errors = append(errors, fmt.Sprintf("creating renders dir: %v", err))
				}

				// Collect node IDs
				nodeIDs := make([]string, 0, len(imageCollector.ExportNodes))
				for id := range imageCollector.ExportNodes {
					nodeIDs = append(nodeIDs, id)
				}

				// Export in each format and scale
				for _, format := range formats {
					for _, scale := range scales {
						images, err := r.Client().GetImages(ctx, args.FileKey, nodeIDs, &figma.ImageExportOptions{
							Format: format,
							Scale:  scale,
						})
						if err != nil {
							errors = append(errors, fmt.Sprintf("exporting images: %v", err))
							continue
						}

						for id, imageURL := range images.Images {
							if imageURL == "" {
								continue
							}

							data, err := r.Client().DownloadImage(ctx, imageURL)
							if err != nil {
								errors = append(errors, fmt.Sprintf("downloading render %s: %v", id, err))
								continue
							}

							// Skip if over size limit
							if args.Assets.MaxSize > 0 && len(data) > args.Assets.MaxSize {
								continue
							}

							// Build filename using node name
							node := imageCollector.ExportNodes[id]
							name := sanitizeName(node.Name)
							if scale != 1 {
								name = fmt.Sprintf("%s@%dx", name, int(scale))
							}
							filename := fmt.Sprintf("%s.%s", name, format)
							filePath := filepath.Join(rendersDir, filename)

							if err := os.WriteFile(filePath, data, 0644); err != nil {
								errors = append(errors, fmt.Sprintf("writing render %s: %v", id, err))
								continue
							}

							stats.Assets++
						}
					}
				}
			}
		}

		// Write tree file
		treeContent := strings.Join(treeLines, "\n")
		if err := os.WriteFile(filepath.Join(exportPath, "_tree.txt"), []byte(treeContent), 0644); err != nil {
			errors = append(errors, fmt.Sprintf("writing tree: %v", err))
		}

		// Write index file
		if err := writeJSON(filepath.Join(exportPath, "_index.json"), nodeIndex); err != nil {
			errors = append(errors, fmt.Sprintf("writing index: %v", err))
		}

		stats.DurationMS = time.Since(startTime).Milliseconds()

		// Build result
		result := &SyncFileResult{
			ExportPath: exportPath,
			Stats:      stats,
			Errors:     errors,
		}

		// Tree preview (first 50 lines)
		previewLines := treeLines
		if len(previewLines) > 50 {
			previewLines = previewLines[:50]
			previewLines = append(previewLines, fmt.Sprintf("... and %d more", len(treeLines)-50))
		}
		result.TreePreview = strings.Join(previewLines, "\n")

		// Format output
		var textOutput string
		if args.Format == "json" {
			b, _ := json.MarshalIndent(result, "", "  ")
			textOutput = string(b)
		} else {
			textOutput = formatSyncResult(result)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: textOutput},
			},
		}, result, nil
	})
}

func exportNode(ctx context.Context, node *figma.Node, basePath string, depth int, treeLines *[]string, nodeIndex map[string]string, imageCollector *ImageCollector) (int, []string) {
	var errors []string
	nodeCount := 1

	// Create directory for this node
	if err := os.MkdirAll(basePath, 0755); err != nil {
		errors = append(errors, fmt.Sprintf("creating dir for %s: %v", node.ID, err))
		return nodeCount, errors
	}

	// Add to index
	nodeIndex[node.ID] = basePath

	// Add to tree
	indent := strings.Repeat("│   ", depth-1) + "├── "
	*treeLines = append(*treeLines, fmt.Sprintf("%s%s [%s] %s", indent, node.Name, node.ID, node.Type))

	// Collect image references from fills
	for _, fill := range node.Fills {
		if fill.Type == "IMAGE" && fill.ImageRef != "" {
			imageCollector.AddImageRef(fill.ImageRef, node.ID)
		}
		if fill.GifRef != "" {
			imageCollector.AddImageRef(fill.GifRef, node.ID)
		}
	}

	// Collect image references from strokes
	for _, stroke := range node.Strokes {
		if stroke.Type == "IMAGE" && stroke.ImageRef != "" {
			imageCollector.AddImageRef(stroke.ImageRef, node.ID)
		}
	}

	// Collect image references from background (for frames, canvases)
	for _, bg := range node.Background {
		if bg.Type == "IMAGE" && bg.ImageRef != "" {
			imageCollector.AddImageRef(bg.ImageRef, node.ID)
		}
	}

	// Collect nodes with export settings
	if len(node.ExportSettings) > 0 {
		imageCollector.AddExportNode(node)
	}

	// Write node data
	if err := writeJSON(filepath.Join(basePath, "_node.json"), node); err != nil {
		errors = append(errors, fmt.Sprintf("writing node %s: %v", node.ID, err))
	}

	// Extract and write CSS properties
	cssProps := extractCSSProperties(node)
	if len(cssProps) > 0 {
		if err := writeJSON(filepath.Join(basePath, "_css.json"), cssProps); err != nil {
			errors = append(errors, fmt.Sprintf("writing css for %s: %v", node.ID, err))
		}
	}

	// Extract and write token references
	tokens := extractTokenReferences(node)
	if len(tokens) > 0 {
		if err := writeJSON(filepath.Join(basePath, "_tokens.json"), tokens); err != nil {
			errors = append(errors, fmt.Sprintf("writing tokens for %s: %v", node.ID, err))
		}
	}

	// Export children
	if len(node.Children) > 0 {
		childrenDir := filepath.Join(basePath, "children")
		for _, child := range node.Children {
			childPath := filepath.Join(childrenDir, sanitizeName(child.Name)+"-"+sanitizeID(child.ID))
			childCount, childErrors := exportNode(ctx, child, childPath, depth+1, treeLines, nodeIndex, imageCollector)
			nodeCount += childCount
			errors = append(errors, childErrors...)
		}
	}

	return nodeCount, errors
}

func extractCSSProperties(node *figma.Node) map[string]interface{} {
	css := make(map[string]interface{})

	// Bounds
	if node.AbsoluteBoundingBox != nil {
		css["width"] = node.AbsoluteBoundingBox.Width
		css["height"] = node.AbsoluteBoundingBox.Height
	}

	// Fills
	if len(node.Fills) > 0 {
		css["fills"] = node.Fills
		// Convert first solid fill to background-color
		for _, fill := range node.Fills {
			if fill.Type == "SOLID" && fill.Color != nil {
				visible := fill.Visible == nil || *fill.Visible
				if visible {
					css["backgroundColor"] = colorToCSS(fill.Color, fill.Opacity)
					break
				}
			}
		}
	}

	// Strokes
	if len(node.Strokes) > 0 {
		css["strokes"] = node.Strokes
		css["strokeWeight"] = node.StrokeWeight
		css["strokeAlign"] = node.StrokeAlign
		// Convert to border
		for _, stroke := range node.Strokes {
			if stroke.Type == "SOLID" && stroke.Color != nil {
				visible := stroke.Visible == nil || *stroke.Visible
				if visible {
					css["borderColor"] = colorToCSS(stroke.Color, stroke.Opacity)
					css["borderWidth"] = node.StrokeWeight
					break
				}
			}
		}
	}

	// Corner radius
	if node.CornerRadius > 0 {
		css["borderRadius"] = node.CornerRadius
	}
	if len(node.RectangleCornerRadii) == 4 {
		css["borderRadii"] = node.RectangleCornerRadii
	}

	// Effects (shadows, blur)
	if len(node.Effects) > 0 {
		css["effects"] = node.Effects
		var shadows []string
		for _, effect := range node.Effects {
			visible := effect.Visible == nil || *effect.Visible
			if !visible {
				continue
			}
			if effect.Type == "DROP_SHADOW" || effect.Type == "INNER_SHADOW" {
				shadow := formatShadow(&effect)
				if shadow != "" {
					shadows = append(shadows, shadow)
				}
			}
		}
		if len(shadows) > 0 {
			css["boxShadow"] = strings.Join(shadows, ", ")
		}
	}

	// Opacity
	if node.Opacity != nil && *node.Opacity < 1 {
		css["opacity"] = *node.Opacity
	}

	// Blend mode
	if node.BlendMode != "" && node.BlendMode != "PASS_THROUGH" {
		css["mixBlendMode"] = blendModeToCSS(node.BlendMode)
	}

	// Layout (auto-layout / flexbox)
	if node.LayoutMode != "" {
		css["display"] = "flex"
		if node.LayoutMode == "VERTICAL" {
			css["flexDirection"] = "column"
		} else {
			css["flexDirection"] = "row"
		}

		// Padding
		if node.PaddingTop > 0 || node.PaddingRight > 0 || node.PaddingBottom > 0 || node.PaddingLeft > 0 {
			css["padding"] = fmt.Sprintf("%.0fpx %.0fpx %.0fpx %.0fpx",
				node.PaddingTop, node.PaddingRight, node.PaddingBottom, node.PaddingLeft)
		}

		// Gap
		if node.ItemSpacing > 0 {
			css["gap"] = node.ItemSpacing
		}

		// Alignment
		css["justifyContent"] = alignToCSS(node.PrimaryAxisAlignItems)
		css["alignItems"] = alignToCSS(node.CounterAxisAlignItems)
	}

	// Typography (for text nodes)
	if node.Style != nil {
		css["fontFamily"] = node.Style.FontFamily
		css["fontSize"] = node.Style.FontSize
		css["fontWeight"] = node.Style.FontWeight
		css["lineHeight"] = node.Style.LineHeightPx
		css["letterSpacing"] = node.Style.LetterSpacing
		css["textAlign"] = strings.ToLower(node.Style.TextAlignHorizontal)
	}

	return css
}

func extractTokenReferences(node *figma.Node) map[string]interface{} {
	tokens := make(map[string]interface{})

	if len(node.BoundVariables) > 0 {
		for prop, varRef := range node.BoundVariables {
			tokens[prop] = map[string]string{
				"type": varRef.Type,
				"id":   varRef.ID,
			}
		}
	}

	// Check fills for variable bindings
	for i, fill := range node.Fills {
		if len(fill.BoundVariables) > 0 {
			tokens[fmt.Sprintf("fills[%d]", i)] = fill.BoundVariables
		}
	}

	// Check strokes
	for i, stroke := range node.Strokes {
		if len(stroke.BoundVariables) > 0 {
			tokens[fmt.Sprintf("strokes[%d]", i)] = stroke.BoundVariables
		}
	}

	// Check effects
	for i, effect := range node.Effects {
		if len(effect.BoundVariables) > 0 {
			tokens[fmt.Sprintf("effects[%d]", i)] = effect.BoundVariables
		}
	}

	return tokens
}

func colorToCSS(c *figma.Color, opacity *float64) string {
	if c == nil {
		return ""
	}
	a := c.A
	if opacity != nil {
		a *= *opacity
	}
	if a >= 1 {
		return fmt.Sprintf("rgb(%.0f, %.0f, %.0f)", c.R*255, c.G*255, c.B*255)
	}
	return fmt.Sprintf("rgba(%.0f, %.0f, %.0f, %.2f)", c.R*255, c.G*255, c.B*255, a)
}

func formatShadow(e *figma.Effect) string {
	if e.Color == nil {
		return ""
	}
	x, y := 0.0, 0.0
	if e.Offset != nil {
		x, y = e.Offset.X, e.Offset.Y
	}
	inset := ""
	if e.Type == "INNER_SHADOW" {
		inset = "inset "
	}
	color := colorToCSS(e.Color, nil)
	return fmt.Sprintf("%s%.0fpx %.0fpx %.0fpx %.0fpx %s", inset, x, y, e.Radius, e.Spread, color)
}

func blendModeToCSS(mode string) string {
	modes := map[string]string{
		"MULTIPLY":    "multiply",
		"SCREEN":      "screen",
		"OVERLAY":     "overlay",
		"DARKEN":      "darken",
		"LIGHTEN":     "lighten",
		"COLOR_DODGE": "color-dodge",
		"COLOR_BURN":  "color-burn",
		"HARD_LIGHT":  "hard-light",
		"SOFT_LIGHT":  "soft-light",
		"DIFFERENCE":  "difference",
		"EXCLUSION":   "exclusion",
		"HUE":         "hue",
		"SATURATION":  "saturation",
		"COLOR":       "color",
		"LUMINOSITY":  "luminosity",
	}
	if css, ok := modes[mode]; ok {
		return css
	}
	return "normal"
}

func alignToCSS(align string) string {
	switch align {
	case "MIN":
		return "flex-start"
	case "CENTER":
		return "center"
	case "MAX":
		return "flex-end"
	case "SPACE_BETWEEN":
		return "space-between"
	default:
		return "flex-start"
	}
}

func writeJSON(path string, data interface{}) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

var invalidCharsRegex = regexp.MustCompile(`[<>:"/\\|?*]`)

func sanitizeName(name string) string {
	// Replace invalid characters
	name = invalidCharsRegex.ReplaceAllString(name, "-")
	// Replace spaces
	name = strings.ReplaceAll(name, " ", "-")
	// Lowercase
	name = strings.ToLower(name)
	// Truncate if too long
	if len(name) > 50 {
		name = name[:50]
	}
	return name
}

func sanitizeID(id string) string {
	return strings.ReplaceAll(id, ":", "-")
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func formatSyncResult(r *SyncFileResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Exported to: %s\n\n", r.ExportPath))
	sb.WriteString("Statistics\n")
	sb.WriteString("----------\n")
	sb.WriteString(fmt.Sprintf("Pages:       %d\n", r.Stats.Pages))
	sb.WriteString(fmt.Sprintf("Nodes:       %d\n", r.Stats.Nodes))
	sb.WriteString(fmt.Sprintf("Components:  %d\n", r.Stats.Components))
	sb.WriteString(fmt.Sprintf("Styles:      %d\n", r.Stats.Styles))
	sb.WriteString(fmt.Sprintf("Variables:   %d\n", r.Stats.Variables))
	sb.WriteString(fmt.Sprintf("Image Fills: %d\n", r.Stats.ImageFills))
	sb.WriteString(fmt.Sprintf("Assets:      %d\n", r.Stats.Assets))
	sb.WriteString(fmt.Sprintf("Duration:    %dms\n", r.Stats.DurationMS))

	if len(r.Errors) > 0 {
		sb.WriteString(fmt.Sprintf("\nWarnings: %d\n", len(r.Errors)))
		for _, e := range r.Errors[:min(5, len(r.Errors))] {
			sb.WriteString(fmt.Sprintf("  - %s\n", e))
		}
	}

	sb.WriteString("\nTree Preview\n")
	sb.WriteString("------------\n")
	sb.WriteString(r.TreePreview)

	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
