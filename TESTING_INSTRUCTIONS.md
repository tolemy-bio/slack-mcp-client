# Testing Multi-Step Agent Debugging

## Quick Start

### 1. Set Environment Variables

```bash
export SLACK_BOT_TOKEN="xoxb-your-bot-token"
export SLACK_APP_TOKEN="xapp-your-app-token"
export OPENAI_API_KEY="sk-your-openai-key"
export LOG_LEVEL="debug"  # Optional, defaults to info
```

### 2. Run the Test Script

```bash
cd /Users/caelana/dev/tolemy/tolemy-core/slack-mcp-client
./test-multi-step.sh
```

This will:
- Check your environment variables
- Build the Docker image if needed
- Start the application with debug logging
- Show you what to look for in the logs

### 3. Send a Multi-Step Message in Slack

Try one of these examples:

**Simple 2-step:**
```
List the files in the current directory, then read the README file if it exists
```

**Complex 3+ step:**
```
Check what files are in the current directory, read the README, and then create a summary of the project structure
```

**With reasoning:**
```
Search for information about Kubernetes deployments, analyze what you find, and then provide recommendations
```

## Analyzing Logs

### Option 1: Watch Logs in Real-Time

If running with Docker, the logs will stream to your terminal. Look for:

```
=== AGENT PATH STARTED ===
=== BRIDGE CallLLMAgent START ===
[AGENT-CALLBACK] === CHAIN START === iteration=1
[TOOL-CALL] === TOOL INVOCATION START ===
[AGENT-CALLBACK] === CHAIN START === iteration=2
=== AGENT EXECUTOR COMPLETED ===
```

### Option 2: Save and Analyze Logs

```bash
# Save logs to file
docker logs <container-name> > agent-logs.txt 2>&1

# Analyze with the helper script
./analyze-logs.sh agent-logs.txt flow      # High-level flow
./analyze-logs.sh agent-logs.txt iterations # All iterations
./analyze-logs.sh agent-logs.txt tools     # Tool calls only
./analyze-logs.sh agent-logs.txt errors    # Errors only
```

### Option 3: Filter with grep

```bash
# See only agent flow
docker logs <container> 2>&1 | grep "==="

# See only iterations
docker logs <container> 2>&1 | grep "\[AGENT-CALLBACK\]"

# See only tool calls
docker logs <container> 2>&1 | grep "\[TOOL-CALL\]"

# See errors
docker logs <container> 2>&1 | grep -i "error\|failed"
```

## What to Look For

### ✅ Success Pattern

When multi-step works correctly, you should see:

1. **Agent starts:**
   ```
   === AGENT PATH STARTED ===
   === BRIDGE CallLLMAgent START ===
   === AGENT START ===
   ```

2. **First iteration:**
   ```
   [AGENT-CALLBACK] === CHAIN START === iteration=1
   [AGENT-CALLBACK] Agent Action - tool=list_directory
   [TOOL-CALL] === TOOL INVOCATION START ===
   [TOOL-CALL] SUCCESS: Tool list_directory returned result
   [AGENT-CALLBACK] === CHAIN END === iteration=1
   ```

3. **Second iteration:**
   ```
   [AGENT-CALLBACK] === CHAIN START === iteration=2
   [AGENT-CALLBACK] Agent Action - tool=read_file
   [TOOL-CALL] === TOOL INVOCATION START ===
   [TOOL-CALL] SUCCESS: Tool read_file returned result
   [AGENT-CALLBACK] === CHAIN END === iteration=2
   ```

4. **Completion:**
   ```
   === AGENT EXECUTOR COMPLETED ===
   Agent output received - output_length=... is_empty=false
   === AGENT END ===
   ```

### ❌ Failure Patterns

#### Context Timeout
```
=== AGENT EXECUTOR FAILED ===
  context_err=context deadline exceeded
  deadline_exceeded=true
```
**Fix:** Increase timeout in config or simplify the request.

#### Iteration Limit Reached
```
=== AGENT EXECUTOR FAILED ===
  Possible iteration limit reached
  error_details=...max iterations...
```
**Fix:** Increase `maxAgentIterations` in config or break down the request.

#### Parsing Error
```
=== AGENT EXECUTOR FAILED ===
  Possible parsing error in agent response
  error_details=...parse...format...
```
**Fix:** This indicates the LLM returned malformed output. Check the raw response in logs.

#### Tool Execution Error
```
[TOOL-CALL] ERROR: Tool call failed: ...
[AGENT-CALLBACK] === TOOL ERROR ===
```
**Fix:** Check tool configuration, permissions, or tool availability.

## Common Issues

### Issue: No logs appearing
**Solution:** Make sure `LOG_LEVEL=debug` is set and `--debug` flag is used.

### Issue: Only seeing first iteration
**Solution:** Check if the agent is hitting an error. Look for `=== FAILED ===` markers.

### Issue: Tool calls not happening
**Solution:** 
- Verify MCP servers are configured correctly
- Check tool allowList/blockList settings
- Ensure tools are discovered (check startup logs)

### Issue: Agent stops after one tool
**Solution:** This is the bug we're debugging! The logs will show exactly where it fails:
- Check `[AGENT-CALLBACK]` logs for errors
- Look for context timeouts
- Check if tool results are being passed correctly

## Getting Help

If you see failures, capture:
1. The full log output (or at least the error section)
2. The message you sent in Slack
3. Your config file (with secrets redacted)

The debugging output will show exactly where in the multi-step process the failure occurs!
