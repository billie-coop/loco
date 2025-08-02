package tools

// safeCommands are commands that are considered safe to run without permission.
// These are read-only commands that don't modify the system.
var safeCommands = []string{
	// Git read operations
	"git status",
	"git log",
	"git diff",
	"git branch",
	"git show",
	"git remote",
	"git config --get",
	
	// Directory navigation and listing
	"pwd",
	"ls",
	"dir",
	"tree",
	
	// File reading
	"cat",
	"head",
	"tail",
	"less",
	"more",
	
	// System information
	"whoami",
	"hostname",
	"uname",
	"date",
	"uptime",
	
	// Process information
	"ps",
	"top",
	"htop",
	
	// Package info (read-only)
	"npm list",
	"pip list",
	"pip show",
	"go list",
	"cargo tree",
	
	// Environment
	"env",
	"printenv",
	"echo",
	
	// Language version checks
	"node --version",
	"python --version",
	"go version",
	"cargo --version",
	"rustc --version",
	"java -version",
	"javac -version",
	"ruby --version",
	"php --version",
	
	// Help commands
	"man",
	"help",
	"--help",
	"-h",
	
	// File stats
	"wc",
	"du",
	"df",
	"stat",
	"file",
	
	// Searching (read-only)
	"grep",
	"rg",
	"ag",
	"find",
	"which",
	"whereis",
	"type",
}