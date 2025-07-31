package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Client interface for LLM operations
type Client interface {
	Complete(ctx context.Context, messages []Message) (string, error)
	Stream(ctx context.Context, messages []Message, onChunk func(string)) error
}

// LMStudioClient implements the Client interface for LM Studio
type LMStudioClient struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewLMStudioClient creates a new LM Studio client
func NewLMStudioClient() *LMStudioClient {
	return &LMStudioClient{
		baseURL: "http://localhost:1234",
		model:   "", // Will use whatever model is loaded
		client:  &http.Client{},
	}
}

// Complete sends messages and returns the full response
func (c *LMStudioClient) Complete(ctx context.Context, messages []Message) (string, error) {
	payload := map[string]interface{}{
		"messages":    messages,
		"temperature": 0.7,
		"max_tokens":  -1,
		"stream":      false,
	}
	
	// Add model if specified
	if c.model != "" {
		payload["model"] = c.model
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
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
		body, _ := io.ReadAll(resp.Body)
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

	return "", fmt.Errorf("no response from LM Studio")
}

// Stream sends messages and streams the response
func (c *LMStudioClient) Stream(ctx context.Context, messages []Message, onChunk func(string)) error {
	payload := map[string]interface{}{
		"messages":    messages,
		"temperature": 0.7,
		"max_tokens":  -1,
		"stream":      true,
	}
	
	// Add model if specified
	if c.model != "" {
		payload["model"] = c.model
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
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
		body, _ := io.ReadAll(resp.Body)
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

// Model represents an available model in LM Studio
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelsResponse represents the response from /v1/models
type ModelsResponse struct {
	Data []Model `json:"data"`
}

// HealthCheck checks if LM Studio is running
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

// GetModels returns the list of available models
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
	
	return modelsResp.Data, nil
}

// SetModel sets the model to use for completions
func (c *LMStudioClient) SetModel(modelID string) {
	c.model = modelID
}