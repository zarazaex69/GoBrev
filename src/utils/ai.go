package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"time"
)

// AIClient handles all AI operations with ZhipuAI
type AIClient struct {
	apiKey        string
	baseURL       string
	httpClient    *http.Client
	maxRetries    int
	retryDelay    time.Duration
	maxRetryDelay time.Duration
}

// ChatMessage represents a message in the conversation
type ChatMessage struct {
	Role    string `json:"role"`    // "user", "assistant", "system"
	Content string `json:"content"`
}

// ChatRequest represents the request to ZhipuAI API
type ChatRequest struct {
	Model       string        `json:"model"`       // e.g., "glm-4", "glm-4v"
	Messages    []ChatMessage `json:"messages"`
	Temperature float64      `json:"temperature,omitempty"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	TopP        float64      `json:"top_p,omitempty"`
	Stream      bool         `json:"stream,omitempty"`
	Tools       []Tool       `json:"tools,omitempty"`
	ToolChoice  string       `json:"tool_choice,omitempty"`
}

// Tool represents a function that AI can call
type Tool struct {
	Type     string     `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction defines the function details
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ChatResponse represents the response from ZhipuAI API
type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// NewAIClient creates a new AI client
func NewAIClient() (*AIClient, error) {
	apiKey := os.Getenv("ZHIPUAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ZHIPUAI_API_KEY not found in environment variables")
	}

	return &AIClient{
		apiKey:         apiKey,
		baseURL:        "https://open.bigmodel.cn/api/paas/v4/chat/completions",
		maxRetries:     3,
		retryDelay:     1 * time.Second,
		maxRetryDelay:  30 * time.Second,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

// Chat sends a chat request to ZhipuAI with retry logic
func (ai *AIClient) Chat(messages []ChatMessage, options ...ChatOption) (*ChatResponse, error) {
	req := &ChatRequest{
		Model:       "glm-4.5",
		Messages:    messages,
		Temperature: 2,
		MaxTokens:   9000,
	}
	
	// Apply options
	for _, option := range options {
		option(req)
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= ai.maxRetries; attempt++ {
		// Create new request for each attempt
		httpReq, err := http.NewRequest("POST", ai.baseURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+ai.apiKey)

		// Log attempt
		if attempt > 0 {
			fmt.Printf("[i] AI request retry attempt %d/%d\n", attempt, ai.maxRetries)
		}

		resp, err := ai.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			
			// Check if error is retryable
			if !ai.isRetryableError(err) {
				return nil, fmt.Errorf("failed to send request (non-retryable): %w", err)
			}
			
			// If this is the last attempt, return the error
			if attempt >= ai.maxRetries {
				return nil, fmt.Errorf("failed to send request after %d attempts: %w", ai.maxRetries+1, err)
			}
			
			// Calculate delay and wait
			delay := ai.calculateRetryDelay(attempt)
			fmt.Printf("[i] Retrying in %v due to error: %v\n", delay, err)
			time.Sleep(delay)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
			
			// Check if HTTP error is retryable (5xx errors)
			if resp.StatusCode >= 500 && resp.StatusCode < 600 {
				if attempt >= ai.maxRetries {
					return nil, lastErr
				}
				
				delay := ai.calculateRetryDelay(attempt)
				fmt.Printf("[i] Retrying in %v due to HTTP error %d\n", delay, resp.StatusCode)
				time.Sleep(delay)
				continue
			}
			
			// Non-retryable HTTP error
			return nil, lastErr
		}

		var chatResp ChatResponse
		if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
			lastErr = err
			
			// JSON decode errors are usually not retryable
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		// Success
		if attempt > 0 {
			fmt.Printf("[+] AI request succeeded on attempt %d\n", attempt+1)
		}
		return &chatResp, nil
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", ai.maxRetries+1, lastErr)
}

// isRetryableError checks if an error is retryable
func (ai *AIClient) isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	// Check for network errors
	if netErr, ok := err.(net.Error); ok {
		if netErr.Timeout() || netErr.Temporary() {
			return true
		}
	}
	
	// Check for DNS errors
	if dnsErr, ok := err.(*net.DNSError); ok {
		return dnsErr.Temporary() || dnsErr.Timeout()
	}
	
	// Check for connection errors
	if opErr, ok := err.(*net.OpError); ok {
		return opErr.Temporary() || opErr.Timeout()
	}
	
	// Check for HTTP client errors that might be temporary
	if err.Error() == "EOF" {
		return true
	}
	
	return false
}

// calculateRetryDelay calculates the delay for the next retry attempt
func (ai *AIClient) calculateRetryDelay(attempt int) time.Duration {
	// Exponential backoff with jitter
	delay := time.Duration(float64(ai.retryDelay) * math.Pow(2, float64(attempt)))
	if delay > ai.maxRetryDelay {
		delay = ai.maxRetryDelay
	}
	
	// Add jitter (±25%)
	delay = delay + time.Duration(float64(delay)*0.5*math.Sin(float64(time.Now().UnixNano())))
	
	return delay
}

// QuickChat is a simplified method for quick AI interactions
func (ai *AIClient) QuickChat(prompt string, options ...ChatOption) (string, error) {
	messages := []ChatMessage{
		{Role: "user", Content: prompt},
	}

	resp, err := ai.Chat(messages, options...)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from AI")
	}

	return resp.Choices[0].Message.Content, nil
}

// ChatOption represents a function that modifies chat request
type ChatOption func(*ChatRequest)

// WithModel sets the AI model
func WithModel(model string) ChatOption {
	return func(req *ChatRequest) {
		req.Model = model
	}
}

// WithTemperature sets the temperature
func WithTemperature(temp float64) ChatOption {
	return func(req *ChatRequest) {
		req.Temperature = temp
	}
}

// WithMaxTokens sets the maximum tokens
func WithMaxTokens(tokens int) ChatOption {
	return func(req *ChatRequest) {
		req.MaxTokens = tokens
	}
}

// WithTopP sets the top_p parameter
func WithTopP(topP float64) ChatOption {
	return func(req *ChatRequest) {
		req.TopP = topP
	}
}

// WithTools sets the tools for function calling
func WithTools(tools []Tool) ChatOption {
	return func(req *ChatRequest) {
		req.Tools = tools
	}
}

// WithToolChoice sets the tool choice strategy
func WithToolChoice(choice string) ChatOption {
	return func(req *ChatRequest) {
		req.ToolChoice = choice
	}
}

// WithSystemMessage adds a system message
func WithSystemMessage(content string) ChatOption {
	return func(req *ChatRequest) {
		// Add system message at the beginning
		systemMsg := ChatMessage{Role: "system", Content: content}
		req.Messages = append([]ChatMessage{systemMsg}, req.Messages...)
	}
}

// CreateTool creates a tool for function calling
func CreateTool(name, description string, parameters map[string]interface{}) Tool {
	return Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        name,
			Description: description,
			Parameters:  parameters,
		},
	}
}

// GetUsageStats returns token usage statistics
func (ai *AIClient) GetUsageStats(resp *ChatResponse) (promptTokens, completionTokens, totalTokens int) {
	if resp != nil {
		return resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens
	}
	return 0, 0, 0
}
