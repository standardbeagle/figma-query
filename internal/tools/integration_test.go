package tools_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/standardbeagle/figma-query/internal/figma"
	"github.com/standardbeagle/figma-query/internal/tools"
)

// testServer creates a connected MCP client session for integration testing.
// It uses in-memory transports for fast, reliable testing without I/O.
func testServer(t *testing.T, registry *tools.Registry) *mcp.ClientSession {
	t.Helper()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "figma-query-test",
		Version: "0.1.0",
	}, nil)

	registry.RegisterTools(server)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	ctx := context.Background()

	// Start server in background
	go func() {
		if err := server.Run(ctx, serverTransport); err != nil {
			t.Logf("server exited: %v", err)
		}
	}()

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "0.1.0",
	}, nil)

	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("failed to connect client: %v", err)
	}

	t.Cleanup(func() {
		session.Close()
	})

	return session
}

// mockFigmaClient creates a mock Figma client for testing.
// Currently returns nil since we test tools that work without API access.
func mockFigmaClient() *figma.Client {
	return nil
}

// testExportDir returns a temporary export directory for testing.
func testExportDir(t *testing.T) string {
	return t.TempDir()
}

func TestIntegration_ListTools(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	// Verify expected tools are registered
	expectedTools := []string{
		"info",
		"sync_file",
		"export_assets",
		"export_tokens",
		"query",
		"search",
		"get_tree",
		"list_components",
		"list_styles",
		"get_node",
		"get_css",
		"get_tokens",
		"wireframe",
		"diff",
	}

	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("expected tool %q not found in registered tools", expected)
		}
	}

	if len(result.Tools) != len(expectedTools) {
		t.Errorf("expected %d tools, got %d", len(expectedTools), len(result.Tools))
	}
}

func TestIntegration_InfoTool_Overview(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Call info tool without arguments (overview)
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "info",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool(info) failed: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	// Verify overview contains expected sections
	expectedStrings := []string{
		"figma-query",
		"Authentication:",
		"Export directory:",
		"Tool Groups",
	}

	for _, expected := range expectedStrings {
		if !containsSubstring(textContent.Text, expected) {
			t.Errorf("overview should contain %q", expected)
		}
	}
}

func TestIntegration_InfoTool_Topics(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	topics := []struct {
		name     string
		contains []string
	}{
		{
			name:     "tools",
			contains: []string{"Available Tools", "info", "query", "search"},
		},
		{
			name:     "projections",
			contains: []string{"Built-in Projections", "@structure", "@css", "@layout"},
		},
		{
			name:     "query",
			contains: []string{"Query DSL", "from", "where", "select"},
		},
		{
			name:     "operators",
			contains: []string{"WHERE Clause Operators", "$eq", "$match", "$contains"},
		},
		{
			name:     "export",
			contains: []string{"Export Directory", "_meta.json", "_tree.txt"},
		},
		{
			name:     "examples",
			contains: []string{"Common Workflows", "sync_file", "get_tree"},
		},
		{
			name:     "status",
			contains: []string{"Server Status", "Version", "Authentication"},
		},
	}

	for _, tc := range topics {
		t.Run(tc.name, func(t *testing.T) {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name: "info",
				Arguments: map[string]any{
					"topic": tc.name,
				},
			})
			if err != nil {
				t.Fatalf("CallTool(info, topic=%s) failed: %v", tc.name, err)
			}

			if len(result.Content) == 0 {
				t.Fatal("expected content in result")
			}

			textContent, ok := result.Content[0].(*mcp.TextContent)
			if !ok {
				t.Fatalf("expected TextContent, got %T", result.Content[0])
			}

			for _, expected := range tc.contains {
				if !containsSubstring(textContent.Text, expected) {
					t.Errorf("topic %q should contain %q", tc.name, expected)
				}
			}
		})
	}
}

