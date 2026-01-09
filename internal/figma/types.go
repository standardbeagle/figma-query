package figma

import (
	"encoding/json"
	"fmt"
)

// APIError represents an error returned by the Figma API.
type APIError struct {
	Status int    `json:"status"`
	Err    string `json:"err"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("figma API error (status %d): %s", e.Status, e.Err)
}

// RateLimitError indicates the API rate limit was exceeded.
type RateLimitError struct {
	RetryAfter string
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded, retry after: %s", e.RetryAfter)
}

// GetFileOptions contains options for the GetFile API call.
type GetFileOptions struct {
	Version    string // File version to retrieve
	Depth      int    // Depth of nodes to retrieve
	Geometry   string // "paths" to include vector path data
	PluginData string // Plugin ID to include plugin data
	BranchData bool   // Include branch metadata
}

// ImageExportOptions contains options for image export.
type ImageExportOptions struct {
	Format            string  // png, jpg, svg, pdf
	Scale             float64 // 0.01 to 4
	SVGIncludeID      bool    // Include IDs in SVG
	SVGSimplifyStroke bool    // Simplify strokes in SVG
	UseAbsoluteBounds bool    // Use absolute bounds
}

// File represents a Figma file.
type File struct {
	Name          string          `json:"name"`
	Role          string          `json:"role"`
	LastModified  string          `json:"lastModified"`
	EditorType    string          `json:"editorType"`
	ThumbnailURL  string          `json:"thumbnailUrl"`
	Version       string          `json:"version"`
	Document      *DocumentNode   `json:"document"`
	Components    map[string]*Component    `json:"components"`
	ComponentSets map[string]*ComponentSet `json:"componentSets"`
	SchemaVersion int             `json:"schemaVersion"`
	Styles        map[string]*Style `json:"styles"`
	MainFileKey   string          `json:"mainFileKey"`
	Branches      []Branch        `json:"branches"`
}

// FileNodes represents a response from the file nodes endpoint.
type FileNodes struct {
	Name         string                  `json:"name"`
	LastModified string                  `json:"lastModified"`
	ThumbnailURL string                  `json:"thumbnailUrl"`
	Version      string                  `json:"version"`
	Role         string                  `json:"role"`
	Nodes        map[string]*NodeWrapper `json:"nodes"`
}

// NodeWrapper wraps a node in the file nodes response.
type NodeWrapper struct {
	Document   *Node             `json:"document"`
	Components map[string]*Component `json:"components"`
	Styles     map[string]*Style     `json:"styles"`
}

// DocumentNode represents the document root node.
type DocumentNode struct {
	Node
	Children []*Node `json:"children"`
}

// Node represents a generic Figma node.
type Node struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Type                NodeType        `json:"type"`
	Visible             *bool           `json:"visible,omitempty"`
	Locked              *bool           `json:"locked,omitempty"`
	PluginData          json.RawMessage `json:"pluginData,omitempty"`
	SharedPluginData    json.RawMessage `json:"sharedPluginData,omitempty"`
	ComponentPropertyReferences json.RawMessage `json:"componentPropertyReferences,omitempty"`

	// Geometry
	AbsoluteBoundingBox   *Rectangle `json:"absoluteBoundingBox,omitempty"`
	AbsoluteRenderBounds  *Rectangle `json:"absoluteRenderBounds,omitempty"`
	Constraints           *Constraints `json:"constraints,omitempty"`
	RelativeTransform     [][]float64 `json:"relativeTransform,omitempty"`
	Size                  *Vector    `json:"size,omitempty"`

	// Visual properties
	Fills               []Paint       `json:"fills,omitempty"`
	Strokes             []Paint       `json:"strokes,omitempty"`
	StrokeWeight        float64       `json:"strokeWeight,omitempty"`
	StrokeAlign         string        `json:"strokeAlign,omitempty"`
	StrokeCap           string        `json:"strokeCap,omitempty"`
	StrokeJoin          string        `json:"strokeJoin,omitempty"`
	StrokeDashes        []float64     `json:"strokeDashes,omitempty"`
	CornerRadius        float64       `json:"cornerRadius,omitempty"`
	RectangleCornerRadii []float64    `json:"rectangleCornerRadii,omitempty"`
	Effects             []Effect      `json:"effects,omitempty"`
	BlendMode           string        `json:"blendMode,omitempty"`
	Opacity             *float64      `json:"opacity,omitempty"`
	IsMask              bool          `json:"isMask,omitempty"`

	// Layout
	LayoutMode              string   `json:"layoutMode,omitempty"`
	LayoutWrap              string   `json:"layoutWrap,omitempty"`
	PrimaryAxisSizingMode   string   `json:"primaryAxisSizingMode,omitempty"`
	CounterAxisSizingMode   string   `json:"counterAxisSizingMode,omitempty"`
	PrimaryAxisAlignItems   string   `json:"primaryAxisAlignItems,omitempty"`
	CounterAxisAlignItems   string   `json:"counterAxisAlignItems,omitempty"`
	CounterAxisAlignContent string   `json:"counterAxisAlignContent,omitempty"`
	PaddingLeft             float64  `json:"paddingLeft,omitempty"`
	PaddingRight            float64  `json:"paddingRight,omitempty"`
	PaddingTop              float64  `json:"paddingTop,omitempty"`
	PaddingBottom           float64  `json:"paddingBottom,omitempty"`
	ItemSpacing             float64  `json:"itemSpacing,omitempty"`
	CounterAxisSpacing      float64  `json:"counterAxisSpacing,omitempty"`
	LayoutPositioning       string   `json:"layoutPositioning,omitempty"`
	ItemReverseZIndex       bool     `json:"itemReverseZIndex,omitempty"`
	StrokesIncludedInLayout bool     `json:"strokesIncludedInLayout,omitempty"`
	LayoutAlign             string   `json:"layoutAlign,omitempty"`
	LayoutGrow              float64  `json:"layoutGrow,omitempty"`
	LayoutSizingHorizontal  string   `json:"layoutSizingHorizontal,omitempty"`
	LayoutSizingVertical    string   `json:"layoutSizingVertical,omitempty"`
	MinWidth                *float64 `json:"minWidth,omitempty"`
	MaxWidth                *float64 `json:"maxWidth,omitempty"`
	MinHeight               *float64 `json:"minHeight,omitempty"`
	MaxHeight               *float64 `json:"maxHeight,omitempty"`

	// Text
	Characters         string          `json:"characters,omitempty"`
	Style              *TypeStyle      `json:"style,omitempty"`
	CharacterStyleOverrides []int      `json:"characterStyleOverrides,omitempty"`
	StyleOverrideTable map[string]*TypeStyle `json:"styleOverrideTable,omitempty"`
	LineTypes          []string        `json:"lineTypes,omitempty"`
	LineIndentations   []int           `json:"lineIndentations,omitempty"`

	// Component
	ComponentID        string                 `json:"componentId,omitempty"`
	ComponentProperties map[string]*ComponentProperty `json:"componentProperties,omitempty"`
	Overrides          []Override             `json:"overrides,omitempty"`

	// Export settings
	ExportSettings []ExportSetting `json:"exportSettings,omitempty"`

	// Styles
	FillStyleID    string `json:"fillStyleId,omitempty"`
	StrokeStyleID  string `json:"strokeStyleId,omitempty"`
	EffectStyleID  string `json:"effectStyleId,omitempty"`
	GridStyleID    string `json:"gridStyleId,omitempty"`
	TextStyleID    string `json:"textStyleId,omitempty"`

	// Variables
	BoundVariables map[string]*VariableAlias `json:"boundVariables,omitempty"`

	// Children
	Children []*Node `json:"children,omitempty"`

	// Additional properties for specific node types
	ClipsContent        *bool          `json:"clipsContent,omitempty"`
	Background          []Paint        `json:"background,omitempty"`
	BackgroundColor     *Color         `json:"backgroundColor,omitempty"`
	LayoutGrids         []LayoutGrid   `json:"layoutGrids,omitempty"`
	Guides              []Guide        `json:"guides,omitempty"`
	FlowStartingPoints  []FlowStartingPoint `json:"flowStartingPoints,omitempty"`
	Prototypedevice     *PrototypeDevice    `json:"prototypeDevice,omitempty"`

	// Vector
	FillGeometry    []VectorPath `json:"fillGeometry,omitempty"`
	StrokeGeometry  []VectorPath `json:"strokeGeometry,omitempty"`
}

// NodeType represents the type of a Figma node.
type NodeType string

const (
	NodeTypeDocument      NodeType = "DOCUMENT"
	NodeTypeCanvas        NodeType = "CANVAS"
	NodeTypeFrame         NodeType = "FRAME"
	NodeTypeGroup         NodeType = "GROUP"
	NodeTypeSection       NodeType = "SECTION"
	NodeTypeVector        NodeType = "VECTOR"
	NodeTypeBooleanOperation NodeType = "BOOLEAN_OPERATION"
	NodeTypeStar          NodeType = "STAR"
	NodeTypeLine          NodeType = "LINE"
	NodeTypeEllipse       NodeType = "ELLIPSE"
	NodeTypeRegularPolygon NodeType = "REGULAR_POLYGON"
	NodeTypeRectangle     NodeType = "RECTANGLE"
	NodeTypeTable         NodeType = "TABLE"
	NodeTypeTableCell     NodeType = "TABLE_CELL"
	NodeTypeText          NodeType = "TEXT"
	NodeTypeSlice         NodeType = "SLICE"
	NodeTypeComponent     NodeType = "COMPONENT"
	NodeTypeComponentSet  NodeType = "COMPONENT_SET"
	NodeTypeInstance      NodeType = "INSTANCE"
	NodeTypeSticky        NodeType = "STICKY"
	NodeTypeShapeWithText NodeType = "SHAPE_WITH_TEXT"
	NodeTypeConnector     NodeType = "CONNECTOR"
	NodeTypeWashi         NodeType = "WASHI"
	NodeTypeWidget        NodeType = "WIDGET"
	NodeTypeEmbed         NodeType = "EMBED"
	NodeTypeLinkUnfurl    NodeType = "LINK_UNFURL"
	NodeTypeMedia         NodeType = "MEDIA"
)

// Rectangle represents a bounding box.
type Rectangle struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Vector represents a 2D vector.
type Vector struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Constraints represents layout constraints.
type Constraints struct {
	Vertical   string `json:"vertical"`
	Horizontal string `json:"horizontal"`
}

// Color represents an RGBA color.
type Color struct {
	R float64 `json:"r"`
	G float64 `json:"g"`
	B float64 `json:"b"`
	A float64 `json:"a"`
}

// Paint represents a fill or stroke.
type Paint struct {
	Type            string          `json:"type"`
	Visible         *bool           `json:"visible,omitempty"`
	Opacity         *float64        `json:"opacity,omitempty"`
	Color           *Color          `json:"color,omitempty"`
	BlendMode       string          `json:"blendMode,omitempty"`
	GradientHandlePositions []Vector `json:"gradientHandlePositions,omitempty"`
	GradientStops   []ColorStop     `json:"gradientStops,omitempty"`
	ScaleMode       string          `json:"scaleMode,omitempty"`
	ImageTransform  [][]float64     `json:"imageTransform,omitempty"`
	ImageRef        string          `json:"imageRef,omitempty"`
	Filters         *ImageFilters   `json:"filters,omitempty"`
	GifRef          string          `json:"gifRef,omitempty"`
	BoundVariables  map[string]*VariableAlias `json:"boundVariables,omitempty"`
}

// ColorStop represents a gradient color stop.
type ColorStop struct {
	Position       float64         `json:"position"`
	Color          Color           `json:"color"`
	BoundVariables map[string]*VariableAlias `json:"boundVariables,omitempty"`
}

// ImageFilters represents image filters.
type ImageFilters struct {
	Exposure   float64 `json:"exposure,omitempty"`
	Contrast   float64 `json:"contrast,omitempty"`
	Saturation float64 `json:"saturation,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	Tint       float64 `json:"tint,omitempty"`
	Highlights float64 `json:"highlights,omitempty"`
	Shadows    float64 `json:"shadows,omitempty"`
}

