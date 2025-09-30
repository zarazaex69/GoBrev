package utils

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fogleman/gg"
	"gopkg.in/telebot.v3"
	"gobrev/src/models"
)

// AvatarCache manages avatar caching
type AvatarCache struct {
	cacheDir string
	httpClient *http.Client
	bot *telebot.Bot
}

// NewAvatarCache creates a new avatar cache
func NewAvatarCache(bot *telebot.Bot) *AvatarCache {
	cacheDir := ".cache/avatars"
	os.MkdirAll(cacheDir, 0755)
	
	return &AvatarCache{
		cacheDir: cacheDir,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		bot: bot,
	}
}

// GetUserAvatar gets user avatar from cache or downloads it
func (ac *AvatarCache) GetUserAvatar(userID int64) (image.Image, error) {
	cachePath := filepath.Join(ac.cacheDir, fmt.Sprintf("%d.jpg", userID))
	
	// Check if avatar exists in cache
	if _, err := os.Stat(cachePath); err == nil {
		// Load from cache
		return ac.loadImageFromFile(cachePath)
	}
	
	// Try to download avatar from Telegram
	if ac.bot != nil {
		if avatarImg, err := ac.downloadUserAvatar(userID, cachePath); err == nil {
			return avatarImg, nil
		}
	}
	
	// Avatar not found
	return nil, fmt.Errorf("avatar not found")
}

// downloadUserAvatar downloads user avatar from Telegram and saves to cache
func (ac *AvatarCache) downloadUserAvatar(userID int64, cachePath string) (image.Image, error) {
	// Create user object
	user := &telebot.User{ID: userID}
	
	// Get user profile photos
	photos, err := ac.bot.ProfilePhotosOf(user)
	if err != nil || len(photos) == 0 {
		return nil, fmt.Errorf("no profile photos found")
	}
	
	// Get the first (most recent) photo
	photo := photos[0]
	
	// Get the largest photo size (last element in sizes array)
	largestPhoto := photo
	
	// Get file info
	file, err := ac.bot.FileByID(largestPhoto.FileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}
	
	// Download file
	resp, err := ac.httpClient.Get(fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", ac.bot.Token, file.FilePath))
	if err != nil {
		return nil, fmt.Errorf("failed to download avatar: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download avatar: status %d", resp.StatusCode)
	}
	
	// Read image data
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}
	
	// Decode image
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	
	// Save to cache
	go func() {
		if cacheFile, err := os.Create(cachePath); err == nil {
			defer cacheFile.Close()
			jpeg.Encode(cacheFile, img, &jpeg.Options{Quality: 85})
		}
	}()
	
	return img, nil
}

// loadImageFromFile loads image from file
func (ac *AvatarCache) loadImageFromFile(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	img, _, err := image.Decode(file)
	return img, err
}

