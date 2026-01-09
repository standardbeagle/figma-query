"""figma-query: Token-efficient Figma MCP server."""

__version__ = "0.1.0"

from .cli import get_binary_path, main

__all__ = ["get_binary_path", "main", "__version__"]
