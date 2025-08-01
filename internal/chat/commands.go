package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/llm"
	"github.com/billie-coop/loco/internal/project"
	"github.com/billie-coop/loco/internal/session"
	tea "github.com/charmbracelet/bubbletea/v2"
)

func (m *Model) handleSlashCommand(input string) (tea.Model, tea.Cmd) {
	m.input.Reset()
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return m, nil
	}
	command := strings.ToLower(parts[0])

	switch command {
	case "/debug":
		return m.handleDebugCommand()
	case "/list":
		return m.handleListCommand()
	case "/new":
		return m.handleNewCommand()
	case "/switch":
		return m.handleSwitchCommand(parts)
	case "/project":
		return m.handleProjectCommand()
	case "/analyze":
		return m.handleAnalyzeCommand()
	case "/analyze-files":
		return m.handleAnalyzeFilesCommand()
	case "/reset":
		return m.handleResetCommand()
	case "/screenshot":
		return m, m.captureScreen()
	case "/copy":
		return m.handleCopyCommand(parts)
	case "/confirm-write":
		return m.handleConfirmWriteCommand()
	case "/team":
		return m.handleTeamCommand()
	case "/knowledge":
		return m.handleKnowledgeCommand(parts)
	case "/help":
		return m.handleHelpCommand()
	default:
		return m.handleUnknownCommand(command)
	}
}

func (m *Model) handleDebugCommand() (tea.Model, tea.Cmd) {
	m.showDebug = !m.showDebug
	m.viewport.SetContent(m.renderMessages())
	return m, nil
}

func (m *Model) handleListCommand() (tea.Model, tea.Cmd) {
	sessions := m.sessionManager.ListSessions()
	var msg strings.Builder
	msg.WriteString("📋 Available sessions:\n\n")

	for i, s := range sessions {
		current := ""
		currentSession, err := m.sessionManager.GetCurrent()
		if err != nil {
			currentSession = nil
		}
		if currentSession != nil && s.ID == currentSession.ID {
			current = " (current)"
		}
		msg.WriteString(fmt.Sprintf("%d. %s%s\n   Created: %s\n",
			i+1, s.Title, current, s.Created.Format("Jan 2 15:04")))
	}

	m.viewport.SetContent(msg.String())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *Model) handleNewCommand() (tea.Model, tea.Cmd) {
	_, err := m.sessionManager.NewSession(m.modelName)
	if err != nil {
		return m, nil
	}

	systemPrompt := "You are Loco, a helpful AI coding assistant running locally via LM Studio."
	if m.projectContext != nil {
		systemPrompt += "\n\n" + m.projectContext.FormatForPrompt()
	}

	m.messages = []llm.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	if err := m.sessionManager.UpdateCurrentMessages(m.messages); err != nil {
		// Log but continue - session updates are not critical
		_ = err
	}

	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *Model) handleSwitchCommand(parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m.showStatus("Usage: /switch <session-number>")
		m.viewport.SetContent(m.renderMessages())
		return m, nil
	}

	var sessionNum int
	if _, err := fmt.Sscanf(parts[1], "%d", &sessionNum); err != nil {
		m.showStatus("Invalid session number")
		m.viewport.SetContent(m.renderMessages())
		return m, nil
	}

	sessions := m.sessionManager.ListSessions()
	if sessionNum < 1 || sessionNum > len(sessions) {
		m.showStatus("Session number out of range")
		m.viewport.SetContent(m.renderMessages())
		return m, nil
	}

	selectedSession := sessions[sessionNum-1]
	if err := m.sessionManager.SetCurrent(selectedSession.ID); err != nil {
		m.showStatus("Failed to switch session")
		m.viewport.SetContent(m.renderMessages())
		return m, nil
	}

	m.messages = selectedSession.Messages
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *Model) handleProjectCommand() (tea.Model, tea.Cmd) {
	if m.projectContext == nil {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: "No project context available",
		})
		m.viewport.SetContent(m.renderMessages())
		return m, nil
	}

	info := "📁 Project Context:\n" + m.projectContext.FormatForPrompt()
	m.messages = append(m.messages, llm.Message{
		Role:    "system",
		Content: info,
	})
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *Model) handleAnalyzeCommand() (tea.Model, tea.Cmd) {
	workingDir, err := os.Getwd()
	if err != nil || workingDir == "" {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: "Cannot determine working directory for analysis",
		})
		m.viewport.SetContent(m.renderMessages())
		return m, nil
	}

	m.messages = append(m.messages, llm.Message{
		Role:    "system",
		Content: "🔍 Re-analyzing project with deep file reading...",
	})
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()

	analyzer := project.NewAnalyzer()
	ctx, err := analyzer.AnalyzeProject(workingDir)
	if err != nil {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: fmt.Sprintf("❌ Analysis failed: %v", err),
		})
	} else {
		m.projectContext = ctx
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: "✅ Project re-analyzed successfully!\n\n📁 Updated Context:\n" + ctx.FormatForPrompt(),
		})
	}

	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *Model) handleResetCommand() (tea.Model, tea.Cmd) {
	if err := m.resetProject(); err != nil {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: fmt.Sprintf("Failed to reset project: %v", err),
		})
		m.viewport.SetContent(m.renderMessages())
		return m, nil
	}

	m.handleNewCommand()
	m.messages = append(m.messages, llm.Message{
		Role:    "system",
		Content: "Project reset - all sessions moved to trash",
	})
	m.viewport.SetContent(m.renderMessages())
	return m, nil
}

