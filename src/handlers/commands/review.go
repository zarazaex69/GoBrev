package commands

import (
	"fmt"
	"gobrev/src/models"
	"gobrev/src/utils"
	"strings"
	"time"

	"gopkg.in/telebot.v3"
)

// ReviewCommand handles daily review generation
type ReviewCommand struct {
	*BaseCommand
	aiClient        *utils.AIClient
	reviewManager   *models.ReviewManager
	statsManager    *models.StatsManager
	messageSplitter *utils.MessageSplitter
}

// NewReviewCommand creates a new review command
func NewReviewCommand(reviewManager *models.ReviewManager, statsManager *models.StatsManager) (*ReviewCommand, error) {
	aiClient, err := utils.NewAIClient()
	if err != nil {
		return nil, err
	}

	return &ReviewCommand{
		BaseCommand:     NewBaseCommand(".рев", false),
		aiClient:        aiClient,
		reviewManager:   reviewManager,
		statsManager:    statsManager,
		messageSplitter: utils.NewMessageSplitter(),
	}, nil
}

// Execute executes the review command
func (cmd *ReviewCommand) Execute(c telebot.Context, metrics *models.Metrics) error {
	metrics.RecordCommand()

	// Send "generating" message
	generatingMsg, err := c.Bot().Send(c.Chat(), "📰 <b>Генерирую дейли новости чата...</b>", &telebot.SendOptions{
		ParseMode: telebot.ModeHTML,
		ReplyTo:   c.Message(),
	})
	if err != nil {
		return fmt.Errorf("failed to send generating message: %w", err)
	}

	userID := c.Sender().ID
	chatID := c.Chat().ID
	
	// Check if user is admin
	isAdmin := cmd.isUserAdmin(c, chatID, userID)

	// Get messages after last review
	messages, err := cmd.reviewManager.GetMessagesAfterLastReview(chatID, 50) // Get up to 50 messages
	if err != nil {
		_, editErr := c.Bot().Edit(generatingMsg, "❌ <b>Ошибка получения сообщений:</b> <code>"+err.Error()+"</code>", &telebot.SendOptions{
			ParseMode: telebot.ModeHTML,
		})
		return editErr
	}

	if len(messages) == 0 {
		_, editErr := c.Bot().Edit(generatingMsg, "📭 <b>Нет новых сообщений для ревью</b>\n\n<i>Все сообщения уже были использованы для генерации новостей</i>", &telebot.SendOptions{
			ParseMode: telebot.ModeHTML,
		})
		return editErr
	}

	// Prepare messages for AI
	var messageTexts []string
	var messageIDs []string
	
	for _, msg := range messages {
		// Format: [Username] (время): сообщение
		timestamp := time.Unix(msg.Timestamp, 0)
		timeStr := timestamp.Format("15:04")
		
		var formattedMsg string
		if msg.ReplyToMessageID != "" {
			// Message with reply: [Username] (время) отвечает на [ReplyUsername]: "ReplyContent" -> сообщение
			formattedMsg = fmt.Sprintf("[%s] (%s) отвечает на [%s]: \"%s\" -> %s", 
				msg.Username, timeStr, msg.ReplyToUsername, msg.ReplyToContent, msg.Content)
		} else {
			// Regular message: [Username] (время): сообщение
			formattedMsg = fmt.Sprintf("[%s] (%s): %s", msg.Username, timeStr, msg.Content)
		}
		
		messageTexts = append(messageTexts, formattedMsg)
		messageIDs = append(messageIDs, msg.MessageID)
	}

	// Create AI prompt for daily news generation
	prompt := cmd.createDailyNewsPrompt(messageTexts, isAdmin)

	// Get AI response
	fmt.Printf("[i] Generating daily news for %d messages\n", len(messages))
	response, err := cmd.aiClient.QuickChat(prompt,
		utils.WithTemperature(0.9),
		utils.WithMaxTokens(4000))
	if err != nil {
		fmt.Printf("[-] AI request failed: %v\n", err)
		_, editErr := c.Bot().Edit(generatingMsg, "❌ <b>Ошибка ИИ:</b> <code>"+err.Error()+"</code>", &telebot.SendOptions{
			ParseMode: telebot.ModeHTML,
		})
		return editErr
	}

	fmt.Printf("[i] AI response received, length: %d chars\n", len(response))
	
	// Convert Markdown to HTML
	htmlContent := cmd.convertMarkdownToHTML(response)
	fmt.Printf("[i] Converted to HTML, length: %d chars\n", len(htmlContent))

	// Mark messages as used
	err = cmd.reviewManager.MarkMessagesAsUsed(messageIDs)
	if err != nil {
		fmt.Printf("[-] Failed to mark messages as used: %v\n", err)
		// Don't return error, just log it
	} else {
		fmt.Printf("[+] Marked %d messages as used\n", len(messageIDs))
	}

	// Format final response
	finalResponse := fmt.Sprintf(`📰 <b>Дейли новости чата</b>

%s

<i>📊 Обработано сообщений: %d</i>`, htmlContent, len(messages))
	
	fmt.Printf("[i] Sending final response, length: %d chars\n", len(finalResponse))

	// Check if message is too long for Telegram (4096 chars limit)
	if len(finalResponse) > 4000 {
		fmt.Printf("[-] Review message too long (%d chars), splitting into parts\n", len(finalResponse))
		
		// Delete the generating message
		err := c.Bot().Delete(generatingMsg)
		if err != nil {
			fmt.Printf("[-] Failed to delete generating message: %v\n", err)
		}
		
		// Send in parts
		err = cmd.sendLongMessage(c, finalResponse)
		if err != nil {
			return err
		}
		
		// Save current time as last review time
		currentTime := time.Now().Unix()
		err = cmd.reviewManager.SetLastReviewTime(chatID, currentTime)
		if err != nil {
			fmt.Printf("[-] Failed to save last review time: %v\n", err)
		} else {
			fmt.Printf("[+] Last review time saved: %d\n", currentTime)
		}
		
		fmt.Printf("[+] Daily news generated successfully for %d messages\n", len(messages))
		return nil
	}

	// Edit message with final response
	_, editErr := c.Bot().Edit(generatingMsg, finalResponse, &telebot.SendOptions{
		ParseMode: telebot.ModeHTML,
	})
	if editErr != nil {
		fmt.Printf("[-] Failed to edit message: %v\n", editErr)
		return editErr
	}
	
	fmt.Printf("[+] Message edited successfully\n")

	// Save current time as last review time
	currentTime := time.Now().Unix()
	err = cmd.reviewManager.SetLastReviewTime(chatID, currentTime)
	if err != nil {
		fmt.Printf("[-] Failed to save last review time: %v\n", err)
		// Don't return error, just log it
	} else {
		fmt.Printf("[+] Last review time saved: %d\n", currentTime)
	}

	fmt.Printf("[+] Daily news generated successfully for %d messages\n", len(messages))
	return nil
}

