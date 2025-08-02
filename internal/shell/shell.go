package shell

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// PersistentShell maintains shell state across multiple executions.
type PersistentShell struct {
	workingDir string
	env        map[string]string
	runner     *interp.Runner
	mu         sync.Mutex
	blockFuncs []BlockFunc
}

// BlockFunc is a function that can block command execution.
type BlockFunc func(cmd string, args []string) error

var (
	shells   = make(map[string]*PersistentShell)
	shellsMu sync.Mutex
)

// GetPersistentShell returns a persistent shell for the given working directory.
func GetPersistentShell(workingDir string) *PersistentShell {
	shellsMu.Lock()
	defer shellsMu.Unlock()

	if shell, exists := shells[workingDir]; exists {
		return shell
	}

	shell := NewPersistentShell(workingDir)
	shells[workingDir] = shell
	return shell
}

// NewPersistentShell creates a new persistent shell.
func NewPersistentShell(workingDir string) *PersistentShell {
	ps := &PersistentShell{
		workingDir: workingDir,
		env:        make(map[string]string),
	}

	// Copy current environment
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			ps.env[parts[0]] = parts[1]
		}
	}

	// Set working directory
	ps.env["PWD"] = workingDir

	ps.initRunner()
	return ps
}

// initRunner initializes the shell runner.
func (ps *PersistentShell) initRunner() {
	// Convert environment map to slice
	envSlice := make([]string, 0, len(ps.env))
	for k, v := range ps.env {
		envSlice = append(envSlice, k+"="+v)
	}
	
	ps.runner, _ = interp.New(
		interp.Dir(ps.workingDir),
		interp.Env(expand.ListEnviron(envSlice...)),
		interp.OpenHandler(openHandler),
		interp.ExecHandler(ps.execHandler),
	)
}

// Exec executes a command in the persistent shell.
func (ps *PersistentShell) Exec(ctx context.Context, command string) (stdout, stderr string, err error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Parse the command
	prog, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return "", "", fmt.Errorf("parse error: %w", err)
	}

	// Capture output
	var stdoutBuf, stderrBuf bytes.Buffer
	
	// Convert environment map to slice for this execution
	envSlice := make([]string, 0, len(ps.env))
	for k, v := range ps.env {
		envSlice = append(envSlice, k+"="+v)
	}

	// Create a new runner for this execution with output capture
	runner, err := interp.New(
		interp.Dir(ps.workingDir),
		interp.Env(expand.ListEnviron(envSlice...)),
		interp.StdIO(nil, &stdoutBuf, &stderrBuf),
		interp.OpenHandler(openHandler),
		interp.ExecHandler(ps.execHandler),
	)
	if err != nil {
		return "", "", fmt.Errorf("runner creation error: %w", err)
	}

	// Run the command
	if err := runner.Run(ctx, prog); err != nil {
		return stdoutBuf.String(), stderrBuf.String(), err
	}

	// Update working directory if it changed
	if pwd := runner.Dir; pwd != "" && pwd != ps.workingDir {
		ps.workingDir = pwd
		ps.env["PWD"] = pwd
	}

	// Update environment from runner
	runner.Env.Each(func(name string, vr expand.Variable) bool {
		ps.env[name] = vr.String()
		return true
	})

	return stdoutBuf.String(), stderrBuf.String(), nil
}

// GetWorkingDir returns the current working directory.
func (ps *PersistentShell) GetWorkingDir() string {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.workingDir
}

// SetBlockFuncs sets the command blocking functions.
func (ps *PersistentShell) SetBlockFuncs(funcs []BlockFunc) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.blockFuncs = funcs
}

// execHandler is called for each command execution.
func (ps *PersistentShell) execHandler(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return nil
	}

	cmd := args[0]
	cmdArgs := args[1:]

	// Check block functions
	for _, blockFunc := range ps.blockFuncs {
		if err := blockFunc(cmd, cmdArgs); err != nil {
			return err
		}
	}

	// Use default exec handler
	return interp.DefaultExecHandler(2*1024*1024)(ctx, args)
}

// openHandler handles file operations.
func openHandler(ctx context.Context, path string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	if path == "/dev/null" {
		return devNull{}, nil
	}
	return interp.DefaultOpenHandler()(ctx, path, flag, perm)
}

// devNull implements a /dev/null device.
type devNull struct{}

func (devNull) Read(p []byte) (int, error)  { return 0, io.EOF }
func (devNull) Write(p []byte) (int, error) { return len(p), nil }
func (devNull) Close() error                { return nil }

// IsInterrupt checks if an error is due to interruption.
func IsInterrupt(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "interrupt") || 
	       strings.Contains(err.Error(), "canceled")
}

// ExitCode extracts the exit code from an error.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(interp.ExitStatus); ok {
		return int(exitErr)
	}
	return 1
}

// CommandsBlocker blocks specific commands.
func CommandsBlocker(blockedCommands []string) BlockFunc {
	blocked := make(map[string]bool)
	for _, cmd := range blockedCommands {
		blocked[cmd] = true
	}

	return func(cmd string, args []string) error {
		// Check command name
		cmdName := filepath.Base(cmd)
		if blocked[cmdName] {
			return fmt.Errorf("command '%s' is not allowed", cmdName)
		}
		return nil
	}
}

// ArgumentsBlocker blocks specific command-argument combinations.
func ArgumentsBlocker(blockedCombos [][]string) BlockFunc {
	return func(cmd string, args []string) error {
		cmdName := filepath.Base(cmd)
		
		for _, combo := range blockedCombos {
			if len(combo) == 0 {
				continue
			}
			
			// Check if command matches
			if combo[0] != cmdName {
				continue
			}
			
			// Check if arguments match
			if len(combo) > 1 && len(args) >= len(combo)-1 {
				match := true
				for i := 1; i < len(combo); i++ {
					if i-1 >= len(args) || args[i-1] != combo[i] {
						match = false
						break
					}
				}
				if match {
					return fmt.Errorf("command '%s %s' is not allowed", 
						cmdName, strings.Join(combo[1:], " "))
				}
			}
		}
		return nil
	}
}