#!/bin/bash

# Test script for multi-step agent debugging using stdio mode (no Slack connection)
# This allows testing the agent logic without needing Slack connectivity

set -e

echo "=========================================="
echo "Multi-Step Agent Debugging Test (STDIO Mode)"
echo "=========================================="
echo ""

# Load environment variables from .env.testing if it exists
ENV_FILE=".env.testing"
if [ -f "$ENV_FILE" ]; then
    echo "📄 Loading environment variables from $ENV_FILE..."
    set -a
    source "$ENV_FILE" 2>/dev/null || {
        echo "⚠️  Warning: Error loading $ENV_FILE, using shell environment variables"
    }
    set +a
    echo "✅ Loaded $ENV_FILE (shell env vars will override if set)"
    echo ""
else
    echo "ℹ️  No $ENV_FILE file found"
    echo "   Tip: Run ./extract-terraform-creds.sh to create it"
    echo "   Using environment variables from shell instead"
    echo ""
fi

# Check if required environment variables are set (only API key needed for stdio mode)
if [ -z "$OPENAI_API_KEY" ]; then
    echo "❌ ERROR: OPENAI_API_KEY not set!"
    echo ""
    echo "Please either:"
    echo "  1. Create a $ENV_FILE file with your values, or"
    echo "  2. Set the environment variable:"
    echo ""
    echo "  export OPENAI_API_KEY='sk-...'"
    echo ""
    echo "Optional:"
    echo "  export LOG_LEVEL='debug'  # Default: debug"
    echo ""
    exit 1
fi

# Set default log level if not set
export LOG_LEVEL=${LOG_LEVEL:-debug}

# Set LiteLLM variables if available
export LITELLM_BASE_URL=${LITELLM_BASE_URL:-}
export LITELLM_MODEL=${LITELLM_MODEL:-}

# Check if config file exists
CONFIG_FILE="config-test-stdio.json"
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
echo "Starting Slack MCP Client in STDIO Mode"
echo "=========================================="
echo ""
echo "📋 Test Instructions:"
echo "   1. Type a multi-step message below and press Enter"
echo "   2. Watch the logs for debugging output"
echo "   3. Look for these key markers:"
echo "      - === AGENT PATH STARTED ==="
echo "      - [AGENT-CALLBACK] === CHAIN START === iteration=N"
echo "      - === AGENT EXECUTOR COMPLETED ==="
echo ""
echo "💡 Example multi-step messages to test:"
echo "   - 'Explain what multi-step reasoning is, then give me an example'"
echo "   - 'First, list 3 steps to solve a problem, then prioritize them'"
echo "   - 'Think about planning a trip: first list the steps, then explain each one'"
echo ""
echo "Press Ctrl+C to exit"
echo ""
echo "=========================================="
echo ""

# Set LiteLLM variables if available (for proxy support)
LITELLM_BASE_URL=${LITELLM_BASE_URL:-}
LITELLM_MODEL=${LITELLM_MODEL:-}

echo "✅ Environment variables configured"
echo "   OPENAI_API_KEY: ${OPENAI_API_KEY:0:20}..."
if [ -n "$LITELLM_BASE_URL" ]; then
    echo "   LITELLM_BASE_URL: $LITELLM_BASE_URL"
fi
if [ -n "$LITELLM_MODEL" ]; then
    echo "   LITELLM_MODEL: $LITELLM_MODEL"
fi
echo "   LOG_LEVEL: $LOG_LEVEL"
echo ""

# Run the container with stdio mode
# Use -i for interactive input, no -t since we're piping
docker run -i --rm \
    -e OPENAI_API_KEY="$OPENAI_API_KEY" \
    -e LOG_LEVEL="$LOG_LEVEL" \
    -e LITELLM_BASE_URL="$LITELLM_BASE_URL" \
    -e LITELLM_MODEL="$LITELLM_MODEL" \
    -v "$(pwd)/$CONFIG_FILE:/app/config.json:ro" \
    slack-mcp-client:test-debug \
    --config /app/config.json \
    --debug