// createDailyNewsPrompt creates a prompt for AI to generate daily news
func (cmd *ReviewCommand) createDailyNewsPrompt(messages []string, isAdmin bool) string {
	messagesText := strings.Join(messages, "\n")
	
	userStatus := "обычный участник"
	if isAdmin {
		userStatus = "администратор"
	}
	
	promptTemplate := `Ты - опытный журналист и блогер, который создает захватывающие ежедневные новости чата в Telegram.

Твоя задача: проанализировать сообщения и создать ПОДРОБНЫЙ, РАЗВЕРНУТЫЙ и УВЛЕКАТЕЛЬНЫЙ обзор того, что происходило в чате.

КОНТЕКСТ: Запрос делает %s чата.

СООБЩЕНИЯ ЧАТА:
%s

ТРЕБОВАНИЯ К ОТВЕТУ:
1. Создай РАЗВЕРНУТЫЙ и ДЕТАЛЬНЫЙ обзор событий в чате (не менее 800-1500 символов)
2. Выдели ВСЕ интересные моменты, темы, обсуждения, конфликты, шутки, ОТВЕТЫ НА СООБЩЕНИЯ
3. Обязательно упоминай авторов сообщений по именам и описывай их действия
4. Анализируй ДИАЛОГИ и ОТВЕТЫ между пользователями (когда кто-то отвечает на сообщение)
5. Используй живой, неформальный, журналистский стиль с элементами юмора
6. Добавляй эмоциональные комментарии и оценки происходящего
7. Форматируй текст в Markdown:
   - Используй *жирный текст* для выделения важных моментов
   - Для цитирования сообщений используй формат: @username: текст сообщения (в четырех обратных кавычках)
   - Используй заголовки ## для разделения тем
8. Язык: русский

СТРУКТУРА ОТВЕТА:
- Яркое введение с оценкой общей атмосферы дня
- Подробное описание 3-5 основных событий/тем с именами участников
- Интересные цитаты с комментариями
- Анализ конфликтов, споров, шуток
- Описание самых активных участников
- Заключение с прогнозом или шуткой

СТИЛЬ: Пиши как популярный блогер - живо, с юмором, подробно, интересно!

ПРИМЕР ЦИТИРОВАНИЯ:
` + "````" + `
@Иван: Привет всем!
` + "````" + `

Создай МАКСИМАЛЬНО ПОДРОБНЫЕ и увлекательные "дейли новости" этого чата!`
	
	return fmt.Sprintf(promptTemplate, userStatus, messagesText)
}

