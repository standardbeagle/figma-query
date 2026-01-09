package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// InfoArgs contains the arguments for the info tool.
type InfoArgs struct {
	Topic  string `json:"topic,omitempty" jsonschema:"Specific topic: tools, projections, query, operators, export, examples, status. Omit for overview."`
	Format string `json:"format,omitempty" jsonschema:"Output format: text (default) or json"`
}

// InfoResult contains the result of the info tool.
type InfoResult struct {
	Topic   string         `json:"topic"`
	Content string         `json:"content,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

func registerInfoTool(server *mcp.Server, r *Registry) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "info",
		Description: "List available tools, projections, query syntax, and server status. Use without arguments for overview.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args InfoArgs) (*mcp.CallToolResult, *InfoResult, error) {
		topic := args.Topic
		if topic == "" {
			topic = "overview"
		}
		format := args.Format
		if format == "" {
			format = "text"
		}

		var content string
		var data interface{}

		switch topic {
		case "overview":
			content, data = infoOverview(r)
		case "tools":
			content, data = infoTools()
		case "projections":
			content, data = infoProjections()
		case "query":
			content, data = infoQuery()
		case "operators":
			content, data = infoOperators()
		case "export":
			content, data = infoExport()
		case "examples":
			content, data = infoExamples()
		case "status":
			content, data = infoStatus(r)
		default:
			content = fmt.Sprintf("Unknown topic: %s. Available: tools, projections, query, operators, export, examples, status", topic)
		}

		result := &InfoResult{
			Topic: topic,
		}

		if format == "json" {
			if m, ok := data.(map[string]any); ok {
				result.Data = m
			}
		} else {
			result.Content = content
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: content},
			},
		}, result, nil
	})
}

func infoOverview(r *Registry) (string, interface{}) {
	authStatus := "not configured"
	if r.HasClient() {
		authStatus = "configured"
	}

	text := `figma-query v0.1.0 - Token-efficient Figma MCP
================================================

Authentication: ` + authStatus + `
Export directory: ` + r.ExportDir() + `

Tool Groups
-----------
Group     | Count | Purpose
--------- | ----- | --------
discovery | 1     | info - help & status
export    | 4     | sync_file, export_assets, export_tokens, download_image
query     | 5     | query, search, get_tree, list_components, list_styles
detail    | 3     | get_node, get_css, get_tokens
render    | 1     | wireframe (ASCII/SVG with IDs)
analysis  | 1     | diff (version comparison)

Quick Start
-----------
1. sync_file(file_key) - Export full file to disk for grep fallback
2. get_tree(file_key) - See structure with node IDs
3. query(file_key, q={...}) - Query with data shaping
4. get_css(file_key, node_ids) - Extract CSS for implementation

Use info(topic="<topic>") for detailed help on:
  tools, projections, query, operators, export, examples, status`

	data := map[string]interface{}{
		"version":      "0.1.0",
		"auth_status":  authStatus,
		"export_dir":   r.ExportDir(),
		"tool_count":   15,
		"tool_groups": []map[string]interface{}{
			{"name": "discovery", "count": 1, "tools": []string{"info"}},
			{"name": "export", "count": 4, "tools": []string{"sync_file", "export_assets", "export_tokens", "download_image"}},
			{"name": "query", "count": 5, "tools": []string{"query", "search", "get_tree", "list_components", "list_styles"}},
			{"name": "detail", "count": 3, "tools": []string{"get_node", "get_css", "get_tokens"}},
			{"name": "render", "count": 1, "tools": []string{"wireframe"}},
			{"name": "analysis", "count": 1, "tools": []string{"diff"}},
		},
	}

	return text, data
}

func infoTools() (string, interface{}) {
	tools := []map[string]string{
		{"name": "info", "group": "discovery", "desc": "List tools, projections, query syntax, status"},
		{"name": "sync_file", "group": "export", "desc": "Export entire file to nested folders (includes assets by default)"},
		{"name": "export_assets", "group": "export", "desc": "Export images/icons for specific nodes"},
		{"name": "export_tokens", "group": "export", "desc": "Export design tokens to CSS/JSON/etc"},
		{"name": "download_image", "group": "export", "desc": "Download images by ref ID or render nodes as images"},
		{"name": "query", "group": "query", "desc": "Query nodes with JSON DSL and data shaping"},
		{"name": "search", "group": "query", "desc": "Full-text search across names, text, properties"},
		{"name": "get_tree", "group": "query", "desc": "Get file structure as ASCII tree with node IDs"},
		{"name": "list_components", "group": "query", "desc": "List all components with usage stats"},
		{"name": "list_styles", "group": "query", "desc": "List all styles (color, text, effect, grid)"},
		{"name": "get_node", "group": "detail", "desc": "Get full details for a specific node"},
		{"name": "get_css", "group": "detail", "desc": "Extract CSS properties for node(s)"},
		{"name": "get_tokens", "group": "detail", "desc": "Get design token references and resolved values"},
		{"name": "wireframe", "group": "render", "desc": "Generate annotated wireframe with node IDs"},
		{"name": "diff", "group": "analysis", "desc": "Compare exports or file versions"},
	}

	var sb strings.Builder
	sb.WriteString("Available Tools\n")
	sb.WriteString("===============\n\n")
	sb.WriteString("Name             | Group     | Description\n")
	sb.WriteString("---------------- | --------- | -----------\n")

	for _, t := range tools {
		sb.WriteString(fmt.Sprintf("%-16s | %-9s | %s\n", t["name"], t["group"], t["desc"]))
	}

	sb.WriteString("\nAll tools support format='text'|'json' for scriptability.\n")

	return sb.String(), tools
}

func infoProjections() (string, interface{}) {
	projections := map[string][]string{
		"@structure":  {"id", "name", "type", "visible", "parent_id"},
		"@bounds":     {"x", "y", "width", "height", "rotation"},
		"@css":        {"fills", "strokes", "effects", "cornerRadius", "opacity", "blendMode"},
		"@layout":     {"layoutMode", "primaryAxisSizingMode", "counterAxisSizingMode", "padding*", "itemSpacing", "constraints"},
		"@typography": {"fontFamily", "fontSize", "fontWeight", "lineHeight", "letterSpacing", "textAlign*"},
		"@tokens":     {"boundVariables", "resolvedTokens"},
		"@images":     {"imageRefs (from fills/strokes/backgrounds)", "exportSettings"},
		"@children":   {"children (recursive with depth)"},
		"@all":        {"All properties including @images"},
	}

	var sb strings.Builder
	sb.WriteString("Built-in Projections\n")
	sb.WriteString("====================\n\n")
	sb.WriteString("Use projections in the 'select' array to get specific property groups.\n\n")

	for name, props := range projections {
		sb.WriteString(fmt.Sprintf("%-12s : %s\n", name, strings.Join(props, ", ")))
	}

	sb.WriteString("\nExample: query(file_key, q={select: ['@css', '@bounds'], from: 'FRAME'})\n")

	return sb.String(), projections
}

func infoQuery() (string, interface{}) {
	text := `Query DSL Reference
===================

The query tool accepts a JSON query object with these fields:

{
  "from": "COMPONENT",              // Node type(s) or "#node_id"
  "where": {"name": {"$match": "Button*"}},  // Filter conditions
  "select": ["@css", "@bounds"],    // Properties to include
  "depth": 2,                       // Child traversal depth
  "limit": 50,                      // Max results
  "offset": 0                       // Skip N results
}

FROM clause
-----------
- Single type: "FRAME", "COMPONENT", "TEXT", "INSTANCE"
- Multiple: ["FRAME", "GROUP"]
- Specific node: "#1:234"
- Path expression: "PAGE > FRAME > COMPONENT"

SELECT clause
-------------
- Property names: ["fills", "strokes", "name"]
- Projections: ["@css", "@layout", "@typography"]
- Mixed: ["@structure", "effects", "componentId"]

See info(topic="operators") for WHERE clause operators.
See info(topic="projections") for available @projections.`

	data := map[string]interface{}{
		"fields": map[string]string{
			"from":   "Node type(s), '#id', or path expression",
			"where":  "Filter conditions with operators",
			"select": "Properties or @projections to include",
			"depth":  "Child traversal depth (0=node only, -1=unlimited)",
			"limit":  "Max results per page",
			"offset": "Skip N results for pagination",
		},
	}

	return text, data
}

func infoOperators() (string, interface{}) {
	operators := map[string]string{
		"$eq":       "Exact match: {name: {$eq: 'Button'}}",
		"$match":    "Glob pattern: {name: {$match: 'Button*'}}",
		"$regex":    "Regex: {name: {$regex: '^Icon-.*'}}",
		"$contains": "Substring: {name: {$contains: 'primary'}}",
		"$in":       "Value in array: {type: {$in: ['FRAME', 'GROUP']}}",
		"$gt":       "Greater than: {width: {$gt: 100}}",
		"$gte":      "Greater or equal: {opacity: {$gte: 0.5}}",
		"$lt":       "Less than: {height: {$lt: 50}}",
		"$lte":      "Less or equal: {cornerRadius: {$lte: 8}}",
		"$exists":   "Property exists: {fills: {$exists: true}}",
		"$not":      "Negate: {visible: {$not: false}}",
	}

	var sb strings.Builder
	sb.WriteString("WHERE Clause Operators\n")
	sb.WriteString("======================\n\n")

	for op, desc := range operators {
		sb.WriteString(fmt.Sprintf("%-10s  %s\n", op, desc))
	}

	sb.WriteString("\nCompound conditions:\n")
	sb.WriteString("  {name: {$match: 'Button*'}, visible: true}  // AND\n")
	sb.WriteString("  Multiple conditions on same field are ANDed.\n")

	return sb.String(), operators
}

func infoExport() (string, interface{}) {
	text := `Export Directory Structure
==========================

sync_file creates a grep-friendly folder structure with assets (enabled by default):

<export_dir>/<file-name>/
├── _meta.json          # File metadata, export timestamp
├── _tree.txt           # ASCII tree with node IDs
├── _index.json         # Flat lookup: node_id → path
├── pages/
│   └── <page-name>/
│       └── children/
│           └── <node-name>/
│               ├── _node.json   # Full node data
│               ├── _css.json    # CSS properties
│               ├── _tokens.json # Variable refs
│               └── children/
├── components/
│   └── _components.json
├── styles/
│   ├── colors.json
│   ├── typography.json
│   ├── effects.json
│   └── grids.json
├── variables/
│   ├── tokens.json
│   └── collections/
└── assets/              # Included by default
    ├── fills/           # Image fills (backgrounds, etc.)
    │   └── <imageRef>.png
    └── renders/         # Nodes with export settings
        └── <node-name>.png

Image Workflow
--------------
1. Query with @images: query(q={select: ["@images"]}) - Get imageRefs
2. Download by ref: download_image(image_refs=["ref123"], output_dir="./")
3. Or render nodes: download_image(node_ids=["1:234"], format="svg")

Grep Examples
-------------
grep -r "Button" ./figma-export/          # Find all Button refs
find . -name "_css.json" -exec jq . {} \; # All CSS files
jq '.fills' ./figma-export/**/_node.json  # Extract fills`

	data := map[string]interface{}{
		"root_files":  []string{"_meta.json", "_tree.txt", "_index.json"},
		"directories": []string{"pages/", "components/", "styles/", "variables/", "assets/"},
		"assets_subdirs": []string{"fills/", "renders/"},
	}

	return text, data
}

