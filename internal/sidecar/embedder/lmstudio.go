package embedder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// LMStudioEmbedder uses LM Studio's OpenAI-compatible embeddings endpoint
type LMStudioEmbedder struct {
	baseURL    string
	client     *http.Client
	dimension  int
	model      string // The embedding model to use
}

// NewLMStudioEmbedder creates a new LM Studio embedder
// Note: LM Studio needs an embedding model loaded (like nomic-embed-text)
func NewLMStudioEmbedder(baseURL string) *LMStudioEmbedder {
	if baseURL == "" {
		baseURL = "http://localhost:1234"
	}
	return &LMStudioEmbedder{
		baseURL:   baseURL,
		client:    &http.Client{},
		dimension: -1, // Will be set after first embedding
		model:     "", // Use whatever embedding model is loaded
	}
}

// embeddingRequest matches OpenAI's embedding request format
type embeddingRequest struct {
	Input string `json:"input"`
	Model string `json:"model,omitempty"`
}

// embeddingResponse matches OpenAI's embedding response format
type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// Embed generates embedding for a single text using LM Studio
func (e *LMStudioEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Prepare request
	reqBody := embeddingRequest{
		Input: text,
		Model: e.model,
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/v1/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	// Send request
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	// Check status
	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err == nil && errorResp.Error.Message != "" {
			return nil, fmt.Errorf("LM Studio error: %s", errorResp.Error.Message)
		}
		return nil, fmt.Errorf("LM Studio returned status %d", resp.StatusCode)
	}
	
	// Parse response
	var embResp embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	
	embedding := embResp.Data[0].Embedding
	
	// Update dimension if first time
	if e.dimension == -1 {
		e.dimension = len(embedding)
	}
	
	return embedding, nil
}

// EmbedBatch generates embeddings for multiple texts
func (e *LMStudioEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	// LM Studio's OpenAI endpoint might support batch, but for safety we'll do one at a time
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := e.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to embed text %d: %w", i, err)
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}

// Dimension returns the embedding dimension
func (e *LMStudioEmbedder) Dimension() int {
	if e.dimension == -1 {
		// Default to common embedding size
		return 768 // Common for many models
	}
	return e.dimension
}

// SetModel sets the embedding model to use
func (e *LMStudioEmbedder) SetModel(model string) {
	e.model = model
}