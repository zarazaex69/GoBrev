package commands

import (
	"gopkg.in/telebot.v3"
	"gobrev/src/models"
	"gobrev/src/utils"
)

// Command interface defines the contract for all bot commands
type Command interface {
	Name() string
	Execute(c telebot.Context, metrics *models.Metrics) error
	IsPrivateOnly() bool
}

// BaseCommand provides common functionality for all commands
type BaseCommand struct {
	name        string
	privateOnly bool
	safeSender  *utils.SafeSender
}

// Name returns the command name
func (b *BaseCommand) Name() string {
	return b.name
}

// IsPrivateOnly returns whether command should only work in private chats
func (b *BaseCommand) IsPrivateOnly() bool {
	return b.privateOnly
}

// NewBaseCommand creates a new base command
func NewBaseCommand(name string, privateOnly bool) *BaseCommand {
	return &BaseCommand{
		name:        name,
		privateOnly: privateOnly,
		safeSender:  utils.NewSafeSender(),
	}
}

// SafeSend safely sends a message with UTF-8 validation
func (b *BaseCommand) SafeSend(c telebot.Context, text string, options ...*telebot.SendOptions) error {
	return b.safeSender.SafeSend(c, text, options...)
}