func infoExamples() (string, interface{}) {
	text := `Common Workflows
================

1. Initial Setup
----------------
sync_file(file_key="abc123")
get_tree(file_key="abc123", depth=2)

2. Implement a Component
------------------------
search(file_key="abc123", pattern="Button")
wireframe(file_key="abc123", node_id="1:234")
get_css(file_key="abc123", node_ids="1:234")
export_assets(file_key="abc123", node_ids=["1:235"], formats=["svg"])

3. Find All Buttons and Get CSS
-------------------------------
query(file_key="abc123", q={
  "from": "COMPONENT",
  "where": {"name": {"$match": "Button*"}},
  "select": ["@structure"]
})
# Then for each result:
get_css(file_key="abc123", node_ids="<id>")

4. Export Design Tokens
-----------------------
export_tokens(file_key="abc123", output_path="./tokens.css", format="css")

5. Use grep as Fallback
-----------------------
sync_file(file_key="abc123")
# Then in terminal:
grep -r "fill" ./figma-export/pages/
jq '.fills[0].color' ./figma-export/pages/**/_node.json`

	examples := []map[string]interface{}{
		{
			"name":  "Initial Setup",
			"steps": []string{"sync_file(file_key)", "get_tree(file_key, depth=2)"},
		},
		{
			"name":  "Implement Component",
			"steps": []string{"search(pattern)", "wireframe(node_id)", "get_css(node_ids)", "export_assets(node_ids)"},
		},
	}

	return text, examples
}

