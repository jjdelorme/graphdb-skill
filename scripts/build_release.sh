#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check for zig
if ! command -v zig &> /dev/null; then
    echo -e "${RED}Error: zig is not installed. Please install zig to continue.${NC}"
    echo "Visit https://ziglang.org/download/ for installation instructions."
    exit 1
fi

echo -e "${GREEN}Found zig: $(zig version)${NC}"

# Create dist directory
mkdir -p dist

# Ensure we are in the project root
cd "$(dirname "$0")/.."

# Define targets
# Format: "GOOS/GOARCH/ZIG_TARGET"
TARGETS=(
    "windows/amd64/x86_64-windows-gnu"
)

for target in "${TARGETS[@]}"; do
    IFS="/" read -r goos goarch zig_target <<< "$target"
    
    echo -e "Building for ${GREEN}${goos}/${goarch}${NC} using ${zig_target}..."
    
    output_name="dist/graphdb-${goos}-${goarch}"
    if [ "$goos" == "windows" ]; then
        output_name="dist/graphdb-win.exe"
    fi

    # Run go build
    # We use zig cc as the C compiler for CGO
    env CGO_ENABLED=1 \
        GOOS="$goos" \
        GOARCH="$goarch" \
        CC="zig cc -target $zig_target" \
        CXX="zig c++ -target $zig_target" \
        go build -o "$output_name" ./cmd/graphdb

    echo -e "${GREEN}✓ Built ${output_name}${NC}"
done

echo -e "${GREEN}All builds completed successfully! Artifacts in dist/${NC}"
