package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/permission"
	"github.com/billie-coop/loco/internal/shell"
)

// BashParams represents parameters for bash command execution.
type BashParams struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

// BashPermissionsParams represents parameters for permission requests.
type BashPermissionsParams struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

// BashResponseMetadata contains metadata about command execution.
type BashResponseMetadata struct {
	StartTime        int64  `json:"start_time"`
	EndTime          int64  `json:"end_time"`
	Output           string `json:"output"`
	WorkingDirectory string `json:"working_directory"`
}

// bashTool implements the bash command execution tool.
type bashTool struct {
	permissions permission.Service
	workingDir  string
}

const (
	// BashToolName is the name of this tool
	BashToolName = "bash"

	// DefaultTimeout is 1 minute in milliseconds
	DefaultTimeout = 1 * 60 * 1000
	// MaxTimeout is 10 minutes in milliseconds  
	MaxTimeout = 10 * 60 * 1000
	// MaxOutputLength limits output size
	MaxOutputLength = 30000
	// BashNoOutput indicates no output was produced
	BashNoOutput = "no output"
)

// bannedCommands are commands that are not allowed for security.
var bannedCommands = []string{
	// Network/Download tools
	"alias",
	"aria2c",
	"axel",
	"chrome",
	"curl",
	"curlie", 
	"firefox",
	"http-prompt",
	"httpie",
	"links",
	"lynx",
	"nc",
	"safari",
	"scp",
	"ssh",
	"telnet",
	"w3m",
	"wget",
	"xh",

	// System administration
	"doas",
	"su", 
	"sudo",

	// Package managers
	"apk",
	"apt",
	"apt-cache",
	"apt-get",
	"dnf",
	"dpkg",
	"emerge",
	"home-manager",
	"makepkg",
	"opkg",
	"pacman",
	"paru",
	"pkg",
	"pkg_add",
	"pkg_delete",
	"portage",
	"rpm",
	"yay",
	"yum",
	"zypper",

	// System modification
	"at",
	"batch",
	"chkconfig",
	"crontab",
	"fdisk",
	"mkfs",
	"mount",
	"parted",
	"service",
	"systemctl",
	"umount",

	// Network configuration
	"firewall-cmd",
	"ifconfig",
	"ip",
	"iptables",
	"netstat",
	"pfctl",
	"route",
	"ufw",
}

