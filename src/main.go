package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gopkg.in/telebot.v3"
	"gobrev/src/config"
	"gobrev/src/handlers"
	"gobrev/src/middleware"
	"gobrev/src/models"
)

func main() {
	// Load configuration
	cfg := config.Load()
	
	// Create metrics instance
	metrics := models.NewMetrics()
	
	// Create user history manager
	historyManager := models.NewUserHistoryManager()
	
	// Create message ID manager
	messageIDManager, err := models.NewMessageIDManager("./data/message_ids")
	if err != nil {
		log.Fatal("Failed to create message ID manager:", err)
	}
	defer messageIDManager.Close()
	
	// Setup bot
	bot, err := telebot.NewBot(telebot.Settings{
		Token:  cfg.BotToken,
		Poller: &telebot.LongPoller{Timeout: cfg.PollTimeout},
	})
	if err != nil {
		log.Fatal("Failed to create bot:", err)
	}
	
	// Setup middleware
	middleware.SetupMiddleware(bot, metrics)
	
	// Register handlers
	handlers.SetupHandlers(bot, metrics, historyManager, messageIDManager, cfg.StartTime)
	
	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Start bot in separate goroutine
	go func() {
		log.Printf("[+] Bot starting...")
		bot.Start()
	}()
	
	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Log statistics every 5 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				stats := metrics.GetStats()
				log.Printf("[#] Stats: Messages: %v, Commands: %v, Errors: %v",
					stats["messages_processed"],
					stats["commands_processed"],
					stats["errors_count"])
			case <-ctx.Done():
				return
			}
		}
	}()
	
	// Wait for shutdown signal
	<-sigChan
	log.Println("[-] Shutting down bot...")
	
	// Stop bot
	bot.Stop()
	
	// Print final statistics
	finalStats := metrics.GetStats()
	log.Printf("[#] Final stats: %+v", finalStats)
	
	log.Println("[+] Bot stopped gracefully")
}