func (m *Model) handleConfirmWriteCommand() (tea.Model, tea.Cmd) {
	if m.pendingWrite == nil {
		m.showStatus("No pending write operation")
		return m, nil
	}

	result := m.toolRegistry.Execute(m.pendingWrite.Name, m.pendingWrite.Params)
	if result.Success {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: "✅ Write confirmed: " + result.Output,
		})
	} else {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: "❌ Write failed: " + result.Error,
		})
	}

	m.pendingWrite = nil
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *Model) handleTeamCommand() (tea.Model, tea.Cmd) {
	return m, func() tea.Msg {
		return RequestTeamSelectMsg{}
	}
}

func (m *Model) handleKnowledgeCommand(parts []string) (tea.Model, tea.Cmd) {
	if m.knowledgeManager == nil {
		m.showStatus("Knowledge base not initialized")
		return m, nil
	}

	files := []string{"overview.md", "structure.md", "patterns.md", "context.md"}
	var content strings.Builder

	// Check if specific file requested
	if len(parts) > 1 && strings.HasSuffix(parts[1], ".md") {
		fileContent, err := m.knowledgeManager.GetFile(parts[1])
		if err == nil {
			content.WriteString(fmt.Sprintf("📄 %s:\n\n%s", parts[1], fileContent))
			m.viewport.SetContent(content.String())
			m.viewport.GotoTop()
			return m, nil
		}
	}

	// Show all files overview
	content.WriteString("📚 Knowledge Base Files:\n\n")

	for _, file := range files {
		fileContent, err := m.knowledgeManager.GetFile(file)
		if err != nil {
			content.WriteString(fmt.Sprintf("❌ %s: Error reading file\n", file))
		} else {
			lines := strings.Split(fileContent, "\n")
			preview := ""
			if len(lines) > 0 {
				preview = strings.TrimSpace(lines[0])
				if len(preview) > 50 {
					preview = preview[:47] + "..."
				}
			}
			content.WriteString(fmt.Sprintf("📄 %s: %s\n", file, preview))
		}
	}
	content.WriteString("\nUse /knowledge <filename> to view full content")

	m.viewport.SetContent(content.String())
	m.viewport.GotoTop()
	return m, nil
}

