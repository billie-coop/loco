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
	// This is a temporary solution - should be moved to a separate type
	ToolExecution *ToolExecution `json:"tool_execution,omitempty"`
}

// ToolExecution represents tool execution details
type ToolExecution struct {
	Name     string `json:"name"`
	Status   string `json:"status"` // pending, running, complete, error
	Progress string `json:"progress,omitempty"`
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
	client      *http.Client
	baseURL     string
	model       string
	contextSize int // default n_ctx to send
	numKeep     int // default n_keep to send
}

// NewLMStudioClient creates a new LM Studio client.
func NewLMStudioClient() *LMStudioClient {
	return &LMStudioClient{
		baseURL:     "http://localhost:1234",
		model:       "", // Will use whatever model is loaded
		client:      &http.Client{},
		contextSize: 8192,
		numKeep:     0,
	}
}

// SetContextSize sets the default context window (n_ctx) to request.
func (c *LMStudioClient) SetContextSize(n int) { c.contextSize = n }

// SetNumKeep sets the number of tokens to keep from the initial prompt.
func (c *LMStudioClient) SetNumKeep(n int) { c.numKeep = n }

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
		ContextSize: 0, // 0 means use client default
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
		payload["model"] = c.model
	}

	// Add context window (LM Studio / llama.cpp style) only when set
	if opts.ContextSize > 0 {
		payload["n_ctx"] = opts.ContextSize
	} else if c.contextSize > 0 {
		payload["n_ctx"] = c.contextSize
	}
	// Include n_keep only when > 0
	if c.numKeep > 0 {
		payload["n_keep"] = c.numKeep
	}

	body, _ := json.Marshal(payload)

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
		data, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LM Studio returned status %d: %s", resp.StatusCode, string(data))
	}

	var result struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", errors.New("no choices returned")
	}

	return result.Choices[0].Message.Content, nil
}

// Stream streams the response from the LLM.
func (c *LMStudioClient) Stream(ctx context.Context, messages []Message, onChunk func(string)) error {
	payload := map[string]interface{}{
		"messages":    messages,
		"temperature": 0.7,
		"max_tokens":  -1,
		"stream":      true,
	}
	if c.model != "" {
		payload["model"] = c.model
	}
	if c.contextSize > 0 {
		payload["n_ctx"] = c.contextSize
	}
	if c.numKeep > 0 {
		payload["n_keep"] = c.numKeep
	}

	body, _ := json.Marshal(payload)

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
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("LM Studio returned status %d: %s", resp.StatusCode, string(data))
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if len(line) == 0 {
			continue
		}
		onChunk(string(line))
	}

	return nil
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

// CurrentModel returns the currently set model ID.
func (c *LMStudioClient) CurrentModel() string { return c.model }

// CheckEmbeddingModel checks if an embedding model is available in LM Studio.
func (c *LMStudioClient) CheckEmbeddingModel(modelID string) error {
	models, err := c.GetModels()
	if err != nil {
		return fmt.Errorf("failed to get models: %w", err)
	}
	
	// Check if the model is available
	for _, model := range models {
		if model.ID == modelID {
			// Found the model, it's available
			return nil
		}
	}
	
	// Model not found, return helpful error
	var embedModels []string
	for _, model := range models {
		// Check if it's an embedding model (contains "embed" in the name)
		if containsEmbedding(model.ID) {
			embedModels = append(embedModels, model.ID)
		}
	}
	
	if len(embedModels) > 0 {
		return fmt.Errorf("embedding model %s not loaded in LM Studio. Available embedding models: %v", modelID, embedModels)
	}
	return fmt.Errorf("embedding model %s not loaded in LM Studio. No embedding models found. Please load an embedding model like text-embedding-nomic-embed-text-v1.5", modelID)
}

// containsEmbedding checks if a model ID contains embedding-related keywords
func containsEmbedding(modelID string) bool {
	embedKeywords := []string{"embed", "embedding", "e5", "bge", "gte"}
	lower := modelID // Already lowercase from model detection
	for _, keyword := range embedKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}
