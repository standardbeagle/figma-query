// Package tools implements the MCP tools for figma-query.
package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/standardbeagle/figma-query/internal/figma"
)

// Registry holds shared state for all tools.
type Registry struct {
	client    *figma.Client
	exportDir string
}

// NewRegistry creates a new tool registry.
func NewRegistry(client *figma.Client, exportDir string) *Registry {
	return &Registry{
		client:    client,
		exportDir: exportDir,
	}
}

// RegisterTools registers all tools with the MCP server.
func (r *Registry) RegisterTools(server *mcp.Server) {
	// Discovery tools
	registerInfoTool(server, r)

	// Export tools
	registerSyncFileTool(server, r)
	registerExportAssetsTool(server, r)
	registerExportTokensTool(server, r)
	registerDownloadImageTool(server, r)

	// Query tools
	registerQueryTool(server, r)
	registerSearchTool(server, r)
	registerGetTreeTool(server, r)
	registerListComponentsTool(server, r)
	registerListStylesTool(server, r)

	// Detail tools
	registerGetNodeTool(server, r)
	registerGetCSSTool(server, r)
	registerGetTokensTool(server, r)

	// Render tools
	registerWireframeTool(server, r)

	// Analysis tools
	registerDiffTool(server, r)
}

// HasClient returns true if a Figma client is configured.
func (r *Registry) HasClient() bool {
	return r.client != nil
}

// Client returns the Figma client.
func (r *Registry) Client() *figma.Client {
	return r.client
}

// ExportDir returns the export directory.
func (r *Registry) ExportDir() string {
	return r.exportDir
}
