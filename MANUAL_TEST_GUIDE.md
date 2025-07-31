# Manual Testing Guide for Loco

## Pre-Test Setup

1. **Start LM Studio**
   - Make sure it's running on http://localhost:1234
   - Load at least one model (ideally 2-3 different ones to compare)

2. **Build Loco**
   ```bash
   go build -o loco
   ```

## Testing Phase 1: Basic Smoke Test (5 mins)

### Goal: Verify tools work at all

```bash
./loco
```

Try these exact prompts first:

1. **Test read_file**
   ```
   Show me the main.go file
   ```
   - ✅ Should see file contents with line numbers
   - ❌ If no output, check debug logs in sidebar

2. **Test list_directory**
   ```
   List files in internal/
   ```
   - ✅ Should see directory listing
   - ❌ If error, check if path exists

3. **Test write_file** 
   ```
   Create test.txt with "Hello from Loco!"
   ```
   - ✅ Should create file
   - ❌ Check permissions if fails

4. **Test no tools**
   ```
   What is a parser?
   ```
   - ✅ Should just explain, no tools
   - ❌ If tries to use tools unnecessarily

## Testing Phase 2: Model Comparison (10 mins)

### Goal: Find which models work best

For each model you have loaded:

1. **Switch models** (restart Loco, select different model)

2. **Test each model with**:
   - Simple: "read README.md"
   - Natural: "Can you check what's in the parser.go file?"
   - Multiple: "List internal/ then read the first .go file"

3. **Track in notes**:
   ```
   Model: llama-3.2-1b
   - Follows <tool> format: Yes/No/Sometimes
   - Natural language detection: Good/Bad
   - Multiple tools: Works/Fails
   - Speed: Fast/Slow
   ```

## Testing Phase 3: Edge Cases (10 mins)

### Goal: Find parser limitations

1. **Ambiguous requests**
   ```
   Show me the file
   Show me that thing we were just talking about
   The main one
   ```

2. **Typos and variations**
   ```
   reed the main.go file
   plz show parser.go
   main.go ?
   ```

3. **Complex requests**
   ```
   If there's a TODO file, show it, otherwise create one
   Show me all test files
   Find and read any config files
   ```

4. **Model confusion**
   ```
   I already read main.go (should not read again)
   Tell me about files named main.go (should not read)
   "read main.go" doesn't work (should not execute)
   ```

## Testing Phase 4: UI/UX Testing (5 mins)

### Goal: Ensure good user experience

1. **Long outputs**
   - Read a large file (100+ lines)
   - Check scrolling works
   - Check line wrapping

2. **Streaming**
   - Watch for smooth token display
   - Check if UI stays responsive

3. **Sessions**
   ```
   /list
   /new
   (chat a bit)
   /list (should see both sessions)
   /switch 1
   ```

4. **Debug visibility**
   - Are debug logs helpful?
   - Can you see parse method?
   - Tool execution visible?

## What to Document

Create a file `TEST_RESULTS.md`:

```markdown
# Test Results - [Date]

## Models Tested

### llama-3.2-1b
- Tool format compliance: 8/10
- Best for: Quick responses
- Issues: Sometimes forgets to close JSON

### mistral-7b
- Tool format compliance: 9/10  
- Best for: Complex requests
- Issues: Slower responses

## Parser Success Rate

- Direct JSON: 100%
- <tool> tags: 95%
- Markdown JSON: 90%
- Natural language: 60%

## Edge Cases Found

1. Model X always adds "Sure!" before tool calls
2. Model Y uses single quotes in JSON
3. Multiple tools only work with models > 7B

## UI Issues

- [ ] Text wrapping breaks with very long lines
- [ ] Debug logs overlap on narrow terminal
- [x] Scrolling works well

## Recommended Improvements

1. Need better prompt for Model X
2. Parser should handle single quotes
3. Add "Executing tool..." message
```

## Quick Test Script

If you want to test systematically:

```bash
#!/bin/bash
# test-loco.sh

echo "Test 1: Basic read"
echo "Prompt: Show me the main.go file"
echo "Expected: File contents with line numbers"
echo "Press enter after testing..."
read

echo "Test 2: Natural language"  
echo "Prompt: What's inside the README?"
echo "Expected: README contents"
echo "Press enter after testing..."
read

# ... more tests
```

## When You're Done

1. Note which models work best
2. Save any problematic responses
3. List UI/UX improvements needed
4. We'll fix issues before moving to orchestrator

Ready to start testing! Run `./loco` and let me know what you find!