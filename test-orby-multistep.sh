#!/bin/bash

# Test script for multi-step agent debugging using Orby MCP server
# This tests multi-step agent work by using Orby tools that require multiple iterations

set -e

echo "=========================================="
echo "Multi-Step Agent Test with Orby MCP"
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
    echo "✅ Loaded $ENV_FILE"
    echo ""
else
    echo "ℹ️  No $ENV_FILE file found"
    echo "   Tip: Run ./extract-terraform-creds.sh to create it"
    echo ""
fi

# Check if required environment variables are set
if [ -z "$OPENAI_API_KEY" ]; then
    echo "❌ ERROR: OPENAI_API_KEY not set!"
    exit 1
fi

# Set defaults
export LOG_LEVEL=${LOG_LEVEL:-debug}
export LITELLM_BASE_URL=${LITELLM_BASE_URL:-}
export LITELLM_MODEL=${LITELLM_MODEL:-}

echo "✅ Environment configured"
echo "   OPENAI_API_KEY: ${OPENAI_API_KEY:0:20}..."
if [ -n "$LITELLM_BASE_URL" ]; then
    echo "   LITELLM_BASE_URL: $LITELLM_BASE_URL"
fi
if [ -n "$LITELLM_MODEL" ]; then
    echo "   LITELLM_MODEL: $LITELLM_MODEL"
fi
echo ""

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
    echo "⚠️  Building Docker image..."
    docker build \
        --build-arg BUILDPLATFORM=linux/amd64 \
        --build-arg TARGETPLATFORM=linux/amd64 \
        --build-arg TARGETOS=linux \
        --build-arg TARGETARCH=amd64 \
        -t slack-mcp-client:test-debug . > /dev/null 2>&1
    echo "✅ Docker image built"
    echo ""
fi

# Test message that requires multiple steps with Orby
MESSAGE="${1:-First, list all the main product areas in Orby, then query bugs in the first area you find, and finally summarize what bugs exist there}"

echo "=========================================="
echo "Testing Multi-Step Agent with Orby"
echo "=========================================="
echo ""
echo "Test message: $MESSAGE"
echo ""
echo "Expected flow:"
echo "  1. Iteration 1: list_main_areas tool call"
echo "  2. Iteration 2: query_bugs tool call (using result from step 1)"
echo "  3. Iteration 3: Generate summary (no tool, final response)"
echo ""
echo "Watch for these debugging markers:"
echo "  - === AGENT PATH STARTED ==="
echo "  - [AGENT-CALLBACK] === CHAIN START === iteration=1"
echo "  - [TOOL-CALL] === TOOL INVOCATION START === (list_main_areas)"
echo "  - [AGENT-CALLBACK] === CHAIN START === iteration=2"
echo "  - [TOOL-CALL] === TOOL INVOCATION START === (query_bugs)"
echo "  - [AGENT-CALLBACK] === CHAIN START === iteration=3"
echo "  - === AGENT EXECUTOR COMPLETED ==="
echo ""
echo "=========================================="
echo ""

# Run test with longer timeout to allow multi-step processing
(sleep 120; echo "") | docker run -i --rm \
    -e OPENAI_API_KEY="$OPENAI_API_KEY" \
    -e LOG_LEVEL="$LOG_LEVEL" \
    -e LITELLM_BASE_URL="$LITELLM_BASE_URL" \
    -e LITELLM_MODEL="$LITELLM_MODEL" \
    --network host \
    -v "$(pwd)/$CONFIG_FILE:/app/config.json:ro" \
    slack-mcp-client:test-debug \
    --config /app/config.json \
    --debug 2>&1 | tee /tmp/orby-test.log &
    
DOCKER_PID=$!

# Send the message after a short delay
sleep 2
echo "$MESSAGE" | docker exec -i $(docker ps -q -f ancestor=slack-mcp-client:test-debug) sh -c "cat" 2>/dev/null || echo "$MESSAGE" > /proc/$DOCKER_PID/fd/0 2>/dev/null || echo "Note: Message sent via stdin"

# Wait for the container to process
wait $DOCKER_PID 2>/dev/null || true

echo ""
echo "=========================================="
echo "Test Complete!"
echo ""
echo "Full logs saved to: /tmp/orby-test.log"
echo ""
echo "To analyze the logs:"
echo "  grep 'iteration=' /tmp/orby-test.log"
echo "  grep 'TOOL-CALL' /tmp/orby-test.log"
echo "  grep '=== AGENT' /tmp/orby-test.log | head -20"
echo "=========================================="
