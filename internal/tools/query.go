package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/standardbeagle/figma-query/internal/figma"
)

// Query represents a query DSL object.
type Query struct {
	From   []string               `json:"from,omitempty" jsonschema:"Node types to query (e.g. FRAME, TEXT, COMPONENT)"`
	Where  map[string]any         `json:"where,omitempty" jsonschema:"Filter conditions"`
	Select []string               `json:"select,omitempty" jsonschema:"Properties or @projections to return"`
	Path   string                 `json:"path,omitempty" jsonschema:"CSS-like path expression"`
	Depth  int                    `json:"depth,omitempty" jsonschema:"Child traversal depth"`
	Limit  int                    `json:"limit,omitempty" jsonschema:"Max results to return"`
	Offset int                    `json:"offset,omitempty" jsonschema:"Pagination offset"`
}

// QueryArgs contains arguments for the query tool.
type QueryArgs struct {
	FileKey   string `json:"file_key" jsonschema:"Figma file key"`
	Q         Query  `json:"q" jsonschema:"Query object with from/where/select/depth/limit"`
	FromCache bool   `json:"from_cache,omitempty" jsonschema:"Read from local export if available (default: true)"`
	Format    string `json:"format,omitempty" jsonschema:"Response format: text (default) or json"`
}

// QueryResult contains the result of a query.
type QueryResult struct {
	Results  []map[string]any `json:"results"`
	Total    int              `json:"total"`
	Returned int              `json:"returned"`
	HasMore  bool             `json:"has_more"`
	Cursor   string           `json:"cursor,omitempty"`
	CacheHit bool             `json:"cache_hit"`
}

func registerQueryTool(server *mcp.Server, r *Registry) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "query",
		Description: "Query nodes using JSON DSL with data shaping. Reads from cache or API.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args QueryArgs) (*mcp.CallToolResult, *QueryResult, error) {
		if args.FileKey == "" {
			return nil, nil, fmt.Errorf("file_key is required")
		}

		// Set defaults
		fromCache := args.FromCache
		if args.Format == "" {
			args.Format = "text"
		}
		limit := args.Q.Limit
		if limit == 0 {
			limit = 50
		}

		var nodes []*figma.Node
		var cacheHit bool

		// Try to read from cache first
		if fromCache {
			cachedNodes, err := readNodesFromCache(r.ExportDir(), args.FileKey)
			if err == nil && len(cachedNodes) > 0 {
				nodes = cachedNodes
				cacheHit = true
			}
		}

		// Fall back to API
		if len(nodes) == 0 {
			if !r.HasClient() {
				return nil, nil, fmt.Errorf("no cache found and Figma API not configured")
			}

			file, err := r.Client().GetFile(ctx, args.FileKey, nil)
			if err != nil {
				return nil, nil, fmt.Errorf("fetching file: %w", err)
			}

			nodes = flattenNodes(file.Document)
		}

		// Apply query filters
		filtered := filterNodes(nodes, &args.Q)

		// Apply pagination
		total := len(filtered)
		start := args.Q.Offset
		if start > total {
			start = total
		}
		end := start + limit
		if end > total {
			end = total
		}

		pageNodes := filtered[start:end]

		// Project selected properties
		results := make([]map[string]interface{}, 0, len(pageNodes))
		for _, node := range pageNodes {
			projected := projectNode(node, args.Q.Select)
			results = append(results, projected)
		}

		result := &QueryResult{
			Results:  results,
			Total:    total,
			Returned: len(results),
			HasMore:  end < total,
			CacheHit: cacheHit,
		}

		if result.HasMore {
			result.Cursor = fmt.Sprintf("%d", end)
		}

		// Format output
		var textOutput string
		if args.Format == "json" {
			b, _ := json.MarshalIndent(result, "", "  ")
			textOutput = string(b)
		} else {
			textOutput = formatQueryResult(result)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: textOutput},
			},
		}, result, nil
	})
}

