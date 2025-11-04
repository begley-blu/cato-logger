#!/bin/bash
set -e

# Cato Logger Installation Script
# Version: 3.2
# Supported: Ubuntu Linux

BINARY_NAME="cato-logger"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/cato-logger"
SERVICE_FILE="deployments/systemd/cato-logger.service"
CONFIG_TEMPLATE="configs/config.json"
USER_NAME="cato-logger"
GROUP_NAME="cato-logger"
DEFAULT_BINARY_URL="https://raw.githubusercontent.com/begley-blu/cato-logger/main/bin/cato-logger"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check if running as root
if [ "$EUID" -ne 0 ]; then
  echo -e "${RED}Error: This script must be run as root (use sudo)${NC}"
  exit 1
fi

# Check if running on Ubuntu
if [ ! -f /etc/os-release ] || ! grep -q "Ubuntu" /etc/os-release; then
  echo -e "${YELLOW}Warning: This script is designed for Ubuntu. Other distributions may work but are not officially supported.${NC}"
  read -p "Continue anyway? (y/N): " -r
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    exit 1
  fi
fi

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Cato Logger Installation Script${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

echo -e "${BLUE}Step 1/8: Pre-flight checks${NC}"

# Check for curl or wget (needed for downloading from GitHub)
DOWNLOAD_CMD=""
if command -v curl &> /dev/null; then
  DOWNLOAD_CMD="curl"
  echo "  Found: curl"
elif command -v wget &> /dev/null; then
  DOWNLOAD_CMD="wget"
  echo "  Found: wget"
else
  echo -e "  ${YELLOW}Warning: Neither curl nor wget found. You'll need to provide a local binary path.${NC}"
fi

# Check if service file exists (only if running from cloned repo)
if [ -f "$SERVICE_FILE" ]; then
  echo "  Found: $SERVICE_FILE"
  SERVICE_FILE_FOUND=true
else
  echo "  Note: Service file not in current directory (will download if needed)"
  SERVICE_FILE_FOUND=false
fi

# Check if config template exists (only if running from cloned repo)
if [ -f "$CONFIG_TEMPLATE" ]; then
  echo "  Found: $CONFIG_TEMPLATE"
  CONFIG_TEMPLATE_FOUND=true
else
  echo "  Note: Config template not in current directory (will download if needed)"
  CONFIG_TEMPLATE_FOUND=false
fi

echo -e "${GREEN}Step 1/8: Pre-flight checks complete${NC}"
echo ""

# Prompt for configuration values
echo -e "${BLUE}Step 2/8: Configuration Setup${NC}"
echo "You can enter configuration values now or press Enter to skip and configure manually later."
echo ""

read -p "Enter Cato API Key (or press Enter to skip): " CATO_API_KEY
read -p "Enter Cato Account ID (or press Enter to skip): " CATO_ACCOUNT_ID
read -p "Enter Syslog Server IP (or press Enter to skip, default: localhost): " SYSLOG_SERVER

# Set defaults for empty values
if [ -z "$SYSLOG_SERVER" ]; then
  SYSLOG_SERVER="localhost"
fi

echo ""
echo -e "${GREEN}Step 2/8: Configuration values collected${NC}"
echo ""

# Prompt for binary source
echo -e "${BLUE}Step 3/8: Binary Source${NC}"
echo "Specify where to get the cato-logger binary."
echo ""
echo -e "Default: ${GREEN}${DEFAULT_BINARY_URL}${NC}"
echo "Or provide a local file path (e.g., ./bin/cato-logger or /path/to/cato-logger)"
echo ""
read -p "Binary source (press Enter for default): " BINARY_SOURCE

# Use default if empty
if [ -z "$BINARY_SOURCE" ]; then
  BINARY_SOURCE="$DEFAULT_BINARY_URL"
  echo "  Using default: $DEFAULT_BINARY_URL"
else
  echo "  Using custom source: $BINARY_SOURCE"
fi

echo -e "${GREEN}Step 3/8: Binary source selected${NC}"
echo ""

# Create user and group if they don't exist
echo -e "${BLUE}Step 4/8: Creating service user and group${NC}"
if ! getent group "$GROUP_NAME" > /dev/null 2>&1; then
  groupadd --system "$GROUP_NAME"
  echo "  Created group: $GROUP_NAME"
else
  echo "  Group already exists: $GROUP_NAME"
fi

if ! getent passwd "$USER_NAME" > /dev/null 2>&1; then
  useradd --system --no-create-home --shell /usr/sbin/nologin -g "$GROUP_NAME" "$USER_NAME"
  echo "  Created user: $USER_NAME"
else
  echo "  User already exists: $USER_NAME"
fi
echo -e "${GREEN}Step 4/8: User and group ready${NC}"
echo ""

# Create config directory
echo -e "${BLUE}Step 5/8: Setting up directories${NC}"
mkdir -p "$CONFIG_DIR"
chown "$USER_NAME:$GROUP_NAME" "$CONFIG_DIR"
chmod 750 "$CONFIG_DIR"
echo "  Created: $CONFIG_DIR"
echo -e "${GREEN}Step 5/8: Directories ready${NC}"
echo ""

# Copy and configure config file
echo -e "${BLUE}Step 6/8: Installing configuration${NC}"

# Download config template if not found locally
if [ "$CONFIG_TEMPLATE_FOUND" = false ]; then
  echo "  Downloading config template from GitHub..."
  TEMP_CONFIG="/tmp/cato-logger-config.json"
  if [ "$DOWNLOAD_CMD" = "curl" ]; then
    curl -fsSL "https://raw.githubusercontent.com/begley-blu/cato-logger/main/configs/config.json" -o "$TEMP_CONFIG"
  elif [ "$DOWNLOAD_CMD" = "wget" ]; then
    wget -q "https://raw.githubusercontent.com/begley-blu/cato-logger/main/configs/config.json" -O "$TEMP_CONFIG"
  else
    echo -e "${RED}Error: Cannot download config template (no curl or wget available)${NC}"
    exit 1
  fi
  CONFIG_TEMPLATE="$TEMP_CONFIG"
  echo "  Downloaded config template"
fi

if [ -f "$CONFIG_DIR/config.json" ]; then
  echo -e "  ${YELLOW}Config file already exists at $CONFIG_DIR/config.json${NC}"
  read -p "  Overwrite existing config? (y/N): " -r
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "  Keeping existing config file"
  else
    cp "$CONFIG_TEMPLATE" "$CONFIG_DIR/config.json"
    echo "  Copied new config template"
  fi
else
  cp "$CONFIG_TEMPLATE" "$CONFIG_DIR/config.json"
  echo "  Copied config template to $CONFIG_DIR/config.json"
fi

# Update config with provided values
if [ -n "$CATO_API_KEY" ] || [ -n "$CATO_ACCOUNT_ID" ] || [ "$SYSLOG_SERVER" != "localhost" ]; then
  echo "  Updating config with provided values..."

  if [ -n "$CATO_API_KEY" ]; then
    sed -i "s|\"api_key\": \"\"|\"api_key\": \"$CATO_API_KEY\"|g" "$CONFIG_DIR/config.json"
    echo "    - Set API key"
  fi

  if [ -n "$CATO_ACCOUNT_ID" ]; then
    sed -i "s|\"account_id\": \"\"|\"account_id\": \"$CATO_ACCOUNT_ID\"|g" "$CONFIG_DIR/config.json"
    echo "    - Set Account ID"
  fi

  if [ "$SYSLOG_SERVER" != "localhost" ]; then
    sed -i "s|\"server\": \"localhost\"|\"server\": \"$SYSLOG_SERVER\"|g" "$CONFIG_DIR/config.json"
    echo "    - Set Syslog server to $SYSLOG_SERVER"
  fi
fi

chown "$USER_NAME:$GROUP_NAME" "$CONFIG_DIR/config.json"
chmod 640 "$CONFIG_DIR/config.json"
echo -e "${GREEN}Step 6/8: Configuration installed${NC}"
echo ""

# Install binary
echo -e "${BLUE}Step 7/8: Installing binary${NC}"

# Check if source and destination are the same
BINARY_DEST="$INSTALL_DIR/$BINARY_NAME"
if [ -f "$BINARY_SOURCE" ] && [ "$(realpath "$BINARY_SOURCE" 2>/dev/null)" = "$(realpath "$BINARY_DEST" 2>/dev/null)" ]; then
  echo -e "  ${YELLOW}Note: Binary source and destination are the same, skipping copy${NC}"
  echo "  Binary already at: $BINARY_DEST"
else
  # Determine if source is a URL or local file
  if [[ "$BINARY_SOURCE" =~ ^https?:// ]]; then
    # Download from URL
    echo "  Downloading binary from: $BINARY_SOURCE"
    TEMP_BINARY="/tmp/cato-logger-binary"

    if [ -z "$DOWNLOAD_CMD" ]; then
      echo -e "${RED}Error: Cannot download binary (no curl or wget available)${NC}"
      echo ""
      echo "Manual installation instructions:"
      echo "  1. Download the binary from: $BINARY_SOURCE"
      echo "  2. Copy it to: $BINARY_DEST"
      echo "  3. Make it executable: sudo chmod 755 $BINARY_DEST"
      echo "  4. Set ownership: sudo chown root:root $BINARY_DEST"
      exit 1
    fi

    if [ "$DOWNLOAD_CMD" = "curl" ]; then
      if ! curl -fsSL "$BINARY_SOURCE" -o "$TEMP_BINARY"; then
        echo -e "${RED}Error: Failed to download binary from $BINARY_SOURCE${NC}"
        echo ""
        echo "Manual installation instructions:"
        echo "  1. Download the binary from: $BINARY_SOURCE"
        echo "  2. Copy it to: $BINARY_DEST"
        echo "  3. Make it executable: sudo chmod 755 $BINARY_DEST"
        echo "  4. Set ownership: sudo chown root:root $BINARY_DEST"
        exit 1
      fi
    elif [ "$DOWNLOAD_CMD" = "wget" ]; then
      if ! wget -q "$BINARY_SOURCE" -O "$TEMP_BINARY"; then
        echo -e "${RED}Error: Failed to download binary from $BINARY_SOURCE${NC}"
        echo ""
        echo "Manual installation instructions:"
        echo "  1. Download the binary from: $BINARY_SOURCE"
        echo "  2. Copy it to: $BINARY_DEST"
        echo "  3. Make it executable: sudo chmod 755 $BINARY_DEST"
        echo "  4. Set ownership: sudo chown root:root $BINARY_DEST"
        exit 1
      fi
    fi

    mv "$TEMP_BINARY" "$BINARY_DEST"
    echo "  Downloaded and installed binary"
  else
    # Copy from local file
    if [ ! -f "$BINARY_SOURCE" ]; then
      echo -e "${RED}Error: Binary not found at: $BINARY_SOURCE${NC}"
      echo ""
      echo "Please ensure the binary exists at the specified path, or:"
      echo "  1. Compile it with: make build"
      echo "  2. Download from: $DEFAULT_BINARY_URL"
      exit 1
    fi

    echo "  Copying binary from: $BINARY_SOURCE"
    cp "$BINARY_SOURCE" "$BINARY_DEST"
    echo "  Copied binary to $BINARY_DEST"
  fi
fi

# Set permissions and ownership
chmod 755 "$BINARY_DEST"
chown root:root "$BINARY_DEST"
echo "  Set permissions: 755"
echo "  Set ownership: root:root"
echo "  Installed: $BINARY_DEST"
echo -e "${GREEN}Step 7/8: Binary installed${NC}"
echo ""

# Install systemd service
echo -e "${BLUE}Step 8/8: Installing systemd service${NC}"

# Download service file if not found locally
if [ "$SERVICE_FILE_FOUND" = false ]; then
  echo "  Downloading service file from GitHub..."
  TEMP_SERVICE="/tmp/cato-logger.service"
  if [ "$DOWNLOAD_CMD" = "curl" ]; then
    curl -fsSL "https://raw.githubusercontent.com/begley-blu/cato-logger/main/deployments/systemd/cato-logger.service" -o "$TEMP_SERVICE"
  elif [ "$DOWNLOAD_CMD" = "wget" ]; then
    wget -q "https://raw.githubusercontent.com/begley-blu/cato-logger/main/deployments/systemd/cato-logger.service" -O "$TEMP_SERVICE"
  else
    echo -e "${RED}Error: Cannot download service file (no curl or wget available)${NC}"
    exit 1
  fi
  SERVICE_FILE="$TEMP_SERVICE"
  echo "  Downloaded service file"
fi

cp "$SERVICE_FILE" "/etc/systemd/system/$BINARY_NAME.service"
systemctl daemon-reload
systemctl enable "$BINARY_NAME"
echo "  Installed: /etc/systemd/system/$BINARY_NAME.service"
echo "  Service enabled (will start on boot)"
echo -e "${GREEN}Step 8/8: Service installed${NC}"
echo ""

# Installation complete
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Installation Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# Check if config needs manual setup
NEEDS_CONFIG=false
if [ -z "$CATO_API_KEY" ] || [ -z "$CATO_ACCOUNT_ID" ]; then
  NEEDS_CONFIG=true
fi

if [ "$NEEDS_CONFIG" = true ]; then
  echo -e "${YELLOW}Configuration Required:${NC}"
  echo "Your config file needs to be updated before starting the service."
  echo ""
  echo "Edit the config file:"
  echo "  sudo nano $CONFIG_DIR/config.json"
  echo ""
  echo "Required fields to set:"
  [ -z "$CATO_API_KEY" ] && echo "  - cato.api_key"
  [ -z "$CATO_ACCOUNT_ID" ] && echo "  - cato.account_id"
  echo ""
fi

echo -e "${BLUE}Next Steps:${NC}"
echo ""
if [ "$NEEDS_CONFIG" = true ]; then
  echo "1. Edit the configuration file (see above)"
  echo "2. Start the service:"
else
  echo "1. Start the service:"
fi
echo "     sudo systemctl start $BINARY_NAME"
echo ""
echo "3. Check service status:"
echo "     sudo systemctl status $BINARY_NAME"
echo ""
echo "4. View logs:"
echo "     sudo journalctl -u $BINARY_NAME -f"
echo ""
echo -e "${BLUE}Additional Commands:${NC}"
echo "  Stop:    sudo systemctl stop $BINARY_NAME"
echo "  Restart: sudo systemctl restart $BINARY_NAME"
echo "  Disable: sudo systemctl disable $BINARY_NAME"
echo ""
echo -e "${GREEN}Installation completed successfully!${NC}"