func infoStatus(r *Registry) (string, interface{}) {
	authStatus := "not configured"
	authDetail := "Set FIGMA_ACCESS_TOKEN environment variable"
	if r.HasClient() {
		authStatus = "configured"
		authDetail = "Token loaded from environment"
	}

	text := fmt.Sprintf(`Server Status
=============

Version: 0.1.0
Authentication: %s
  %s

Export Directory: %s

Environment Variables
---------------------
FIGMA_ACCESS_TOKEN     : %s
FIGMA_TOKEN            : (fallback)
FIGMA_PERSONAL_ACCESS_TOKEN : (fallback)
FIGMA_EXPORT_DIR       : %s`, authStatus, authDetail, r.ExportDir(),
		maskToken(authStatus == "configured"),
		r.ExportDir())

	data := map[string]interface{}{
		"version":     "0.1.0",
		"auth_status": authStatus,
		"export_dir":  r.ExportDir(),
		"ready":       r.HasClient(),
	}

	return text, data
}

func maskToken(configured bool) string {
	if configured {
		return "****configured****"
	}
	return "(not set)"
}

// FormatOutput formats the result as text or JSON based on format parameter.
func FormatOutput(format string, textContent string, jsonData interface{}) (string, error) {
	if format == "json" {
		b, err := json.MarshalIndent(jsonData, "", "  ")
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return textContent, nil
}
