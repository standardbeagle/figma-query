// figma-query is a token-efficient MCP server for Figma with query-based data shaping.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/standardbeagle/figma-query/internal/figma"
	"github.com/standardbeagle/figma-query/internal/tools"
)

const (
	serverName    = "figma-query"
	serverVersion = "0.1.0"
)

// debugLog is a logger that writes to a file for debugging MCP communication
var debugLog *log.Logger

func initDebugLog() {
	// Create log directory if needed
	logDir := os.Getenv("HOME")
	if logDir == "" {
		logDir = "/tmp"
	}
	logPath := filepath.Join(logDir, ".figma-query-debug.log")

	// Open log file (append mode)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		// Can't log to file, use a no-op logger
		debugLog = log.New(os.Stderr, "", 0)
		return
	}

	debugLog = log.New(f, "", log.LstdFlags|log.Lmicroseconds)
	debugLog.Printf("\n\n=== figma-query started at %s ===", time.Now().Format(time.RFC3339))
	debugLog.Printf("PID: %d", os.Getpid())
	debugLog.Printf("Args: %v", os.Args)
}

func main() {
	// Initialize debug logging first (writes to ~/.figma-query-debug.log)
	initDebugLog()

	// Parse CLI flags
	showVersion := flag.Bool("version", false, "Show version and exit")
	showHelp := flag.Bool("help", false, "Show help and exit")
	flag.Parse()

	debugLog.Printf("Flags parsed: version=%v, help=%v", *showVersion, *showHelp)

	if *showHelp {
		fmt.Printf(`%s v%s - Token-efficient MCP server for Figma

Usage: %s [options]

This server runs on stdio transport for MCP clients.

Environment Variables:
  FIGMA_ACCESS_TOKEN          Figma personal access token (required for API)
  FIGMA_TOKEN                 Alternative name for access token
  FIGMA_PERSONAL_ACCESS_TOKEN Alternative name for access token
  FIGMA_EXPORT_DIR            Directory for file exports (default: ./figma-export)

Options:
`, serverName, serverVersion, os.Args[0])
		flag.PrintDefaults()
		os.Exit(0)
	}

	if *showVersion {
		fmt.Printf("%s v%s\n", serverName, serverVersion)
		os.Exit(0)
	}

	// Get Figma access token from environment
	accessToken := os.Getenv("FIGMA_ACCESS_TOKEN")
	if accessToken == "" {
		// Also check for common alternative env var names
		accessToken = os.Getenv("FIGMA_TOKEN")
	}
	if accessToken == "" {
		accessToken = os.Getenv("FIGMA_PERSONAL_ACCESS_TOKEN")
	}

	// Create Figma client (may be nil if no token)
	var figmaClient *figma.Client
	if accessToken != "" {
		figmaClient = figma.NewClient(accessToken)
		debugLog.Printf("Figma client created with token")
	} else {
		debugLog.Printf("No Figma token - client will be nil")
	}

	// Get export directory from environment or use default
	exportDir := os.Getenv("FIGMA_EXPORT_DIR")
	if exportDir == "" {
		exportDir = "./figma-export"
	}
	debugLog.Printf("Export directory: %s", exportDir)

	// Create tool registry
	debugLog.Printf("Creating tool registry...")
	registry := tools.NewRegistry(figmaClient, exportDir)

	// Create MCP server
	// Using nil ServerOptions like test-mcp which works with Claude Code
	debugLog.Printf("Creating MCP server with nil ServerOptions")
	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)
	debugLog.Printf("MCP server created")

	// Register all tools
	debugLog.Printf("Registering tools with server...")
	registry.RegisterTools(server)
	debugLog.Printf("Tools registered")

	// Run server on stdio transport
	debugLog.Printf("Starting server on stdio transport...")
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		debugLog.Printf("Server error: %v", err)
		log.Fatalf("Server error: %v", err)
	}
	debugLog.Printf("Server stopped")
}
