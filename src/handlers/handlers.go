package handlers

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/telebot.v3"
	"gobrev/src/handlers/factory"
	"gobrev/src/models"
)

// containsBrev checks if text contains "брев" in any form (case insensitive)
func containsBrev(text string) bool {
	text = strings.ToLower(text)
	
	// Check for various forms of "брев"
	brevVariants := []string{
		"брев",     // брев
		"брева",    // брева
		"бреве",    // бреве
		"бревом",   // бревом
		"бревец",   // бревец
		"бревик",   // бревик
		"бревочка", // бревочка
		"бревоныш", // бревоныш
	}
	
	for _, variant := range brevVariants {
		if strings.Contains(text, variant) {
			return true
		}
	}
	
	return false
}

// isReplyToBot checks if the message is a reply to bot's AI message
func isReplyToBot(c telebot.Context, messageIDManager *models.MessageIDManager) bool {
	// Check if message is a reply
	if c.Message().ReplyTo == nil {
		return false
	}
	
	// Get the replied message ID
	repliedMessage := c.Message().ReplyTo
	messageID := repliedMessage.ID
	
	// Check if this message ID is stored as an AI message
	return messageIDManager.IsAIMessage(messageID)
}

// SetupHandlers registers all command handlers using command factory
func SetupHandlers(bot *telebot.Bot, metrics *models.Metrics, historyManager *models.UserHistoryManager, messageIDManager *models.MessageIDManager, startTime time.Time) {
	// Create command factory
	cmdFactory := factory.NewCommandFactory(metrics, historyManager, messageIDManager, startTime)
	
	// Register each command individually
	bot.Handle("/start", func(c telebot.Context) error {
		return cmdFactory.Execute("/start", c)
	})
	
	bot.Handle("/test", func(c telebot.Context) error {
		return cmdFactory.Execute("/test", c)
	})
	
	// Register AI command with text handler
	bot.Handle(telebot.OnText, func(c telebot.Context) error {
		text := c.Text()
		
		// Check if message contains "брев" in any form
		if containsBrev(text) {
			fmt.Printf("[i] Брев detected in text: %s\n", text)
			err := cmdFactory.Execute(".ии", c)
			if err != nil {
				fmt.Printf("[-] AI command failed: %v\n", err)
			}
			return err
		}
		
		// Check if this is a reply to bot's message
		if isReplyToBot(c, cmdFactory.GetMessageIDManager()) {
			fmt.Printf("[i] Reply to bot detected: %s\n", text)
			err := cmdFactory.Execute(".ии", c)
			if err != nil {
				fmt.Printf("[-] AI command failed: %v\n", err)
			}
			return err
		}
		
		// Ignore other messages
		return nil
	})
}
