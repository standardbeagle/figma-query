package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/standardbeagle/figma-query/internal/figma"
)

// GetNodeArgs contains arguments for the get_node tool.
type GetNodeArgs struct {
	FileKey string   `json:"file_key" jsonschema:"Figma file key"`
	NodeID  string   `json:"node_id" jsonschema:"Node ID to retrieve"`
	Select  []string `json:"select,omitempty" jsonschema:"Properties to include (default: @all)"`
	Depth   int      `json:"depth,omitempty" jsonschema:"Include children to this depth (default: 0)"`
	Format  string   `json:"format,omitempty" jsonschema:"Response format: text (default) or json"`
}

// GetNodeResult contains the result of get_node.
type GetNodeResult struct {
	Node          map[string]any `json:"node"`
	Path          string         `json:"path"`
	ParentID      string         `json:"parent_id,omitempty"`
	ChildrenCount int            `json:"children_count"`
}

func registerGetNodeTool(server *mcp.Server, r *Registry) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_node",
		Description: "Get full details for a specific node by ID.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetNodeArgs) (*mcp.CallToolResult, *GetNodeResult, error) {
		if args.FileKey == "" {
			return nil, nil, fmt.Errorf("file_key is required")
		}
		if args.NodeID == "" {
			return nil, nil, fmt.Errorf("node_id is required")
		}

		// Set defaults
		selects := args.Select
		if len(selects) == 0 {
			selects = []string{"@all"}
		}

		// Fetch node from API
		if !r.HasClient() {
			return nil, nil, fmt.Errorf("Figma API not configured")
		}

		nodes, err := r.Client().GetFileNodes(ctx, args.FileKey, []string{args.NodeID}, &figma.GetFileOptions{
			Depth: args.Depth,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("fetching node: %w", err)
		}

		wrapper, ok := nodes.Nodes[args.NodeID]
		if !ok || wrapper.Document == nil {
			return nil, nil, fmt.Errorf("node %s not found", args.NodeID)
		}

		node := wrapper.Document

		// Project node
		projected := projectNode(node, selects)

		result := &GetNodeResult{
			Node:          projected,
			ChildrenCount: len(node.Children),
		}

		// Format output
		var textOutput string
		if args.Format == "json" {
			b, _ := json.MarshalIndent(result, "", "  ")
			textOutput = string(b)
		} else {
			textOutput = formatNodeResult(result)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: textOutput},
			},
		}, result, nil
	})
}

// GetCSSArgs contains arguments for the get_css tool.
type GetCSSArgs struct {
	FileKey string   `json:"file_key" jsonschema:"Figma file key"`
	NodeIDs []string `json:"node_ids" jsonschema:"Node IDs to get CSS for"`
	Style   string   `json:"style,omitempty" jsonschema:"CSS output style: vanilla (default), cssmodules, tailwind, styled-components, or tokens"`
	Include []string `json:"include,omitempty" jsonschema:"What to include: layout spacing colors typography effects all"`
	Format  string   `json:"format,omitempty" jsonschema:"Response format: text (default) or json"`
}

// GetCSSResult contains the result of get_css.
type GetCSSResult struct {
	CSS       map[string]string   `json:"css"`
	Variables map[string]string   `json:"variables,omitempty"`
	Warnings  []string            `json:"warnings,omitempty"`
}

