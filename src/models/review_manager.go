package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// ReviewManager manages messages for daily review generation
type ReviewManager struct {
	db *badger.DB
}

// ReviewMessage represents a message stored for review
type ReviewMessage struct {
	MessageID     string `json:"message_id"`     // Unique message identifier
	ChatID        int64  `json:"chat_id"`        // Chat where message was sent
	UserID        int64  `json:"user_id"`        // User who sent the message
	Username      string `json:"username"`       // Username of the sender
	Content       string `json:"content"`        // Message content
	Timestamp     int64  `json:"timestamp"`      // When message was sent
	UsedForReview bool   `json:"used_for_review"` // Whether message was used for review
	
	// Reply information
	ReplyToMessageID string `json:"reply_to_message_id,omitempty"` // ID of message being replied to
	ReplyToUsername  string `json:"reply_to_username,omitempty"`   // Username of original message author
	ReplyToContent   string `json:"reply_to_content,omitempty"`    // Content of original message
}

// NewReviewManager creates a new review manager
func NewReviewManager(db *badger.DB) *ReviewManager {
	return &ReviewManager{
		db: db,
	}
}

// AddMessage adds a message to the review database
func (rm *ReviewManager) AddMessage(chatID, userID int64, username, content string, replyToMessageID, replyToUsername, replyToContent string) error {
	now := time.Now()
	messageID := fmt.Sprintf("%d_%d_%d", chatID, userID, now.UnixNano())
	
	message := ReviewMessage{
		MessageID:        messageID,
		ChatID:           chatID,
		UserID:           userID,
		Username:         username,
		Content:          content,
		Timestamp:        now.Unix(),
		UsedForReview:    false,
		ReplyToMessageID: replyToMessageID,
		ReplyToUsername:  replyToUsername,
		ReplyToContent:   replyToContent,
	}
	
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal review message: %w", err)
	}
	
	key := fmt.Sprintf("review_msg_%s", messageID)
	
	return rm.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), jsonData)
	})
}

// GetUnusedMessages returns messages that haven't been used for review yet
func (rm *ReviewManager) GetUnusedMessages(chatID int64, limit int) ([]ReviewMessage, error) {
	var messages []ReviewMessage
	
	err := rm.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte("review_msg_")
		
		it := txn.NewIterator(opts)
		defer it.Close()
		
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			
			err := item.Value(func(val []byte) error {
				var message ReviewMessage
				if err := json.Unmarshal(val, &message); err != nil {
					return err
				}
				
				// Filter by chat ID and unused status
				if message.ChatID == chatID && !message.UsedForReview {
					messages = append(messages, message)
				}
				
				return nil
			})
			
			if err != nil {
				return err
			}
		}
		
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	// Sort by timestamp (oldest first - chronological order)
	for i := 0; i < len(messages)-1; i++ {
		for j := 0; j < len(messages)-i-1; j++ {
			if messages[j].Timestamp > messages[j+1].Timestamp {
				messages[j], messages[j+1] = messages[j+1], messages[j]
			}
		}
	}
	
	// Limit results
	if limit > 0 && len(messages) > limit {
		messages = messages[:limit]
	}
	
	return messages, nil
}

// GetMessagesAfterLastReview returns messages after the last review timestamp
func (rm *ReviewManager) GetMessagesAfterLastReview(chatID int64, limit int) ([]ReviewMessage, error) {
	// Get last review timestamp
	lastReviewTime, err := rm.GetLastReviewTime(chatID)
	if err != nil {
		fmt.Printf("[-] Failed to get last review time: %v, using all unused messages\n", err)
		return rm.GetUnusedMessages(chatID, 0) // 0 = no limit for first review
	}
	
	fmt.Printf("[i] Last review time for chat %d: %d (%s)\n", chatID, lastReviewTime, time.Unix(lastReviewTime, 0).Format("2006-01-02 15:04:05"))
	
	var messages []ReviewMessage
	
	err = rm.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte("review_msg_")
		
		it := txn.NewIterator(opts)
		defer it.Close()
		
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			
			err := item.Value(func(val []byte) error {
				var message ReviewMessage
				if err := json.Unmarshal(val, &message); err != nil {
					return err
				}
				
				// Filter by chat ID and timestamp after last review
				if message.ChatID == chatID {
					if message.Timestamp > lastReviewTime {
						fmt.Printf("[+] Including message from %s at %s: %s\n", 
							message.Username, 
							time.Unix(message.Timestamp, 0).Format("15:04:05"),
							message.Content[:min(50, len(message.Content))])
						fmt.Printf("[+] Including message from %s at %s: %.50s\n", message.Username, time.Unix(message.Timestamp, 0).Format("15:04:05"), message.Content)
						messages = append(messages, message)
					} else {
						fmt.Printf("[-] Skipping old message from %s at %s\n", 
							message.Username, 
							time.Unix(message.Timestamp, 0).Format("15:04:05"))
					}
				}
				
				return nil
			})
			
			if err != nil {
				return err
			}
		}
		
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	// Sort by timestamp (oldest first - chronological order)
	for i := 0; i < len(messages)-1; i++ {
		for j := 0; j < len(messages)-i-1; j++ {
			if messages[j].Timestamp > messages[j+1].Timestamp {
				messages[j], messages[j+1] = messages[j+1], messages[j]
			}
		}
	}
	
	// Limit results
	if limit > 0 && len(messages) > limit {
		messages = messages[:limit]
	}
		fmt.Printf("[i] Found %d messages after last review\n", len(messages))
	
	return messages, nil
}