// Effect represents a visual effect.
type Effect struct {
	Type            string   `json:"type"`
	Visible         *bool    `json:"visible,omitempty"`
	Radius          float64  `json:"radius,omitempty"`
	Color           *Color   `json:"color,omitempty"`
	BlendMode       string   `json:"blendMode,omitempty"`
	Offset          *Vector  `json:"offset,omitempty"`
	Spread          float64  `json:"spread,omitempty"`
	ShowShadowBehindNode bool `json:"showShadowBehindNode,omitempty"`
	BoundVariables  map[string]*VariableAlias `json:"boundVariables,omitempty"`
}

// TypeStyle represents text style properties.
type TypeStyle struct {
	FontFamily           string             `json:"fontFamily,omitempty"`
	FontPostScriptName   string             `json:"fontPostScriptName,omitempty"`
	FontWeight           float64            `json:"fontWeight,omitempty"`
	FontSize             float64            `json:"fontSize,omitempty"`
	TextAlignHorizontal  string             `json:"textAlignHorizontal,omitempty"`
	TextAlignVertical    string             `json:"textAlignVertical,omitempty"`
	LetterSpacing        float64            `json:"letterSpacing,omitempty"`
	LineHeightPx         float64            `json:"lineHeightPx,omitempty"`
	LineHeightPercent    float64            `json:"lineHeightPercent,omitempty"`
	LineHeightPercentFontSize float64       `json:"lineHeightPercentFontSize,omitempty"`
	LineHeightUnit       string             `json:"lineHeightUnit,omitempty"`
	TextCase             string             `json:"textCase,omitempty"`
	TextDecoration       string             `json:"textDecoration,omitempty"`
	TextAutoResize       string             `json:"textAutoResize,omitempty"`
	Italic               bool               `json:"italic,omitempty"`
	Fills                []Paint            `json:"fills,omitempty"`
	Hyperlink            *Hyperlink         `json:"hyperlink,omitempty"`
	OpenTypeFeatures     map[string]int     `json:"opentypeFeatures,omitempty"`
	BoundVariables       map[string]*VariableAlias `json:"boundVariables,omitempty"`
}

