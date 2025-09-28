package utils

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	maxUserInputLength  = 3500
	maxHistoryMessages  = 30
	frontendVersion     = "prod-fe-1.0.57"
	defaultUserLocation = "Russia"
	defaultUserLanguage = "ru-RU"
)

var weekdaysRu = [...]string{
	"воскресенье",
	"понедельник",
	"вторник",
	"среда",
	"четверг",
	"пятница",
	"суббота",
}

// AIClient handles all AI operations with Z.ai
type AIClient struct {
	authToken     string
	baseURL       string
	httpClient    *http.Client
	maxRetries    int
	retryDelay    time.Duration
	maxRetryDelay time.Duration
	userAgent     string
	defaultModel  string
}

// ChatMessage represents a message in the conversation
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents the request to Z.ai API
type ChatRequest struct {
	Model        string        `json:"model"`
	Messages     []ChatMessage `json:"messages"`
	Temperature  float64       `json:"temperature,omitempty"`
	MaxTokens    int           `json:"max_tokens,omitempty"`
	TopP         float64       `json:"top_p,omitempty"`
	Stream       bool          `json:"stream,omitempty"`
	Tools        []Tool        `json:"tools,omitempty"`
	ToolChoice   string        `json:"tool_choice,omitempty"`
	UserName     string        `json:"-"`
	UserLocation string        `json:"-"`
}

// Tool represents a function that AI can call
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction defines the function details
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolCallFunction describes called tool metadata
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolCall represents a tool invocation returned by Z.ai
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ChoiceMessage is a single assistant or user message in completion response
type ChoiceMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ChatChoice represents one streamed completion segment
type ChatChoice struct {
	Index        int           `json:"index"`
	Message      ChoiceMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

// UsageStats contains token accounting information
type UsageStats struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResponse represents the response from Z.ai API
type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   UsageStats   `json:"usage"`
}

// ChatOption represents a function that modifies chat request
type ChatOption func(*ChatRequest)

// NewAIClient creates a new AI client
func NewAIClient() (*AIClient, error) {
	authToken := os.Getenv("ZAI_AUTH_TOKEN")
	if authToken == "" {
		return nil, fmt.Errorf("ZAI_AUTH_TOKEN not found in environment variables")
	}

	return &AIClient{
		authToken:     authToken,
		baseURL:       "https://chat.z.ai/api",
		maxRetries:    3,
		retryDelay:    1 * time.Second,
		maxRetryDelay: 30 * time.Second,
		userAgent:     "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0",
		defaultModel:  "0727-360B-API",
		httpClient: &http.Client{
			Timeout: 0,
		},
	}, nil
}

