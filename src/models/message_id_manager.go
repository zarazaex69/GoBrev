package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// MessageIDManager manages message IDs for AI responses
type MessageIDManager struct {
	db *badger.DB
}

// MessageIDData represents data stored for a message ID
type MessageIDData struct {
	MessageID   int    `json:"message_id"`   // Telegram message ID
	UserID      int64  `json:"user_id"`      // User who received the message
	ChatID      int64  `json:"chat_id"`      // Chat where message was sent
	Timestamp   int64  `json:"timestamp"`   // When message was sent
	Content    string `json:"content"`      // Message content (for debugging)
}

// NewMessageIDManager creates a new message ID manager
func NewMessageIDManager(dbPath string) (*MessageIDManager, error) {
	// Open BadgerDB
	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil // Disable logging
	
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB: %w", err)
	}
	
	return &MessageIDManager{
		db: db,
	}, nil
}

// Close closes the database connection
func (mim *MessageIDManager) Close() error {
	if mim.db != nil {
		return mim.db.Close()
	}
	return nil
}

// StoreMessageID stores a message ID for an AI response
func (mim *MessageIDManager) StoreMessageID(messageID int, userID, chatID int64, content string) error {
	data := MessageIDData{
		MessageID:  messageID,
		UserID:     userID,
		ChatID:     chatID,
		Timestamp:  time.Now().Unix(),
		Content:    content,
	}
	
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal message ID data: %w", err)
	}
	
	key := fmt.Sprintf("msg_%d", messageID)
	
	return mim.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), jsonData)
	})
}

// GetMessageIDData retrieves message ID data
func (mim *MessageIDManager) GetMessageIDData(messageID int) (*MessageIDData, error) {
	key := fmt.Sprintf("msg_%d", messageID)
	
	var data MessageIDData
	err := mim.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &data)
		})
	})
	
	if err != nil {
		return nil, err
	}
	
	return &data, nil
}

// IsAIMessage checks if a message ID belongs to an AI response
func (mim *MessageIDManager) IsAIMessage(messageID int) bool {
	_, err := mim.GetMessageIDData(messageID)
	return err == nil
}

// DeleteMessageID removes a message ID from storage
func (mim *MessageIDManager) DeleteMessageID(messageID int) error {
	key := fmt.Sprintf("msg_%d", messageID)
	
	return mim.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// CleanupOldMessages removes message IDs older than specified duration
func (mim *MessageIDManager) CleanupOldMessages(maxAge time.Duration) error {
	cutoff := time.Now().Add(-maxAge).Unix()
	
	return mim.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		
		it := txn.NewIterator(opts)
		defer it.Close()
		
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()
			
			// Check if this is a message ID key
			if len(key) > 4 && string(key[:4]) == "msg_" {
				err := item.Value(func(val []byte) error {
					var data MessageIDData
					if err := json.Unmarshal(val, &data); err != nil {
						return err
					}
					
					// Delete if older than cutoff
					if data.Timestamp < cutoff {
						return txn.Delete(key)
					}
					
					return nil
				})
				
				if err != nil {
					return err
				}
			}
		}
		
		return nil
	})
}

// GetMessageCount returns the number of stored message IDs
func (mim *MessageIDManager) GetMessageCount() (int, error) {
	count := 0
	
	err := mim.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		
		it := txn.NewIterator(opts)
		defer it.Close()
		
		for it.Rewind(); it.Valid(); it.Next() {
			key := it.Item().Key()
			if len(key) > 4 && string(key[:4]) == "msg_" {
				count++
			}
		}
		
		return nil
	})
	
	return count, err
}
