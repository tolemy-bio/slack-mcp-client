#!/bin/bash

# Test script for multi-step agent debugging
# This script helps test the debugging output for multi-step agent work

set -e

echo "=========================================="
echo "Multi-Step Agent Debugging Test"
echo "=========================================="
echo ""

# Load environment variables from .env.testing if it exists
ENV_FILE=".env.testing"
if [ -f "$ENV_FILE" ]; then
    echo "📄 Loading environment variables from $ENV_FILE..."
    # Export variables from .env.testing, but don't override existing env vars
    # Use set -a to automatically export all variables
    set -a
    source "$ENV_FILE" 2>/dev/null || {
        echo "⚠️  Warning: Error loading $ENV_FILE, using shell environment variables"
    }
    set +a
    echo "✅ Loaded $ENV_FILE (shell env vars will override if set)"
    echo ""
else
    echo "ℹ️  No $ENV_FILE file found"
    echo "   Tip: Copy .env.testing.example to .env.testing and fill in your values"
    echo "   Using environment variables from shell instead"
    echo ""
fi

# Check if required environment variables are set
if [ -z "$SLACK_BOT_TOKEN" ] || [ -z "$SLACK_APP_TOKEN" ] || [ -z "$OPENAI_API_KEY" ]; then
    echo "❌ ERROR: Required environment variables not set!"
    echo ""
    echo "Please either:"
    echo "  1. Create a $ENV_FILE file with your values, or"
    echo "  2. Set the following environment variables:"
    echo ""
    echo "  export SLACK_BOT_TOKEN='xoxb-...'"
    echo "  export SLACK_APP_TOKEN='xapp-...'"
    echo "  export OPENAI_API_KEY='sk-...'"
    echo ""
    echo "Optional:"
    echo "  export LOG_LEVEL='debug'  # Default: debug"
    echo "  export MCP_FILESYSTEM_PATH='\${HOME}'  # Default: \$HOME"
    echo ""
    echo "Example $ENV_FILE file:"
    echo "  SLACK_BOT_TOKEN=xoxb-your-token"
    echo "  SLACK_APP_TOKEN=xapp-your-token"
    echo "  OPENAI_API_KEY=sk-your-key"
    echo "  LOG_LEVEL=debug"
    echo ""
    exit 1
fi

# Set default log level if not set
export LOG_LEVEL=${LOG_LEVEL:-debug}

# Set default MCP filesystem path if not set
export MCP_FILESYSTEM_PATH=${MCP_FILESYSTEM_PATH:-${HOME}}

echo "✅ Environment variables configured"
echo "   SLACK_BOT_TOKEN: ${SLACK_BOT_TOKEN:0:20}..." # Show first 20 chars only
echo "   SLACK_APP_TOKEN: ${SLACK_APP_TOKEN:0:20}..."
echo "   OPENAI_API_KEY: ${OPENAI_API_KEY:0:20}..."
echo "   LOG_LEVEL: $LOG_LEVEL"
echo "   MCP_FILESYSTEM_PATH: $MCP_FILESYSTEM_PATH"
echo ""

# Check if config file exists
CONFIG_FILE="config-test.json"
if [ ! -f "$CONFIG_FILE" ]; then
    echo "❌ ERROR: Config file not found: $CONFIG_FILE"
    exit 1
fi

echo "✅ Config file found: $CONFIG_FILE"
echo ""

# Check if Docker image exists
if ! docker image inspect slack-mcp-client:test-debug > /dev/null 2>&1; then
    echo "⚠️  Docker image 'slack-mcp-client:test-debug' not found"
    echo "   Building it now..."
    docker build \
        --build-arg BUILDPLATFORM=linux/amd64 \
        --build-arg TARGETPLATFORM=linux/amd64 \
        --build-arg TARGETOS=linux \
        --build-arg TARGETARCH=amd64 \
        -t slack-mcp-client:test-debug . > /dev/null 2>&1
    echo "✅ Docker image built"
    echo ""
fi

echo "=========================================="
echo "Starting Slack MCP Client with Debug Logging"
echo "=========================================="
echo ""
echo "📋 Test Instructions:"
echo "   1. Send a message in Slack that requires multiple steps"
echo "   2. Watch the logs below for debugging output"
echo "   3. Look for these key markers:"
echo "      - === AGENT PATH STARTED ==="
echo "      - [AGENT-CALLBACK] === CHAIN START === iteration=N"
echo "      - [TOOL-CALL] === TOOL INVOCATION START ==="
echo "      - === AGENT EXECUTOR COMPLETED ==="
echo ""
echo "💡 Example multi-step messages to test:"
echo "   - 'List files in the current directory, then read the README file'"
echo "   - 'Search for information about X, then summarize what you found'"
echo "   - 'Check the git status, then show me the latest commit'"
echo ""
echo "=========================================="
echo ""

# Run the container with debug logging
# Use -i instead of -it for non-interactive environments, or detect TTY
TTY_FLAG=""
if [ -t 0 ]; then
    TTY_FLAG="-it"
else
    TTY_FLAG="-i"
fi

echo "🚀 Starting Docker container..."
echo ""

docker run $TTY_FLAG --rm \
    -e SLACK_BOT_TOKEN="$SLACK_BOT_TOKEN" \
    -e SLACK_APP_TOKEN="$SLACK_APP_TOKEN" \
    -e OPENAI_API_KEY="$OPENAI_API_KEY" \
    -e LOG_LEVEL="$LOG_LEVEL" \
    -e MCP_FILESYSTEM_PATH="$MCP_FILESYSTEM_PATH" \
    -v "$(pwd)/$CONFIG_FILE:/app/config.json:ro" \
    slack-mcp-client:test-debug \
    --config /app/config.json \
    --debug
