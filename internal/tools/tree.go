package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/standardbeagle/figma-query/internal/figma"
)

// GetTreeArgs contains arguments for the get_tree tool.
type GetTreeArgs struct {
	FileKey    string   `json:"file_key" jsonschema:"Figma file key"`
	RootNodeID string   `json:"root_node_id,omitempty" jsonschema:"Start from specific node (default: entire file)"`
	Depth      int      `json:"depth,omitempty" jsonschema:"Max depth to show (default: 3)"`
	MaxNodes   int      `json:"max_nodes,omitempty" jsonschema:"Maximum nodes to return (default: 500, max: 2000)"`
	HideIDs    bool     `json:"hide_ids,omitempty" jsonschema:"Hide node IDs in tree (default: false, IDs shown)"`
	NodeTypes  []string `json:"node_types,omitempty" jsonschema:"Only show these node types"`
	Format     string   `json:"format,omitempty" jsonschema:"Response format: text (default) or json"`
	OutputFile string   `json:"output_file,omitempty" jsonschema:"Write full output to file path (useful for large trees)"`
}

// TreeNode represents a node in the tree output.
type TreeNode struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Children []*TreeNode `json:"children,omitempty"`
}

// GetTreeResult contains the result of get_tree.
type GetTreeResult struct {
	Tree      []*TreeNode `json:"tree"`
	Text      string      `json:"text,omitempty"`
	Total     int         `json:"total"`
	Returned  int         `json:"returned"`
	MaxDepth  int         `json:"max_depth"`
	Truncated bool        `json:"truncated"`
	FilePath  string      `json:"file_path,omitempty"`
}

