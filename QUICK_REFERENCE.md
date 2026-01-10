# Quick Reference: Multi-Step Agent Debugging Logs

## 🔍 Key Log Patterns to Watch For

### ✅ Success Indicators

```
=== AGENT PATH STARTED ===
=== AGENT EXECUTOR STARTING ===
[AGENT-CALLBACK] === CHAIN START === iteration=1
[TOOL-CALL] SUCCESS: Tool ...
[AGENT-CALLBACK] === CHAIN START === iteration=2  ← Second iteration!
[TOOL-CALL] SUCCESS: Tool ...
=== AGENT EXECUTOR COMPLETED ===
Agent output received - is_empty=false
```

### ❌ Failure Indicators

```
=== AGENT EXECUTOR FAILED ===
[AGENT-CALLBACK] === CHAIN ERROR ===
[AGENT-CALLBACK] === LLM ERROR ===
[AGENT-CALLBACK] === TOOL ERROR ===
[TOOL-CALL] ERROR:
```

### 🔎 What Each Marker Means

| Marker | Meaning |
|--------|---------|
| `=== AGENT PATH STARTED ===` | Agent mode activated |
| `=== AGENT EXECUTOR STARTING ===` | Agent executor begins |
| `[AGENT-CALLBACK] === CHAIN START === iteration=N` | New reasoning step |
| `[AGENT-CALLBACK] Agent Action - tool=...` | Agent chose a tool |
| `[TOOL-CALL] === TOOL INVOCATION START ===` | Tool execution begins |
| `[TOOL-CALL] SUCCESS:` | Tool completed successfully |
| `[AGENT-CALLBACK] === CHAIN END ===` | Step completed |
| `=== AGENT EXECUTOR COMPLETED ===` | All steps done |
| `=== AGENT EXECUTOR FAILED ===` | Something went wrong |

## 🎯 Quick Diagnostic Commands

```bash
# See only the flow
docker logs <container> 2>&1 | grep "==="

# Count iterations
docker logs <container> 2>&1 | grep "CHAIN START" | wc -l

# See all tool calls
docker logs <container> 2>&1 | grep "\[TOOL-CALL\]"

# See only errors
docker logs <container> 2>&1 | grep -i "error\|failed"

# See iteration sequence
docker logs <container> 2>&1 | grep "iteration="
```

## 🐛 Common Failure Patterns

### Pattern 1: Stops After First Iteration
```
iteration=1 → SUCCESS
iteration=2 → (missing or fails immediately)
```
**Likely cause:** Tool result not being passed to next iteration

### Pattern 2: Context Timeout
```
=== AGENT EXECUTOR FAILED ===
context_err=context deadline exceeded
```
**Fix:** Increase timeout or simplify request

### Pattern 3: Iteration Limit
```
Possible iteration limit reached
```
**Fix:** Increase `maxAgentIterations` in config

### Pattern 4: Parsing Error
```
Possible parsing error in agent response
```
**Fix:** LLM returned malformed output - check raw response

## 📊 Expected Iteration Count

- **1-step request:** `iteration=1` only
- **2-step request:** `iteration=1`, `iteration=2`
- **3-step request:** `iteration=1`, `iteration=2`, `iteration=3`

If you see fewer iterations than expected, that's the bug!
