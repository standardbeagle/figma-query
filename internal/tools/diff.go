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

// DiffArgs contains arguments for the diff tool.
type DiffArgs struct {
	FileKey   string   `json:"file_key" jsonschema:"Figma file key"`
	Compare   string   `json:"compare,omitempty" jsonschema:"What to compare: last_sync or version"`
	VersionID string   `json:"version_id,omitempty" jsonschema:"Specific version ID (if compare=version)"`
	Scope     []string `json:"scope,omitempty" jsonschema:"What to compare: structure properties styles components"`
	Format    string   `json:"format,omitempty" jsonschema:"Response format: text (default) or json"`
}

// DiffResult contains the result of diff comparison.
type DiffResult struct {
	Added    []NodeChange `json:"added"`
	Removed  []NodeChange `json:"removed"`
	Modified []NodeChange `json:"modified"`
	Summary  string       `json:"summary"`
}

// NodeChange represents a change to a node.
type NodeChange struct {
	ID      string                 `json:"id"`
	Name    string                 `json:"name"`
	Type    string                 `json:"type"`
	Changes map[string]interface{} `json:"changes,omitempty"`
}

func registerDiffTool(server *mcp.Server, r *Registry) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "diff",
		Description: "Compare two exports or file versions.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args DiffArgs) (*mcp.CallToolResult, *DiffResult, error) {
		if args.FileKey == "" {
			return nil, nil, fmt.Errorf("file_key is required")
		}

		// Set defaults
		compare := args.Compare
		if compare == "" {
			compare = "last_sync"
		}
		scope := args.Scope
		if len(scope) == 0 {
			scope = []string{"structure", "properties"}
		}

		// Get current state from API
		if !r.HasClient() {
			return nil, nil, fmt.Errorf("Figma API not configured")
		}

		currentFile, err := r.Client().GetFile(ctx, args.FileKey, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("fetching current file: %w", err)
		}

		// Get comparison state
		var previousNodes map[string]*figma.Node

		switch compare {
		case "last_sync":
			// Read from local cache
			previousNodes, err = readCachedNodes(r.ExportDir(), args.FileKey)
			if err != nil {
				return nil, nil, fmt.Errorf("no previous sync found: %w", err)
			}

		case "version":
			if args.VersionID == "" {
				return nil, nil, fmt.Errorf("version_id required when compare=version")
			}
			// Fetch specific version
			prevFile, err := r.Client().GetFile(ctx, args.FileKey, &figma.GetFileOptions{
				Version: args.VersionID,
			})
			if err != nil {
				return nil, nil, fmt.Errorf("fetching version %s: %w", args.VersionID, err)
			}
			previousNodes = flattenToMap(prevFile.Document)

		default:
			return nil, nil, fmt.Errorf("invalid compare mode: %s", compare)
		}

		// Flatten current nodes
		currentNodes := flattenToMap(currentFile.Document)

		// Compare
		result := compareNodes(previousNodes, currentNodes, scope)

		// Build summary
		result.Summary = fmt.Sprintf("%d added, %d removed, %d modified",
			len(result.Added), len(result.Removed), len(result.Modified))

		// Format output
		var textOutput string
		if args.Format == "json" {
			b, _ := json.MarshalIndent(result, "", "  ")
			textOutput = string(b)
		} else {
			textOutput = formatDiffResult(result)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: textOutput},
			},
		}, result, nil
	})
}

func readCachedNodes(exportDir, fileKey string) (map[string]*figma.Node, error) {
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
			// Found matching export
			nodes, _ := readNodesFromExport(filepath.Join(exportDir, entry.Name()))
			nodeMap := make(map[string]*figma.Node)
			for _, n := range nodes {
				nodeMap[n.ID] = n
			}
			return nodeMap, nil
		}
	}

	return nil, fmt.Errorf("no cache found for file %s", fileKey)
}

func flattenToMap(doc *figma.DocumentNode) map[string]*figma.Node {
	nodes := make(map[string]*figma.Node)

	var walk func(*figma.Node)
	walk = func(n *figma.Node) {
		nodes[n.ID] = n
		for _, child := range n.Children {
			walk(child)
		}
	}

	if doc != nil {
		for _, page := range doc.Children {
			walk(page)
		}
	}

	return nodes
}

