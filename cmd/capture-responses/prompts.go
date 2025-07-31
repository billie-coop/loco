package main

// ExtendedTestPrompts contains a comprehensive set of prompts for testing
var ExtendedTestPrompts = []TestPrompt{
	// === READ FILE VARIATIONS ===
	{
		Name:        "read_direct_command",
		Description: "Direct command style",
		Prompt:      "read main.go",
	},
	{
		Name:        "read_polite_request",
		Description: "Polite request",
		Prompt:      "Could you please show me the contents of the README.md file?",
	},
	{
		Name:        "read_question_form",
		Description: "Question form",
		Prompt:      "What's in the parser.go file?",
	},
	{
		Name:        "read_specific_lines",
		Description: "Read specific lines",
		Prompt:      "Show me lines 10-20 of main.go",
	},
	{
		Name:        "read_multiple_implicit",
		Description: "Implicit multiple reads",
		Prompt:      "I need to see both the go.mod and go.sum files",
	},
	{
		Name:        "read_with_context",
		Description: "Read with context",
		Prompt:      "I'm debugging an issue. Can you show me the error handling in internal/tools/read.go?",
	},
	{
		Name:        "read_check_syntax",
		Description: "Check-style request",
		Prompt:      "Check what's in the test file parser_test.go",
	},

	// === LIST DIRECTORY VARIATIONS ===
	{
		Name:        "list_root",
		Description: "List root directory",
		Prompt:      "What files are in this project?",
	},
	{
		Name:        "list_specific_dir",
		Description: "List specific directory",
		Prompt:      "Show me all the files in internal/parser/",
	},
	{
		Name:        "list_explore_style",
		Description: "Exploration style",
		Prompt:      "Let's explore what's in the tools folder",
	},
	{
		Name:        "list_question_contents",
		Description: "Question about contents",
		Prompt:      "What does the internal directory contain?",
	},
	{
		Name:        "list_find_files",
		Description: "Find-style request",
		Prompt:      "Help me find all the test files",
	},
	{
		Name:        "list_browse",
		Description: "Browse-style request",
		Prompt:      "Browse the cmd directory",
	},

	// === WRITE FILE VARIATIONS ===
	{
		Name:        "write_create_simple",
		Description: "Simple file creation",
		Prompt:      "Create a file called hello.txt with 'Hello World'",
	},
	{
		Name:        "write_save_code",
		Description: "Save code snippet",
		Prompt:      "Save this to test.go: func main() { fmt.Println(\"test\") }",
	},
	{
		Name:        "write_update_existing",
		Description: "Update existing file",
		Prompt:      "Update the README to add '## New Section' at the end",
	},
	{
		Name:        "write_create_config",
		Description: "Create config file",
		Prompt:      "Generate a basic config.json file with server settings",
	},

	// === COMPLEX MULTI-TOOL ===
	{
		Name:        "multi_explore_and_read",
		Description: "List then read",
		Prompt:      "First show me what's in the internal directory, then read the most important looking file",
	},
	{
		Name:        "multi_check_and_fix",
		Description: "Read then write",
		Prompt:      "Check if there's a .gitignore file, if not create one for a Go project",
	},
	{
		Name:        "multi_analyze_structure",
		Description: "Multiple lists and reads",
		Prompt:      "Analyze the project structure - list the main directories and read any configuration files",
	},
	{
		Name:        "multi_sequential_reads",
		Description: "Sequential file reads",
		Prompt:      "Read these files in order: main.go, then go.mod, then README.md",
	},

	// === EDGE CASES ===
	{
		Name:        "edge_ambiguous",
		Description: "Ambiguous request",
		Prompt:      "Show me the file",
	},
	{
		Name:        "edge_typo",
		Description: "Request with typo",
		Prompt:      "Reed the main.go file plz",
	},
	{
		Name:        "edge_mixed_request",
		Description: "Mixed tool and non-tool",
		Prompt:      "Explain what a parser does and then show me our parser.go implementation",
	},
	{
		Name:        "edge_conditional",
		Description: "Conditional tool use",
		Prompt:      "If there's a TODO.md file, show it to me",
	},

	// === NON-TOOL REQUESTS ===
	{
		Name:        "no_tool_explain",
		Description: "Pure explanation",
		Prompt:      "Explain the difference between a lexer and a parser",
	},
	{
		Name:        "no_tool_opinion",
		Description: "Opinion question",
		Prompt:      "What's the best way to structure a Go project?",
	},
	{
		Name:        "no_tool_general",
		Description: "General chat",
		Prompt:      "How's it going?",
	},
	{
		Name:        "no_tool_help",
		Description: "Help request",
		Prompt:      "What can you help me with?",
	},
	{
		Name:        "no_tool_debug_discuss",
		Description: "Debugging discussion",
		Prompt:      "I'm getting a nil pointer error, what are common causes?",
	},

	// === TRICKY CASES ===
	{
		Name:        "tricky_looks_like_tool",
		Description: "Mentions files but doesn't want tool",
		Prompt:      "Tell me about common patterns in main.go files",
	},
	{
		Name:        "tricky_quoted_command",
		Description: "Command in quotes",
		Prompt:      "The user typed 'read file.txt' but it didn't work",
	},
	{
		Name:        "tricky_past_tense",
		Description: "Past tense tool mention",
		Prompt:      "I already read the main.go file, now what?",
	},

	// === DIFFERENT PHRASINGS ===
	{
		Name:        "phrase_examine",
		Description: "Examine phrasing",
		Prompt:      "Examine the contents of config.yaml",
	},
	{
		Name:        "phrase_inspect",
		Description: "Inspect phrasing",
		Prompt:      "Inspect the parser implementation",
	},
	{
		Name:        "phrase_look_at",
		Description: "Look at phrasing",
		Prompt:      "Look at what's inside the tools folder",
	},
	{
		Name:        "phrase_peek",
		Description: "Peek phrasing",
		Prompt:      "Take a peek at the test files",
	},
	{
		Name:        "phrase_display",
		Description: "Display phrasing",
		Prompt:      "Display the go.mod",
	},
}

