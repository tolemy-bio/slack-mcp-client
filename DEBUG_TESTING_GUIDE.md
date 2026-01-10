# Multi-Step Agent Debugging Test Guide

This guide helps you test and understand the debugging output for multi-step agent work.

## Build Status

✅ **Build successful!** The Docker image `slack-mcp-client:test-debug` has been built with all debugging enhancements.

## Running with Debug Logging

To see all the debugging output, make sure to:

1. **Set log level to DEBUG or INFO:**
   ```bash
   export LOG_LEVEL=debug
   # or in your config.json:
   {
     "monitoring": {
       "loggingLevel": "debug"
     }
   }
   ```

2. **Run the application:**
   ```bash
   # Using Docker
   docker run -it --rm \
     -e SLACK_BOT_TOKEN="xoxb-..." \
     -e SLACK_APP_TOKEN="xapp-..." \
     -e OPENAI_API_KEY="sk-..." \
     -e LOG_LEVEL=debug \
     -v $(pwd)/config.json:/app/config.json:ro \
     slack-mcp-client:test-debug --config /app/config.json --debug
   
   # Or using docker-compose
   docker-compose up
   ```

## What to Look For in Logs

### 1. **High-Level Flow Markers**

When a multi-step agent request starts, you'll see these markers in sequence:

```
=== AGENT PATH STARTED ===
  channel=... thread=... provider=... tools_count=... max_iterations=...

=== BRIDGE CallLLMAgent START ===
  user=... prompt_length=... available_tools_count=...

=== REGISTRY GenerateAgentCompletion START ===
  provider_name=... tools_count=... max_iterations=...

=== AGENT START ===
  user=... prompt_length=... tools_count=... max_iterations=...

=== AGENT EXECUTOR STARTING ===
  input_preview=...
```

### 2. **Iteration Callbacks (Key for Multi-Step Debugging)**

For each step in the agent's reasoning process, you'll see:

```
[AGENT-CALLBACK] === CHAIN START === iteration=1
[AGENT-CALLBACK] LLM Start - prompts_count=1
[AGENT-CALLBACK] LLM GenerateContent End - choices_count=1
[AGENT-CALLBACK] Agent Action - tool=... input=... log=...
[AGENT-CALLBACK] Tool Start - input (len=...): ...
[TOOL-CALL] === TOOL INVOCATION START === tool=... server=...
[TOOL-CALL] Parsed args: ...
[TOOL-CALL] SUCCESS: Tool ... returned result (len=...): ...
[AGENT-CALLBACK] Tool End - output (len=...): ...
[AGENT-CALLBACK] === CHAIN END === iteration=1

[AGENT-CALLBACK] === CHAIN START === iteration=2  # Second iteration!
[AGENT-CALLBACK] LLM Start - prompts_count=1
...
```

### 3. **Success Completion**

When the agent completes successfully:

```
=== AGENT EXECUTOR COMPLETED === call_keys_count=...
Agent output received - output_length=... is_empty=false
=== AGENT END === final_output_length=...
=== BRIDGE CallLLMAgent SUCCESS ===
=== AGENT PATH RESPONSE RECEIVED ===
```

### 4. **Error Patterns**

If something fails, you'll see specific error markers:

#### Context Timeout:
```
=== AGENT EXECUTOR FAILED ===
  context_err=context deadline exceeded
  deadline_exceeded=true
```

#### Iteration Limit:
```
=== AGENT EXECUTOR FAILED ===
  Possible iteration limit reached
  error_details=...max iterations...
```

#### Parsing Error:
```
=== AGENT EXECUTOR FAILED ===
  Possible parsing error in agent response
  error_details=...parse...format...
```

#### Tool Execution Error:
```
[TOOL-CALL] ERROR: Tool call failed: ...
[AGENT-CALLBACK] === TOOL ERROR === error=...
```

#### LLM Error:
```
[AGENT-CALLBACK] === LLM ERROR === error=...
```

#### Chain Error:
```
[AGENT-CALLBACK] === CHAIN ERROR === iteration=... error=...
```

## Testing Multi-Step Scenarios

### Test Case 1: Simple Multi-Step (2 steps)
**Send in Slack:**
```
"Can you search for information about Kubernetes and then summarize what you found?"
```

**Expected Log Pattern:**
1. First iteration: Agent decides to use search tool
2. Tool execution with `[TOOL-CALL]` logs
3. Second iteration: Agent processes tool result and generates summary
4. Success completion

### Test Case 2: Complex Multi-Step (3+ steps)
**Send in Slack:**
```
"Find all files in the project, then read the README, and create a summary of the project structure"
```

**Expected Log Pattern:**
1. Iteration 1: List directory tool
2. Iteration 2: Read file tool  
3. Iteration 3: Generate summary (no tool)
4. Success completion

### Test Case 3: Failure Scenario
**Send in Slack:**
```
"Use a tool that doesn't exist and then try to use another tool"
```

**Expected Log Pattern:**
1. Iteration 1: Tool error
2. Agent may retry or fail
3. Error markers appear

## Key Debugging Questions Answered

The logs will now answer:

1. **How many iterations did the agent run?**
   - Look for `iteration=N` in `[AGENT-CALLBACK] === CHAIN START ===`

2. **What tools were called in each iteration?**
   - Look for `[AGENT-CALLBACK] Agent Action - tool=...`

3. **What was the input/output of each tool?**
   - Look for `[TOOL-CALL]` logs with input/output

4. **Where did it fail?**
   - Look for `=== FAILED ===` or `=== ERROR ===` markers

5. **Was it a timeout, parsing error, or tool error?**
   - Check the error type in the failure logs

6. **What was the final agent output?**
   - Look for `Agent output received` and `final_output_length`

## Filtering Logs

To focus on specific aspects:

```bash
# See only agent flow markers
docker logs <container> 2>&1 | grep "==="

# See only tool calls
docker logs <container> 2>&1 | grep "\[TOOL-CALL\]"

# See only callback events
docker logs <container> 2>&1 | grep "\[AGENT-CALLBACK\]"

# See errors only
docker logs <container> 2>&1 | grep -i "error\|failed"
```

## Next Steps

1. **Run the application** with debug logging enabled
2. **Send a multi-step message** in Slack
3. **Watch the logs** for the patterns above
4. **Identify where failures occur** using the error markers
5. **Share the logs** if you need help interpreting them

The debugging output will show you exactly where in the multi-step process any failure occurs!
