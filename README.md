# figma-query

Token-efficient Figma MCP server with image export, query DSL, and design token support.

## Installation

### npm

```bash
npm install -g @standardbeagle/figma-query
```

Or use with npx:

```bash
npx @standardbeagle/figma-query
```

### Python (uvx/pipx)

```bash
# Using uvx (recommended)
uvx figma-query

# Using pipx
pipx install figma-query

# Using pip
pip install figma-query
```

## Configuration

### Environment Variables

- `FIGMA_ACCESS_TOKEN` - Your Figma personal access token (required)
- `FIGMA_EXPORT_DIR` - Directory for file exports (default: `./figma-export`)

### Claude Desktop / MCP Client

Add to your MCP configuration:

```json
{
  "mcpServers": {
    "figma-query": {
      "command": "npx",
      "args": ["@standardbeagle/figma-query"],
      "env": {
        "FIGMA_ACCESS_TOKEN": "your-token-here"
      }
    }
  }
}
```

Or if installed globally:

```json
{
  "mcpServers": {
    "figma-query": {
      "command": "figma-query",
      "env": {
        "FIGMA_ACCESS_TOKEN": "your-token-here"
      }
    }
  }
}
```

Using uvx (Python):

```json
{
  "mcpServers": {
    "figma-query": {
      "command": "uvx",
      "args": ["figma-query"],
      "env": {
        "FIGMA_ACCESS_TOKEN": "your-token-here"
      }
    }
  }
}
```

## Tools

### Export Tools

| Tool | Description |
|------|-------------|
| `sync_file` | Export entire file to nested folders (includes assets by default) |
| `export_assets` | Export images/icons for specific nodes |
| `export_tokens` | Export design tokens to CSS/JSON/Tailwind |
| `download_image` | Download images by ref ID or render nodes as images |

### Query Tools

| Tool | Description |
|------|-------------|
| `query` | Query nodes with JSON DSL and data shaping |
| `search` | Full-text search across names, text, properties |
| `get_tree` | Get file structure as ASCII tree with node IDs |
| `list_components` | List all components with usage stats |
| `list_styles` | List all styles (color, text, effect, grid) |

### Detail Tools

| Tool | Description |
|------|-------------|
| `get_node` | Get full details for a specific node |
| `get_css` | Extract CSS properties for node(s) |
| `get_tokens` | Get design token references and resolved values |

### Other Tools

| Tool | Description |
|------|-------------|
| `wireframe` | Generate annotated wireframe with node IDs |
| `diff` | Compare exports or file versions |
| `info` | Help and status |

## Projections

Use projections in the `select` array to get specific property groups:

| Projection | Properties |
|------------|------------|
| `@structure` | id, name, type, visible |
| `@bounds` | x, y, width, height |
| `@css` | fills, strokes, effects, cornerRadius, opacity |
| `@layout` | layoutMode, padding, itemSpacing, constraints |
| `@typography` | fontFamily, fontSize, fontWeight, lineHeight |
| `@tokens` | boundVariables |
| `@images` | imageRefs (from fills/strokes/backgrounds), exportSettings |
| `@all` | All properties |

## Examples

### Query all buttons

```json
{
  "file_key": "abc123",
  "q": {
    "from": ["COMPONENT"],
    "where": { "name": { "$match": "Button*" } },
    "select": ["@structure", "@css"]
  }
}
```

### Get images from a node

```json
{
  "file_key": "abc123",
  "q": {
    "from": ["FRAME"],
    "select": ["@structure", "@images"]
  }
}
```

### Download image fills

```json
{
  "file_key": "abc123",
  "image_refs": ["ref123", "ref456"],
  "output_dir": "./images"
}
```

## License

MIT
