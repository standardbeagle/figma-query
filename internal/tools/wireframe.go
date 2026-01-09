package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/standardbeagle/figma-query/internal/figma"
)

// WireframeArgs contains arguments for the wireframe tool.
type WireframeArgs struct {
	FileKey      string   `json:"file_key" jsonschema:"Figma file key"`
	NodeID       string   `json:"node_id" jsonschema:"Node to render"`
	Style        string   `json:"style,omitempty" jsonschema:"Output format: ascii (default), svg, or png"`
	Annotations  []string `json:"annotations,omitempty" jsonschema:"What to annotate: ids names dimensions spacing"`
	Depth        int      `json:"depth,omitempty" jsonschema:"How deep to render children (default: 2)"`
	MaxChildren  int      `json:"max_children,omitempty" jsonschema:"Max children per node (default: 20, max: 50)"`
	MaxLegend    int      `json:"max_legend,omitempty" jsonschema:"Max legend entries (default: 50)"`
	OutputPath   string   `json:"output_path,omitempty" jsonschema:"Save to file (for svg/png)"`
	OutputFile   string   `json:"output_file,omitempty" jsonschema:"Write full text output to file path"`
	Format       string   `json:"format,omitempty" jsonschema:"Response format: text (default) or json"`
}

// WireframeResult contains the result of wireframe rendering.
type WireframeResult struct {
	Wireframe     string            `json:"wireframe"`
	Legend        map[string]string `json:"legend"`
	Bounds        Bounds            `json:"bounds"`
	TotalNodes    int               `json:"total_nodes"`
	RenderedNodes int               `json:"rendered_nodes"`
	Truncated     bool              `json:"truncated"`
	FilePath      string            `json:"file_path,omitempty"`
}

// Bounds represents dimensions.
type Bounds struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