func (m *Model) handleCopyCommand(parts []string) (tea.Model, tea.Cmd) {
	// Default to copying last message
	target := "last"
	count := 1

	if len(parts) > 1 {
		target = parts[1]
		// Check if it's a number (e.g., /copy 10)
		if n, err := strconv.Atoi(target); err == nil {
			target = "last"
			count = n
		}
	}

	var content string
	switch target {
	case "last":
		// Copy last N messages (excluding system messages)
		var messages []string
		collected := 0
		for i := len(m.messages) - 1; i >= 0 && collected < count; i-- {
			msg := m.messages[i]
			// Skip system startup messages
			if msg.Role == "system" && (strings.Contains(msg.Content, "🚂 Loco starting") ||
				strings.Contains(msg.Content, "📁 Working directory") ||
				strings.Contains(msg.Content, "🔍 Analyzing project") ||
				strings.Contains(msg.Content, "✨ Ready to chat")) {
				continue
			}
			if msg.Content != "" {
				// Capitalize role name
				roleName := msg.Role
				if len(roleName) > 0 {
					roleName = strings.ToUpper(roleName[:1]) + roleName[1:]
				}
				messages = append([]string{fmt.Sprintf("%s: %s", roleName, msg.Content)}, messages...)
				collected++
			}
		}
		content = strings.Join(messages, "\n\n---\n\n")
	case "error":
		// Find the last error message
		for i := len(m.messages) - 1; i >= 0; i-- {
			if strings.Contains(m.messages[i].Content, "❌") || strings.Contains(m.messages[i].Content, "Failed") {
				content = m.messages[i].Content
				break
			}
		}
	case "all":
		// Copy all messages
		content = m.renderMessages()
	default:
		m.showStatus("Usage: /copy [last|error|all|N]")
		return m, nil
	}

	if content == "" {
		m.showStatus("No content to copy")
		return m, nil
	}

	// Copy to clipboard
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(content)
	if err := cmd.Run(); err != nil {
		m.showStatus("Failed to copy to clipboard")
	} else {
		// Truncate preview for status
		preview := content
		if len(preview) > 30 {
			preview = preview[:27] + "..."
		}
		m.showStatus("📋 Copied: " + preview)
	}

	return m, nil
}

func (m *Model) handleHelpCommand() (tea.Model, tea.Cmd) {
	help := `🚂 Loco Commands:
		
/debug      - Toggle debug metadata visibility
/analyze    - Re-analyze project with deep file reading
/analyze-files - Run parallel analysis on all project files
/copy       - Copy messages to clipboard (last/error/all)
/list       - List all chat sessions
/new        - Start a new chat session
/switch N   - Switch to session number N
/team       - Change your model team (S/M/L)
/knowledge  - View knowledge base files
/project    - Show project context
/reset      - Move all sessions to trash and start fresh
/screenshot - Capture UI state to file (also: Ctrl+S)
/confirm-write - Confirm a pending file write operation
/help       - Show this help message`

	m.messages = append(m.messages, llm.Message{
		Role:    "system",
		Content: help,
	})
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
	return m, nil
}

func (m *Model) handleUnknownCommand(command string) (tea.Model, tea.Cmd) {
	m.messages = append(m.messages, llm.Message{
		Role:    "system",
		Content: "Unknown command: " + command,
	})
	m.viewport.SetContent(m.renderMessages())
	return m.handleHelpCommand()
}

func (m *Model) resetProject() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	sessionsPath := filepath.Join(m.sessionManager.ProjectPath, ".loco", "sessions")
	trashPath := filepath.Join(homeDir, ".loco", "trash")

	if err := os.MkdirAll(trashPath, 0o755); err != nil {
		return err
	}

	timestamp := time.Now().Format("20060102_150405")
	projectName := filepath.Base(m.sessionManager.ProjectPath)
	trashedSessions := filepath.Join(trashPath, fmt.Sprintf("%s_sessions_%s", projectName, timestamp))

	if _, err := os.Stat(sessionsPath); err == nil {
		if err := os.Rename(sessionsPath, trashedSessions); err != nil {
			return err
		}
	}

	m.sessionManager = session.NewManager(m.sessionManager.ProjectPath)
	return m.sessionManager.Initialize()
}