// SystemPromptVariations provides different system prompts to test model behavior
var SystemPromptVariations = map[string]string{
	"standard": `You are Loco, a helpful AI coding assistant.

You have access to the following tools. When you need to use a tool, output it in this format:
<tool>{"name": "tool_name", "params": {"param1": "value1"}}</tool>

Available tools:
- read_file: Read contents of a file
- list_directory: List files in a directory  
- write_file: Create or update a file`,

	"detailed": `You are Loco, a helpful AI coding assistant.

You have access to the following tools. When you need to use a tool, output it in this format:
<tool>{"name": "tool_name", "params": {"param1": "value1"}}</tool>

Available tools:

read_file: Read contents of a file
Parameters:
- path: The file path (required)
- start_line: Starting line number (optional)
- num_lines: Number of lines to read (optional)
Example: <tool>{"name": "read_file", "params": {"path": "main.go"}}</tool>

list_directory: List files in a directory
Parameters:
- path: Directory path (optional, defaults to current)
Example: <tool>{"name": "list_directory", "params": {"path": "src/"}}</tool>

write_file: Create or update a file
Parameters:
- path: File path (required)
- content: File content (required)
Example: <tool>{"name": "write_file", "params": {"path": "test.txt", "content": "Hello"}}</tool>

Always use tools when the user asks to see, read, list, or modify files.`,

	"minimal": `You can use these tools:
<tool>{"name": "read_file", "params": {"path": "filename"}}</tool>
<tool>{"name": "list_directory", "params": {"path": "dir"}}</tool>
<tool>{"name": "write_file", "params": {"path": "file", "content": "text"}}</tool>`,

	"conversational": `Hey! I'm Loco, your coding buddy. I can help you explore and work with files.

When I need to look at files or directories, I'll use special tools. Don't worry about the technical details - I'll handle that! Just ask me what you need.

(For technical users: I output tool calls as <tool>JSON</tool>)`,
}