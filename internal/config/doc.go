// Package config provides simple, local-first configuration management for Loco.
//
// This package implements a minimal configuration system that focuses on simplicity
// and the DDAID (Development-Driven AI Development) workflow. All configuration
// is stored locally in the project's .loco/ directory.
//
// Configuration File Structure:
//
//	.loco/
//	├── config.json        # Main configuration (committed to git)
//	├── .gitignore         # Smart defaults for what to ignore
//	└── sessions/          # Session data
//
// The config.json file contains simple key-value settings:
//
//	{
//	  "lm_studio_url": "http://localhost:1234",
//	  "preferred_model": "auto",
//	  "theme": "fire",
//	  "debug": false,
//	  "tools_enabled": true
//	}
//
// Environment Variable Support:
//
// Configuration values can reference environment variables using $VAR or ${VAR} syntax:
//
//	{
//	  "lm_studio_url": "${LM_STUDIO_URL}",
//	  "api_key": "$OPENAI_API_KEY"
//	}
//
// Design Philosophy:
//
// - Local-first: Everything lives in the project directory
// - Simple: Single JSON file, no complex hierarchies
// - Smart defaults: Works out of the box
// - Git-friendly: Includes sensible .gitignore patterns
// - YAGNI: Only implements what's actually needed
//
// Example usage:
//
//	manager := config.NewManager("/path/to/project")
//	if err := manager.Load(); err != nil {
//		log.Fatal(err)
//	}
//	
//	cfg := manager.Get()
//	fmt.Println("LM Studio URL:", cfg.LMStudioURL)
//	
//	// Update a setting
//	manager.Set("theme", "dark")
package config