func TestIntegration_InfoTool_JSONFormat(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "info",
		Arguments: map[string]any{
			"topic":  "status",
			"format": "json",
		},
	})
	if err != nil {
		t.Fatalf("CallTool(info, format=json) failed: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	// Verify the text output is still text (the result contains text representation)
	// The JSON data is in the typed result which isn't directly accessible via CallTool
	if textContent.Text == "" {
		t.Error("expected non-empty text content")
	}
}

func TestIntegration_InfoTool_UnknownTopic(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "info",
		Arguments: map[string]any{
			"topic": "unknown_topic",
		},
	})
	if err != nil {
		t.Fatalf("CallTool(info, topic=unknown) failed: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	if !containsSubstring(textContent.Text, "Unknown topic") {
		t.Error("expected 'Unknown topic' message for invalid topic")
	}
}

func TestIntegration_SearchTool_MissingFileKey(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "search",
		Arguments: map[string]any{
			"pattern": "Button",
		},
	})

	// Should return error for missing file_key
	if err == nil {
		t.Fatal("expected error for missing file_key")
	}
}

func TestIntegration_SearchTool_MissingPattern(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "search",
		Arguments: map[string]any{
			"file_key": "abc123",
		},
	})

	// Should return error for missing pattern
	if err == nil {
		t.Fatal("expected error for missing pattern")
	}
}

func TestIntegration_GetTreeTool_MissingFileKey(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_tree",
		Arguments: map[string]any{},
	})

	// Should return error for missing file_key
	if err == nil {
		t.Fatal("expected error for missing file_key")
	}
}

func TestIntegration_QueryTool_MissingFileKey(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "query",
		Arguments: map[string]any{
			"q": map[string]any{
				"from": "FRAME",
			},
		},
	})

	// Should return error for missing file_key
	if err == nil {
		t.Fatal("expected error for missing file_key")
	}
}

func TestIntegration_ToolsRequireAPIorCache(t *testing.T) {
	// Test that tools properly return errors when no API or cache is available
	registry := tools.NewRegistry(nil, testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	toolsRequiringData := []struct {
		name string
		args map[string]any
	}{
		{
			name: "search",
			args: map[string]any{
				"file_key": "test123",
				"pattern":  "Button",
			},
		},
		{
			name: "get_tree",
			args: map[string]any{
				"file_key": "test123",
			},
		},
		{
			name: "query",
			args: map[string]any{
				"file_key": "test123",
				"q":        map[string]any{"from": "FRAME"},
			},
		},
		{
			name: "list_components",
			args: map[string]any{
				"file_key": "test123",
			},
		},
		{
			name: "list_styles",
			args: map[string]any{
				"file_key": "test123",
			},
		},
	}

	for _, tc := range toolsRequiringData {
		t.Run(tc.name, func(t *testing.T) {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      tc.name,
				Arguments: tc.args,
			})

			// MCP tool errors are returned as IsError=true in the result,
			// not as Go errors from CallTool
			if err != nil {
				t.Fatalf("unexpected transport error: %v", err)
			}

			// These should have IsError=true since no API client or cache exists
			if !result.IsError {
				t.Errorf("tool %q should return IsError=true when no API or cache available", tc.name)
			}
		})
	}
}

func TestIntegration_Ping(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := session.Ping(ctx, nil)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestIntegration_ServerCapabilities(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	// The session should have been initialized with server capabilities
	// We can verify the session is working by making calls
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// ListTools should work, confirming tools capability
	result, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	if len(result.Tools) == 0 {
		t.Error("expected at least one tool to be registered")
	}
}

func TestIntegration_ToolInputSchema(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	// Find the info tool and check its schema
	var infoTool *mcp.Tool
	for _, tool := range result.Tools {
		if tool.Name == "info" {
			infoTool = tool
			break
		}
	}

	if infoTool == nil {
		t.Fatal("info tool not found")
	}

	// Verify the tool has a description
	if infoTool.Description == "" {
		t.Error("info tool should have a description")
	}

	// Verify input schema exists
	if infoTool.InputSchema == nil {
		t.Error("info tool should have an input schema")
	}
}

func TestIntegration_ConcurrentToolCalls(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Make multiple concurrent calls to verify thread safety
	const numCalls = 10
	results := make(chan error, numCalls)

	for i := 0; i < numCalls; i++ {
		go func() {
			_, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "info",
				Arguments: map[string]any{},
			})
			results <- err
		}()
	}

	// Collect results
	for i := 0; i < numCalls; i++ {
		if err := <-results; err != nil {
			t.Errorf("concurrent call %d failed: %v", i, err)
		}
	}
}

