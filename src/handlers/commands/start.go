package commands

import (
	"fmt"
	"gopkg.in/telebot.v3"
	"gobrev/src/models"
	"gobrev/src/utils"
)

// StartCommand handles /start command
type StartCommand struct {
	*BaseCommand
}

// NewStartCommand creates a new start command
func NewStartCommand() *StartCommand {
	return &StartCommand{
		BaseCommand: NewBaseCommand("/start", false),
	}
}

// Execute executes the start command
func (cmd *StartCommand) Execute(c telebot.Context, metrics *models.Metrics) error {
	metrics.RecordCommand()
	
	// Check if it's private chat
	isPrivate := c.Chat().Type == telebot.ChatPrivate
	
	// Get admin manager
	adminManager := utils.NewAdminManager()
	
	// Get bot info dynamically
	botInfo := c.Bot().Me
	botName := botInfo.FirstName
	if botInfo.LastName != "" {
		botName += " " + botInfo.LastName
	}
	
	// Check if user is admin (for future use)
	_ = adminManager.IsAdmin(c)
	
	message := fmt.Sprintf(`🤖 <b>%s</b> — помощник для тг-групп!  

<i>🧬Brev Переписанный на Go</i>
<i>для скорости 🧬 и стабильности</i>

💡 <b>ИИ • Игры • Модерация</b>`, botName)
	
	// Add button only for private chats
	if isPrivate {
		// Generate dynamic URL for adding bot to group
		botUsername := botInfo.Username
		addToGroupURL := fmt.Sprintf("https://t.me/%s?startgroup=true", botUsername)
		
		return c.Send(message, &telebot.SendOptions{
			ParseMode: telebot.ModeHTML,
			ReplyMarkup: &telebot.ReplyMarkup{
				InlineKeyboard: [][]telebot.InlineButton{
					{{
						Text: "➕ Добавить в чат",
						URL:  addToGroupURL,					}},
				},
			},
		})
	}
	
	// Send without button for group chats
	return c.Send(message, &telebot.SendOptions{
		ParseMode: telebot.ModeHTML,
	})
}