// Hyperlink represents a hyperlink.
type Hyperlink struct {
	Type   string `json:"type"`
	URL    string `json:"url,omitempty"`
	NodeID string `json:"nodeID,omitempty"`
}

// Component represents a Figma component.
type Component struct {
	Key              string                 `json:"key"`
	Name             string                 `json:"name"`
	Description      string                 `json:"description"`
	ComponentSetID   string                 `json:"componentSetId,omitempty"`
	DocumentationLinks []DocumentationLink  `json:"documentationLinks,omitempty"`
	Remote           bool                   `json:"remote,omitempty"`
}

// ComponentSet represents a component set.
type ComponentSet struct {
	Key              string                `json:"key"`
	Name             string                `json:"name"`
	Description      string                `json:"description"`
	DocumentationLinks []DocumentationLink `json:"documentationLinks,omitempty"`
	Remote           bool                  `json:"remote,omitempty"`
}

// DocumentationLink represents a documentation link.
type DocumentationLink struct {
	URI string `json:"uri"`
}

// ComponentProperty represents a component property.
type ComponentProperty struct {
	Type           string          `json:"type"`
	Value          json.RawMessage `json:"value"`
	PreferredValues []json.RawMessage `json:"preferredValues,omitempty"`
	BoundVariables map[string]*VariableAlias `json:"boundVariables,omitempty"`
}

