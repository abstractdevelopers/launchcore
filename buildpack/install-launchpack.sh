#!/bin/bash
# =============================================================================
# LAUNCHPACK INSTALLATION SCRIPT FOR COOLIFY
# =============================================================================
# Run this on your Kamatera server where Coolify is installed
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/abstractdevelopers/launchcore/layer/buildpack/install-launchpack.sh | bash
#
# Or download and run manually:
#   wget https://raw.githubusercontent.com/abstractdevelopers/launchcore/layer/buildpack/install-launchpack.sh
#   chmod +x install-launchpack.sh
#   ./install-launchpack.sh
# =============================================================================

set -e

BOLD='\033[1m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BOLD}========================================${NC}"
echo -e "${BOLD}  LaunchPack Installer for Coolify${NC}"
echo -e "${BOLD}========================================${NC}"
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    echo -e "${YELLOW}Please run as root or with sudo${NC}"
    exit 1
fi

# Detect Coolify container name
COOLIFY_CONTAINER=$(docker ps --format '{{.Names}}' | grep -E 'coolify|launchcore' | head -1)

if [ -z "$COOLIFY_CONTAINER" ]; then
    echo -e "${RED}Error: Coolify container not found${NC}"
    echo "Docker containers running:"
    docker ps --format '{{.Names}}'
    exit 1
fi

echo -e "${GREEN}Found Coolify container: ${COOLIFY_CONTAINER}${NC}"
echo ""

# Step 1: Download the buildpack binary
echo -e "${BOLD}[1/4] Downloading LaunchPack binary...${NC}"

# Detect architecture
ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then
    ARCH="amd64"
elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
    ARCH="arm64"
fi

BINARY_URL="https://github.com/abstractdevelopers/launchcore/releases/latest/download/buildpack-linux-${ARCH}"

echo "Architecture: $ARCH"
echo "Downloading from: $BINARY_URL"

# Create temp directory
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

# Download binary (using GitHub releases when available)
if curl -fsSL "$BINARY_URL" -o buildpack; then
    chmod +x buildpack
    echo -e "${GREEN}Binary downloaded successfully${NC}"
else
    echo -e "${YELLOW}Binary not found on releases. Building from source...${NC}"
    
    # Try to build from source if Go is available
    if command -v go &> /dev/null; then
        echo "Building from source with Go..."
        cd /tmp
        
        # Clone if not already cloned
        if [ ! -d "/tmp/launchcore" ]; then
            git clone --depth 1 --branch layer https://github.com/abstractdevelopers/launchcore.git /tmp/launchcore
        fi
        
        cd /tmp/launchcore/buildpack
        go build -o buildpack ./cmd/buildpack
        
        cp buildpack "$TEMP_DIR/buildpack"
        chmod +x "$TEMP_DIR/buildpack"
        echo -e "${GREEN}Binary built successfully${NC}"
    else
        echo -e "${RED}Go not installed and binary not available.${NC}"
        echo "Please either:"
        echo "  1. Install Go: curl -fsSL https://go.dev/dl/go1.22.linux-${ARCH}.tar.gz | tar -C /tmp -xzf -"
        echo "  2. Or download binary manually from GitHub releases"
        exit 1
    fi
fi

# Step 2: Copy binary to container
echo ""
echo -e "${BOLD}[2/4] Installing binary in Coolify container...${NC}"

docker cp "$TEMP_DIR/buildpack" "${COOLIFY_CONTAINER}:/usr/local/bin/buildpack"
docker exec "${COOLIFY_CONTAINER}" chmod +x /usr/local/bin/buildpack

# Verify installation
VERSION=$(docker exec "${COOLIFY_CONTAINER}" /usr/local/bin/buildpack --version 2>/dev/null || echo "unknown")
echo -e "${GREEN}Installed version: ${VERSION}${NC}"

# Step 3: Add PHP code changes
echo ""
echo -e "${BOLD}[3/4] PHP Integration Instructions${NC}"
echo ""
echo -e "${YELLOW}Manual steps required:${NC}"
echo ""
echo "Add the following code to your Coolify installation:"
echo ""
echo "1. Edit: app/Enums/BuildPackTypes.php"
echo "   Add: case LAUNCHPACK = 'launchpack';"
echo ""
echo "2. Edit: app/Jobs/ApplicationDeploymentJob.php"
echo "   - Add case in decide_what_to_do() switch"
echo "   - Add deploy_launchpack_buildpack() method"
echo "   - Add build_launchpack_image() method"
echo ""
echo "   See: https://github.com/abstractdevelopers/launchcore/blob/layer/buildpack/launchpack-deployment.php"
echo ""
echo "3. Edit: resources/views/livewire/project/application/general.blade.php"
echo "   Add option: <option value=\"launchpack\">LaunchPack (Ultra-Fast)</option>"
echo ""

# Step 4: Restart Coolify
echo ""
echo -e "${BOLD}[4/4] Restarting Coolify...${NC}"

docker restart "${COOLIFY_CONTAINER}"
echo -e "${GREEN}Coolify restarted!${NC}"

# Cleanup
rm -rf "$TEMP_DIR"

# Summary
echo ""
echo -e "${BOLD}========================================${NC}"
echo -e "${GREEN}  Installation Complete!${NC}"
echo -e "${BOLD}========================================${NC}"
echo ""
echo "Next steps:"
echo "1. Add PHP code changes (see step 3 above)"
echo "2. Clear Laravel cache:"
echo "   docker exec ${COOLIFY_CONTAINER} php artisan optimize:clear"
echo "3. Test by creating a new application with Build Pack: LaunchPack (Ultra-Fast)"
echo ""
echo -e "${YELLOW}Note: After adding PHP code, you must clear cache and restart Coolify.${NC}"
echo ""
