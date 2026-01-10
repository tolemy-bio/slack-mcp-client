#!/bin/bash

# Test script that sends a predefined message to test multi-step agent
# Usage: ./test-with-input.sh "your message here"

set -e

MESSAGE="${1:-Explain what multi-step reasoning is, then give me a concrete example of how it works}"

echo "Testing with message: $MESSAGE"
echo ""

# Load environment variables
if [ -f ".env.testing" ]; then
    source .env.testing
fi

# Run the test with the message piped in
echo "$MESSAGE" | ./test-multi-step-stdio.sh
