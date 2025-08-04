package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Message represents a chat message.
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	
	// For tool execution messages (role="tool")
	ToolName   string `json:"tool_name,omitempty"`
	ToolStatus string `json:"tool_status,omitempty"` // pending, running, complete, error
	ToolProgress string `json:"tool_progress,omitempty"`
}

// ToolCall represents a tool invocation by the assistant
type ToolCall struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Parameters string `json:"parameters"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Output     string `json:"output"`
	Error      error  `json:"error,omitempty"`
}

// Client interface for LLM operations.
type Client interface {
	Complete(ctx context.Context, messages []Message) (string, error)
	Stream(ctx context.Context, messages []Message, onChunk func(string)) error
}

// LMStudioClient implements the Client interface for LM Studio.
type LMStudioClient struct {
	client  *http.Client
	baseURL string
	model   string
}

// NewLMStudioClient creates a new LM Studio client.
func NewLMStudioClient() *LMStudioClient {
	return &LMStudioClient{
		baseURL: "http://localhost:1234",
		model:   "", // Will use whatever model is loaded
		client:  &http.Client{},
	}
}

// CompleteOptions contains options for completion requests.
type CompleteOptions struct {
	Temperature float64
	MaxTokens   int
	ContextSize int // n_ctx for LM Studio
}

// DefaultCompleteOptions returns default options.
func DefaultCompleteOptions() CompleteOptions {
	return CompleteOptions{
		Temperature: 0.7,
		MaxTokens:   -1,
		ContextSize: 0, // 0 means use model default
	}
}

// Complete sends messages and returns the full response.
func (c *LMStudioClient) Complete(ctx context.Context, messages []Message) (string, error) {
	return c.CompleteWithOptions(ctx, messages, DefaultCompleteOptions())
}

// CompleteWithOptions sends messages with custom options and returns the full response.
func (c *LMStudioClient) CompleteWithOptions(ctx context.Context, messages []Message, opts CompleteOptions) (string, error) {
	payload := map[string]interface{}{
		"messages":    messages,
		"temperature": opts.Temperature,
		"max_tokens":  opts.MaxTokens,
		"stream":      false,
	}

	// Add model if specified
	if c.model != "" {
		// Only log in detailed scenarios to reduce noise
		payload["model"] = c.model
	}

	// Add context size if specified (LM Studio uses n_ctx)
	if opts.ContextSize > 0 {
		payload["n_ctx"] = opts.ContextSize
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("LM Studio returned status %d but failed to read body: %w", resp.StatusCode, err)
		}
		return "", fmt.Errorf("LM Studio error: %s", string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}

	return "", errors.New("no response from LM Studio")
}

// Stream sends messages and streams the response.
func (c *LMStudioClient) Stream(ctx context.Context, messages []Message, onChunk func(string)) error {
	payload := map[string]interface{}{
		"messages":    messages,
		"temperature": 0.7,
		"max_tokens":  -1,
		"stream":      true,
	}

	// Add model if specified
	if c.model != "" {
		// Only log in detailed scenarios to reduce noise
		payload["model"] = c.model
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("LM Studio returned status %d but failed to read body: %w", resp.StatusCode, err)
		}
		return fmt.Errorf("LM Studio error: %s", string(body))
	}

	// Parse SSE stream
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue // Skip malformed chunks
			}

			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				onChunk(chunk.Choices[0].Delta.Content)
			}
		}
	}

	return scanner.Err()
}

// Model represents an available model in LM Studio.
type Model struct {
	ID      string    `json:"id"`
	Object  string    `json:"object"`
	OwnedBy string    `json:"owned_by"`
	Created int64     `json:"created"`
	Name    string    `json:"name"` // Human-friendly name
	Size    ModelSize `json:"size"` // Model size category
}

// ModelsResponse represents the response from /v1/models.
type ModelsResponse struct {
	Data []Model `json:"data"`
}

// HealthCheck checks if LM Studio is running.
func (c *LMStudioClient) HealthCheck() error {
	resp, err := c.client.Get(c.baseURL + "/v1/models")
	if err != nil {
		return fmt.Errorf("LM Studio not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LM Studio returned status %d", resp.StatusCode)
	}

	return nil
}

// GetModels returns the list of available models.
func (c *LMStudioClient) GetModels() ([]Model, error) {
	resp, err := c.client.Get(c.baseURL + "/v1/models")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LM Studio returned status %d", resp.StatusCode)
	}

	var modelsResp ModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	// Enhance models with size detection
	for i := range modelsResp.Data {
		model := &modelsResp.Data[i]
		model.Size = DetectModelSize(model.ID)
		model.Name = model.ID // Default name to ID
	}

	return modelsResp.Data, nil
}

// SetModel sets the model to use for completions.
func (c *LMStudioClient) SetModel(modelID string) {
	c.model = modelID
}

// SetEndpoint sets the base URL for the LM Studio API.
func (c *LMStudioClient) SetEndpoint(endpoint string) {
	c.baseURL = endpoint
}
