package utils

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// UTF8Validator handles UTF-8 validation and cleaning
type UTF8Validator struct{}

// NewUTF8Validator creates a new UTF-8 validator
func NewUTF8Validator() *UTF8Validator {
	return &UTF8Validator{}
}

// ValidateAndClean validates and cleans a string to ensure it's valid UTF-8
func (v *UTF8Validator) ValidateAndClean(text string) string {
	if utf8.ValidString(text) {
		return v.cleanControlChars(text)
	}
	
	// If not valid UTF-8, clean it
	return v.cleanInvalidUTF8(text)
}

// cleanInvalidUTF8 removes invalid UTF-8 sequences and replaces them with replacement character
func (v *UTF8Validator) cleanInvalidUTF8(text string) string {
	var result strings.Builder
	result.Grow(len(text))
	
	for len(text) > 0 {
		r, size := utf8.DecodeRuneInString(text)
		if r == utf8.RuneError && size == 1 {
			// Invalid UTF-8 byte, skip it
			text = text[1:]
		} else {
			// Valid rune, add it to result
			if v.isAllowedRune(r) {
				result.WriteRune(r)
			}
			text = text[size:]
		}
	}
	
	return result.String()
}

// cleanControlChars removes or replaces problematic control characters
func (v *UTF8Validator) cleanControlChars(text string) string {
	var result strings.Builder
	result.Grow(len(text))
	
	for _, r := range text {
		if v.isAllowedRune(r) {
			result.WriteRune(r)
		}
	}
	
	return result.String()
}

// isAllowedRune checks if a rune is allowed in Telegram messages
func (v *UTF8Validator) isAllowedRune(r rune) bool {
	// Allow printable characters, spaces, and common control characters
	if unicode.IsPrint(r) || r == '\n' || r == '\t' || r == '\r' {
		return true
	}
	
	// Allow some specific Unicode categories that are safe
	if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
		return true
	}
	
	// Allow space characters
	if unicode.IsSpace(r) {
		return true
	}
	
	// Reject other control characters and invalid runes
	return false
}

// SanitizeForTelegram sanitizes text specifically for Telegram API
func (v *UTF8Validator) SanitizeForTelegram(text string) string {
	// First, validate and clean UTF-8
	cleaned := v.ValidateAndClean(text)
	
	// Remove or replace problematic sequences
	cleaned = strings.ReplaceAll(cleaned, "\x00", "") // Remove null bytes
	cleaned = strings.ReplaceAll(cleaned, "\ufeff", "") // Remove BOM
	
	// Normalize line endings
	cleaned = strings.ReplaceAll(cleaned, "\r\n", "\n")
	cleaned = strings.ReplaceAll(cleaned, "\r", "\n")
	
	// Remove excessive whitespace
	cleaned = v.normalizeWhitespace(cleaned)
	
	return cleaned
}

// normalizeWhitespace normalizes whitespace in the text
func (v *UTF8Validator) normalizeWhitespace(text string) string {
	// Replace multiple spaces with single space
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}
	
	// Remove excessive newlines (more than 2 consecutive)
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	
	// Trim leading and trailing whitespace
	return strings.TrimSpace(text)
}

// ValidateUTF8 checks if a string is valid UTF-8
func (v *UTF8Validator) ValidateUTF8(text string) bool {
	return utf8.ValidString(text)
}

// GetInvalidUTF8Positions returns positions of invalid UTF-8 sequences
func (v *UTF8Validator) GetInvalidUTF8Positions(text string) []int {
	var positions []int
	pos := 0
	
	for len(text) > 0 {
		r, size := utf8.DecodeRuneInString(text)
		if r == utf8.RuneError && size == 1 {
			positions = append(positions, pos)
		}
		text = text[size:]
		pos += size
	}
	
	return positions
}