func readNodesFromCache(exportDir, fileKey string) ([]*figma.Node, error) {
	// Find export directory for this file
	entries, err := os.ReadDir(exportDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		metaPath := filepath.Join(exportDir, entry.Name(), "_meta.json")
		metaData, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}

		var meta map[string]interface{}
		if err := json.Unmarshal(metaData, &meta); err != nil {
			continue
		}

		if meta["fileKey"] == fileKey {
			// Found matching export, read index and nodes
			return readNodesFromExport(filepath.Join(exportDir, entry.Name()))
		}
	}

	return nil, fmt.Errorf("no cache found for file %s", fileKey)
}

func readNodesFromExport(exportPath string) ([]*figma.Node, error) {
	var nodes []*figma.Node

	// Walk the export directory and read _node.json files
	err := filepath.Walk(exportPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Name() == "_node.json" {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil // skip this file
			}

			var node figma.Node
			if err := json.Unmarshal(data, &node); err != nil {
				return nil // skip this file
			}

			nodes = append(nodes, &node)
		}

		return nil
	})

	return nodes, err
}

func flattenNodes(doc *figma.DocumentNode) []*figma.Node {
	var nodes []*figma.Node

	var walk func(*figma.Node)
	walk = func(n *figma.Node) {
		nodes = append(nodes, n)
		for _, child := range n.Children {
			walk(child)
		}
	}

	for _, page := range doc.Children {
		walk(page)
	}

	return nodes
}

func filterNodes(nodes []*figma.Node, q *Query) []*figma.Node {
	var result []*figma.Node

	for _, node := range nodes {
		if matchesQuery(node, q) {
			result = append(result, node)
		}
	}

	return result
}

func matchesQuery(node *figma.Node, q *Query) bool {
	// Check FROM clause
	if q.From != nil {
		if !matchesFrom(node, q.From) {
			return false
		}
	}

	// Check WHERE clause
	if len(q.Where) > 0 {
		if !matchesWhere(node, q.Where) {
			return false
		}
	}

	return true
}

func matchesFrom(node *figma.Node, from []string) bool {
	if len(from) == 0 {
		return true
	}
	for _, f := range from {
		// Check if it's a node ID reference
		if strings.HasPrefix(f, "#") {
			if node.ID == f[1:] {
				return true
			}
		} else if string(node.Type) == f {
			// Check node type
			return true
		}
	}
	return false
}

func matchesWhere(node *figma.Node, where map[string]any) bool {
	for field, condition := range where {
		if !matchesCondition(node, field, condition) {
			return false
		}
	}
	return true
}

func matchesCondition(node *figma.Node, field string, condition interface{}) bool {
	// Get field value from node
	value := getNodeField(node, field)

	// Handle operator objects
	if condMap, ok := condition.(map[string]interface{}); ok {
		for op, operand := range condMap {
			if !applyOperator(value, op, operand) {
				return false
			}
		}
		return true
	}

	// Direct equality
	return fmt.Sprintf("%v", value) == fmt.Sprintf("%v", condition)
}

func getNodeField(node *figma.Node, field string) interface{} {
	switch field {
	case "id":
		return node.ID
	case "name":
		return node.Name
	case "type":
		return string(node.Type)
	case "visible":
		if node.Visible == nil {
			return true
		}
		return *node.Visible
	case "width":
		if node.AbsoluteBoundingBox != nil {
			return node.AbsoluteBoundingBox.Width
		}
		return 0
	case "height":
		if node.AbsoluteBoundingBox != nil {
			return node.AbsoluteBoundingBox.Height
		}
		return 0
	case "fills":
		return node.Fills
	case "strokes":
		return node.Strokes
	case "effects":
		return node.Effects
	case "characters":
		return node.Characters
	case "componentId":
		return node.ComponentID
	case "opacity":
		if node.Opacity != nil {
			return *node.Opacity
		}
		return 1.0
	case "cornerRadius":
		return node.CornerRadius
	case "layoutMode":
		return node.LayoutMode
	default:
		return nil
	}
}

