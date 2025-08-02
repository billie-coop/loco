package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/billie-coop/loco/internal/llm"
)

// ModelTeam represents the S/M/L model configuration for a session.
type ModelTeam struct {
	Name   string `json:"name"`
	Small  string `json:"small"`
	Medium string `json:"medium"`
	Large  string `json:"large"`
}

// Session represents a chat session.
type Session struct {
	Created     time.Time     `json:"created"`
	LastUpdated time.Time     `json:"last_updated"`
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Team        *ModelTeam    `json:"team"`
	Messages    []llm.Message `json:"messages"`
}

// Manager handles multiple chat sessions.
type Manager struct {
	sessions     map[string]*Session
	ProjectPath  string
	sessionsPath string
	currentID    string
}

// NewManager creates a new session manager.
func NewManager(projectPath string) *Manager {
	return &Manager{
		ProjectPath:  projectPath,
		sessionsPath: filepath.Join(projectPath, ".loco", "sessions"),
		sessions:     make(map[string]*Session),
	}
}

// Initialize sets up the session directory and loads existing sessions.
func (m *Manager) Initialize() error {
	// Create sessions directory if it doesn't exist
	if err := os.MkdirAll(m.sessionsPath, 0o755); err != nil {
		return fmt.Errorf("failed to create sessions directory: %w", err)
	}

	// Load existing sessions
	return m.loadSessions()
}

// NewSession creates a new chat session.
func (m *Manager) NewSession(model string) (*Session, error) {
	session := &Session{
		ID:          m.generateID(),
		Title:       "New Chat",
		Messages:    []llm.Message{},
		Created:     time.Now(),
		LastUpdated: time.Now(),
	}

	// Add to map
	m.sessions[session.ID] = session
	m.currentID = session.ID

	// Save immediately
	if err := m.saveSession(session); err != nil {
		return nil, err
	}

	return session, nil
}

// GetCurrent returns the current session.
func (m *Manager) GetCurrent() (*Session, error) {
	if m.currentID == "" {
		// No current session, create a new one
		return m.NewSession("")
	}

	session, exists := m.sessions[m.currentID]
	if !exists {
		return nil, fmt.Errorf("current session not found: %s", m.currentID)
	}

	return session, nil
}

// GetSession returns a specific session by ID.
func (m *Manager) GetSession(id string) (*Session, error) {
	session, exists := m.sessions[id]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	return session, nil
}

// SetCurrent sets the current session.
func (m *Manager) SetCurrent(id string) error {
	if _, exists := m.sessions[id]; !exists {
		return fmt.Errorf("session not found: %s", id)
	}
	m.currentID = id
	return nil
}

// ListSessions returns all sessions sorted by last updated.
func (m *Manager) ListSessions() []*Session {
	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}

	// Sort by last updated (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastUpdated.After(sessions[j].LastUpdated)
	})

	return sessions
}

// AddMessage adds a message to the current session.
func (m *Manager) AddMessage(msg llm.Message) error {
	session, err := m.GetCurrent()
	if err != nil {
		return err
	}

	session.Messages = append(session.Messages, msg)
	session.LastUpdated = time.Now()

	// Update title based on first user message if still "New Chat"
	if session.Title == "New Chat" && msg.Role == "user" && len(session.Messages) <= 2 {
		session.Title = m.generateTitle(msg.Content)
	}

	return m.saveSession(session)
}

// UpdateCurrentMessages replaces all messages in the current session.
func (m *Manager) UpdateCurrentMessages(messages []llm.Message) error {
	session, err := m.GetCurrent()
	if err != nil {
		return err
	}

	session.Messages = messages
	session.LastUpdated = time.Now()

	// Update title if needed
	if session.Title == "New Chat" && len(messages) > 0 {
		for _, msg := range messages {
			if msg.Role == "user" {
				session.Title = m.generateTitle(msg.Content)
				break
			}
		}
	}

	return m.saveSession(session)
}

// DeleteSession removes a session.
func (m *Manager) DeleteSession(id string) error {
	if id == m.currentID {
		return errors.New("cannot delete current session")
	}

	delete(m.sessions, id)

	// Remove file
	sessionPath := filepath.Join(m.sessionsPath, id+".json")
	return os.Remove(sessionPath)
}

// Private methods

func (m *Manager) loadSessions() error {
	entries, err := os.ReadDir(m.sessionsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No sessions yet
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		sessionPath := filepath.Join(m.sessionsPath, entry.Name())
		data, err := os.ReadFile(sessionPath)
		if err != nil {
			continue // Skip bad files
		}

		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue // Skip bad JSON
		}

		m.sessions[session.ID] = &session
	}

	// Set current to most recent if not set
	if m.currentID == "" && len(m.sessions) > 0 {
		sessions := m.ListSessions()
		m.currentID = sessions[0].ID
	}

	return nil
}

func (m *Manager) saveSession(session *Session) error {
	sessionPath := filepath.Join(m.sessionsPath, session.ID+".json")

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(sessionPath, data, 0o644)
}

func (m *Manager) generateID() string {
	// Simple timestamp-based ID
	return fmt.Sprintf("chat_%d", time.Now().Unix())
}

func (m *Manager) generateTitle(firstMessage string) string {
	// Take first 50 chars of the message as title
	title := strings.TrimSpace(firstMessage)
	if len(title) > 50 {
		title = title[:47] + "..."
	}

	// Clean up newlines
	title = strings.ReplaceAll(title, "\n", " ")

	return title
}

// GetPredefinedTeams returns a list of predefined model teams
func GetPredefinedTeams() []*ModelTeam {
	return []*ModelTeam{
		{
			Name:   "Qwen Team",
			Small:  "qwen2.5-coder:1.5b",
			Medium: "qwen2.5-coder:7b",
			Large:  "qwen2.5-coder:32b",
		},
		{
			Name:   "DeepSeek Team",
			Small:  "deepseek-coder-v2:16b",
			Medium: "deepseek-coder-v2:16b",
			Large:  "deepseek-coder-v2:236b",
		},
		{
			Name:   "Llama Team",
			Small:  "llama3.2:3b",
			Medium: "llama3.1:8b",
			Large:  "llama3.1:70b",
		},
		{
			Name:   "Mistral Team",
			Small:  "mistral:7b",
			Medium: "mixtral:8x7b",
			Large:  "mixtral:8x22b",
		},
	}
}