// Override represents a component override.
type Override struct {
	ID             string          `json:"id"`
	OverriddenFields []string      `json:"overriddenFields"`
}

// Style represents a Figma style.
type Style struct {
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Remote      bool      `json:"remote,omitempty"`
	StyleType   StyleType `json:"styleType"`
}

// StyleType represents the type of a style.
type StyleType string

const (
	StyleTypeFill   StyleType = "FILL"
	StyleTypeText   StyleType = "TEXT"
	StyleTypeEffect StyleType = "EFFECT"
	StyleTypeGrid   StyleType = "GRID"
)

// ExportSetting represents an export setting.
type ExportSetting struct {
	Suffix      string     `json:"suffix"`
	Format      string     `json:"format"`
	Constraint  *Constraint `json:"constraint"`
}

// Constraint represents an export constraint.
type Constraint struct {
	Type  string  `json:"type"`
	Value float64 `json:"value"`
}

// LayoutGrid represents a layout grid.
type LayoutGrid struct {
	Pattern     string  `json:"pattern"`
	SectionSize float64 `json:"sectionSize"`
	Visible     bool    `json:"visible"`
	Color       Color   `json:"color"`
	Alignment   string  `json:"alignment"`
	GutterSize  float64 `json:"gutterSize"`
	Offset      float64 `json:"offset"`
	Count       int     `json:"count"`
	BoundVariables map[string]*VariableAlias `json:"boundVariables,omitempty"`
}

// Guide represents a guide.
type Guide struct {
	Axis   string  `json:"axis"`
	Offset float64 `json:"offset"`
}

