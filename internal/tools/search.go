package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/standardbeagle/figma-query/internal/figma"
)

// SearchArgs contains arguments for the search tool.
type SearchArgs struct {
	FileKey   string   `json:"file_key" jsonschema:"Figma file key"`
	Pattern   string   `json:"pattern" jsonschema:"Search pattern (supports glob * and regex /pattern/)"`
	Scope     []string `json:"scope,omitempty" jsonschema:"Where to search: names text properties styles variables"`
	NodeTypes []string `json:"node_types,omitempty" jsonschema:"Filter by node type"`
	Select    []string `json:"select,omitempty" jsonschema:"Properties to return for matches"`
	Limit     int      `json:"limit,omitempty" jsonschema:"Max results (default: 50)"`
	Format    string   `json:"format,omitempty" jsonschema:"Response format: text (default) or json"`
}

// SearchResult contains the result of a search.
type SearchResult struct {
	Results []SearchMatch `json:"results"`
	Total   int           `json:"total"`
	HasMore bool          `json:"has_more"`
}

// SearchMatch represents a single search match.
type SearchMatch struct {
	NodeID       string `json:"node_id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Path         string `json:"path"`
	MatchContext string `json:"match_context"`
	MatchField   string `json:"match_field"`
}

func registerSearchTool(server *mcp.Server, r *Registry) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search",
		Description: "Full-text search across node names, text content, and properties.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args SearchArgs) (*mcp.CallToolResult, *SearchResult, error) {
		if args.FileKey == "" {
			return nil, nil, fmt.Errorf("file_key is required")
		}
		if args.Pattern == "" {
			return nil, nil, fmt.Errorf("pattern is required")
		}

		// Set defaults
		limit := args.Limit
		if limit == 0 {
			limit = 50
		}
		scope := args.Scope
		if len(scope) == 0 {
			scope = []string{"names", "text"}
		}

		// Try cache first, then API
		var nodes []*figma.Node
		cachedNodes, err := readNodesFromCache(r.ExportDir(), args.FileKey)
		if err == nil && len(cachedNodes) > 0 {
			nodes = cachedNodes
		} else if r.HasClient() {
			file, err := r.Client().GetFile(ctx, args.FileKey, nil)
			if err != nil {
				return nil, nil, fmt.Errorf("fetching file: %w", err)
			}
			nodes = flattenNodes(file.Document)
		} else {
			return nil, nil, fmt.Errorf("no cache found and Figma API not configured")
		}

		// Build regex from pattern
		re, err := buildSearchRegex(args.Pattern)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid pattern: %w", err)
		}

		// Search nodes
		var matches []SearchMatch
		for _, node := range nodes {
			// Filter by node type if specified
			if len(args.NodeTypes) > 0 && !containsString(args.NodeTypes, string(node.Type)) {
				continue
			}

			// Search in each scope
			for _, s := range scope {
				if match := searchInScope(node, s, re); match != nil {
					matches = append(matches, *match)
					break // Only add once per node
				}
			}

			if len(matches) >= limit {
				break
			}
		}

		result := &SearchResult{
			Results: matches,
			Total:   len(matches),
			HasMore: len(nodes) > limit,
		}

		// Format output
		var textOutput string
		if args.Format == "json" {
			b, _ := json.MarshalIndent(result, "", "  ")
			textOutput = string(b)
		} else {
			textOutput = formatSearchResult(result)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: textOutput},
			},
		}, result, nil
	})
}

func buildSearchRegex(pattern string) (*regexp.Regexp, error) {
	// Check if it's a regex pattern
	if strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") {
		return regexp.Compile(pattern[1 : len(pattern)-1])
	}

	// Convert glob to regex
	escaped := regexp.QuoteMeta(pattern)
	escaped = strings.ReplaceAll(escaped, `\*`, ".*")
	escaped = strings.ReplaceAll(escaped, `\?`, ".")

	return regexp.Compile("(?i)" + escaped)
}

func searchInScope(node *figma.Node, scope string, re *regexp.Regexp) *SearchMatch {
	switch scope {
	case "names":
		if re.MatchString(node.Name) {
			return &SearchMatch{
				NodeID:       node.ID,
				Name:         node.Name,
				Type:         string(node.Type),
				MatchContext: node.Name,
				MatchField:   "name",
			}
		}

	case "text":
		if node.Characters != "" && re.MatchString(node.Characters) {
			context := node.Characters
			if len(context) > 100 {
				context = context[:100] + "..."
			}
			return &SearchMatch{
				NodeID:       node.ID,
				Name:         node.Name,
				Type:         string(node.Type),
				MatchContext: context,
				MatchField:   "characters",
			}
		}

	case "properties":
		// Search in component ID, style IDs, etc.
		if node.ComponentID != "" && re.MatchString(node.ComponentID) {
			return &SearchMatch{
				NodeID:       node.ID,
				Name:         node.Name,
				Type:         string(node.Type),
				MatchContext: node.ComponentID,
				MatchField:   "componentId",
			}
		}
	}

	return nil
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

func formatSearchResult(r *SearchResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Found %d matches\n\n", r.Total))

	if len(r.Results) == 0 {
		sb.WriteString("No matches found.\n")
		return sb.String()
	}

	sb.WriteString("ID       | Name                           | Type      | Match\n")
	sb.WriteString("-------- | ------------------------------ | --------- | -----\n")

	for _, m := range r.Results {
		name := m.Name
		if len(name) > 30 {
			name = name[:27] + "..."
		}
		context := m.MatchContext
		if len(context) > 30 {
			context = context[:27] + "..."
		}

		sb.WriteString(fmt.Sprintf("%-8s | %-30s | %-9s | %s\n", m.NodeID, name, m.Type, context))
	}

	return sb.String()
}