// Chat sends a chat request to Z.ai with retry logic
func (ai *AIClient) Chat(messages []ChatMessage, options ...ChatOption) (*ChatResponse, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages provided")
	}

	msgCopy := make([]ChatMessage, len(messages))
	copy(msgCopy, messages)

	req := &ChatRequest{
		Model:       ai.defaultModel,
		Messages:    trimMessages(msgCopy),
		Temperature: 0.8,
		MaxTokens:   4000,
		TopP:        0.95,
		Stream:      true,
	}

	for _, option := range options {
		option(req)
	}

	if req.Model == "" {
		req.Model = ai.defaultModel
	}

	firstUser := ""
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			firstUser = clipUserInput(msg.Content)
			break
		}
	}
	if firstUser == "" {
		firstUser = "hello"
	}

	var lastErr error
	for attempt := 0; attempt <= ai.maxRetries; attempt++ {
		chatID, err := ai.createChat(firstUser)
		if err != nil {
			lastErr = err
			if !ai.isRetryableError(err) || attempt == ai.maxRetries {
				return nil, fmt.Errorf("failed to create Z.ai chat: %w", err)
			}
			time.Sleep(ai.calculateRetryDelay(attempt))
			continue
		}

		answer, usage, err := ai.streamCompletion(chatID, req)
		if err != nil {
			lastErr = err
			if strings.Contains(err.Error(), "no content chunks received") {
				fmt.Printf("[-] AI not responding with content chunks on attempt %d, retrying...\n", attempt+1)
			} else if strings.Contains(err.Error(), "response incomplete") {
				fmt.Printf("[-] AI response incomplete on attempt %d, retrying...\n", attempt+1)
			} else if strings.Contains(err.Error(), "AI response timeout") {
				fmt.Printf("[-] AI response timeout on attempt %d, retrying...\n", attempt+1)
			}
			if !ai.isRetryableError(err) || attempt == ai.maxRetries {
				return nil, fmt.Errorf("failed to complete Z.ai chat: %w", err)
			}
			time.Sleep(ai.calculateRetryDelay(attempt))
			continue
		}

		resp := &ChatResponse{
			ID:      chatID,
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []ChatChoice{
				{
					Index: 0,
					Message: ChoiceMessage{
						Role:    "assistant",
						Content: answer,
					},
					FinishReason: "stop",
				},
			},
		}

		if usage != nil {
			resp.Usage = *usage
		}

		if attempt > 0 {
			fmt.Printf("[+] Z.ai request succeeded on attempt %d\n", attempt+1)
		}
		return resp, nil
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", ai.maxRetries+1, lastErr)
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

// WithUserContext sets user metadata for template variables
func WithUserContext(name, location string) ChatOption {
	return func(req *ChatRequest) {
		req.UserName = strings.TrimSpace(name)
		req.UserLocation = strings.TrimSpace(location)
	}
}

// WithSystemMessage adds a system message
func WithSystemMessage(content string) ChatOption {
	return func(req *ChatRequest) {
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

func (ai *AIClient) createChat(firstMessage string) (string, error) {
	firstMessage = clipUserInput(firstMessage)
	timestamp := time.Now().Unix()
	messageID := uuid.NewString()

	payload := map[string]interface{}{
		"chat": map[string]interface{}{
			"id":     "",
			"title":  "BrevX Chat",
			"models": []string{ai.defaultModel},
			"params": map[string]interface{}{},
			"history": map[string]interface{}{
				"messages": map[string]interface{}{
					messageID: map[string]interface{}{
						"id":          messageID,
						"parentId":    nil,
						"childrenIds": []string{},
						"role":        "user",
						"content":     firstMessage,
						"timestamp":   timestamp,
						"models":      []string{ai.defaultModel},
					},
				},
				"currentId": messageID,
			},
			"messages": []map[string]interface{}{
				{
					"id":          messageID,
					"parentId":    nil,
					"childrenIds": []string{},
					"role":        "user",
					"content":     firstMessage,
					"timestamp":   timestamp,
					"models":      []string{ai.defaultModel},
				},
			},
			"tags":  []string{},
			"flags": []string{},
			"features": []map[string]interface{}{
				{"type": "mcp", "server": "vibe-coding", "status": "hidden"},
				{"type": "mcp", "server": "ppt-maker", "status": "hidden"},
				{"type": "mcp", "server": "image-search", "status": "hidden"},
			},
			"enable_thinking": false,
			"timestamp":       timestamp * 1000,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal chat payload: %w", err)
	}

	req, err := http.NewRequest("POST", ai.baseURL+"/v1/chats/new", bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create chat request: %w", err)
	}

	ai.prepareHeaders(req.Header)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ai.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create chat failed with status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to decode create chat response: %w", err)
	}

	if chatResp.ID == "" {
		return "", fmt.Errorf("Z.ai returned empty chat id")
	}

	return chatResp.ID, nil
}

func (ai *AIClient) streamCompletion(chatID string, req *ChatRequest) (string, *UsageStats, error) {
	now := time.Now().In(time.FixedZone("Europe/Moscow", 3*3600))
	variables := map[string]string{
		"{{USER_NAME}}":        req.UserName,
		"{{USER_LOCATION}}":    req.UserLocation,
		"{{CURRENT_DATETIME}}": now.Format("02.01.2006 15:04:05"),
		"{{CURRENT_DATE}}":     now.Format("02.01.2006"),
		"{{CURRENT_TIME}}":     now.Format("15:04:05"),
		"{{CURRENT_WEEKDAY}}":  formatWeekdayRu(now),
		"{{CURRENT_TIMEZONE}}": "Europe/Moscow",
		"{{USER_LANGUAGE}}":    defaultUserLanguage,
	}

	if variables["{{USER_LOCATION}}"] == "" {
		variables["{{USER_LOCATION}}"] = defaultUserLocation
	}

	payload := map[string]interface{}{
		"stream":   true,
		"model":    req.Model,
		"messages": req.Messages,
		"params": map[string]interface{}{
			"temperature": req.Temperature,
			"top_p":       req.TopP,
			"max_tokens":  req.MaxTokens,
		},
		"tool_servers": []interface{}{},
		"features": map[string]interface{}{
			"image_generation": false,
			"code_interpreter": false,
			"web_search":       false,
			"auto_web_search":  false,
			"preview_mode":     true,
			"flags":            []string{},
			"features": []map[string]interface{}{
				{"type": "mcp", "server": "vibe-coding", "status": "hidden"},
				{"type": "mcp", "server": "ppt-maker", "status": "hidden"},
				{"type": "mcp", "server": "image-search", "status": "hidden"},
			},
			"enable_thinking": false,
		},
		"variables": variables,
		"chat_id":   chatID,
		"id":        uuid.NewString(),
	}

	if len(req.Tools) > 0 {
		payload["tools"] = req.Tools
	}
	if req.ToolChoice != "" {
		payload["tool_choice"] = req.ToolChoice
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal completion payload: %w", err)
	}

	httpReq, err := http.NewRequest("POST", ai.baseURL+"/chat/completions", bytes.NewReader(jsonData))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create completion request: %w", err)
	}

	ai.prepareHeaders(httpReq.Header)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "*/*")
	httpReq.Header.Set("X-FE-Version", frontendVersion)
	httpReq.Header.Set("Referer", fmt.Sprintf("https://chat.z.ai/c/%s", chatID))

	resp, err := ai.httpClient.Do(httpReq)
	if err != nil {
		return "", nil, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return "", nil, fmt.Errorf("completion failed with status %d: %s", resp.StatusCode, string(body))
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	var builder strings.Builder
	var usage *UsageStats
	
	// Channel to signal first content chunk received
	firstContentReceived := make(chan bool, 1)
	responseComplete := make(chan bool, 1)
	var streamErr error
	
	// Start reading stream in goroutine
	go func() {
		defer func() {
			responseComplete <- true
		}()
		
		firstContentChunk := true
		startTime := time.Now()
		
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				streamErr = err
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			payload := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			if payload == "[DONE]" {
				break
			}

			var chunk zaiStreamChunk
			if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
				continue
			}

			// Check if we got actual content (not just metadata)
			if chunk.Data.DeltaContent != "" {
				if firstContentChunk {
					// Signal that we received first content chunk
					select {
					case firstContentReceived <- true:
					default:
					}
					firstContentChunk = false
					fmt.Printf("[+] First AI content chunk received after %v\n", time.Since(startTime))
				}
				builder.WriteString(chunk.Data.DeltaContent)
			}

			if chunk.Data.Usage != nil {
				usage = chunk.Data.Usage
			}

			if chunk.Data.Done {
				fmt.Printf("[+] AI response marked as done\n")
				break
			}
		}
		
		// If we never got content chunks, signal timeout
		if firstContentChunk && time.Since(startTime) >= 3*time.Second {
			streamErr = fmt.Errorf("AI response timeout: no content chunks received within 3 seconds")
		}
	}()
	
	// Wait for first content chunk or timeout
	select {
	case <-firstContentReceived:
		// First content chunk received, now wait for completion with longer timeout
		fmt.Printf("[i] Waiting for AI response completion...\n")
		select {
		case <-responseComplete:
			if streamErr != nil {
				return "", nil, streamErr
			}
		case <-time.After(30 * time.Second):
			// Timeout waiting for completion
			return "", nil, fmt.Errorf("AI response timeout: response incomplete after 30 seconds")
		}
	case <-time.After(3 * time.Second):
		// Timeout - no content chunks in 3 seconds
		return "", nil, fmt.Errorf("AI response timeout: no content chunks received within 3 seconds")
	}

	return cleanResponse(builder.String()), usage, nil
}

