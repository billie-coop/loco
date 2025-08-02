# Debugging TUI Apps with Delve

## The Problem
When debugging a TUI app in the integrated terminal, breakpoints mess up the display because the TUI can't redraw while paused.

## The Solution: Delve Server

### Step 1: Start Delve Server
In Terminal 1:
```bash
make run-dlv
```

You'll see:
```
API server listening at: [::]:2345
```

### Step 2: Attach from VSCode
1. In VSCode, select "Attach to Delve Server" from the debug dropdown
2. Press F5
3. The terminal will show the Loco TUI running normally

### Step 3: Debug without UI corruption
- Set your breakpoints
- Interact with Loco in Terminal 1
- When breakpoints hit, VSCode shows variables but Terminal 1 stays clean
- Step through code in VSCode while TUI remains visible

## Why This Works
- TUI runs in its own terminal (not affected by debugger)
- Debugger communicates over network port (2345)
- You can see both the TUI and debug info simultaneously

## Pro Tips
- Use two monitors: TUI on one, VSCode on the other
- Or split terminal below VSCode: TUI bottom, debug top
- The TUI will "freeze" at breakpoints but won't corrupt