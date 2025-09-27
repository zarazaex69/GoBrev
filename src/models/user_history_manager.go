package models

import (
	"sync"
)

// UserHistoryManager manages user conversation histories
type UserHistoryManager struct {
	histories map[int64]*UserHistory // Map of user ID to their history
	mu        sync.RWMutex           // Mutex for thread safety
}

// NewUserHistoryManager creates a new user history manager
func NewUserHistoryManager() *UserHistoryManager {
	return &UserHistoryManager{
		histories: make(map[int64]*UserHistory),
	}
}

// GetUserHistory gets or creates user history for a given user ID
func (uhm *UserHistoryManager) GetUserHistory(userID int64) *UserHistory {
	uhm.mu.Lock()
	defer uhm.mu.Unlock()
	
	// Check if user history exists
	if history, exists := uhm.histories[userID]; exists {
		return history
	}
	
	// Create new history for user
	history := NewUserHistory(userID)
	uhm.histories[userID] = history
	return history
}

// AddUserMessage adds a message to user's history
func (uhm *UserHistoryManager) AddUserMessage(userID int64, role, content string) {
	history := uhm.GetUserHistory(userID)
	history.AddMessage(role, content)
}

// GetUserMessages returns user's message history
func (uhm *UserHistoryManager) GetUserMessages(userID int64) []UserMessage {
	history := uhm.GetUserHistory(userID)
	return history.GetMessages()
}

// GetUserLastMessages returns the last N messages from user's history
func (uhm *UserHistoryManager) GetUserLastMessages(userID int64, count int) []UserMessage {
	history := uhm.GetUserHistory(userID)
	return history.GetLastMessages(count)
}

// ClearUserHistory clears user's conversation history
func (uhm *UserHistoryManager) ClearUserHistory(userID int64) {
	uhm.mu.Lock()
	defer uhm.mu.Unlock()
	
	if history, exists := uhm.histories[userID]; exists {
		history.ClearHistory()
	}
}

// DeleteUserHistory completely removes user's history from memory
func (uhm *UserHistoryManager) DeleteUserHistory(userID int64) {
	uhm.mu.Lock()
	defer uhm.mu.Unlock()
	
	delete(uhm.histories, userID)
}

// GetActiveUsersCount returns the number of users with active histories
func (uhm *UserHistoryManager) GetActiveUsersCount() int {
	uhm.mu.RLock()
	defer uhm.mu.RUnlock()
	
	return len(uhm.histories)
}

// GetAllUserIDs returns all user IDs that have histories
func (uhm *UserHistoryManager) GetAllUserIDs() []int64 {
	uhm.mu.RLock()
	defer uhm.mu.RUnlock()
	
	userIDs := make([]int64, 0, len(uhm.histories))
	for userID := range uhm.histories {
		userIDs = append(userIDs, userID)
	}
	return userIDs
}

// GetUserMessageCount returns the number of messages for a specific user
func (uhm *UserHistoryManager) GetUserMessageCount(userID int64) int {
	history := uhm.GetUserHistory(userID)
	return history.GetMessageCount()
}

// SetUserMaxHistorySize sets the maximum history size for a specific user
func (uhm *UserHistoryManager) SetUserMaxHistorySize(userID int64, maxSize int) {
	history := uhm.GetUserHistory(userID)
	history.SetMaxSize(maxSize)
}
