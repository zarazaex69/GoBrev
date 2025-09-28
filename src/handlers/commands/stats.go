package commands

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"gopkg.in/telebot.v3"
	"gobrev/src/models"
	"gobrev/src/utils"
)

// StatsCommand handles .стат command
type StatsCommand struct {
	*BaseCommand
	statsManager    *models.StatsManager
	messageSplitter *utils.MessageSplitter
}

// NewStatsCommand creates a new stats command
func NewStatsCommand(statsManager *models.StatsManager) *StatsCommand {
	return &StatsCommand{
		BaseCommand:     NewBaseCommand(".стат", false),
		statsManager:    statsManager,
		messageSplitter: utils.NewMessageSplitter(),
	}
}

// Execute executes the stats command
func (cmd *StatsCommand) Execute(c telebot.Context, metrics *models.Metrics) error {
	metrics.RecordCommand()
	
	// Check if it's private chat
	if c.Chat().Type == telebot.ChatPrivate {
		return c.Send("❌ Команда доступна только в групповых чатах и каналах")
	}
	
	chatID := c.Chat().ID
	text := c.Text()
	
	// Determine if showing all time stats
	showAllTime := strings.Contains(strings.ToLower(text), "все")
	
	// Get top users
	topUsers, err := cmd.statsManager.GetTopUsers(chatID, 20, showAllTime)
	if err != nil {
		return c.Send("❌ Ошибка получения статистики: " + err.Error())
	}
	
	if len(topUsers) == 0 {
		return c.Send("📊 Пока нет статистики для этого чата. Просто начните общаться!")
	}
	
	// Get total messages
	totalMessages, err := cmd.statsManager.GetTotalMessages(chatID, showAllTime)
	if err != nil {
		return c.Send("❌ Ошибка получения статистики: " + err.Error())
	}
	
	// Get popular words (only for today)
	var popularWords []models.WordStats
	if !showAllTime {
		popularWords, _ = cmd.statsManager.GetPopularWords(chatID, 2)
	}
	
	// Generate image for top 3 users
	imageBuffer, err := utils.GenerateTopUsersImage(topUsers[:min(3, len(topUsers))])
	if err != nil {
		// If image generation fails, send text-only stats
		return cmd.sendTextStats(c, topUsers, totalMessages, popularWords, showAllTime)
	}
	
	// Prepare simple caption without emojis or special characters
	caption := cmd.buildSimpleCaption(topUsers, totalMessages, showAllTime)
	
	// Check caption length and truncate if necessary
	isValid, length := cmd.messageSplitter.ValidateCaptionLength(caption)
	if !isValid {
		fmt.Printf("[-] Caption too long (%d chars), truncating\n", length)
		caption = cmd.messageSplitter.CleanAndTruncate(caption, utils.SafeCaptionLength)
	}
	
	// Send photo with caption
	return c.Send(&telebot.Photo{
		File:    telebot.FromReader(bytes.NewReader(imageBuffer)),
		Caption: caption,
	}, &telebot.SendOptions{
		ReplyTo: c.Message(),
	})
}

// sendTextStats sends text-only statistics when image generation fails
func (cmd *StatsCommand) sendTextStats(c telebot.Context, topUsers []models.UserStats, totalMessages int, popularWords []models.WordStats, showAllTime bool) error {
	var message strings.Builder
	
	title := "📊 <b>Статистика активности в чате</b>"
	if showAllTime {
		title += " <i>(за всё время)</i>"
	} else {
		title += " <i>(за день)</i>"
	}
	message.WriteString(title + "\n\n")
	
	message.WriteString(fmt.Sprintf("💬 Общее количество сообщений: <b>%d</b>\n\n", totalMessages))
	
	// Top users
	message.WriteString("<b>Топ пользователей:</b>\n")
	for i, user := range topUsers {
		medal := ""
		if i == 0 {
			medal = "🥇 "
		} else if i == 1 {
			medal = "🥈 "
		} else if i == 2 {
			medal = "🥉 "
		}
		
		username := cmd.cleanUTF8(user.Username)
		username = cmd.escapeHTML(username)
		message.WriteString(fmt.Sprintf("%d. %s<a href=\"tg://user?id=%d\">%s</a>: <b>%d</b> сообщений\n",
			i+1, medal, user.UserID, username, user.MessageCount))
	}
	
	// Popular words
	if len(popularWords) > 0 {
		message.WriteString("\n<b>Популярные слова:</b>\n")
		for i, word := range popularWords {
			wordText := cmd.escapeHTML(cmd.cleanUTF8(word.Word))
			if wordText == "" {
				wordText = "(пусто)"
			}
			message.WriteString(fmt.Sprintf("%d. \"%s\" (<b>%d</b> раз)\n",
				i+1, wordText, word.Count))
		}
	}
	
	// Check message length and handle accordingly
	messageText := message.String()
	isValid, length := cmd.messageSplitter.ValidateMessageLength(messageText)
	
	if isValid {
		return c.Send(messageText, &telebot.SendOptions{
			ParseMode: telebot.ModeHTML,
			ReplyTo:   c.Message(),
		})
	} else {
		// Message is too long, use splitter
		fmt.Printf("[-] Stats message too long (%d chars), splitting\n", length)
		return cmd.messageSplitter.SendLongMessage(c, messageText, &telebot.SendOptions{
			ParseMode: telebot.ModeHTML,
			ReplyTo:   c.Message(),
		})
	}
}

