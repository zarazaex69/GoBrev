package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// StatsManager manages chat statistics using BadgerDB
type StatsManager struct {
	db *badger.DB
}

// UserStats represents user statistics
type UserStats struct {
	UserID       int64  `json:"user_id"`
	Username     string `json:"username"`
	MessageCount int    `json:"message_count"`
	LastSeen     int64  `json:"last_seen"`
}

// MessageStats represents message statistics for a day
type MessageStats struct {
	ChatID       int64  `json:"chat_id"`
	Date         string `json:"date"` // YYYY-MM-DD format
	TotalMessages int   `json:"total_messages"`
	UserCount    int    `json:"user_count"`
}

// WordStats represents word frequency for a day
type WordStats struct {
	Word  string `json:"word"`
	Count int    `json:"count"`
}

// NewStatsManager creates a new stats manager
func NewStatsManager(db *badger.DB) *StatsManager {
	return &StatsManager{
		db: db,
	}
}

// AddMessage adds a message to statistics
func (sm *StatsManager) AddMessage(chatID, userID int64, username, text string) error {
	now := time.Now()
	date := now.Format("2006-01-02")
	
	// Clean username
	cleanUsername := strings.TrimSpace(username)
	if cleanUsername == "" {
		cleanUsername = "Anonymous"
	}
	
	// Clean text for word analysis
	cleanText := strings.ToLower(text)
	cleanText = strings.ReplaceAll(cleanText, "\n", " ")
	cleanText = strings.ReplaceAll(cleanText, "\r", " ")
	
	// Extract words (3+ characters, letters only)
	words := extractWords(cleanText)
	
	return sm.db.Update(func(txn *badger.Txn) error {
		// Update user stats
		userKey := fmt.Sprintf("stats_user_%d_%d", chatID, userID)
		var userStats UserStats
		
		item, err := txn.Get([]byte(userKey))
		if err == nil {
			// User exists, update stats
			err = item.Value(func(val []byte) error {
				return json.Unmarshal(val, &userStats)
			})
			if err != nil {
				return err
			}
			userStats.MessageCount++
			userStats.Username = cleanUsername
			userStats.LastSeen = now.Unix()
		} else {
			// New user
			userStats = UserStats{
				UserID:       userID,
				Username:     cleanUsername,
				MessageCount: 1,
				LastSeen:     now.Unix(),
			}
		}
		
		userData, err := json.Marshal(userStats)
		if err != nil {
			return err
		}
		
		if err := txn.Set([]byte(userKey), userData); err != nil {
			return err
		}
		
		// Update daily message count
		msgKey := fmt.Sprintf("stats_msg_%d_%s", chatID, date)
		var msgStats MessageStats
		
		item, err = txn.Get([]byte(msgKey))
		if err == nil {
			err = item.Value(func(val []byte) error {
				return json.Unmarshal(val, &msgStats)
			})
			if err != nil {
				return err
			}
		}
		
		msgStats.ChatID = chatID
		msgStats.Date = date
		msgStats.TotalMessages++
		
		msgData, err := json.Marshal(msgStats)
		if err != nil {
			return err
		}
		
		if err := txn.Set([]byte(msgKey), msgData); err != nil {
			return err
		}
		
		// Update word statistics
		for _, word := range words {
			wordKey := fmt.Sprintf("stats_word_%d_%s_%s", chatID, date, word)
			
			var count int
			item, err := txn.Get([]byte(wordKey))
			if err == nil {
				err = item.Value(func(val []byte) error {
					return json.Unmarshal(val, &count)
				})
				if err != nil {
					return err
				}
			}
			count++
			
			countData, err := json.Marshal(count)
			if err != nil {
				return err
			}
			
			if err := txn.Set([]byte(wordKey), countData); err != nil {
				return err
			}
		}
		
		return nil
	})
}

