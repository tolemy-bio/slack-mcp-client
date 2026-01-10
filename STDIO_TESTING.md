# Testing Multi-Step Agent in STDIO Mode

## Overview

You can now test the multi-step agent debugging **without connecting to Slack** using stdio mode. This allows you to test the agent logic locally and see all the debugging output.

## Quick Start

### Option 1: Interactive Mode

Run the test script and type your message:

```bash
./test-multi-step-stdio.sh
```

Then type your message and press Enter. The agent will process it and show debugging output.

### Option 2: Pipe a Message

Test with a predefined message:

```bash
echo "First explain what multi-step reasoning is, then give me a concrete example" | ./test-multi-step-stdio.sh
```

### Option 3: Quick Test Script

Use the quick test script:

```bash
./test-stdio-quick.sh "Your multi-step message here"
```

## What You'll See

The debugging output will show:

```
=== AGENT PATH STARTED ===
  channel=xxx thread= provider=openai tools_count=0 max_iterations=20

=== BRIDGE CallLLMAgent START ===
  user=... prompt_length=... available_tools_count=0

=== REGISTRY GenerateAgentCompletion START ===
  provider_name=openai tools_count=0 max_iterations=20

=== AGENT START ===
  user=... prompt_length=... tools_count=0 max_iterations=20

=== AGENT EXECUTOR STARTING ===

[AGENT-CALLBACK] === CHAIN START === iteration=1
[AGENT-CALLBACK] LLM Start - prompts_count=1
[AGENT-CALLBACK] LLM GenerateContent End - choices_count=1
[AGENT-CALLBACK] === CHAIN END === iteration=1

[AGENT-CALLBACK] === CHAIN START === iteration=2
...
[AGENT-CALLBACK] === CHAIN END === iteration=2

=== AGENT EXECUTOR COMPLETED ===
Agent output received - output_length=... is_empty=false
=== AGENT END ===
```

## Analyzing Output

### Count Iterations

```bash
grep "iteration=" /tmp/stdio-test.log
```

### See All Flow Markers

```bash
grep "===" /tmp/stdio-test.log | grep -E "AGENT|BRIDGE|REGISTRY|EXECUTOR"
```

### See All Callbacks

```bash
grep "\[AGENT-CALLBACK\]" /tmp/stdio-test.log
```

### Find Errors

```bash
grep -i "error\|failed" /tmp/stdio-test.log
```

## Example Test Messages

**Simple 2-step:**
```
Explain what multi-step reasoning is, then give me a concrete example
```

**Complex 3+ step:**
```
First explain what multi-step reasoning is, then give me a concrete example, and finally summarize the key benefits
```

**With explicit steps:**
```
Think about planning a trip: first list the steps, then explain each one, and finally prioritize them
```

## Configuration

The stdio mode uses `config-test-stdio.json` which:
- Sets `useStdIOClient: true` (no Slack connection needed)
- Enables agent mode (`useAgent: true`)
- Sets debug logging
- No MCP servers (pure LLM testing)

## Benefits of STDIO Mode

✅ **No Slack connection required** - Test locally without network  
✅ **Faster iteration** - No need to wait for Slack messages  
✅ **Full debugging output** - See all the markers we added  
✅ **Easy to automate** - Pipe messages in for testing  
✅ **Same agent logic** - Uses the exact same code path as Slack mode  

## Troubleshooting

### No output appearing
- Make sure `LOG_LEVEL=debug` is set
- Check that the message is being piped correctly
- Look for errors in the startup logs

### Agent not doing multiple steps
- Check the prompt - it needs to explicitly ask for multiple things
- Look for `iteration=` in the logs to see how many iterations ran
- Check for `=== AGENT EXECUTOR FAILED ===` markers

### Want to see full logs
The quick test script saves logs to `/tmp/stdio-test.log` - check that file for complete output.