// SetLastReviewTime sets the timestamp of the last review for a chat
func (rm *ReviewManager) SetLastReviewTime(chatID int64, timestamp int64) error {
	key := fmt.Sprintf("last_review_%d", chatID)
	value := fmt.Sprintf("%d", timestamp)
	
	return rm.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), []byte(value))
	})
}

// GetLastReviewTime gets the timestamp of the last review for a chat
func (rm *ReviewManager) GetLastReviewTime(chatID int64) (int64, error) {
	key := fmt.Sprintf("last_review_%d", chatID)
	var timestamp int64
	
	err := rm.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		
		return item.Value(func(val []byte) error {
			var parseErr error
			timestamp, parseErr = strconv.ParseInt(string(val), 10, 64)
			return parseErr
		})
	})
	
	return timestamp, err
}

// MarkMessagesAsUsed marks messages as used for review
func (rm *ReviewManager) MarkMessagesAsUsed(messageIDs []string) error {
	return rm.db.Update(func(txn *badger.Txn) error {
		for _, messageID := range messageIDs {
			key := fmt.Sprintf("review_msg_%s", messageID)
			
			// Get existing message
			item, err := txn.Get([]byte(key))
			if err != nil {
				continue // Skip if message not found
			}
			
			var message ReviewMessage
			err = item.Value(func(val []byte) error {
				return json.Unmarshal(val, &message)
			})
			if err != nil {
				continue
			}
			
			// Mark as used
			message.UsedForReview = true
			
			// Save back
			jsonData, err := json.Marshal(message)
			if err != nil {
				continue
			}
			
			if err := txn.Set([]byte(key), jsonData); err != nil {
				return err
			}
		}
		
		return nil
	})
}

// CleanupOldMessages removes messages older than specified days
func (rm *ReviewManager) CleanupOldMessages(maxDays int) error {
	cutoff := time.Now().AddDate(0, 0, -maxDays).Unix()
	
	return rm.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte("review_msg_")
		
		it := txn.NewIterator(opts)
		defer it.Close()
		
		var keysToDelete [][]byte
		
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()
			
			err := item.Value(func(val []byte) error {
				var message ReviewMessage
				if err := json.Unmarshal(val, &message); err != nil {
					return err
				}
				
				// Delete if older than cutoff
				if message.Timestamp < cutoff {
					keysToDelete = append(keysToDelete, append([]byte(nil), key...))
				}
				
				return nil
			})
			
			if err != nil {
				return err
			}
		}
		
		// Delete old messages
		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		
		return nil
	})
}

// GetMessageCount returns the number of unused messages for a chat
func (rm *ReviewManager) GetMessageCount(chatID int64) (int, error) {
	count := 0
	
	err := rm.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte("review_msg_")
		
		it := txn.NewIterator(opts)
		defer it.Close()
		
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			
			err := item.Value(func(val []byte) error {
				var message ReviewMessage
				if err := json.Unmarshal(val, &message); err != nil {
					return err
				}
				
				if message.ChatID == chatID && !message.UsedForReview {
					count++
				}
				
				return nil
			})
			
			if err != nil {
				return err
			}
		}
		
		return nil
	})
	
	return count, err
}