// GetTopUsers returns top users for a chat
func (sm *StatsManager) GetTopUsers(chatID int64, limit int, allTime bool) ([]UserStats, error) {
	var users []UserStats
	
	err := sm.db.View(func(txn *badger.Txn) error {
		prefix := fmt.Sprintf("stats_user_%d_", chatID)
		
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefix)
		
		it := txn.NewIterator(opts)
		defer it.Close()
		
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			
			err := item.Value(func(val []byte) error {
				var userStats UserStats
				if err := json.Unmarshal(val, &userStats); err != nil {
					return err
				}
				
				// Filter by time if not all time
				if !allTime {
					now := time.Now()
					startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
					if time.Unix(userStats.LastSeen, 0).Before(startOfDay) {
						return nil // Skip old messages
					}
				}
				
				users = append(users, userStats)
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
	
	// Sort by message count (simple bubble sort for small datasets)
	for i := 0; i < len(users)-1; i++ {
		for j := 0; j < len(users)-i-1; j++ {
			if users[j].MessageCount < users[j+1].MessageCount {
				users[j], users[j+1] = users[j+1], users[j]
			}
		}
	}
	
	// Limit results
	if limit > 0 && len(users) > limit {
		users = users[:limit]
	}
	
	return users, nil
}

// GetTotalMessages returns total message count for a chat
func (sm *StatsManager) GetTotalMessages(chatID int64, allTime bool) (int, error) {
	var total int
	
	if allTime {
		// Count all users' messages
		err := sm.db.View(func(txn *badger.Txn) error {
			prefix := fmt.Sprintf("stats_user_%d_", chatID)
			
			opts := badger.DefaultIteratorOptions
			opts.Prefix = []byte(prefix)
			
			it := txn.NewIterator(opts)
			defer it.Close()
			
			for it.Rewind(); it.Valid(); it.Next() {
				item := it.Item()
				
				err := item.Value(func(val []byte) error {
					var userStats UserStats
					if err := json.Unmarshal(val, &userStats); err != nil {
						return err
					}
					total += userStats.MessageCount
					return nil
				})
				
				if err != nil {
					return err
				}
			}
			
			return nil
		})
		return total, err
	} else {
		// Count today's messages
		date := time.Now().Format("2006-01-02")
		msgKey := fmt.Sprintf("stats_msg_%d_%s", chatID, date)
		
		err := sm.db.View(func(txn *badger.Txn) error {
			item, err := txn.Get([]byte(msgKey))
			if err != nil {
				if err == badger.ErrKeyNotFound {
					return nil // No messages today
				}
				return err
			}
			
			return item.Value(func(val []byte) error {
				var msgStats MessageStats
				if err := json.Unmarshal(val, &msgStats); err != nil {
					return err
				}
				total = msgStats.TotalMessages
				return nil
			})
		})
		return total, err
	}
}

// GetPopularWords returns popular words for a chat
func (sm *StatsManager) GetPopularWords(chatID int64, limit int) ([]WordStats, error) {
	var words []WordStats
	date := time.Now().Format("2006-01-02")
	
	err := sm.db.View(func(txn *badger.Txn) error {
		prefix := fmt.Sprintf("stats_word_%d_%s_", chatID, date)
		
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(prefix)
		
		it := txn.NewIterator(opts)
		defer it.Close()
		
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			
			err := item.Value(func(val []byte) error {
				var count int
				if err := json.Unmarshal(val, &count); err != nil {
					return err
				}
				
				// Extract word from key
				key := string(item.Key())
				word := strings.TrimPrefix(key, prefix)
				
				words = append(words, WordStats{
					Word:  word,
					Count: count,
				})
				
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
	
	// Sort by count
	for i := 0; i < len(words)-1; i++ {
		for j := 0; j < len(words)-i-1; j++ {
			if words[j].Count < words[j+1].Count {
				words[j], words[j+1] = words[j+1], words[j]
			}
		}
	}
	
	// Limit results
	if limit > 0 && len(words) > limit {
		words = words[:limit]
	}
	
	return words, nil
}

// CleanupOldStats removes statistics older than specified days
func (sm *StatsManager) CleanupOldStats(maxDays int) error {
	cutoff := time.Now().AddDate(0, 0, -maxDays)
	
	return sm.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte("stats_")
		
		it := txn.NewIterator(opts)
		defer it.Close()
		
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := string(item.Key())
			
			// Check if this is a date-based key
			if strings.Contains(key, "_") {
				parts := strings.Split(key, "_")
				if len(parts) >= 3 {
					// Try to parse date from key
					dateStr := parts[len(parts)-1]
					if date, err := time.Parse("2006-01-02", dateStr); err == nil {
						if date.Before(cutoff) {
							if err := txn.Delete([]byte(key)); err != nil {
								return err
							}
						}
					}
				}
			}
		}
		
		return nil
	})
}

// extractWords extracts meaningful words from text
func extractWords(text string) []string {
	// Remove punctuation and split by spaces
	words := strings.Fields(text)
	var result []string
	
	for _, word := range words {
		// Keep only words with 3+ characters and letters only
		if len(word) >= 3 && isAlpha(word) {
			result = append(result, word)
		}
	}
	
	return result
}

// isAlpha checks if string contains only letters
func isAlpha(s string) bool {
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}
