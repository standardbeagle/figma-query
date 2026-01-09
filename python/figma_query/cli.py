"""CLI entry point for figma-query."""

import os
import platform
import subprocess
import sys
from pathlib import Path


def get_platform_binary_name() -> str:
    """Get the binary name for the current platform."""
    system = platform.system().lower()
    machine = platform.machine().lower()

    # Map platform names
    if system == "darwin":
        os_name = "darwin"
    elif system == "linux":
        os_name = "linux"
    elif system == "windows":
        os_name = "windows"
    else:
        raise RuntimeError(f"Unsupported operating system: {system}")

    # Map architecture names
    if machine in ("x86_64", "amd64"):
        arch = "amd64"
    elif machine in ("arm64", "aarch64"):
        arch = "arm64"
    else:
        raise RuntimeError(f"Unsupported architecture: {machine}")

    binary_name = f"figma-query-{os_name}-{arch}"
    if system == "windows":
        binary_name += ".exe"

    return binary_name


def get_binary_path() -> Path:
    """Get the path to the figma-query binary."""
    binary_name = get_platform_binary_name()

    # Check in package bin directory
    package_dir = Path(__file__).parent
    bin_dir = package_dir / "bin"
    binary_path = bin_dir / binary_name

    if binary_path.exists():
        return binary_path

    # Check in installed data directory
    # When installed via pip, binaries may be in a different location
    for search_path in [
        package_dir / "bin" / binary_name,
        package_dir.parent / "bin" / binary_name,
        Path(sys.prefix) / "share" / "figma_query" / "bin" / binary_name,
    ]:
        if search_path.exists():
            return search_path

    raise FileNotFoundError(
        f"figma-query binary not found for {platform.system()}/{platform.machine()}. "
        f"Expected: {binary_name}"
    )


def main() -> int:
    """Run the figma-query binary."""
    try:
        binary_path = get_binary_path()
    except (RuntimeError, FileNotFoundError) as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1

    # Ensure binary is executable on Unix
    if platform.system() != "Windows":
        os.chmod(binary_path, 0o755)

    # Pass through all arguments and environment
    result = subprocess.run(
        [str(binary_path)] + sys.argv[1:],
        env=os.environ,
    )

    return result.returncode


if __name__ == "__main__":
    sys.exit(main())