// FlowStartingPoint represents a prototype flow starting point.
type FlowStartingPoint struct {
	NodeID      string `json:"nodeId"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// PrototypeDevice represents a prototype device.
type PrototypeDevice struct {
	Type     string  `json:"type"`
	Size     *Vector `json:"size,omitempty"`
	PresetIdentifier string `json:"presetIdentifier,omitempty"`
	Rotation string  `json:"rotation,omitempty"`
}

// VectorPath represents a vector path.
type VectorPath struct {
	Path        string `json:"path"`
	WindingRule string `json:"windingRule"`
	OverrideID  int    `json:"overrideID,omitempty"`
}

// VariableAlias represents a variable binding.
type VariableAlias struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// Branch represents a file branch.
type Branch struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	ThumbnailURL string `json:"thumbnail_url"`
	LastModified string `json:"last_modified"`
	LinkAccess   string `json:"link_access"`
}

// ImageExport represents an image export response.
type ImageExport struct {
	Err    string            `json:"err,omitempty"`
	Images map[string]string `json:"images"`
}

// FileStyles represents a file styles response.
type FileStyles struct {
	Status int               `json:"status,omitempty"`
	Error  bool              `json:"error,omitempty"`
	Meta   *StylesMeta       `json:"meta,omitempty"`
}

// StylesMeta contains styles metadata.
type StylesMeta struct {
	Styles []PublishedStyle `json:"styles"`
}

// PublishedStyle represents a published style.
type PublishedStyle struct {
	Key         string    `json:"key"`
	FileKey     string    `json:"file_key"`
	NodeID      string    `json:"node_id"`
	StyleType   StyleType `json:"style_type"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
	User        *User     `json:"user"`
	SortPosition string   `json:"sort_position"`
}

// User represents a Figma user.
type User struct {
	ID       string `json:"id"`
	Handle   string `json:"handle"`
	ImgURL   string `json:"img_url"`
	Email    string `json:"email,omitempty"`
}

// FileComponents represents a file components response.
type FileComponents struct {
	Status int             `json:"status,omitempty"`
	Error  bool            `json:"error,omitempty"`
	Meta   *ComponentsMeta `json:"meta,omitempty"`
}

// ComponentsMeta contains components metadata.
type ComponentsMeta struct {
	Components []PublishedComponent `json:"components"`
}

// PublishedComponent represents a published component.
type PublishedComponent struct {
	Key            string   `json:"key"`
	FileKey        string   `json:"file_key"`
	NodeID         string   `json:"node_id"`
	ThumbnailURL   string   `json:"thumbnail_url"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
	ContainingFrame *FrameInfo `json:"containing_frame,omitempty"`
	User           *User    `json:"user"`
}

// FrameInfo represents containing frame info.
type FrameInfo struct {
	Name            string `json:"name,omitempty"`
	NodeID          string `json:"nodeId,omitempty"`
	PageID          string `json:"pageId,omitempty"`
	PageName        string `json:"pageName,omitempty"`
	BackgroundColor string `json:"backgroundColor,omitempty"`
}

// LocalVariables represents the local variables response.
type LocalVariables struct {
	Status int                   `json:"status,omitempty"`
	Error  bool                  `json:"error,omitempty"`
	Meta   *LocalVariablesMeta   `json:"meta,omitempty"`
}

// LocalVariablesMeta contains local variables metadata.
type LocalVariablesMeta struct {
	Variables           map[string]*Variable           `json:"variables"`
	VariableCollections map[string]*VariableCollection `json:"variableCollections"`
}

// Variable represents a design variable.
type Variable struct {
	ID                  string                       `json:"id"`
	Name                string                       `json:"name"`
	Key                 string                       `json:"key"`
	VariableCollectionID string                      `json:"variableCollectionId"`
	ResolvedType        string                       `json:"resolvedType"`
	Description         string                       `json:"description,omitempty"`
	HiddenFromPublishing bool                        `json:"hiddenFromPublishing,omitempty"`
	Scopes              []string                     `json:"scopes,omitempty"`
	CodeSyntax          map[string]string            `json:"codeSyntax,omitempty"`
	ValuesByMode        map[string]json.RawMessage   `json:"valuesByMode"`
}

// VariableCollection represents a variable collection.
type VariableCollection struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Key                  string   `json:"key"`
	Modes                []Mode   `json:"modes"`
	DefaultModeID        string   `json:"defaultModeId"`
	Remote               bool     `json:"remote,omitempty"`
	HiddenFromPublishing bool     `json:"hiddenFromPublishing,omitempty"`
	VariableIDs          []string `json:"variableIds"`
}

// Mode represents a variable mode.
type Mode struct {
	ModeID string `json:"modeId"`
	Name   string `json:"name"`
}
