#!/bin/bash

# Quick test script for multi-step agent debugging in stdio mode
# Usage: ./test-stdio-quick.sh "your message"

MESSAGE="${1:-Explain what multi-step reasoning is, then give me a concrete example of how it works step by step}"

echo "=========================================="
echo "Testing Multi-Step Agent (STDIO Mode)"
echo "=========================================="
echo ""
echo "Message: $MESSAGE"
echo ""
echo "Watch for these debugging markers:"
echo "  - === AGENT PATH STARTED ==="
echo "  - === BRIDGE CallLLMAgent START ==="
echo "  - [AGENT-CALLBACK] === CHAIN START === iteration=N"
echo "  - === AGENT EXECUTOR COMPLETED ==="
echo ""
echo "=========================================="
echo ""

# Load environment
if [ -f ".env.testing" ]; then
    source .env.testing
fi

# Run test and capture output
echo "$MESSAGE" | docker run -i --rm \
    -e OPENAI_API_KEY="${OPENAI_API_KEY}" \
    -e LOG_LEVEL="debug" \
    -v "$(pwd)/config-test-stdio.json:/app/config.json:ro" \
    slack-mcp-client:test-debug \
    --config /app/config.json \
    --debug 2>&1 | tee /tmp/agent-test.log

echo ""
echo "=========================================="
echo "Test Complete!"
echo ""
echo "Full logs saved to: /tmp/agent-test.log"
echo ""
echo "To analyze the logs:"
echo "  grep '===' /tmp/agent-test.log | head -20"
echo "  grep 'iteration=' /tmp/agent-test.log"
echo "  grep 'AGENT-CALLBACK' /tmp/agent-test.log | head -30"
echo "=========================================="