func applyOperator(value interface{}, op string, operand interface{}) bool {
	switch op {
	case "$eq":
		return fmt.Sprintf("%v", value) == fmt.Sprintf("%v", operand)

	case "$match":
		pattern, ok := operand.(string)
		if !ok {
			return false
		}
		// Convert glob to regex
		pattern = strings.ReplaceAll(pattern, "*", ".*")
		pattern = "^" + pattern + "$"
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return false
		}
		return re.MatchString(fmt.Sprintf("%v", value))

	case "$regex":
		pattern, ok := operand.(string)
		if !ok {
			return false
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(fmt.Sprintf("%v", value))

	case "$contains":
		substr, ok := operand.(string)
		if !ok {
			return false
		}
		return strings.Contains(strings.ToLower(fmt.Sprintf("%v", value)), strings.ToLower(substr))

	case "$in":
		arr, ok := operand.([]interface{})
		if !ok {
			return false
		}
		valueStr := fmt.Sprintf("%v", value)
		for _, item := range arr {
			if fmt.Sprintf("%v", item) == valueStr {
				return true
			}
		}
		return false

	case "$gt":
		return compareNumbers(value, operand) > 0

	case "$gte":
		return compareNumbers(value, operand) >= 0

	case "$lt":
		return compareNumbers(value, operand) < 0

	case "$lte":
		return compareNumbers(value, operand) <= 0

	case "$exists":
		exists, ok := operand.(bool)
		if !ok {
			return false
		}
		if exists {
			return value != nil
		}
		return value == nil

	case "$not":
		return !applyOperator(value, "$eq", operand)

	default:
		return true
	}
}

func compareNumbers(a, b interface{}) int {
	af := toFloat(a)
	bf := toFloat(b)
	if af < bf {
		return -1
	}
	if af > bf {
		return 1
	}
	return 0
}

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}

func projectNode(node *figma.Node, selects []string) map[string]interface{} {
	result := make(map[string]interface{})

	// Always include ID
	result["id"] = node.ID

	// If no selects, use @structure
	if len(selects) == 0 {
		selects = []string{"@structure"}
	}

	for _, sel := range selects {
		if strings.HasPrefix(sel, "@") {
			// Apply projection
			applyProjection(node, sel, result)
		} else {
			// Get specific field
			result[sel] = getNodeField(node, sel)
		}
	}

	return result
}

func applyProjection(node *figma.Node, projection string, result map[string]interface{}) {
	switch projection {
	case "@structure":
		result["id"] = node.ID
		result["name"] = node.Name
		result["type"] = node.Type
		if node.Visible != nil {
			result["visible"] = *node.Visible
		}

	case "@bounds":
		if node.AbsoluteBoundingBox != nil {
			result["x"] = node.AbsoluteBoundingBox.X
			result["y"] = node.AbsoluteBoundingBox.Y
			result["width"] = node.AbsoluteBoundingBox.Width
			result["height"] = node.AbsoluteBoundingBox.Height
		}

	case "@css":
		result["fills"] = node.Fills
		result["strokes"] = node.Strokes
		result["effects"] = node.Effects
		result["cornerRadius"] = node.CornerRadius
		if node.Opacity != nil {
			result["opacity"] = *node.Opacity
		}
		result["blendMode"] = node.BlendMode

	case "@layout":
		result["layoutMode"] = node.LayoutMode
		result["primaryAxisSizingMode"] = node.PrimaryAxisSizingMode
		result["counterAxisSizingMode"] = node.CounterAxisSizingMode
		result["paddingTop"] = node.PaddingTop
		result["paddingRight"] = node.PaddingRight
		result["paddingBottom"] = node.PaddingBottom
		result["paddingLeft"] = node.PaddingLeft
		result["itemSpacing"] = node.ItemSpacing
		if node.Constraints != nil {
			result["constraints"] = node.Constraints
		}

	case "@typography":
		if node.Style != nil {
			result["fontFamily"] = node.Style.FontFamily
			result["fontSize"] = node.Style.FontSize
			result["fontWeight"] = node.Style.FontWeight
			result["lineHeight"] = node.Style.LineHeightPx
			result["letterSpacing"] = node.Style.LetterSpacing
			result["textAlignHorizontal"] = node.Style.TextAlignHorizontal
			result["textAlignVertical"] = node.Style.TextAlignVertical
		}
		result["characters"] = node.Characters

	case "@tokens":
		if len(node.BoundVariables) > 0 {
			result["boundVariables"] = node.BoundVariables
		}

	case "@images":
		// Extract image references from fills, strokes, and backgrounds
		imageRefs := extractImageRefs(node)
		if len(imageRefs) > 0 {
			result["imageRefs"] = imageRefs
		}
		if len(node.ExportSettings) > 0 {
			result["exportSettings"] = node.ExportSettings
		}

	case "@all":
		// Include everything
		applyProjection(node, "@structure", result)
		applyProjection(node, "@bounds", result)
		applyProjection(node, "@css", result)
		applyProjection(node, "@layout", result)
		applyProjection(node, "@typography", result)
		applyProjection(node, "@tokens", result)
		applyProjection(node, "@images", result)
	}
}

