# Loco Testing Plan

## Pre-Test Checklist

Before testing, make sure:
- [ ] LM Studio is running on http://localhost:1234
- [ ] You have at least one model loaded in LM Studio
- [ ] The project builds without errors: `go build ./...`

## Phase 1: Basic Tool Testing (RIGHT NOW)

Let's first create a simple test script to verify tools work in isolation:

```bash
# Test the parser alone
go run cmd/test-parser/main.go

# Build and run the main app
go build -o loco
./loco
```

## What to Test First (Quick Smoke Test - 5 mins)

1. **Tool Parsing**
   - "Show me the main.go file"
   - "List the files in internal/"
   - "Read the README"

2. **Natural Language**
   - "I want to see what's in the tools directory"
   - "Can you check what's in parser.go?"

3. **Edge Cases**
   - Send a normal message (no tools)
   - Ask about the project

## Phase 2: Manual Testing Session (BEST TIME TO PAUSE)

**This is the perfect pause point!** Once basic tools work, you'll want to:

1. **Test Different Models**
   - Try with different models in LM Studio
   - See which ones follow the tool format best
   - Note which models need better prompting

2. **Test Tool Combinations**
   - "Show me all Go files then read main.go"
   - "List directories and read any README files"

3. **Test Error Cases**
   - Ask to read non-existent files
   - Try invalid paths
   - Test permission errors

4. **Session Features**
   - `/new` - Create new sessions
   - `/list` - List sessions
   - `/switch 1` - Switch between sessions
   - `/reset` - Clear and start fresh

## What to Look For

### 🟢 Success Indicators
- Tools execute and show results
- AI incorporates tool results in response
- Multiple tools in one message work
- Sessions save/load correctly

### 🔴 Common Issues
- Model doesn't use tool format → Try different model or adjust prompt
- Parser misses tools → Check parser patterns
- Tools error out → Check file paths and permissions
- UI glitches → Window resizing issues

## Debug Commands

If something goes wrong:
```bash
# Check logs
tail -f ~/.loco/debug.log  # (if we add logging)

# Check session files
ls -la .loco/sessions/

# Test parser directly
go test ./internal/parser -v

# Run with debug environment
DEBUG=1 ./loco
```

## After Testing

Document what you find:
1. Which models work best?
2. What prompts help models use tools?
3. Any UI issues?
4. Performance problems?

## Next Steps Based on Results

- **If tools work perfectly** → Move to orchestrator
- **If parsing is flaky** → Add AI post-processor
- **If models won't cooperate** → Improve prompting
- **If UI has issues** → Fix before adding complexity