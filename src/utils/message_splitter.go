package utils

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"gopkg.in/telebot.v3"
)

const (
	// Telegram message limits
	MaxMessageLength = 4096  // Maximum message length for Telegram
	MaxCaptionLength = 1024  // Maximum caption length for photos
	SafeMessageLength = 4000 // Safe length with some buffer
	SafeCaptionLength = 1000 // Safe caption length with buffer
)

// MessageSplitter handles splitting long messages for Telegram
type MessageSplitter struct{}

// NewMessageSplitter creates a new message splitter
func NewMessageSplitter() *MessageSplitter {
	return &MessageSplitter{}
}

// SplitMessage splits a long message into multiple parts if needed
func (ms *MessageSplitter) SplitMessage(text string, maxLength int) []string {
	if maxLength <= 0 {
		maxLength = SafeMessageLength
	}

	// If message is short enough, return as is
	if utf8.RuneCountInString(text) <= maxLength {
		return []string{text}
	}

	var parts []string
	runes := []rune(text)
	
	for len(runes) > 0 {
		// Determine the split point
		splitPoint := maxLength
		if len(runes) < splitPoint {
			splitPoint = len(runes)
		}

		// Try to find a good break point (space, newline, etc.)
		if splitPoint < len(runes) {
			// Look for natural break points within the last 200 characters
			searchStart := splitPoint - 200
			if searchStart < 0 {
				searchStart = 0
			}

			bestBreak := -1
			for i := splitPoint - 1; i >= searchStart; i-- {
				char := runes[i]
				if char == '\n' {
					bestBreak = i + 1
					break
				} else if char == ' ' || char == '.' || char == '!' || char == '?' {
					bestBreak = i + 1
				}
			}

			if bestBreak > 0 {
				splitPoint = bestBreak
			}
		}

		// Extract the part
		part := string(runes[:splitPoint])
		parts = append(parts, strings.TrimSpace(part))

		// Remove the processed part
		runes = runes[splitPoint:]
	}

	return parts
}

// SendLongMessage sends a message, splitting it if necessary
func (ms *MessageSplitter) SendLongMessage(c telebot.Context, text string, options *telebot.SendOptions) error {
	parts := ms.SplitMessage(text, SafeMessageLength)
	
	for i, part := range parts {
		if i > 0 {
			// Add part indicator for subsequent messages
			part = fmt.Sprintf("(%d/%d) %s", i+1, len(parts), part)
		}
		
		err := c.Send(part, options)
		if err != nil {
			return fmt.Errorf("failed to send message part %d: %w", i+1, err)
		}
	}
	
	return nil
}

// EditLongMessage edits a message, handling length limits
func (ms *MessageSplitter) EditLongMessage(bot *telebot.Bot, message *telebot.Message, text string, options *telebot.SendOptions) error {
	// If the message is short enough, just edit it
	if utf8.RuneCountInString(text) <= SafeMessageLength {
		_, err := bot.Edit(message, text, options)
		return err
	}

	// If too long, edit with truncated version and send continuation
	parts := ms.SplitMessage(text, SafeMessageLength)
	
	if len(parts) == 0 {
		return fmt.Errorf("no parts to send")
	}

	// Edit original message with first part
	firstPart := parts[0]
	if len(parts) > 1 {
		firstPart += fmt.Sprintf("\n\n<i>(1/%d) Продолжение следует...</i>", len(parts))
	}
	
	_, err := bot.Edit(message, firstPart, options)
	if err != nil {
		return fmt.Errorf("failed to edit original message: %w", err)
	}

	// Send remaining parts as new messages
	for i := 1; i < len(parts); i++ {
		part := fmt.Sprintf("<i>(%d/%d)</i>\n\n%s", i+1, len(parts), parts[i])
		
		_, err := bot.Send(message.Chat, part, &telebot.SendOptions{
			ParseMode: options.ParseMode,
			ReplyTo:   message,
		})
		if err != nil {
			return fmt.Errorf("failed to send continuation part %d: %w", i+1, err)
		}
	}

	return nil
}

// TruncateMessage truncates a message to fit within limits
func (ms *MessageSplitter) TruncateMessage(text string, maxLength int) string {
	if maxLength <= 0 {
		maxLength = SafeMessageLength
	}

	runes := []rune(text)
	if len(runes) <= maxLength {
		return text
	}

	// Truncate and add ellipsis
	truncated := string(runes[:maxLength-3]) + "..."
	return truncated
}

// ValidateMessageLength checks if a message exceeds Telegram limits
func (ms *MessageSplitter) ValidateMessageLength(text string) (bool, int) {
	length := utf8.RuneCountInString(text)
	return length <= MaxMessageLength, length
}

// ValidateCaptionLength checks if a caption exceeds Telegram limits
func (ms *MessageSplitter) ValidateCaptionLength(text string) (bool, int) {
	length := utf8.RuneCountInString(text)
	return length <= MaxCaptionLength, length
}

// CleanAndTruncate cleans text and truncates if necessary
func (ms *MessageSplitter) CleanAndTruncate(text string, maxLength int) string {
	// Clean the text
	cleaned := strings.TrimSpace(text)
	cleaned = strings.ReplaceAll(cleaned, "\r\n", "\n")
	
	// Remove excessive newlines
	for strings.Contains(cleaned, "\n\n\n") {
		cleaned = strings.ReplaceAll(cleaned, "\n\n\n", "\n\n")
	}
	
	// Truncate if needed
	return ms.TruncateMessage(cleaned, maxLength)
}
