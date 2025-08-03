// Package main is the entry point for the loco application.
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/billie-coop/loco/internal/app"
	"github.com/billie-coop/loco/internal/tui"
	"github.com/billie-coop/loco/internal/tui/events"
	tea "github.com/charmbracelet/bubbletea/v2"
)

func main() {
	// Get current working directory
	workingDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	// Create event broker
	eventBroker := events.NewBroker()

	// Create app with all services
	appInstance := app.New(workingDir, eventBroker)

	// Initialize LLM client from config
	if err := appInstance.InitLLMFromConfig(); err != nil {
		log.Fatalf("Failed to initialize LLM client: %v", err)
	}

	// Create TUI model
	tuiModel := tui.New(appInstance, eventBroker)

	// Trigger startup analysis after a small delay
	// This is system-initiated and will show permission dialog on first run
	go func() {
		// Small delay to let UI initialize
		time.Sleep(500 * time.Millisecond)
		appInstance.RunStartupAnalysis()
	}()

	// Create and run Bubble Tea program
	program := tea.NewProgram(tuiModel, tea.WithAltScreen())

	_, err = program.Run()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