func compareNodes(previous, current map[string]*figma.Node, scope []string) *DiffResult {
	result := &DiffResult{
		Added:    make([]NodeChange, 0),
		Removed:  make([]NodeChange, 0),
		Modified: make([]NodeChange, 0),
	}

	includeStructure := containsString(scope, "structure")
	includeProperties := containsString(scope, "properties")

	// Find added and modified nodes
	for id, currNode := range current {
		prevNode, exists := previous[id]

		if !exists {
			if includeStructure {
				result.Added = append(result.Added, NodeChange{
					ID:   id,
					Name: currNode.Name,
					Type: string(currNode.Type),
				})
			}
			continue
		}

		// Check for modifications
		changes := make(map[string]interface{})

		if includeStructure {
			if currNode.Name != prevNode.Name {
				changes["name"] = map[string]string{
					"from": prevNode.Name,
					"to":   currNode.Name,
				}
			}
			if currNode.Type != prevNode.Type {
				changes["type"] = map[string]string{
					"from": string(prevNode.Type),
					"to":   string(currNode.Type),
				}
			}
		}

		if includeProperties {
			// Compare key properties
			if currNode.AbsoluteBoundingBox != nil && prevNode.AbsoluteBoundingBox != nil {
				if currNode.AbsoluteBoundingBox.Width != prevNode.AbsoluteBoundingBox.Width ||
					currNode.AbsoluteBoundingBox.Height != prevNode.AbsoluteBoundingBox.Height {
					changes["size"] = map[string]interface{}{
						"from": fmt.Sprintf("%.0fx%.0f", prevNode.AbsoluteBoundingBox.Width, prevNode.AbsoluteBoundingBox.Height),
						"to":   fmt.Sprintf("%.0fx%.0f", currNode.AbsoluteBoundingBox.Width, currNode.AbsoluteBoundingBox.Height),
					}
				}
			}

			// Compare fills
			if len(currNode.Fills) != len(prevNode.Fills) {
				changes["fills"] = map[string]int{
					"from": len(prevNode.Fills),
					"to":   len(currNode.Fills),
				}
			}

			// Compare strokes
			if len(currNode.Strokes) != len(prevNode.Strokes) {
				changes["strokes"] = map[string]int{
					"from": len(prevNode.Strokes),
					"to":   len(currNode.Strokes),
				}
			}

			// Compare effects
			if len(currNode.Effects) != len(prevNode.Effects) {
				changes["effects"] = map[string]int{
					"from": len(prevNode.Effects),
					"to":   len(currNode.Effects),
				}
			}

			// Compare text content
			if currNode.Characters != prevNode.Characters {
				if currNode.Characters != "" || prevNode.Characters != "" {
					changes["characters"] = map[string]string{
						"from": truncate(prevNode.Characters, 50),
						"to":   truncate(currNode.Characters, 50),
					}
				}
			}

			// Compare layout
			if currNode.LayoutMode != prevNode.LayoutMode {
				changes["layoutMode"] = map[string]string{
					"from": prevNode.LayoutMode,
					"to":   currNode.LayoutMode,
				}
			}
		}

		if len(changes) > 0 {
			result.Modified = append(result.Modified, NodeChange{
				ID:      id,
				Name:    currNode.Name,
				Type:    string(currNode.Type),
				Changes: changes,
			})
		}
	}

	// Find removed nodes
	if includeStructure {
		for id, prevNode := range previous {
			if _, exists := current[id]; !exists {
				result.Removed = append(result.Removed, NodeChange{
					ID:   id,
					Name: prevNode.Name,
					Type: string(prevNode.Type),
				})
			}
		}
	}

	return result
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatDiffResult(r *DiffResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Diff Summary: %s\n\n", r.Summary))

	if len(r.Added) > 0 {
		sb.WriteString(fmt.Sprintf("Added (%d):\n", len(r.Added)))
		for _, n := range r.Added[:min(10, len(r.Added))] {
			sb.WriteString(fmt.Sprintf("  + [%s] %s (%s)\n", n.ID, n.Name, n.Type))
		}
		if len(r.Added) > 10 {
			sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(r.Added)-10))
		}
		sb.WriteString("\n")
	}

	if len(r.Removed) > 0 {
		sb.WriteString(fmt.Sprintf("Removed (%d):\n", len(r.Removed)))
		for _, n := range r.Removed[:min(10, len(r.Removed))] {
			sb.WriteString(fmt.Sprintf("  - [%s] %s (%s)\n", n.ID, n.Name, n.Type))
		}
		if len(r.Removed) > 10 {
			sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(r.Removed)-10))
		}
		sb.WriteString("\n")
	}

	if len(r.Modified) > 0 {
		sb.WriteString(fmt.Sprintf("Modified (%d):\n", len(r.Modified)))
		for _, n := range r.Modified[:min(10, len(r.Modified))] {
			sb.WriteString(fmt.Sprintf("  ~ [%s] %s\n", n.ID, n.Name))
			for prop, change := range n.Changes {
				sb.WriteString(fmt.Sprintf("      %s: %v\n", prop, change))
			}
		}
		if len(r.Modified) > 10 {
			sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(r.Modified)-10))
		}
	}

	return sb.String()
}
