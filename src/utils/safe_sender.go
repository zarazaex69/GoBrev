package utils

import (
	"fmt"
	"gopkg.in/telebot.v3"
)

// SafeSender handles safe message sending with UTF-8 validation
type SafeSender struct {
	utf8Validator *UTF8Validator
}

// NewSafeSender creates a new safe sender
func NewSafeSender() *SafeSender {
	return &SafeSender{
		utf8Validator: NewUTF8Validator(),
	}
}

// SafeSend safely sends a message with UTF-8 validation
func (s *SafeSender) SafeSend(c telebot.Context, text string, options ...*telebot.SendOptions) error {
	// Sanitize text for Telegram
	sanitizedText := s.utf8Validator.SanitizeForTelegram(text)
	
	var opts *telebot.SendOptions
	if len(options) > 0 {
		opts = options[0]
	}
	
	return c.Send(sanitizedText, opts)
}

// SafeBotSend safely sends a message using bot.Send with UTF-8 validation
func (s *SafeSender) SafeBotSend(bot *telebot.Bot, to telebot.Recipient, text string, options ...*telebot.SendOptions) (*telebot.Message, error) {
	// Sanitize text for Telegram
	sanitizedText := s.utf8Validator.SanitizeForTelegram(text)
	
	var opts *telebot.SendOptions
	if len(options) > 0 {
		opts = options[0]
	}
	
	return bot.Send(to, sanitizedText, opts)
}

// SafeEdit safely edits a message with UTF-8 validation
func (s *SafeSender) SafeEdit(bot *telebot.Bot, message *telebot.Message, text string, options ...*telebot.SendOptions) (*telebot.Message, error) {
	// Sanitize text for Telegram
	sanitizedText := s.utf8Validator.SanitizeForTelegram(text)
	
	var opts *telebot.SendOptions
	if len(options) > 0 {
		opts = options[0]
	}
	
	return bot.Edit(message, sanitizedText, opts)
}

// SafeSendPhoto safely sends a photo with UTF-8 validation for caption
func (s *SafeSender) SafeSendPhoto(c telebot.Context, photo *telebot.Photo, options ...*telebot.SendOptions) error {
	if photo.Caption != "" {
		// Sanitize caption for Telegram
		photo.Caption = s.utf8Validator.SanitizeForTelegram(photo.Caption)
	}
	
	var opts *telebot.SendOptions
	if len(options) > 0 {
		opts = options[0]
	}
	
	return c.Send(photo, opts)
}

// ValidateAndLog validates text and logs any issues
func (s *SafeSender) ValidateAndLog(text string, context string) string {
	if !s.utf8Validator.ValidateUTF8(text) {
		positions := s.utf8Validator.GetInvalidUTF8Positions(text)
		fmt.Printf("[-] Invalid UTF-8 detected in %s at positions: %v\n", context, positions)
	}
	
	return s.utf8Validator.SanitizeForTelegram(text)
}