func TestIntegration_DiffTool_MissingArgs(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "diff",
		Arguments: map[string]any{},
	})

	// Should return error for missing required arguments
	if err == nil {
		t.Fatal("expected error for missing diff arguments")
	}
}

func TestIntegration_WireframeTool_MissingArgs(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "wireframe",
		Arguments: map[string]any{},
	})

	// Should return error for missing required arguments
	if err == nil {
		t.Fatal("expected error for missing wireframe arguments")
	}
}

func TestIntegration_GetNodeTool_MissingArgs(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_node",
		Arguments: map[string]any{},
	})

	// Should return error for missing required arguments
	if err == nil {
		t.Fatal("expected error for missing get_node arguments")
	}
}

func TestIntegration_GetCSSTool_MissingArgs(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_css",
		Arguments: map[string]any{},
	})

	// Should return error for missing required arguments
	if err == nil {
		t.Fatal("expected error for missing get_css arguments")
	}
}

func TestIntegration_GetTokensTool_MissingArgs(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_tokens",
		Arguments: map[string]any{},
	})

	// Should return error for missing required arguments
	if err == nil {
		t.Fatal("expected error for missing get_tokens arguments")
	}
}

func TestIntegration_ExportAssetsTool_MissingArgs(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "export_assets",
		Arguments: map[string]any{},
	})

	// Should return error for missing required arguments
	if err == nil {
		t.Fatal("expected error for missing export_assets arguments")
	}
}

func TestIntegration_ExportTokensTool_MissingArgs(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "export_tokens",
		Arguments: map[string]any{},
	})

	// Should return error for missing required arguments
	if err == nil {
		t.Fatal("expected error for missing export_tokens arguments")
	}
}

func TestIntegration_SyncFileTool_MissingArgs(t *testing.T) {
	registry := tools.NewRegistry(mockFigmaClient(), testExportDir(t))
	session := testServer(t, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "sync_file",
		Arguments: map[string]any{},
	})

	// Should return error for missing required arguments
	if err == nil {
		t.Fatal("expected error for missing sync_file arguments")
	}
}

func TestIntegration_RegistryWithClient(t *testing.T) {
	// Test that HasClient returns correct values
	withoutClient := tools.NewRegistry(nil, testExportDir(t))
	if withoutClient.HasClient() {
		t.Error("expected HasClient() to return false when no client provided")
	}

	// We can't easily create a real client without a token,
	// but we can test the nil case works
	if withoutClient.Client() != nil {
		t.Error("expected Client() to return nil when no client provided")
	}

	// Test ExportDir returns the correct value
	exportDir := testExportDir(t)
	registry := tools.NewRegistry(nil, exportDir)
	if registry.ExportDir() != exportDir {
		t.Errorf("expected ExportDir() to return %q, got %q", exportDir, registry.ExportDir())
	}
}

// containsSubstring checks if s contains substr (case-sensitive).
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Benchmark tests for performance validation
func BenchmarkInfoTool(b *testing.B) {
	registry := tools.NewRegistry(nil, b.TempDir())

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "figma-query-bench",
		Version: "0.1.0",
	}, nil)
	registry.RegisterTools(server)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()

	go server.Run(ctx, serverTransport)

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "bench-client",
		Version: "0.1.0",
	}, nil)

	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		b.Fatalf("failed to connect: %v", err)
	}
	defer session.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "info",
			Arguments: map[string]any{},
		})
		if err != nil {
			b.Fatalf("CallTool failed: %v", err)
		}
	}
}

func BenchmarkListTools(b *testing.B) {
	registry := tools.NewRegistry(nil, b.TempDir())

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "figma-query-bench",
		Version: "0.1.0",
	}, nil)
	registry.RegisterTools(server)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()

	go server.Run(ctx, serverTransport)

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "bench-client",
		Version: "0.1.0",
	}, nil)

	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		b.Fatalf("failed to connect: %v", err)
	}
	defer session.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := session.ListTools(ctx, nil)
		if err != nil {
			b.Fatalf("ListTools failed: %v", err)
		}
	}
}

// Helper for printing JSON responses in debugging
func prettyJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
