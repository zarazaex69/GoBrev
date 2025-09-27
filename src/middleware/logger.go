package middleware

import (
	"log"
	"time"

	"gopkg.in/telebot.v3"
)

// LoggerMiddleware creates logging middleware
func LoggerMiddleware() telebot.MiddlewareFunc {
	return func(next telebot.HandlerFunc) telebot.HandlerFunc {
		return func(c telebot.Context) error {
			start := time.Now()
			
			// Log incoming message
			log.Printf("[i] User: %d, Chat: %d, Text: %s", 
				c.Sender().ID, c.Chat().ID, c.Text())
			
			// Execute next handler
			err := next(c)
			
			// Log result
			duration := time.Since(start)
			if err != nil {
				log.Printf("[-] Handler failed after %v: %v", duration, err)
			} else {
				log.Printf("[+] Handler completed in %v", duration)
			}
			
			return err
		}
	}
}