func registerGetCSSTool(server *mcp.Server, r *Registry) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_css",
		Description: "Extract CSS properties for node(s). Returns production-ready CSS.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetCSSArgs) (*mcp.CallToolResult, *GetCSSResult, error) {
		if args.FileKey == "" {
			return nil, nil, fmt.Errorf("file_key is required")
		}

		// Parse node IDs
		nodeIDs := args.NodeIDs
		if len(nodeIDs) == 0 {
			return nil, nil, fmt.Errorf("node_ids is required")
		}

		// Set defaults
		style := args.Style
		if style == "" {
			style = "vanilla"
		}

		if !r.HasClient() {
			return nil, nil, fmt.Errorf("Figma API not configured")
		}

		// Fetch nodes
		nodes, err := r.Client().GetFileNodes(ctx, args.FileKey, nodeIDs, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("fetching nodes: %w", err)
		}

		result := &GetCSSResult{
			CSS:       make(map[string]string),
			Variables: make(map[string]string),
		}

		for id, wrapper := range nodes.Nodes {
			if wrapper.Document == nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("node %s not found", id))
				continue
			}

			css := generateCSS(wrapper.Document, style, args.Include)
			result.CSS[id] = css
		}

		// Format output
		var textOutput string
		if args.Format == "json" {
			b, _ := json.MarshalIndent(result, "", "  ")
			textOutput = string(b)
		} else {
			textOutput = formatCSSResult(result)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: textOutput},
			},
		}, result, nil
	})
}

// GetTokensArgs contains arguments for the get_tokens tool.
type GetTokensArgs struct {
	FileKey string   `json:"file_key" jsonschema:"Figma file key"`
	NodeIDs []string `json:"node_ids" jsonschema:"Node IDs to get tokens for"`
	Resolve bool     `json:"resolve,omitempty" jsonschema:"Resolve token references to actual values (default: true)"`
	Mode    string   `json:"mode,omitempty" jsonschema:"Variable mode to resolve (e.g., dark, light)"`
	Format  string   `json:"format,omitempty" jsonschema:"Response format: text (default) or json"`
}

// GetTokensResult contains the result of get_tokens.
type GetTokensResult struct {
	Tokens      map[string]any `json:"tokens"`
	Resolved    map[string]any `json:"resolved,omitempty"`
	Collections []string       `json:"collections,omitempty"`
}

func registerGetTokensTool(server *mcp.Server, r *Registry) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_tokens",
		Description: "Get design token references and resolved values for node(s).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetTokensArgs) (*mcp.CallToolResult, *GetTokensResult, error) {
		if args.FileKey == "" {
			return nil, nil, fmt.Errorf("file_key is required")
		}

		nodeIDs := args.NodeIDs
		if len(nodeIDs) == 0 {
			return nil, nil, fmt.Errorf("node_ids is required")
		}

		if !r.HasClient() {
			return nil, nil, fmt.Errorf("Figma API not configured")
		}

		// Fetch nodes
		nodes, err := r.Client().GetFileNodes(ctx, args.FileKey, nodeIDs, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("fetching nodes: %w", err)
		}

		// Fetch variables for resolution
		var variables *figma.LocalVariables
		if args.Resolve {
			variables, _ = r.Client().GetLocalVariables(ctx, args.FileKey)
		}

		result := &GetTokensResult{
			Tokens:   make(map[string]interface{}),
			Resolved: make(map[string]interface{}),
		}

		collectionsSet := make(map[string]bool)

		for id, wrapper := range nodes.Nodes {
			if wrapper.Document == nil {
				continue
			}

			tokens := extractTokenReferences(wrapper.Document)
			result.Tokens[id] = tokens

			// Resolve tokens
			if args.Resolve && variables != nil && variables.Meta != nil {
				resolved := make(map[string]interface{})
				for prop, ref := range wrapper.Document.BoundVariables {
					if v, ok := variables.Meta.Variables[ref.ID]; ok {
						resolved[prop] = map[string]interface{}{
							"name":         v.Name,
							"resolvedType": v.ResolvedType,
							"values":       v.ValuesByMode,
						}
						if coll, ok := variables.Meta.VariableCollections[v.VariableCollectionID]; ok {
							collectionsSet[coll.Name] = true
						}
					}
				}
				result.Resolved[id] = resolved
			}
		}

		for coll := range collectionsSet {
			result.Collections = append(result.Collections, coll)
		}

		// Format output
		var textOutput string
		if args.Format == "json" {
			b, _ := json.MarshalIndent(result, "", "  ")
			textOutput = string(b)
		} else {
			textOutput = formatTokensResult(result)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: textOutput},
			},
		}, result, nil
	})
}

