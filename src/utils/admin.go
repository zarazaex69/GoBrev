package utils

import (
	"gopkg.in/telebot.v3"
)

// AdminManager handles admin-related operations
type AdminManager struct {
	botAdmins []int64 // Hardcoded bot admin IDs
}

// NewAdminManager creates a new admin manager
func NewAdminManager() *AdminManager {
	return &AdminManager{
		botAdmins: []int64{7504118464}, // Add more bot admin IDs here
	}
}

// IsBotAdmin checks if user is a bot admin
func (am *AdminManager) IsBotAdmin(userID int64) bool {
	for _, adminID := range am.botAdmins {
		if userID == adminID {
			return true
		}
	}
	return false
}

// IsChatAdmin checks if user is admin in the current chat
func (am *AdminManager) IsChatAdmin(c telebot.Context) bool {
	// Get chat administrators
	admins, err := c.Bot().AdminsOf(c.Chat())
	if err != nil {
		return false
	}
	
	userID := c.Sender().ID
	for _, admin := range admins {
		if admin.User.ID == userID {
			return true
		}
	}
	
	return false
}

// IsAdmin checks if user is either bot admin or chat admin
func (am *AdminManager) IsAdmin(c telebot.Context) bool {
	userID := c.Sender().ID
	
	// Check if user is bot admin
	if am.IsBotAdmin(userID) {
		return true
	}
	
	// Check if user is chat admin (only in groups/supergroups)
	if c.Chat().Type == telebot.ChatGroup || c.Chat().Type == telebot.ChatSuperGroup {
		return am.IsChatAdmin(c)
	}
	
	return false
}

// GetBotAdmins returns list of bot admin IDs
func (am *AdminManager) GetBotAdmins() []int64 {
	return am.botAdmins
}

// AddBotAdmin adds a new bot admin ID
func (am *AdminManager) AddBotAdmin(adminID int64) {
	// Check if already exists
	for _, id := range am.botAdmins {
		if id == adminID {
			return
		}
	}
	am.botAdmins = append(am.botAdmins, adminID)
}

// RemoveBotAdmin removes a bot admin ID
func (am *AdminManager) RemoveBotAdmin(adminID int64) {
	for i, id := range am.botAdmins {
		if id == adminID {
			am.botAdmins = append(am.botAdmins[:i], am.botAdmins[i+1:]...)
			return
		}
	}
}