// buildSimpleCaption builds a simple caption without emojis or special characters
func (cmd *StatsCommand) buildSimpleCaption(topUsers []models.UserStats, totalMessages int, showAllTime bool) string {
	var caption strings.Builder
	
	title := "📊 Статистика активности в чате"
	if showAllTime {
		title += " (за все время)"
	} else {
		title += " (за день)"
	}
	caption.WriteString(title + "\n\n")
	
	caption.WriteString(fmt.Sprintf("💬 Общее количество сообщений: %d\n\n", totalMessages))
	
	// Top users (limit to fit in caption)
	maxUsers := min(8, len(topUsers))
	for i := 0; i < maxUsers; i++ {
		user := topUsers[i]
		medal := ""
		if i == 0 {
			medal = "🥇 "
		} else if i == 1 {
			medal = "🥈 "
		} else if i == 2 {
			medal = "🥉 "
		}
		
		// Clean username very aggressively
		username := cmd.sanitizeUsername(user.Username)
		
		caption.WriteString(fmt.Sprintf("%d. %s%s: %d сообщений\n",
			i+1, medal, username, user.MessageCount))
	}
	
	return caption.String()
}

// buildHTMLCaption builds HTML caption with proper escaping
func (cmd *StatsCommand) buildHTMLCaption(topUsers []models.UserStats, totalMessages int, popularWords []models.WordStats, showAllTime bool) string {
	var caption strings.Builder
	
	title := "<b>Статистика активности в чате</b>"
	if showAllTime {
		title += " <i>(за все время)</i>"
	} else {
		title += " <i>(за день)</i>"
	}
	caption.WriteString(title + "\n\n")
	
	caption.WriteString(fmt.Sprintf("Общее количество сообщений: <b>%d</b>\n\n", totalMessages))
	
	// Top users (limit to fit in caption)
	maxUsers := min(8, len(topUsers))
	for i := 0; i < maxUsers; i++ {
		user := topUsers[i]
		medal := ""
		if i == 0 {
			medal = "🥇 "
		} else if i == 1 {
			medal = "🥈 "
		} else if i == 2 {
			medal = "🥉 "
		}
		
		// Clean username and escape HTML
		username := cmd.escapeHTML(cmd.sanitizeUsername(user.Username))
		
		caption.WriteString(fmt.Sprintf("%d. %s<a href=\"tg://user?id=%d\">%s</a>: <b>%d</b> сообщений\n",
			i+1, medal, user.UserID, username, user.MessageCount))
	}
	
	// Popular words (only if we have space)
	if len(popularWords) > 0 && maxUsers < 6 {
		caption.WriteString("\n<b>Популярные слова:</b>\n")
		for i, word := range popularWords {
			if i >= 2 { // Limit to 2 words
				break
			}
			wordText := cmd.escapeHTML(cmd.cleanUTF8(word.Word))
			if wordText == "" {
				wordText = "(пусто)"
			}
			caption.WriteString(fmt.Sprintf("%d. \"%s\" (<b>%d</b> раз)\n",
				i+1, wordText, word.Count))
		}
	}
	
	return caption.String()
}

