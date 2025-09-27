package factory

import (
	"fmt"
	"time"

	"gopkg.in/telebot.v3"
	"gobrev/src/handlers/commands"
	"gobrev/src/models"
)

// CommandFactory manages command registration and execution
type CommandFactory struct {
	commands         map[string]commands.Command
	metrics          *models.Metrics
	historyManager   *models.UserHistoryManager
	messageIDManager *models.MessageIDManager
}

// NewCommandFactory creates a new command factory
func NewCommandFactory(metrics *models.Metrics, historyManager *models.UserHistoryManager, messageIDManager *models.MessageIDManager, startTime time.Time) *CommandFactory {
	factory := &CommandFactory{
		commands:         make(map[string]commands.Command),
		metrics:           metrics,
		historyManager:    historyManager,
		messageIDManager:  messageIDManager,
	}
	
	// Register all commands
	factory.registerCommands(startTime)
	
	return factory
}

// registerCommands registers all available commands
func (f *CommandFactory) registerCommands(startTime time.Time) {
	// Register start command
	f.Register(commands.NewStartCommand())
	
	// Register test command
	f.Register(commands.NewTestCommand(startTime))
	
	// Register AI command
	aiCommand, err := commands.NewAICommand(f.historyManager, f.messageIDManager)
	if err != nil {
		// Log error but don't fail - AI is optional
		fmt.Printf("Warning: Failed to initialize AI command: %v\n", err)
		fmt.Printf("AI command will not be available. Please set ZHIPUAI_API_KEY in .env\n")
	} else {
		f.Register(aiCommand)
		fmt.Printf("AI command registered successfully\n")
	}
}

// Register adds a command to the factory
func (f *CommandFactory) Register(cmd commands.Command) {
	f.commands[cmd.Name()] = cmd
}

// Get retrieves a command by name
func (f *CommandFactory) Get(name string) commands.Command {
	return f.commands[name]
}

// Execute executes a command
func (f *CommandFactory) Execute(cmdName string, c telebot.Context) error {
	fmt.Printf("[i] Factory executing command: %s\n", cmdName)
	cmd := f.Get(cmdName)
	if cmd == nil {
		fmt.Printf("[-] Command not found: %s\n", cmdName)
		return nil // Command not found, ignore
	}
	
	fmt.Printf("[+] Command found: %s\n", cmdName)
	
	// Check if command is private only and we're not in private chat
	if cmd.IsPrivateOnly() && c.Chat().Type != telebot.ChatPrivate {
		fmt.Printf("[-] Command is private only, ignoring in group\n")
		return nil // Ignore private-only commands in groups
	}
	
	fmt.Printf("[i] Executing command: %s\n", cmdName)
	return cmd.Execute(c, f.metrics)
}

// GetAllCommands returns all registered command names
func (f *CommandFactory) GetAllCommands() []string {
	var names []string
	for name := range f.commands {
		names = append(names, name)
	}
	return names
}

// GetMessageIDManager returns the message ID manager
func (f *CommandFactory) GetMessageIDManager() *models.MessageIDManager {
	return f.messageIDManager
}