// getProjectCommitHash returns the current HEAD commit hash.
func getProjectCommitHash(workingDir string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = workingDir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// indexOf returns the index of an element in a slice, or -1 if not found.
func indexOf(slice []int, value int) int {
	for i, v := range slice {
		if v == value {
			return i
		}
	}
	return -1
}

func (m *Model) handleAnalyzeFilesCommand() (tea.Model, tea.Cmd) {
	// Check if we have a small model in our team
	currentSession, err := m.sessionManager.GetCurrent()
	if err != nil || currentSession == nil || currentSession.Team == nil || currentSession.Team.Small == "" {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: "❌ No small model configured. Please select a team with /team first.",
		})
		m.viewport.SetContent(m.renderMessages())
		return m, nil
	}

	smallModel := currentSession.Team.Small
	workingDir, err := os.Getwd()
	if err != nil {
		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: "❌ Cannot determine working directory",
		})
		m.viewport.SetContent(m.renderMessages())
		return m, nil
	}

	// Show starting message
	m.messages = append(m.messages, llm.Message{
		Role:    "system",
		Content: fmt.Sprintf("🚀 Starting parallel file analysis with %s...", smallModel),
	})
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()

	// Initialize analysis state
	m.analysisState = &AnalysisState{
		IsRunning: true,
		StartTime: time.Now(),
	}
	m.viewport.SetContent(m.renderMessages())

	// Run analysis in background
	go func() {
		startTime := m.analysisState.StartTime

		// Create progress channel
		progressChan := make(chan project.AnalysisProgress, 100)

		// Start progress monitor
		go func() {
			for progress := range progressChan {
				m.analysisState.TotalFiles = progress.TotalFiles
				m.analysisState.CompletedFiles = progress.CompletedFiles

				if progress.CompletedFiles == 0 {
					m.showStatus(fmt.Sprintf("🔍 Analyzing %d files...", progress.TotalFiles))
				} else {
					m.showStatus(fmt.Sprintf("📊 Analyzed %d/%d files", progress.CompletedFiles, progress.TotalFiles))
				}
			}
		}()

		// Create analyzer
		analyzer := project.NewFileAnalyzer(workingDir, smallModel)

		// Run analysis with 10 parallel workers
		analyses, err := analyzer.AnalyzeAllFiles(10, progressChan)
		close(progressChan) // Stop progress monitor

		if err != nil {
			m.analysisState.IsRunning = false
			m.analysisState.EndTime = time.Now()
			m.showStatus(fmt.Sprintf("❌ Analysis failed: %v", err))
			return
		}

		totalDuration := time.Since(startTime)

		// Count failures
		failedCount := 0
		for _, analysis := range analyses {
			if analysis.Error != nil || analysis.Summary == "Could not analyze file" {
				failedCount++
			}
		}

		// Update analysis state
		m.analysisState.IsRunning = false
		m.analysisState.EndTime = time.Now()
		m.analysisState.FailedFiles = failedCount

		// Save summary as JSON
		if err := project.SaveAnalysisJSON(workingDir, analyses, totalDuration); err != nil {
			m.showStatus(fmt.Sprintf("❌ Failed to save summary: %v", err))
			return
		}

		// Show completion message
		summaryPath := filepath.Join(workingDir, ".loco", "file_analysis.json")
		m.showStatus(fmt.Sprintf("✅ Analysis complete in %s", totalDuration))

		// Also add to messages
		successCount := len(analyses) - failedCount
		analysisMsg := fmt.Sprintf("✅ File analysis complete!\n\nAnalyzed %d files in %s\n✅ Successful: %d\n❌ Failed: %d\nResults saved to: %s\n\nView with: jq . %s | less",
			len(analyses), totalDuration, successCount, failedCount, summaryPath, summaryPath)

		m.messages = append(m.messages, llm.Message{
			Role:    "system",
			Content: analysisMsg,
		})
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

		// Now generate codebase summary with medium model
		if currentSession != nil && currentSession.Team != nil && currentSession.Team.Medium != "" {
			m.messages = append(m.messages, llm.Message{
				Role:    "system",
				Content: fmt.Sprintf("🧠 Generating codebase summary with %s...", currentSession.Team.Medium),
			})
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()

			// Update status bar to show we're generating summary
			m.showStatus("🧠 Generating codebase summary...")

			// Track summary generation time
			summaryStartTime := time.Now()

			// Read the JSON analysis to create a compact summary
			jsonData, err := os.ReadFile(summaryPath)
			if err != nil {
				m.messages = append(m.messages, llm.Message{
					Role:    "system",
					Content: fmt.Sprintf("❌ Failed to read analysis file: %v", err),
				})
				m.viewport.SetContent(m.renderMessages())
				m.viewport.GotoBottom()
			} else {
				// Create a new LM Studio client for the medium model
				mediumClient := llm.NewLMStudioClient()
				mediumClient.SetModel(currentSession.Team.Medium)

				var summaryPrompt string

				// Parse the JSON to extract just the essential information
				var analysisSummary project.AnalysisSummary
				if err := json.Unmarshal(jsonData, &analysisSummary); err == nil {
					// Create a compact summary with just file paths and summaries
					var compactSummary strings.Builder
					compactSummary.WriteString(fmt.Sprintf("Project: %s\n", analysisSummary.ProjectPath))
					compactSummary.WriteString(fmt.Sprintf("Files analyzed: %d (success: %d, failed: %d)\n\n",
						analysisSummary.TotalFiles,
						analysisSummary.AnalyzedFiles,
						analysisSummary.ErrorCount))

					// Group files by importance
					highImportance := []project.FileAnalysis{}
					mediumImportance := []project.FileAnalysis{}
					lowImportance := []project.FileAnalysis{}

					for _, file := range analysisSummary.Files {
						if file.Error == nil && file.Summary != "Could not analyze file" {
							if file.Importance >= 8 {
								highImportance = append(highImportance, file)
							} else if file.Importance >= 5 {
								mediumImportance = append(mediumImportance, file)
							} else {
								lowImportance = append(lowImportance, file)
							}
						}
					}

					// Add high importance files
					if len(highImportance) > 0 {
						compactSummary.WriteString("HIGH IMPORTANCE FILES:\n")
						for _, file := range highImportance {
							compactSummary.WriteString(fmt.Sprintf("- %s: %s\n", file.Path, file.Summary))
						}
						compactSummary.WriteString("\n")
					}

					// Add medium importance files (limit to 20)
					if len(mediumImportance) > 0 {
						compactSummary.WriteString("MEDIUM IMPORTANCE FILES:\n")
						limit := len(mediumImportance)
						if limit > 20 {
							limit = 20
						}
						for i := 0; i < limit; i++ {
							file := mediumImportance[i]
							compactSummary.WriteString(fmt.Sprintf("- %s: %s\n", file.Path, file.Summary))
						}
						if len(mediumImportance) > 20 {
							compactSummary.WriteString(fmt.Sprintf("... and %d more\n", len(mediumImportance)-20))
						}
						compactSummary.WriteString("\n")
					}

					// Just count low importance files
					if len(lowImportance) > 0 {
						compactSummary.WriteString(fmt.Sprintf("LOW IMPORTANCE FILES: %d files\n", len(lowImportance)))
					}

					// Generate summary with the compact data
					summaryPrompt = fmt.Sprintf(`Based on this file analysis of a codebase, provide a concise summary of:
1. What this project is and its main purpose
2. The key technologies and frameworks used
3. The overall architecture and structure
4. The most important files and components
5. Any notable patterns or concerns

Here's the analysis data:
%s

Please be concise but thorough, focusing on the most important aspects.`, compactSummary.String())
				} else {
					// Fallback to sending raw JSON but truncated
					jsonStr := string(jsonData)
					if len(jsonStr) > 10000 {
						jsonStr = jsonStr[:10000] + "\n... (truncated)"
					}

					// Generate summary
					summaryPrompt = fmt.Sprintf(`Based on this comprehensive file analysis of a codebase, provide a concise summary of:
1. What this project is and its main purpose
2. The key technologies and frameworks used
3. The overall architecture and structure
4. The most important files and components
5. Any notable patterns or concerns

Here's the analysis data:
%s

Please be concise but thorough, focusing on the most important aspects.`, jsonStr)
				}

				// Start a goroutine to update status periodically
				done := make(chan bool)
				go func() {
					ticker := time.NewTicker(2 * time.Second)
					defer ticker.Stop()

					for {
						select {
						case <-done:
							return
						case <-ticker.C:
							elapsed := time.Since(summaryStartTime)
							m.showStatus(fmt.Sprintf("🧠 Generating summary... %s", elapsed.Round(time.Second)))
						}
					}
				}()

				ctx := context.Background()

				// Try with progressively larger context sizes if needed
				contextSizes := []int{16384, 32768, 65536, 131072} // 16k, 32k, 64k, 128k
				var response string
				var err error

				for _, ctxSize := range contextSizes {
					opts := llm.CompleteOptions{
						Temperature: 0.7,
						MaxTokens:   2000, // Limit output to reasonable size
						ContextSize: ctxSize,
					}

					response, err = mediumClient.CompleteWithOptions(ctx, []llm.Message{
						{
							Role:    "system",
							Content: "You are a code analysis expert. Provide clear, concise summaries of codebases.",
						},
						{
							Role:    "user",
							Content: summaryPrompt,
						},
					}, opts)

					// If successful, break out of the loop
					if err == nil {
						m.messages = append(m.messages, llm.Message{
							Role:    "system",
							Content: fmt.Sprintf("📊 Using context size: %d tokens", ctxSize),
						})
						break
					}

					// Check if it's a context overflow error
					if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "overflow") {
						// Not a context error, don't retry
						break
					}

					// Log the retry
					m.messages = append(m.messages, llm.Message{
						Role:    "system",
						Content: fmt.Sprintf("⚠️ Context size %d too small, trying %d...", ctxSize, contextSizes[minInt(len(contextSizes)-1, indexOf(contextSizes, ctxSize)+1)]),
					})
					m.viewport.SetContent(m.renderMessages())
					m.viewport.GotoBottom()
				}

				// Stop the status updater
				close(done)

				if err != nil {
					summaryDuration := time.Since(summaryStartTime)
					m.messages = append(m.messages, llm.Message{
						Role:    "system",
						Content: fmt.Sprintf("❌ Failed to generate summary after %s: %v", summaryDuration.Round(time.Millisecond), err),
					})
					m.showStatus(fmt.Sprintf("❌ Summary failed in %s", summaryDuration.Round(time.Millisecond)))
				} else {
					summaryDuration := time.Since(summaryStartTime)

					// Save the summary
					summaryMdPath := filepath.Join(workingDir, ".loco", "codebase_summary.md")
					summaryContent := fmt.Sprintf(`# Codebase Summary

Generated: %s
Model: %s
Project Commit: %s
Generation Time: %s

## Summary

%s

---
*This summary was generated from analysis of %d files (%d successful, %d failed)*
`, time.Now().Format("2006-01-02 15:04:05"), currentSession.Team.Medium,
						getProjectCommitHash(workingDir), summaryDuration.Round(time.Millisecond),
						response, len(analyses), successCount, failedCount)

					if err := os.WriteFile(summaryMdPath, []byte(summaryContent), 0o644); err != nil {
						m.messages = append(m.messages, llm.Message{
							Role:    "system",
							Content: fmt.Sprintf("❌ Failed to save summary: %v", err),
						})
					} else {
						m.messages = append(m.messages, llm.Message{
							Role: "system",
							Content: fmt.Sprintf("📝 Codebase summary generated in %s!\n\nSaved to: %s",
								summaryDuration.Round(time.Millisecond), summaryMdPath),
						})
						m.showStatus(fmt.Sprintf("✅ Summary complete in %s", summaryDuration.Round(time.Millisecond)))
					}
				}

				m.viewport.SetContent(m.renderMessages())
				m.viewport.GotoBottom()
			}

			// Generate knowledge files
			m.messages = append(m.messages, llm.Message{
				Role:    "system",
				Content: "📚 Generating knowledge files...",
			})
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()

			knowledgeStartTime := time.Now()

			// Start knowledge generation status updates
			knowledgeDone := make(chan bool)
			go func() {
				ticker := time.NewTicker(2 * time.Second)
				defer ticker.Stop()

				for {
					select {
					case <-knowledgeDone:
						return
					case <-ticker.C:
						elapsed := time.Since(knowledgeStartTime)
						m.showStatus(fmt.Sprintf("📚 Generating knowledge... %s", elapsed.Round(time.Second)))
					}
				}
			}()

			// Parse the analysis summary
			var analysisSummary project.AnalysisSummary
			if err := json.Unmarshal(jsonData, &analysisSummary); err == nil {
				// Create knowledge generator
				kg := project.NewKnowledgeGenerator(workingDir, currentSession.Team.Medium, &analysisSummary)

				// Generate all knowledge files
				if err := kg.GenerateAllKnowledge(); err != nil {
					close(knowledgeDone)
					m.messages = append(m.messages, llm.Message{
						Role:    "system",
						Content: fmt.Sprintf("❌ Knowledge generation failed: %v", err),
					})
				} else {
					close(knowledgeDone)
					knowledgeDuration := time.Since(knowledgeStartTime)
					m.messages = append(m.messages, llm.Message{
						Role: "system",
						Content: fmt.Sprintf("📚 Knowledge files generated in %s!\n\nGenerated:\n- structure.md\n- patterns.md\n- context.md\n- overview.md\n\nView with: /knowledge",
							knowledgeDuration.Round(time.Millisecond)),
					})
					m.showStatus(fmt.Sprintf("✅ Knowledge generated in %s", knowledgeDuration.Round(time.Millisecond)))
				}
			} else {
				close(knowledgeDone)
				m.messages = append(m.messages, llm.Message{
					Role:    "system",
					Content: "❌ Failed to parse analysis for knowledge generation",
				})
			}

			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
		} else {
			// No medium model configured
			m.messages = append(m.messages, llm.Message{
				Role:    "system",
				Content: "ℹ️ Skipping codebase summary and knowledge generation (no medium model configured)",
			})
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
		}
	}()

	return m, nil
}