// bashDescription returns the tool description.
func bashDescription() string {
	bannedCommandsStr := strings.Join(bannedCommands, ", ")
	return fmt.Sprintf(`Executes a given bash command in a persistent shell session with optional timeout, ensuring proper handling and security measures.

CROSS-PLATFORM SHELL SUPPORT:
* This tool uses a shell interpreter (mvdan/sh) that mimics the Bash language,
  so you should use Bash syntax in all platforms, including Windows.
  The most common shell builtins and core utils are available in Windows as
  well.
* Make sure to use forward slashes (/) as path separators in commands, even on
  Windows. Example: "ls C:/foo/bar" instead of "ls C:\foo\bar".

Before executing the command, please follow these steps:

1. Directory Verification:
   - If the command will create new directories or files, first use the LS tool to verify the parent directory exists and is the correct location
   - For example, before running "mkdir foo/bar", first use LS to check that "foo" exists and is the intended parent directory

2. Security Check:
   - For security and to limit the threat of a prompt injection attack, some commands are limited or banned. If you use a disallowed command, you will receive an error message explaining the restriction. Explain the error to the User.
   - Verify that the command is not one of the banned commands: %s.

3. Command Execution:
   - After ensuring proper quoting, execute the command.
   - Capture the output of the command.

4. Output Processing:
   - If the output exceeds %d characters, output will be truncated before being returned to you.
   - Prepare the output for display to the user.

5. Return Result:
   - Provide the processed output of the command.
   - If any errors occurred during execution, include those in the output.
   - The result will also have metadata like the cwd (current working directory) at the end, included with <cwd></cwd> tags.

Usage notes:
- The command argument is required.
- You can specify an optional timeout in milliseconds (up to 600000ms / 10 minutes). If not specified, commands will timeout after %d ms.
- VERY IMPORTANT: You MUST avoid using search commands like 'find' and 'grep'. Instead use Grep, Glob, or other tools to search. You MUST avoid read tools like 'cat', 'head', 'tail', and 'ls', and use View and LS tools to read files.
- When issuing multiple commands, use the ';' or '&&' operator to separate them. DO NOT use newlines (newlines are ok in quoted strings).
- IMPORTANT: All commands share the same shell session. Shell state (environment variables, virtual environments, current directory, etc.) persist between commands. For example, if you set an environment variable as part of a command, the environment variable will persist for subsequent commands.
- Try to maintain your current working directory throughout the session by using absolute paths and avoiding usage of 'cd'. You may use 'cd' if the User explicitly requests it.

# Committing changes with git

When the user asks you to create a new git commit, follow these steps carefully:

1. Start by running these commands in parallel:
   - Run a git status command to see all untracked files.
   - Run a git diff command to see both staged and unstaged changes that will be committed.
   - Run a git log command to see recent commit messages, so that you can follow this repository's commit message style.

2. Analyze all staged changes (both previously staged and newly added) and draft a commit message:
   - Summarize the nature of the changes
   - Check for any sensitive information that shouldn't be committed
   - Draft a concise (1-2 sentences) commit message that focuses on the "why" rather than the "what"
   - Ensure it accurately reflects the changes and their purpose

3. Create the commit with a message ending with:
   🚂 Generated with Loco
   Co-Authored-By: Loco <loco@billie.coop>

Important notes:
- NEVER update the git config
- DO NOT push to the remote repository unless explicitly asked
- IMPORTANT: Never use git commands with the -i flag (like git rebase -i or git add -i) since they require interactive input which is not supported.
- If there are no changes to commit (i.e., no untracked files and no modifications), do not create an empty commit`, bannedCommandsStr, MaxOutputLength, DefaultTimeout)
}

// blockFuncs returns the command blocking functions.
func blockFuncs() []shell.BlockFunc {
	return []shell.BlockFunc{
		shell.CommandsBlocker(bannedCommands),
		shell.ArgumentsBlocker([][]string{
			// System package managers
			{"apk", "add"},
			{"apt", "install"},
			{"apt-get", "install"},
			{"dnf", "install"},
			{"emerge"},
			{"pacman", "-S"},
			{"pkg", "install"},
			{"yum", "install"},
			{"zypper", "install"},

			// Language-specific package managers
			{"brew", "install"},
			{"cargo", "install"},
			{"gem", "install"},
			{"go", "install"},
			{"npm", "install", "-g"},
			{"npm", "install", "--global"},
			{"pip", "install", "--user"},
			{"pip3", "install", "--user"},
			{"pnpm", "add", "-g"},
			{"pnpm", "add", "--global"},
			{"yarn", "global", "add"},
		}),
	}
}

// NewBashTool creates a new bash tool instance.
func NewBashTool(permissions permission.Service, workingDir string) BaseTool {
	// Set up command blocking on the persistent shell
	persistentShell := shell.GetPersistentShell(workingDir)
	persistentShell.SetBlockFuncs(blockFuncs())

	return &bashTool{
		permissions: permissions,
		workingDir:  workingDir,
	}
}

// Name returns the tool name.
func (b *bashTool) Name() string {
	return BashToolName
}

// Info returns the tool information.
func (b *bashTool) Info() ToolInfo {
	return ToolInfo{
		Name:        BashToolName,
		Description: bashDescription(),
		Parameters: map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The command to execute",
			},
			"timeout": map[string]any{
				"type":        "number",
				"description": "Optional timeout in milliseconds (max 600000)",
			},
		},
		Required: []string{"command"},
	}
}

