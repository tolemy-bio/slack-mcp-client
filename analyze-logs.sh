#!/bin/bash

# Log analysis script for multi-step agent debugging
# This helps filter and analyze the debugging output

if [ $# -eq 0 ]; then
    echo "Usage: $0 <log-file> [filter]"
    echo ""
    echo "Filters:"
    echo "  flow      - Show only high-level flow markers (=== markers)"
    echo "  iterations - Show only iteration callbacks"
    echo "  tools     - Show only tool calls"
    echo "  errors    - Show only errors and failures"
    echo "  all       - Show everything (default)"
    echo ""
    echo "Example:"
    echo "  docker logs <container> 2>&1 | $0 - flow"
    echo "  $0 logs.txt errors"
    exit 1
fi

LOG_FILE="$1"
FILTER="${2:-all}"

if [ "$LOG_FILE" = "-" ]; then
    # Read from stdin
    INPUT=""
else
    if [ ! -f "$LOG_FILE" ]; then
        echo "Error: Log file not found: $LOG_FILE"
        exit 1
    fi
    INPUT="$LOG_FILE"
fi

case "$FILTER" in
    flow)
        echo "=== HIGH-LEVEL FLOW MARKERS ==="
        echo ""
        if [ -z "$INPUT" ]; then
            grep -E "===" | grep -E "AGENT|BRIDGE|REGISTRY|EXECUTOR"
        else
            grep -E "===" "$INPUT" | grep -E "AGENT|BRIDGE|REGISTRY|EXECUTOR"
        fi
        ;;
    iterations)
        echo "=== ITERATION CALLBACKS ==="
        echo ""
        if [ -z "$INPUT" ]; then
            grep "\[AGENT-CALLBACK\]"
        else
            grep "\[AGENT-CALLBACK\]" "$INPUT"
        fi
        ;;
    tools)
        echo "=== TOOL CALLS ==="
        echo ""
        if [ -z "$INPUT" ]; then
            grep "\[TOOL-CALL\]"
        else
            grep "\[TOOL-CALL\]" "$INPUT"
        fi
        ;;
    errors)
        echo "=== ERRORS AND FAILURES ==="
        echo ""
        if [ -z "$INPUT" ]; then
            grep -iE "error|failed|fail" | grep -E "==|\["
        else
            grep -iE "error|failed|fail" "$INPUT" | grep -E "==|\["
        fi
        ;;
    all)
        if [ -z "$INPUT" ]; then
            cat
        else
            cat "$INPUT"
        fi
        ;;
    *)
        echo "Unknown filter: $FILTER"
        exit 1
        ;;
esac
