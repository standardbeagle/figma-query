package tools

import (
	"testing"

	"github.com/standardbeagle/figma-query/internal/figma"
)

func TestMatchesFrom(t *testing.T) {
	node := &figma.Node{
		ID:   "1:2",
		Type: figma.NodeTypeFrame,
	}

	tests := []struct {
		name     string
		from     []string
		expected bool
	}{
		{"matches type", []string{"FRAME"}, true},
		{"wrong type", []string{"TEXT"}, false},
		{"matches id", []string{"#1:2"}, true},
		{"wrong id", []string{"#1:3"}, false},
		{"array contains type", []string{"FRAME", "GROUP"}, true},
		{"array missing type", []string{"TEXT", "GROUP"}, false},
		{"empty from matches all", []string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesFrom(node, tt.from)
			if result != tt.expected {
				t.Errorf("matchesFrom(%v) = %v, want %v", tt.from, result, tt.expected)
			}
		})
	}
}

func TestApplyOperator(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		op       string
		operand  interface{}
		expected bool
	}{
		{"$eq match", "Button", "$eq", "Button", true},
		{"$eq no match", "Button", "$eq", "Card", false},
		{"$match glob", "Button/Primary", "$match", "Button*", true},
		{"$match no match", "Card/Primary", "$match", "Button*", false},
		{"$contains match", "Button Primary", "$contains", "primary", true},
		{"$contains no match", "Button Primary", "$contains", "secondary", false},
		{"$gt true", 100.0, "$gt", 50.0, true},
		{"$gt false", 50.0, "$gt", 100.0, false},
		{"$lt true", 50.0, "$lt", 100.0, true},
		{"$exists true", "value", "$exists", true, true},
		{"$exists false", nil, "$exists", true, false},
		{"$in match", "FRAME", "$in", []interface{}{"FRAME", "GROUP"}, true},
		{"$in no match", "TEXT", "$in", []interface{}{"FRAME", "GROUP"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyOperator(tt.value, tt.op, tt.operand)
			if result != tt.expected {
				t.Errorf("applyOperator(%v, %s, %v) = %v, want %v",
					tt.value, tt.op, tt.operand, result, tt.expected)
			}
		})
	}
}

func TestProjectNode(t *testing.T) {
	node := &figma.Node{
		ID:   "1:2",
		Name: "Test Frame",
		Type: figma.NodeTypeFrame,
		AbsoluteBoundingBox: &figma.Rectangle{
			X: 0, Y: 0, Width: 100, Height: 200,
		},
	}

	// Test @structure projection
	result := projectNode(node, []string{"@structure"})
	if result["id"] != "1:2" {
		t.Errorf("expected id '1:2', got %v", result["id"])
	}
	if result["name"] != "Test Frame" {
		t.Errorf("expected name 'Test Frame', got %v", result["name"])
	}

	// Test @bounds projection
	result = projectNode(node, []string{"@bounds"})
	if result["width"] != 100.0 {
		t.Errorf("expected width 100, got %v", result["width"])
	}
	if result["height"] != 200.0 {
		t.Errorf("expected height 200, got %v", result["height"])
	}
}

func TestGetNodeField(t *testing.T) {
	visible := true
	opacity := 0.5
	node := &figma.Node{
		ID:      "1:2",
		Name:    "Test",
		Type:    figma.NodeTypeFrame,
		Visible: &visible,
		Opacity: &opacity,
		AbsoluteBoundingBox: &figma.Rectangle{
			Width: 100, Height: 200,
		},
		LayoutMode: "HORIZONTAL",
	}

	tests := []struct {
		field    string
		expected interface{}
	}{
		{"id", "1:2"},
		{"name", "Test"},
		{"type", "FRAME"},
		{"visible", true},
		{"width", 100.0},
		{"height", 200.0},
		{"opacity", 0.5},
		{"layoutMode", "HORIZONTAL"},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			result := getNodeField(node, tt.field)
			if result != tt.expected {
				t.Errorf("getNodeField(%s) = %v, want %v", tt.field, result, tt.expected)
			}
		})
	}
}