func (ai *AIClient) prepareHeaders(headers http.Header) {
	headers.Set("Authorization", "Bearer "+ai.authToken)
	headers.Set("User-Agent", ai.userAgent)
	headers.Set("Origin", "https://chat.z.ai")
}

// isRetryableError checks if an error is retryable
func (ai *AIClient) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	if netErr, ok := err.(net.Error); ok {
		if netErr.Timeout() || netErr.Temporary() {
			return true
		}
	}

	if dnsErr, ok := err.(*net.DNSError); ok {
		return dnsErr.Temporary() || dnsErr.Timeout()
	}

	if opErr, ok := err.(*net.OpError); ok {
		return opErr.Temporary() || opErr.Timeout()
	}

	if err.Error() == "EOF" {
		return true
	}
	
	// Check for AI response timeout (no content chunks or incomplete response)
	if strings.Contains(err.Error(), "AI response timeout") || 
	   strings.Contains(err.Error(), "no content chunks received") ||
	   strings.Contains(err.Error(), "response incomplete") {
		return true
	}

	return false
}

// calculateRetryDelay calculates the delay for the next retry attempt
func (ai *AIClient) calculateRetryDelay(attempt int) time.Duration {
	delay := time.Duration(float64(ai.retryDelay) * math.Pow(2, float64(attempt)))
	if delay > ai.maxRetryDelay {
		delay = ai.maxRetryDelay
	}

	delay = delay + time.Duration(float64(delay)*0.5*math.Sin(float64(time.Now().UnixNano())))

	return delay
}

