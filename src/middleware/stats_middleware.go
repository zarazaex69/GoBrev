package middleware

import (
	"fmt"
	"strings"

	"gopkg.in/telebot.v3"
	"gobrev/src/models"
)

// StatsMiddleware handles message statistics
type StatsMiddleware struct {
	statsManager *models.StatsManager
}

// NewStatsMiddleware creates a new stats middleware
func NewStatsMiddleware(statsManager *models.StatsManager) *StatsMiddleware {
	return &StatsMiddleware{
		statsManager: statsManager,
	}
}

// HandleMessage processes incoming messages for statistics
func (sm *StatsMiddleware) HandleMessage(c telebot.Context) error {
	// Only process text messages
	if c.Text() == "" {
		return nil
	}
	
	// Skip bot messages
	if c.Sender().IsBot {
		return nil
	}
	
	// Skip commands
	text := strings.TrimSpace(c.Text())
	if strings.HasPrefix(text, "/") || strings.HasPrefix(text, ".") {
		return nil
	}
	
	// Get user info
	user := c.Sender()
	chatID := c.Chat().ID
	userID := user.ID
	
	// Build username
	username := user.FirstName
	if user.LastName != "" {
		username += " " + user.LastName
	}
	if username == "" {
		username = user.Username
	}
	if username == "" {
		username = "Anonymous"
	}
	
	// Add message to statistics
	err := sm.statsManager.AddMessage(chatID, userID, username, text)
	if err != nil {
		fmt.Printf("[-] Failed to add message to stats: %v\n", err)
		// Don't return error to avoid breaking the bot
	}
	
	return nil
}

// SetupStatsMiddleware sets up the stats middleware
func SetupStatsMiddleware(bot *telebot.Bot, statsManager *models.StatsManager) {
	statsMiddleware := NewStatsMiddleware(statsManager)
	
	// Register handler for all text messages
	bot.Handle(telebot.OnText, func(c telebot.Context) error {
		// Process for statistics
		statsMiddleware.HandleMessage(c)
		
		// Continue with other handlers
		return nil
	})
	
	fmt.Printf("[+] Stats middleware registered\n")
}