// GenerateTopUsersImage generates a beautiful image with top users on a podium using gg library
func GenerateTopUsersImage(users []models.UserStats, bot *telebot.Bot) ([]byte, error) {
	if len(users) == 0 {
		return nil, fmt.Errorf("no users provided")
	}

	// Canvas dimensions
	const width = 720
	const height = 480
	const avatarRadius = 50

	// Create canvas
	dc := gg.NewContext(width, height)
	
	// Create avatar cache
	avatarCache := NewAvatarCache(bot)

	// 1. Beautiful gradient background
	gradient := gg.NewLinearGradient(0, 0, 0, height)
	gradient.AddColorStop(0, color.RGBA{44, 62, 80, 255})   // Dark blue
	gradient.AddColorStop(1, color.RGBA{52, 152, 219, 255}) // Light blue
	dc.SetFillStyle(gradient)
	dc.Clear()

	// 2. Draw podium with gradients and shadows
	drawPodium(dc, width, height)

	// 3. Draw users
	positions := []struct {
		x, y int
		rank int
		medal string
	}{
		{360, 110, 1, "🥇"}, // Gold - center, highest
		{150, 180, 2, "🥈"}, // Silver - left
		{570, 210, 3, "🥉"}, // Bronze - right
	}

	for i, user := range users {
		if i >= len(positions) {
			break
		}
		
		pos := positions[i]
		
		// Draw user avatar
		drawUserAvatar(dc, pos.x, pos.y, avatarRadius, user, avatarCache)
		
		// Draw user name
		drawUserName(dc, pos.x, pos.y + avatarRadius + 50, user.Username)
		
		// Draw medal
		drawMedal(dc, pos.x, pos.y + avatarRadius + 80, pos.medal)
		
		// Draw message count
		drawMessageCount(dc, pos.x, pos.y + avatarRadius + 110, user.MessageCount)
	}

	// 4. Draw title
	drawTitle(dc, width, height)

	// Convert to PNG
	var buf bytes.Buffer
	err := dc.EncodePNG(&buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// drawPodium draws a beautiful 3D podium
func drawPodium(dc *gg.Context, width, height int) {
	
	// Gold podium (1st place) - center, highest
	goldX := 260
	goldY := 180
	goldW := 200
	goldH := 300
	drawPodiumStep(dc, goldX, goldY, goldW, goldH, "#ffd700", "#d4af37")
	
	// Silver podium (2nd place) - left, medium height
	silverX := 50
	silverY := 250
	silverW := 200
	silverH := 230
	drawPodiumStep(dc, silverX, silverY, silverW, silverH, "#d7dde4", "#a6b0b8")
	
	// Bronze podium (3rd place) - right, lowest
	bronzeX := 470
	bronzeY := 280
	bronzeW := 200
	bronzeH := 200
	drawPodiumStep(dc, bronzeX, bronzeY, bronzeW, bronzeH, "#cd7f32", "#b87333")
}

// drawPodiumStep draws a single podium step with 3D effect
func drawPodiumStep(dc *gg.Context, x, y, w, h int, color1, color2 string) {
	// Create gradient
	gradient := gg.NewLinearGradient(0, float64(y), 0, float64(y+h))
	gradient.AddColorStop(0, parseColor(color1))
	gradient.AddColorStop(1, parseColor(color2))
	
	// Draw rounded rectangle
	dc.DrawRoundedRectangle(float64(x), float64(y), float64(w), float64(h), 20)
	dc.SetFillStyle(gradient)
	dc.Fill()
}

// drawUserAvatar draws a beautiful user avatar with real avatar or placeholder
func drawUserAvatar(dc *gg.Context, x, y, radius int, user models.UserStats, cache *AvatarCache) {
	// Try to load real avatar
	avatarImg, err := cache.GetUserAvatar(user.UserID)
	if err != nil {
		// If no avatar, draw placeholder
		drawPlaceholderAvatar(dc, x, y, radius, user)
		return
	}
	
	// Draw real avatar with proper scaling
	dc.DrawCircle(float64(x), float64(y), float64(radius))
	dc.Clip()
	
	// Draw scaled image centered in circle
	dc.DrawImageAnchored(avatarImg, x, y, 0.5, 0.5)
	dc.ResetClip()
	
	// Draw border
	dc.DrawCircle(float64(x), float64(y), float64(radius))
	dc.SetColor(color.RGBA{255, 255, 255, 255})
	dc.SetLineWidth(4)
	dc.Stroke()
}

// drawPlaceholderAvatar draws a placeholder avatar with initials
func drawPlaceholderAvatar(dc *gg.Context, x, y, radius int, user models.UserStats) {
	// Choose beautiful color based on user ID
	colors := []string{
		"#3498db", // Blue
		"#2ecc71", // Green
		"#9b59b6", // Purple
		"#f1c40f", // Yellow
		"#e67e22", // Orange
		"#e74c3c", // Red
		"#1abc9c", // Turquoise
		"#8e44ad", // Dark Purple
	}
	
	userColor := colors[int(user.UserID)%len(colors)]
	
	// Draw circle
	dc.DrawCircle(float64(x), float64(y), float64(radius))
	dc.SetColor(parseColor(userColor))
	dc.Fill()
	
	// Draw border
	dc.DrawCircle(float64(x), float64(y), float64(radius))
	dc.SetColor(color.RGBA{255, 255, 255, 255})
	dc.SetLineWidth(4)
	dc.Stroke()
	
	// Draw initials
	initials := getInitials(user.Username)
	dc.SetColor(color.RGBA{255, 255, 255, 255})
	dc.LoadFontFace("", float64(radius))
	dc.DrawStringAnchored(initials, float64(x), float64(y), 0.5, 0.5)
}

// drawUserName draws user name with beautiful typography
func drawUserName(dc *gg.Context, x, y int, username string) {
	// Truncate username if too long
	if len(username) > 20 {
		username = username[:17] + "..."
	}
	
	// Draw name
	dc.SetColor(color.RGBA{255, 255, 255, 255})
	dc.LoadFontFace("", 22)
	dc.DrawStringAnchored(username, float64(x), float64(y), 0.5, 0.5)
}

// drawMedal draws medal
func drawMedal(dc *gg.Context, x, y int, medal string) {
	// Draw medal background circle
	dc.DrawCircle(float64(x), float64(y), 20)
	dc.SetColor(color.RGBA{255, 215, 0, 255}) // Gold
	dc.Fill()
	
	// Draw medal border
	dc.DrawCircle(float64(x), float64(y), 20)
	dc.SetColor(color.RGBA{255, 255, 255, 255})
	dc.SetLineWidth(2)
	dc.Stroke()
	
	// Draw medal emoji (simplified as text)
	dc.SetColor(color.RGBA{255, 255, 255, 255})
	dc.LoadFontFace("", 24)
	dc.DrawStringAnchored(medal, float64(x), float64(y), 0.5, 0.5)
}

// drawMessageCount draws message count
func drawMessageCount(dc *gg.Context, x, y int, count int) {
	text := fmt.Sprintf("%d сообщений", count)
	
	// Draw text
	dc.SetColor(color.RGBA{200, 200, 200, 255})
	dc.LoadFontFace("", 18)
	dc.DrawStringAnchored(text, float64(x), float64(y), 0.5, 0.5)
}

// drawTitle draws title
func drawTitle(dc *gg.Context, width, height int) {
	title := "📊 Статистика активности"
	
	// Draw title
	dc.SetColor(color.RGBA{255, 255, 255, 255})
	dc.LoadFontFace("", 32)
	dc.DrawStringAnchored(title, float64(width/2), 60, 0.5, 0.5)
}

// getInitials extracts initials from username
func getInitials(username string) string {
	words := strings.Fields(username)
	if len(words) == 0 {
		return "?"
	}
	
	var initials strings.Builder
	for _, word := range words {
		if len(word) > 0 {
			initials.WriteRune(rune(word[0]))
		}
		if initials.Len() >= min(2, len(words)) {
			break
		}
	}
	
	if initials.Len() == 0 {
		return "?"
	}
	
	return strings.ToUpper(initials.String())
}

// parseColor parses hex color string to color.RGBA
func parseColor(hex string) color.RGBA {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return color.RGBA{0, 0, 0, 255}
	}
	
	r := hexToInt(hex[0:2])
	g := hexToInt(hex[2:4])
	b := hexToInt(hex[4:6])
	
	return color.RGBA{r, g, b, 255}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// hexToInt converts hex string to int
func hexToInt(hex string) uint8 {
	var result uint8
	for _, c := range hex {
		result *= 16
		if c >= '0' && c <= '9' {
			result += uint8(c - '0')
		} else if c >= 'a' && c <= 'f' {
			result += uint8(c - 'a' + 10)
		} else if c >= 'A' && c <= 'F' {
			result += uint8(c - 'A' + 10)
		}
	}
	return result
}