func registerWireframeTool(server *mcp.Server, r *Registry) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "wireframe",
		Description: "Generate annotated wireframe with node IDs for visual reference.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args WireframeArgs) (*mcp.CallToolResult, *WireframeResult, error) {
		if args.FileKey == "" {
			return nil, nil, fmt.Errorf("file_key is required")
		}
		if args.NodeID == "" {
			return nil, nil, fmt.Errorf("node_id is required")
		}

		// Set defaults
		style := args.Style
		if style == "" {
			style = "ascii"
		}
		depth := args.Depth
		if depth == 0 {
			depth = 2
		}
		maxChildren := args.MaxChildren
		if maxChildren == 0 {
			maxChildren = 20
		}
		if maxChildren > 50 {
			maxChildren = 50
		}
		maxLegend := args.MaxLegend
		if maxLegend == 0 {
			maxLegend = 50
		}
		annotations := args.Annotations
		if len(annotations) == 0 {
			annotations = []string{"ids", "names"}
		}

		if !r.HasClient() {
			return nil, nil, fmt.Errorf("Figma API not configured")
		}

		// Fetch node
		nodes, err := r.Client().GetFileNodes(ctx, args.FileKey, []string{args.NodeID}, &figma.GetFileOptions{
			Depth: depth,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("fetching node: %w", err)
		}

		wrapper, ok := nodes.Nodes[args.NodeID]
		if !ok || wrapper.Document == nil {
			return nil, nil, fmt.Errorf("node %s not found", args.NodeID)
		}

		node := wrapper.Document

		// Create render context for limits
		renderCtx := &wireframeRenderContext{
			maxChildren:   maxChildren,
			maxLegend:     maxLegend,
			renderedNodes: 0,
			totalNodes:    0,
			truncated:     false,
		}

		result := &WireframeResult{
			Legend: make(map[string]string),
		}

		if node.AbsoluteBoundingBox != nil {
			result.Bounds = Bounds{
				Width:  node.AbsoluteBoundingBox.Width,
				Height: node.AbsoluteBoundingBox.Height,
			}
		}

		switch style {
		case "ascii":
			result.Wireframe = renderASCIIWireframeLimited(node, annotations, depth, result.Legend, renderCtx)
		case "svg":
			result.Wireframe = renderSVGWireframeLimited(node, annotations, depth, result.Legend, renderCtx)
			// TODO: Save to file if output_path specified
		default:
			result.Wireframe = renderASCIIWireframeLimited(node, annotations, depth, result.Legend, renderCtx)
		}

		result.TotalNodes = renderCtx.totalNodes
		result.RenderedNodes = renderCtx.renderedNodes
		result.Truncated = renderCtx.truncated

		// Format output
		var textOutput string
		if args.Format == "json" {
			b, _ := json.MarshalIndent(result, "", "  ")
			textOutput = string(b)
		} else {
			textOutput = result.Wireframe

			// Add legend with limits
			if containsStr(annotations, "ids") && len(result.Legend) > 0 {
				textOutput += "\n\nLegend:\n"
				legendCount := 0
				for id, name := range result.Legend {
					if legendCount >= maxLegend {
						textOutput += fmt.Sprintf("  ... and %d more entries\n", len(result.Legend)-legendCount)
						break
					}
					textOutput += fmt.Sprintf("  [%s] %s\n", id, name)
					legendCount++
				}
			}

			if renderCtx.truncated {
				textOutput += fmt.Sprintf("\n[Rendered %d of %d nodes - use smaller node_id or reduce depth]\n", renderCtx.renderedNodes, renderCtx.totalNodes)
			}
		}

		// Handle large output / file writing
		outputCfg := OutputConfig{
			MaxOutputSize: DefaultMaxOutputSize,
			OutputFile:    args.OutputFile,
			OutputDir:     r.ExportDir(),
			ToolName:      "wireframe",
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

// wireframeRenderContext tracks state during wireframe rendering.
type wireframeRenderContext struct {
	maxChildren   int
	maxLegend     int
	renderedNodes int
	totalNodes    int
	truncated     bool
}

func containsStr(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func renderASCIIWireframeLimited(node *figma.Node, annotations []string, maxDepth int, legend map[string]string, ctx *wireframeRenderContext) string {
	var sb strings.Builder

	ctx.totalNodes++
	ctx.renderedNodes++

	// Calculate scale factor to fit in reasonable terminal width
	width := 60.0
	if node.AbsoluteBoundingBox != nil {
		scaleX := 60.0 / node.AbsoluteBoundingBox.Width
		scaleY := 30.0 / node.AbsoluteBoundingBox.Height
		scale := scaleX
		if scaleY < scaleX {
			scale = scaleY
		}
		width = node.AbsoluteBoundingBox.Width * scale
		_ = node.AbsoluteBoundingBox.Height * scale // height for future use
	}

	// Header with dimensions
	showDimensions := containsStr(annotations, "dimensions")
	showNames := containsStr(annotations, "names")
	showIDs := containsStr(annotations, "ids")

	headerParts := []string{node.Name}
	if showIDs {
		headerParts = append(headerParts, fmt.Sprintf("[%s]", node.ID))
	}
	if showDimensions && node.AbsoluteBoundingBox != nil {
		headerParts = append(headerParts, fmt.Sprintf("%.0fx%.0f", node.AbsoluteBoundingBox.Width, node.AbsoluteBoundingBox.Height))
	}

	sb.WriteString(strings.Join(headerParts, " "))
	sb.WriteString("\n")

	// Top border
	sb.WriteString("┌")
	sb.WriteString(strings.Repeat("─", int(width)))
	sb.WriteString("┐\n")

	// Render children as boxes within
	childLines := renderChildrenASCIILimited(node, showIDs, showNames, showDimensions, 0, maxDepth, legend, int(width)-2, ctx)

	for _, line := range childLines {
		sb.WriteString("│ ")
		sb.WriteString(line)
		padding := int(width) - 2 - len(line)
		if padding > 0 {
			sb.WriteString(strings.Repeat(" ", padding))
		}
		sb.WriteString(" │\n")
	}

	// Bottom border
	sb.WriteString("└")
	sb.WriteString(strings.Repeat("─", int(width)))
	sb.WriteString("┘\n")

	return sb.String()
}

func renderChildrenASCIILimited(node *figma.Node, showIDs, showNames, showDimensions bool, depth, maxDepth int, legend map[string]string, maxWidth int, ctx *wireframeRenderContext) []string {
	var lines []string

	if depth >= maxDepth || len(node.Children) == 0 {
		return lines
	}

	childrenRendered := 0
	for i, child := range node.Children {
		ctx.totalNodes++

		// Check per-parent children limit
		if childrenRendered >= ctx.maxChildren {
			ctx.truncated = true
			lines = append(lines, fmt.Sprintf("... %d more children (use max_children to increase)", len(node.Children)-i))
			break
		}

		ctx.renderedNodes++
		childrenRendered++

		// Add to legend (respecting limit)
		if len(legend) < ctx.maxLegend {
			legend[child.ID] = child.Name
		}

		// Build label
		var parts []string
		if showIDs {
			parts = append(parts, fmt.Sprintf("[%s]", child.ID))
		}
		if showNames {
			name := child.Name
			if len(name) > 20 {
				name = name[:17] + "..."
			}
			parts = append(parts, name)
		}
		if showDimensions && child.AbsoluteBoundingBox != nil {
			parts = append(parts, fmt.Sprintf("%.0fx%.0f", child.AbsoluteBoundingBox.Width, child.AbsoluteBoundingBox.Height))
		}

		label := strings.Join(parts, " ")

		// Determine box style based on node type
		boxStyle := "─"
		if child.Type == figma.NodeTypeText {
			// Text node - just show content
			text := child.Characters
			if len(text) > maxWidth-4 {
				text = text[:maxWidth-7] + "..."
			}
			lines = append(lines, fmt.Sprintf("[%s] \"%s\"", child.ID, text))
			continue
		}

		// Draw child box
		boxWidth := maxWidth - depth*2
		if boxWidth < 10 {
			boxWidth = 10
		}

		indent := strings.Repeat("  ", depth)

		// Top of child box
		lines = append(lines, indent+"┌"+strings.Repeat(boxStyle, boxWidth-2)+"┐")

		// Label line
		labelLine := " " + label
		if len(labelLine) > boxWidth-2 {
			labelLine = labelLine[:boxWidth-5] + "..."
		}
		labelLine += strings.Repeat(" ", boxWidth-2-len(labelLine))
		lines = append(lines, indent+"│"+labelLine+"│")

		// Nested children
		if depth+1 < maxDepth && len(child.Children) > 0 {
			childContent := renderChildrenASCIILimited(child, showIDs, showNames, showDimensions, depth+1, maxDepth, legend, boxWidth-4, ctx)
			for _, cl := range childContent {
				lines = append(lines, indent+"│ "+cl+strings.Repeat(" ", boxWidth-4-len(cl))+" │")
			}
		} else if len(child.Children) > 0 {
			lines = append(lines, indent+"│ "+fmt.Sprintf("... %d children", len(child.Children))+strings.Repeat(" ", boxWidth-16)+"│")
		}

		// Bottom of child box
		lines = append(lines, indent+"└"+strings.Repeat(boxStyle, boxWidth-2)+"┘")
	}

	return lines
}

func renderSVGWireframeLimited(node *figma.Node, annotations []string, maxDepth int, legend map[string]string, ctx *wireframeRenderContext) string {
	width := 800.0
	height := 600.0
	if node.AbsoluteBoundingBox != nil {
		width = node.AbsoluteBoundingBox.Width
		height = node.AbsoluteBoundingBox.Height
	}

	ctx.totalNodes++
	ctx.renderedNodes++

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f">`, width, height))
	sb.WriteString("\n<style>")
	sb.WriteString(".frame { fill: none; stroke: #333; stroke-width: 1; }")
	sb.WriteString(".text { fill: none; stroke: #666; stroke-width: 1; stroke-dasharray: 4; }")
	sb.WriteString(".label { font-family: monospace; font-size: 10px; fill: #666; }")
	sb.WriteString("</style>\n")

	// Root frame
	sb.WriteString(fmt.Sprintf(`<rect class="frame" x="0" y="0" width="%.0f" height="%.0f"/>`, width, height))
	sb.WriteString("\n")

	// Render children
	renderChildrenSVGLimited(&sb, node, annotations, 0, maxDepth, legend, 0, 0, ctx)

	sb.WriteString("</svg>")
	return sb.String()
}

func renderChildrenSVGLimited(sb *strings.Builder, node *figma.Node, annotations []string, depth, maxDepth int, legend map[string]string, offsetX, offsetY float64, ctx *wireframeRenderContext) {
	if depth >= maxDepth || len(node.Children) == 0 {
		return
	}

	parentBounds := node.AbsoluteBoundingBox

	childrenRendered := 0
	for _, child := range node.Children {
		ctx.totalNodes++

		if child.AbsoluteBoundingBox == nil {
			continue
		}

		// Check per-parent children limit
		if childrenRendered >= ctx.maxChildren {
			ctx.truncated = true
			break
		}

		ctx.renderedNodes++
		childrenRendered++

		if len(legend) < ctx.maxLegend {
			legend[child.ID] = child.Name
		}

		// Calculate position relative to parent
		x := child.AbsoluteBoundingBox.X - parentBounds.X + offsetX
		y := child.AbsoluteBoundingBox.Y - parentBounds.Y + offsetY
		w := child.AbsoluteBoundingBox.Width
		h := child.AbsoluteBoundingBox.Height

		class := "frame"
		if child.Type == figma.NodeTypeText {
			class = "text"
		}

		sb.WriteString(fmt.Sprintf(`<rect class="%s" x="%.0f" y="%.0f" width="%.0f" height="%.0f"/>`, class, x, y, w, h))
		sb.WriteString("\n")

		// Add label
		if containsStr(annotations, "ids") || containsStr(annotations, "names") {
			label := ""
			if containsStr(annotations, "ids") {
				label = fmt.Sprintf("[%s]", child.ID)
			}
			if containsStr(annotations, "names") {
				if label != "" {
					label += " "
				}
				label += child.Name
			}
			sb.WriteString(fmt.Sprintf(`<text class="label" x="%.0f" y="%.0f">%s</text>`, x+2, y+12, label))
			sb.WriteString("\n")
		}

		// Recurse
		renderChildrenSVGLimited(sb, child, annotations, depth+1, maxDepth, legend, x, y, ctx)
	}
}

// Legacy functions for backward compatibility - deprecated, use Limited versions
func renderASCIIWireframe(node *figma.Node, annotations []string, maxDepth int, legend map[string]string) string {
	var sb strings.Builder

	// Calculate scale factor to fit in reasonable terminal width
	width := 60.0
	if node.AbsoluteBoundingBox != nil {
		scaleX := 60.0 / node.AbsoluteBoundingBox.Width
		scaleY := 30.0 / node.AbsoluteBoundingBox.Height
		scale := scaleX
		if scaleY < scaleX {
			scale = scaleY
		}
		width = node.AbsoluteBoundingBox.Width * scale
		_ = node.AbsoluteBoundingBox.Height * scale // height for future use
	}

	// Header with dimensions
	showDimensions := containsStr(annotations, "dimensions")
	showNames := containsStr(annotations, "names")
	showIDs := containsStr(annotations, "ids")

	headerParts := []string{node.Name}
	if showIDs {
		headerParts = append(headerParts, fmt.Sprintf("[%s]", node.ID))
	}
	if showDimensions && node.AbsoluteBoundingBox != nil {
		headerParts = append(headerParts, fmt.Sprintf("%.0fx%.0f", node.AbsoluteBoundingBox.Width, node.AbsoluteBoundingBox.Height))
	}

	sb.WriteString(strings.Join(headerParts, " "))
	sb.WriteString("\n")

	// Top border
	sb.WriteString("┌")
	sb.WriteString(strings.Repeat("─", int(width)))
	sb.WriteString("┐\n")

	// Render children as boxes within
	childLines := renderChildrenASCII(node, showIDs, showNames, showDimensions, 0, maxDepth, legend, int(width)-2)

	for _, line := range childLines {
		sb.WriteString("│ ")
		sb.WriteString(line)
		padding := int(width) - 2 - len(line)
		if padding > 0 {
			sb.WriteString(strings.Repeat(" ", padding))
		}
		sb.WriteString(" │\n")
	}

	// Bottom border
	sb.WriteString("└")
	sb.WriteString(strings.Repeat("─", int(width)))
	sb.WriteString("┘\n")

	return sb.String()
}

func renderChildrenASCII(node *figma.Node, showIDs, showNames, showDimensions bool, depth, maxDepth int, legend map[string]string, maxWidth int) []string {
	var lines []string

	if depth >= maxDepth || len(node.Children) == 0 {
		return lines
	}

	for _, child := range node.Children {
		// Add to legend
		legend[child.ID] = child.Name

		// Build label
		var parts []string
		if showIDs {
			parts = append(parts, fmt.Sprintf("[%s]", child.ID))
		}
		if showNames {
			name := child.Name
			if len(name) > 20 {
				name = name[:17] + "..."
			}
			parts = append(parts, name)
		}
		if showDimensions && child.AbsoluteBoundingBox != nil {
			parts = append(parts, fmt.Sprintf("%.0fx%.0f", child.AbsoluteBoundingBox.Width, child.AbsoluteBoundingBox.Height))
		}

		label := strings.Join(parts, " ")

		// Determine box style based on node type
		boxStyle := "─"
		if child.Type == figma.NodeTypeText {
			// Text node - just show content
			text := child.Characters
			if len(text) > maxWidth-4 {
				text = text[:maxWidth-7] + "..."
			}
			lines = append(lines, fmt.Sprintf("[%s] \"%s\"", child.ID, text))
			continue
		}

		// Draw child box
		boxWidth := maxWidth - depth*2
		if boxWidth < 10 {
			boxWidth = 10
		}

		indent := strings.Repeat("  ", depth)

		// Top of child box
		lines = append(lines, indent+"┌"+strings.Repeat(boxStyle, boxWidth-2)+"┐")

		// Label line
		labelLine := " " + label
		if len(labelLine) > boxWidth-2 {
			labelLine = labelLine[:boxWidth-5] + "..."
		}
		labelLine += strings.Repeat(" ", boxWidth-2-len(labelLine))
		lines = append(lines, indent+"│"+labelLine+"│")

		// Nested children
		if depth+1 < maxDepth && len(child.Children) > 0 {
			childContent := renderChildrenASCII(child, showIDs, showNames, showDimensions, depth+1, maxDepth, legend, boxWidth-4)
			for _, cl := range childContent {
				lines = append(lines, indent+"│ "+cl+strings.Repeat(" ", boxWidth-4-len(cl))+" │")
			}
		} else if len(child.Children) > 0 {
			lines = append(lines, indent+"│ "+fmt.Sprintf("... %d children", len(child.Children))+strings.Repeat(" ", boxWidth-16)+"│")
		}

		// Bottom of child box
		lines = append(lines, indent+"└"+strings.Repeat(boxStyle, boxWidth-2)+"┘")
	}

	return lines
}

func renderSVGWireframe(node *figma.Node, annotations []string, maxDepth int, legend map[string]string) string {
	width := 800.0
	height := 600.0
	if node.AbsoluteBoundingBox != nil {
		width = node.AbsoluteBoundingBox.Width
		height = node.AbsoluteBoundingBox.Height
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f">`, width, height))
	sb.WriteString("\n<style>")
	sb.WriteString(".frame { fill: none; stroke: #333; stroke-width: 1; }")
	sb.WriteString(".text { fill: none; stroke: #666; stroke-width: 1; stroke-dasharray: 4; }")
	sb.WriteString(".label { font-family: monospace; font-size: 10px; fill: #666; }")
	sb.WriteString("</style>\n")

	// Root frame
	sb.WriteString(fmt.Sprintf(`<rect class="frame" x="0" y="0" width="%.0f" height="%.0f"/>`, width, height))
	sb.WriteString("\n")

	// Render children
	renderChildrenSVG(&sb, node, annotations, 0, maxDepth, legend, 0, 0)

	sb.WriteString("</svg>")
	return sb.String()
}

func renderChildrenSVG(sb *strings.Builder, node *figma.Node, annotations []string, depth, maxDepth int, legend map[string]string, offsetX, offsetY float64) {
	if depth >= maxDepth || len(node.Children) == 0 {
		return
	}

	parentBounds := node.AbsoluteBoundingBox

	for _, child := range node.Children {
		if child.AbsoluteBoundingBox == nil {
			continue
		}

		legend[child.ID] = child.Name

		// Calculate position relative to parent
		x := child.AbsoluteBoundingBox.X - parentBounds.X + offsetX
		y := child.AbsoluteBoundingBox.Y - parentBounds.Y + offsetY
		w := child.AbsoluteBoundingBox.Width
		h := child.AbsoluteBoundingBox.Height

		class := "frame"
		if child.Type == figma.NodeTypeText {
			class = "text"
		}

		sb.WriteString(fmt.Sprintf(`<rect class="%s" x="%.0f" y="%.0f" width="%.0f" height="%.0f"/>`, class, x, y, w, h))
		sb.WriteString("\n")

		// Add label
		if containsStr(annotations, "ids") || containsStr(annotations, "names") {
			label := ""
			if containsStr(annotations, "ids") {
				label = fmt.Sprintf("[%s]", child.ID)
			}
			if containsStr(annotations, "names") {
				if label != "" {
					label += " "
				}
				label += child.Name
			}
			sb.WriteString(fmt.Sprintf(`<text class="label" x="%.0f" y="%.0f">%s</text>`, x+2, y+12, label))
			sb.WriteString("\n")
		}

		// Recurse
		renderChildrenSVG(sb, child, annotations, depth+1, maxDepth, legend, x, y)
	}
}