// buildSafeCaption builds a safe caption with proper UTF-8 handling
func (cmd *StatsCommand) buildSafeCaption(topUsers []models.UserStats, totalMessages int, popularWords []models.WordStats, showAllTime bool) string {
	var caption strings.Builder
	
	title := "📊 Статистика активности в чате"
	if showAllTime {
		title += " (за все время)"
	} else {
		title += " (за день)"
	}
	caption.WriteString(title + "\n\n")
	
	caption.WriteString(fmt.Sprintf("💬 Общее количество сообщений: %d\n\n", totalMessages))
	
	// Top users (limit to fit in caption)
	maxUsers := min(8, len(topUsers)) // Reduced to fit better
	for i := 0; i < maxUsers; i++ {
		user := topUsers[i]
		medal := ""
		if i == 0 {
			medal = "🥇 "
		} else if i == 1 {
			medal = "🥈 "
		} else if i == 2 {
			medal = "🥉 "
		}
		
		// Clean username aggressively
		username := cmd.sanitizeUsername(user.Username)
		
		caption.WriteString(fmt.Sprintf("%d. %s%s: %d сообщений\n",
			i+1, medal, username, user.MessageCount))
	}
	
	// Popular words (only if we have space)
	if len(popularWords) > 0 && maxUsers < 6 {
		caption.WriteString("\nПопулярные слова:\n")
		for i, word := range popularWords {
			if i >= 2 { // Limit to 2 words
				break
			}
			wordText := cmd.cleanUTF8(word.Word)
			wordText = strings.TrimSpace(wordText)
			if wordText == "" {
				wordText = "(пусто)"
			}
			caption.WriteString(fmt.Sprintf("%d. \"%s\" (%d раз)\n",
				i+1, wordText, word.Count))
		}
	}
	
	return caption.String()
}

// buildCaption builds the caption for the image
func (cmd *StatsCommand) buildCaption(topUsers []models.UserStats, totalMessages int, popularWords []models.WordStats, showAllTime bool) string {
	var caption strings.Builder
	
	title := "Статистика активности в чате"
	if showAllTime {
		title += " (за все время)"
	} else {
		title += " (за день)"
	}
	caption.WriteString(title + "\n\n")
	
	caption.WriteString(fmt.Sprintf("Общее количество сообщений: %d\n\n", totalMessages))
	
	// Top users (limit to fit in caption)
	maxUsers := min(10, len(topUsers))
	for i := 0; i < maxUsers; i++ {
		user := topUsers[i]
		medal := ""
		if i == 0 {
			medal = "1st "
		} else if i == 1 {
			medal = "2nd "
		} else if i == 2 {
			medal = "3rd "
		}
		
		username := cmd.cleanUTF8(user.Username)
		username = cmd.escapeHTML(username)
		// Truncate username if too long
		if len(username) > 20 {
			username = username[:17] + "..."
		}
		
		caption.WriteString(fmt.Sprintf("%d. %s%s: %d сообщений\n",
			i+1, medal, username, user.MessageCount))
	}
	
	// Popular words
	if len(popularWords) > 0 {
		caption.WriteString("\nПопулярные слова:\n")
		for i, word := range popularWords {
			wordText := cmd.cleanUTF8(word.Word)
			wordText = cmd.escapeHTML(wordText)
			if len(wordText) > 30 {
				wordText = wordText[:27] + "..."
			}
			caption.WriteString(fmt.Sprintf("%d. \"%s\" (%d раз)\n",
				i+1, wordText, word.Count))
		}
	}
	
	return caption.String()
}

// escapeHTML escapes HTML special characters
func (cmd *StatsCommand) escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "&", "&amp;")
	return s
}

// cleanUTF8 removes invalid UTF-8 characters from string
func (cmd *StatsCommand) cleanUTF8(s string) string {
	if !utf8.ValidString(s) {
		// Replace invalid UTF-8 sequences with replacement character
		return strings.ToValidUTF8(s, "?")
	}
	return s
}

// sanitizeUsername aggressively sanitizes username for safe display
func (cmd *StatsCommand) sanitizeUsername(s string) string {
	// First clean UTF-8
	s = cmd.cleanUTF8(s)
	
	// Remove or replace problematic characters
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\"", "'")
	s = strings.ReplaceAll(s, "\\", "/")
	
	// Trim and limit length
	s = strings.TrimSpace(s)
	if len(s) > 20 {
		s = s[:17] + "..."
	}
	
	// If empty after cleaning, use default
	if s == "" {
		s = "User"
	}
	
	return s
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