// convertMarkdownToHTML converts Markdown formatting to HTML
func (cmd *ReviewCommand) convertMarkdownToHTML(text string) string {
	// Convert bold text: *text* -> <b>text</b>
	result := text
	
	// Handle headers: ## text -> <b>text</b>
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") {
			headerText := strings.TrimPrefix(line, "## ")
			lines[i] = "<b>" + headerText + "</b>"
		}
	}
	result = strings.Join(lines, "\n")
	
	// Handle bold formatting
	for {
		start := strings.Index(result, "*")
		if start == -1 {
			break
		}
		
		end := strings.Index(result[start+1:], "*")
		if end == -1 {
			break
		}
		end += start + 1
		
		// Extract the text between asterisks
		boldText := result[start+1 : end]
		
		// Replace with HTML bold tags
		result = result[:start] + "<b>" + boldText + "</b>" + result[end+1:]
	}
	
	// Handle four backticks code blocks: ```` text ```` -> <pre>text</pre>
	for {
		start := strings.Index(result, "````")
		if start == -1 {
			break
		}
		
		end := strings.Index(result[start+4:], "````")
		if end == -1 {
			break
		}
		end += start + 4
		
		// Extract the text between four backticks
		codeText := strings.TrimSpace(result[start+4 : end])
		
		// Replace with HTML pre tags
		result = result[:start] + "<pre>" + codeText + "</pre>" + result[end+4:]
	}
	
	// Clean up any remaining markdown artifacts
	result = strings.ReplaceAll(result, "**", "")
	result = strings.ReplaceAll(result, "__", "")
	
	// Fix unclosed HTML tags
	result = cmd.fixUnclosedTags(result)
	
	return result
}

// fixUnclosedTags fixes unclosed HTML tags
func (cmd *ReviewCommand) fixUnclosedTags(text string) string {
	result := text
	
	// Fix each tag type
	result = cmd.fixTagPair(result, "<b>", "</b>")
	result = cmd.fixTagPair(result, "<i>", "</i>")
	result = cmd.fixTagPair(result, "<pre>", "</pre>")
	result = cmd.fixTagPair(result, "<code>", "</code>")
	
	return result
}

// fixTagPair fixes a specific tag pair
func (cmd *ReviewCommand) fixTagPair(text, openTag, closeTag string) string {
	openCount := strings.Count(text, openTag)
	closeCount := strings.Count(text, closeTag)
	
	result := text
	
	if openCount > closeCount {
		// Add missing closing tags
		for i := 0; i < openCount-closeCount; i++ {
			result += closeTag
		}
	} else if closeCount > openCount {
		// Remove extra closing tags from the end
		for i := 0; i < closeCount-openCount; i++ {
			lastIndex := strings.LastIndex(result, closeTag)
			if lastIndex != -1 {
				result = result[:lastIndex] + result[lastIndex+len(closeTag):]
			}
		}
	}
	
	return result
}

// sendLongMessage splits and sends long messages
func (cmd *ReviewCommand) sendLongMessage(c telebot.Context, message string) error {
	const maxLength = 4000
	
	if len(message) <= maxLength {
		return c.Send(message, &telebot.SendOptions{
			ParseMode: telebot.ModeHTML,
		})
	}
	
	// Split message into parts
	parts := cmd.splitMessage(message, maxLength)
	
	fmt.Printf("[i] Split message into %d parts\n", len(parts))
	
	for i, part := range parts {
		fmt.Printf("[i] Sending part %d/%d, length: %d chars\n", i+1, len(parts), len(part))
		err := c.Send(part, &telebot.SendOptions{
			ParseMode: telebot.ModeHTML,
		})
		if err != nil {
			fmt.Printf("[-] Failed to send part %d: %v\n", i+1, err)
			return err
		}
		fmt.Printf("[+] Part %d sent successfully\n", i+1)
	}
	
	return nil
}

// splitMessage splits a message into parts respecting HTML tags
func (cmd *ReviewCommand) splitMessage(message string, maxLength int) []string {
	if len(message) <= maxLength {
		return []string{message}
	}
	
	var parts []string
	remaining := message
	
	for len(remaining) > maxLength {
		// Find a good split point (prefer newlines)
		splitPoint := maxLength
		for i := maxLength - 1; i > maxLength/2; i-- {
			if remaining[i] == '\n' {
				splitPoint = i
				break
			}
		}
		
		part := remaining[:splitPoint]
		
		// Fix HTML tags in this part
		part = cmd.fixUnclosedTags(part)
		
		parts = append(parts, part)
		remaining = remaining[splitPoint:]
		
		// Skip leading newlines in remaining text
		for len(remaining) > 0 && remaining[0] == '\n' {
			remaining = remaining[1:]
		}
	}
	
	if len(remaining) > 0 {
		// Fix HTML tags in the last part
		remaining = cmd.fixUnclosedTags(remaining)
		parts = append(parts, remaining)
	}
	
	return parts
}

// isUserAdmin checks if user is admin in the chat
func (cmd *ReviewCommand) isUserAdmin(c telebot.Context, chatID int64, userID int64) bool {
	// In private chats, user is always considered admin
	if c.Chat().Type == telebot.ChatPrivate {
		return true
	}
	
	// Get chat member info
	member, err := c.Bot().ChatMemberOf(c.Chat(), &telebot.User{ID: userID})
	if err != nil {
		fmt.Printf("[-] Failed to get chat member info: %v\n", err)
		return false
	}
	
	// Check if user is admin or creator
	return member.Role == telebot.Administrator || member.Role == telebot.Creator
}