func generateCSS(node *figma.Node, style string, include []string) string {
	props := extractCSSProperties(node)

	var sb strings.Builder

	switch style {
	case "vanilla":
		sb.WriteString(fmt.Sprintf("/* %s */\n", node.Name))
		sb.WriteString(".class {\n")
		for key, value := range props {
			cssKey := camelToKebab(key)
			sb.WriteString(fmt.Sprintf("  %s: %v;\n", cssKey, formatCSSValue(value)))
		}
		sb.WriteString("}\n")

	case "tailwind":
		classes := propsToTailwind(props)
		sb.WriteString(fmt.Sprintf("/* %s */\n", node.Name))
		sb.WriteString(strings.Join(classes, " "))

	default:
		sb.WriteString(fmt.Sprintf("/* %s */\n", node.Name))
		for key, value := range props {
			sb.WriteString(fmt.Sprintf("%s: %v;\n", key, value))
		}
	}

	return sb.String()
}

func camelToKebab(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteRune('-')
			}
			result.WriteRune(r + 32) // lowercase
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func formatCSSValue(v interface{}) string {
	switch val := v.(type) {
	case float64:
		if val == float64(int(val)) {
			return fmt.Sprintf("%dpx", int(val))
		}
		return fmt.Sprintf("%.2fpx", val)
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}

func propsToTailwind(props map[string]interface{}) []string {
	var classes []string

	if w, ok := props["width"].(float64); ok {
		classes = append(classes, fmt.Sprintf("w-[%dpx]", int(w)))
	}
	if h, ok := props["height"].(float64); ok {
		classes = append(classes, fmt.Sprintf("h-[%dpx]", int(h)))
	}
	if r, ok := props["borderRadius"].(float64); ok {
		classes = append(classes, fmt.Sprintf("rounded-[%dpx]", int(r)))
	}
	if props["display"] == "flex" {
		classes = append(classes, "flex")
		if props["flexDirection"] == "column" {
			classes = append(classes, "flex-col")
		}
	}
	if gap, ok := props["gap"].(float64); ok {
		classes = append(classes, fmt.Sprintf("gap-[%dpx]", int(gap)))
	}

	return classes
}

func formatNodeResult(r *GetNodeResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Node: %v\n", r.Node["name"]))
	sb.WriteString(fmt.Sprintf("Type: %v\n", r.Node["type"]))
	sb.WriteString(fmt.Sprintf("ID: %v\n", r.Node["id"]))
	if r.Path != "" {
		sb.WriteString(fmt.Sprintf("Path: %s\n", r.Path))
	}
	sb.WriteString(fmt.Sprintf("Children: %d\n\n", r.ChildrenCount))

	sb.WriteString("Properties:\n")
	for key, val := range r.Node {
		if key == "id" || key == "name" || key == "type" {
			continue
		}
		sb.WriteString(fmt.Sprintf("  %s: %v\n", key, val))
	}

	return sb.String()
}

func formatCSSResult(r *GetCSSResult) string {
	var sb strings.Builder

	for id, css := range r.CSS {
		sb.WriteString(fmt.Sprintf("/* Node: %s */\n", id))
		sb.WriteString(css)
		sb.WriteString("\n")
	}

	if len(r.Warnings) > 0 {
		sb.WriteString("\nWarnings:\n")
		for _, w := range r.Warnings {
			sb.WriteString(fmt.Sprintf("  - %s\n", w))
		}
	}

	return sb.String()
}

func formatTokensResult(r *GetTokensResult) string {
	var sb strings.Builder

	sb.WriteString("Token References\n")
	sb.WriteString("================\n\n")

	for id, tokens := range r.Tokens {
		sb.WriteString(fmt.Sprintf("Node: %s\n", id))
		if t, ok := tokens.(map[string]interface{}); ok {
			for prop, ref := range t {
				sb.WriteString(fmt.Sprintf("  %s: %v\n", prop, ref))
			}
		}
		sb.WriteString("\n")
	}

	if len(r.Collections) > 0 {
		sb.WriteString("Collections: " + strings.Join(r.Collections, ", ") + "\n")
	}

	return sb.String()
}
