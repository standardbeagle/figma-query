"""Allow running as python -m figma_query."""

import sys

from .cli import main

if __name__ == "__main__":
    sys.exit(main())
