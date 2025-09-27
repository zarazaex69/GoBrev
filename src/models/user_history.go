package models

import (
	"sync"
	"time"
)

// UserMessage represents a single message in user's history
type UserMessage struct {
	Role      string    `json:"role"`      // "user" or "assistant"
	Content   string    `json:"content"`   // Message content
	Timestamp time.Time `json:"timestamp"` // When message was created
}

// UserHistory holds conversation history for a single user
type UserHistory struct {
	UserID    int64         `json:"user_id"`    // Telegram user ID
	Messages  []UserMessage `json:"messages"`   // Conversation history
	MaxSize   int           `json:"max_size"`   // Maximum number of messages to keep
	mu        sync.RWMutex  `json:"-"`          // Mutex for thread safety
}

// NewUserHistory creates a new user history with default max size of 12
func NewUserHistory(userID int64) *UserHistory {
	return &UserHistory{
		UserID:   userID,
		Messages: make([]UserMessage, 0),
		MaxSize:  12, // Keep last 12 messages (6 exchanges)
	}
}

// AddMessage adds a new message to user's history
func (uh *UserHistory) AddMessage(role, content string) {
	uh.mu.Lock()
	defer uh.mu.Unlock()
	
	message := UserMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
	
	// Add new message
	uh.Messages = append(uh.Messages, message)
	
	// Trim to max size if needed
	if len(uh.Messages) > uh.MaxSize {
		uh.Messages = uh.Messages[len(uh.Messages)-uh.MaxSize:]
	}
}

// GetMessages returns a copy of user's message history
func (uh *UserHistory) GetMessages() []UserMessage {
	uh.mu.RLock()
	defer uh.mu.RUnlock()
	
	// Return a copy to prevent external modifications
	messages := make([]UserMessage, len(uh.Messages))
	copy(messages, uh.Messages)
	return messages
}

// GetLastMessages returns the last N messages from user's history
func (uh *UserHistory) GetLastMessages(count int) []UserMessage {
	uh.mu.RLock()
	defer uh.mu.RUnlock()
	
	if count <= 0 || len(uh.Messages) == 0 {
		return []UserMessage{}
	}
	
	if count >= len(uh.Messages) {
		// Return all messages
		messages := make([]UserMessage, len(uh.Messages))
		copy(messages, uh.Messages)
		return messages
	}
	
	// Return last N messages
	start := len(uh.Messages) - count
	messages := make([]UserMessage, count)
	copy(messages, uh.Messages[start:])
	return messages
}

// ClearHistory clears all messages from user's history
func (uh *UserHistory) ClearHistory() {
	uh.mu.Lock()
	defer uh.mu.Unlock()
	
	uh.Messages = make([]UserMessage, 0)
}

// GetMessageCount returns the number of messages in user's history
func (uh *UserHistory) GetMessageCount() int {
	uh.mu.RLock()
	defer uh.mu.RUnlock()
	
	return len(uh.Messages)
}

// SetMaxSize sets the maximum number of messages to keep
func (uh *UserHistory) SetMaxSize(size int) {
	uh.mu.Lock()
	defer uh.mu.Unlock()
	
	uh.MaxSize = size
	
	// Trim existing messages if needed
	if len(uh.Messages) > uh.MaxSize {
		uh.Messages = uh.Messages[len(uh.Messages)-uh.MaxSize:]
	}
}