// Run executes the bash command.
func (b *bashTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params BashParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse("invalid parameters"), nil
	}

	if params.Timeout > MaxTimeout {
		params.Timeout = MaxTimeout
	} else if params.Timeout <= 0 {
		params.Timeout = DefaultTimeout
	}

	if params.Command == "" {
		return NewTextErrorResponse("missing command"), nil
	}

	// Check if command is safe (read-only)
	isSafeReadOnly := false
	cmdLower := strings.ToLower(params.Command)

	for _, safe := range safeCommands {
		if strings.HasPrefix(cmdLower, safe) {
			if len(cmdLower) == len(safe) || cmdLower[len(safe)] == ' ' || cmdLower[len(safe)] == '-' {
				isSafeReadOnly = true
				break
			}
		}
	}

	// Request permission for non-safe commands
	sessionID, messageID := GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session ID and message ID are required for bash execution")
	}

	if !isSafeReadOnly {
		p := b.permissions.Request(
			permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        b.workingDir,
				ToolCallID:  call.ID,
				ToolName:    BashToolName,
				Action:      "execute",
				Description: fmt.Sprintf("Execute command: %s", params.Command),
				Params: BashPermissionsParams{
					Command: params.Command,
					Timeout: params.Timeout,
				},
			},
		)
		if !p {
			return ToolResponse{}, permission.ErrorPermissionDenied
		}
	}

	startTime := time.Now()
	if params.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(params.Timeout)*time.Millisecond)
		defer cancel()
	}

	persistentShell := shell.GetPersistentShell(b.workingDir)
	stdout, stderr, err := persistentShell.Exec(ctx, params.Command)

	// Get the current working directory after command execution
	currentWorkingDir := persistentShell.GetWorkingDir()
	interrupted := shell.IsInterrupt(err)
	exitCode := shell.ExitCode(err)

	if exitCode == 0 && !interrupted && err != nil {
		return ToolResponse{}, fmt.Errorf("error executing command: %w", err)
	}

	stdout = truncateOutput(stdout)
	stderr = truncateOutput(stderr)

	errorMessage := stderr
	if errorMessage == "" && err != nil {
		errorMessage = err.Error()
	}

	if interrupted {
		if errorMessage != "" {
			errorMessage += "\n"
		}
		errorMessage += "Command was aborted before completion"
	} else if exitCode != 0 {
		if errorMessage != "" {
			errorMessage += "\n"
		}
		errorMessage += fmt.Sprintf("Exit code %d", exitCode)
	}

	hasBothOutputs := stdout != "" && stderr != ""

	if hasBothOutputs {
		stdout += "\n"
	}

	if errorMessage != "" {
		stdout += "\n" + errorMessage
	}

	metadata := BashResponseMetadata{
		StartTime:        startTime.UnixMilli(),
		EndTime:          time.Now().UnixMilli(),
		Output:           stdout,
		WorkingDirectory: currentWorkingDir,
	}

	if stdout == "" {
		return WithResponseMetadata(NewTextResponse(BashNoOutput), metadata), nil
	}

	stdout += fmt.Sprintf("\n\n<cwd>%s</cwd>", currentWorkingDir)
	return WithResponseMetadata(NewTextResponse(stdout), metadata), nil
}

// truncateOutput truncates long output to prevent overwhelming the UI.
func truncateOutput(content string) string {
	if len(content) <= MaxOutputLength {
		return content
	}

	halfLength := MaxOutputLength / 2
	start := content[:halfLength]
	end := content[len(content)-halfLength:]

	truncatedLinesCount := countLines(content[halfLength : len(content)-halfLength])
	return fmt.Sprintf("%s\n\n... [%d lines truncated] ...\n\n%s", start, truncatedLinesCount, end)
}

// countLines counts the number of lines in a string.
func countLines(s string) int {
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}