func cleanResponse(text string) string {
	if text == "" {
		return ""
	}

	cleaned := strings.ReplaceAll(text, "\r\n", "\n")
	cleaned = strings.TrimSpace(cleaned)

	fenceCount := strings.Count(cleaned, "```")
	if fenceCount%2 == 1 {
		cleaned += "\n```"
	}

	for strings.Contains(cleaned, "\n\n\n\n") {
		cleaned = strings.ReplaceAll(cleaned, "\n\n\n\n", "\n\n\n")
	}

	return cleaned
}

func clipUserInput(text string) string {
	trimmed := strings.TrimSpace(text)
	runes := []rune(trimmed)
	if len(runes) > maxUserInputLength {
		return string(runes[:maxUserInputLength]) + "…"
	}
	return trimmed
}

func trimMessages(messages []ChatMessage) []ChatMessage {
	if len(messages) <= maxHistoryMessages {
		return messages
	}

	var system []ChatMessage
	var rest []ChatMessage
	for _, msg := range messages {
		if msg.Role == "system" && len(system) == 0 {
			system = append(system, msg)
			continue
		}
		rest = append(rest, msg)
	}

	limit := maxHistoryMessages
	if len(system) > 0 {
		limit--
		if limit < 0 {
			limit = 0
		}
	}

	if len(rest) > limit {
		rest = rest[len(rest)-limit:]
	}

	return append(system, rest...)
}

func formatWeekdayRu(t time.Time) string {
	weekday := int(t.Weekday())
	if weekday < 0 || weekday >= len(weekdaysRu) {
		return weekdaysRu[0]
	}
	return weekdaysRu[weekday]
}

type zaiStreamChunk struct {
	Type string       `json:"type"`
	Data zaiChunkData `json:"data"`
}

type zaiChunkData struct {
	DeltaContent string      `json:"delta_content"`
	Phase        string      `json:"phase"`
	Done         bool        `json:"done"`
	Usage        *UsageStats `json:"usage"`
}