// ImageRef represents a reference to an image used in a node.
type ImageRef struct {
	Ref    string `json:"ref"`              // The image reference ID
	Source string `json:"source"`           // Where the image is used: fill, stroke, background
	Index  int    `json:"index,omitempty"`  // Index in the source array
	Type   string `json:"type,omitempty"`   // Type of paint (IMAGE, etc.)
}

// extractImageRefs extracts all image references from a node's fills, strokes, and backgrounds.
func extractImageRefs(node *figma.Node) []ImageRef {
	var refs []ImageRef

	// Check fills
	for i, fill := range node.Fills {
		if fill.Type == "IMAGE" && fill.ImageRef != "" {
			refs = append(refs, ImageRef{
				Ref:    fill.ImageRef,
				Source: "fill",
				Index:  i,
				Type:   fill.Type,
			})
		}
		if fill.GifRef != "" {
			refs = append(refs, ImageRef{
				Ref:    fill.GifRef,
				Source: "fill",
				Index:  i,
				Type:   "GIF",
			})
		}
	}

	// Check strokes
	for i, stroke := range node.Strokes {
		if stroke.Type == "IMAGE" && stroke.ImageRef != "" {
			refs = append(refs, ImageRef{
				Ref:    stroke.ImageRef,
				Source: "stroke",
				Index:  i,
				Type:   stroke.Type,
			})
		}
	}

	// Check background
	for i, bg := range node.Background {
		if bg.Type == "IMAGE" && bg.ImageRef != "" {
			refs = append(refs, ImageRef{
				Ref:    bg.ImageRef,
				Source: "background",
				Index:  i,
				Type:   bg.Type,
			})
		}
	}

	return refs
}

func formatQueryResult(r *QueryResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Found %d results (showing %d)\n\n", r.Total, r.Returned))

	if len(r.Results) == 0 {
		sb.WriteString("No matching nodes found.\n")
		return sb.String()
	}

	// Table format
	sb.WriteString("ID       | Name                           | Type\n")
	sb.WriteString("-------- | ------------------------------ | ----\n")

	for _, res := range r.Results {
		id := fmt.Sprintf("%v", res["id"])
		name := fmt.Sprintf("%v", res["name"])
		nodeType := fmt.Sprintf("%v", res["type"])

		if len(name) > 30 {
			name = name[:27] + "..."
		}

		sb.WriteString(fmt.Sprintf("%-8s | %-30s | %s\n", id, name, nodeType))
	}

	if r.HasMore {
		sb.WriteString(fmt.Sprintf("\n[+%d more, use offset=%s to see next page]\n", r.Total-r.Returned, r.Cursor))
	}

	if r.CacheHit {
		sb.WriteString("\n(from cache)\n")
	}

	return sb.String()
}