func registerGetTreeTool(server *mcp.Server, r *Registry) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_tree",
		Description: "Get file structure as ASCII tree or JSON tree with node IDs.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetTreeArgs) (*mcp.CallToolResult, any, error) {
		if args.FileKey == "" {
			return nil, nil, fmt.Errorf("file_key is required")
		}

		// Set defaults
		depth := args.Depth
		if depth == 0 {
			depth = 3
		}
		maxNodes := args.MaxNodes
		if maxNodes == 0 {
			maxNodes = 500
		}
		if maxNodes > 2000 {
			maxNodes = 2000
		}
		showIDs := !args.HideIDs

		// Try cache first, then API
		var file *figma.File
		cachedNodes, err := readNodesFromCache(r.ExportDir(), args.FileKey)
		if err == nil && len(cachedNodes) > 0 {
			// Build tree from cached nodes - this is simplified
			// In real impl, we'd need parent references
		}

		if file == nil {
			if !r.HasClient() {
				return nil, nil, fmt.Errorf("no cache found and Figma API not configured")
			}

			file, err = r.Client().GetFile(ctx, args.FileKey, &figma.GetFileOptions{
				Depth: depth + 1, // Get one extra level for truncation indicator
			})
			if err != nil {
				return nil, nil, fmt.Errorf("fetching file: %w", err)
			}
		}

		// Build tree with node limit tracking
		var tree []*TreeNode
		var lines []string
		totalNodes := 0
		returnedNodes := 0
		truncated := false

		// TreeBuilder context to track limits
		buildCtx := &treeBuildContext{
			maxNodes:      maxNodes,
			returnedNodes: &returnedNodes,
			truncated:     &truncated,
		}

		if file.Document != nil {
			for _, page := range file.Document.Children {
				if page.Type == figma.NodeTypeCanvas {
					// Filter by root node if specified
					if args.RootNodeID != "" && page.ID != args.RootNodeID {
						// Check children
						rootNode := findNode(page, args.RootNodeID)
						if rootNode != nil {
							treeNode := buildTreeNodeLimited(rootNode, 0, depth, args.NodeTypes, showIDs, &lines, &totalNodes, buildCtx)
							if treeNode != nil {
								tree = append(tree, treeNode)
							}
						}
						continue
					}

					treeNode := buildTreeNodeLimited(page, 0, depth, args.NodeTypes, showIDs, &lines, &totalNodes, buildCtx)
					if treeNode != nil {
						tree = append(tree, treeNode)
					}
				}
			}
		}

		result := &GetTreeResult{
			Tree:      tree,
			Total:     totalNodes,
			Returned:  returnedNodes,
			MaxDepth:  depth,
			Truncated: truncated,
		}

		// Format output
		var textOutput string
		if args.Format == "json" {
			b, _ := json.MarshalIndent(result, "", "  ")
			textOutput = string(b)
		} else {
			result.Text = strings.Join(lines, "\n")
			textOutput = result.Text

			// Add summary
			if truncated {
				textOutput += fmt.Sprintf("\n\n[Showing %d of %d nodes, max depth %d - TRUNCATED]", returnedNodes, totalNodes, depth)
				textOutput += FormatTruncationWarning(totalNodes, returnedNodes, "get_tree")
			} else {
				textOutput += fmt.Sprintf("\n\n[%d nodes, max depth %d]", totalNodes, depth)
			}
		}

		// Handle large output / file writing
		outputCfg := OutputConfig{
			MaxOutputSize: DefaultMaxOutputSize,
			OutputFile:    args.OutputFile,
			OutputDir:     r.ExportDir(),
			ToolName:      "get_tree",
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

// treeBuildContext tracks state during tree building.
type treeBuildContext struct {
	maxNodes      int
	returnedNodes *int
	truncated     *bool
}

// buildTreeNodeLimited builds a tree node with limit tracking.
func buildTreeNodeLimited(node *figma.Node, currentDepth, maxDepth int, nodeTypes []string, showIDs bool, lines *[]string, total *int, ctx *treeBuildContext) *TreeNode {
	*total++

	// Check if we've hit the limit
	if *ctx.returnedNodes >= ctx.maxNodes {
		if !*ctx.truncated {
			*ctx.truncated = true
			*lines = append(*lines, fmt.Sprintf("... truncated at %d nodes (use max_nodes to increase)", ctx.maxNodes))
		}
		return nil
	}

	// Check node type filter
	if len(nodeTypes) > 0 && !containsString(nodeTypes, string(node.Type)) {
		return nil
	}

	*ctx.returnedNodes++

	treeNode := &TreeNode{
		ID:   node.ID,
		Name: node.Name,
		Type: string(node.Type),
	}

	// Build line
	indent := ""
	if currentDepth > 0 {
		indent = strings.Repeat("│   ", currentDepth-1) + "├── "
	}

	line := indent + node.Name
	if showIDs {
		line += fmt.Sprintf(" [%s]", node.ID)
	}
	line += fmt.Sprintf(" (%s)", node.Type)
	*lines = append(*lines, line)

	// Process children
	if currentDepth < maxDepth && len(node.Children) > 0 {
		childCount := 0
		for i, child := range node.Children {
			// Check limit before processing child
			if *ctx.returnedNodes >= ctx.maxNodes {
				if !*ctx.truncated {
					*ctx.truncated = true
					moreIndent := strings.Repeat("│   ", currentDepth) + "├── "
					*lines = append(*lines, fmt.Sprintf("%s... truncated (%d more siblings)", moreIndent, len(node.Children)-i))
				}
				break
			}

			childNode := buildTreeNodeLimited(child, currentDepth+1, maxDepth, nodeTypes, showIDs, lines, total, ctx)
			if childNode != nil {
				treeNode.Children = append(treeNode.Children, childNode)
				childCount++
			}

			// Limit children shown per node to prevent wide trees
			if childCount >= 30 && len(node.Children) > 35 {
				moreIndent := strings.Repeat("│   ", currentDepth) + "├── "
				*lines = append(*lines, fmt.Sprintf("%s... and %d more siblings", moreIndent, len(node.Children)-i-1))
				break
			}
		}
	} else if len(node.Children) > 0 {
		// Indicate there are more children
		childIndent := strings.Repeat("│   ", currentDepth) + "└── "
		*lines = append(*lines, fmt.Sprintf("%s... (%d children)", childIndent, len(node.Children)))
	}

	return treeNode
}

func findNode(root *figma.Node, id string) *figma.Node {
	if root.ID == id {
		return root
	}
	for _, child := range root.Children {
		if found := findNode(child, id); found != nil {
			return found
		}
	}
	return nil
}

