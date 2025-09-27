package middleware

import (
	"gopkg.in/telebot.v3"
	"gobrev/src/models"
)

// SetupMiddleware configures all middleware for the bot
func SetupMiddleware(bot *telebot.Bot, metrics *models.Metrics) {
	bot.Use(LoggerMiddleware())
	bot.Use(MetricsMiddleware(metrics))
}
