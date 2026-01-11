package service

import (
	"unicode"
)

// EmojiValidator validates whether a string is a valid emoji
type EmojiValidator struct{}

// NewEmojiValidator creates a new EmojiValidator instance
func NewEmojiValidator() *EmojiValidator {
	return &EmojiValidator{}
}

// IsValidEmoji checks if the input string is a valid emoji character.
// It supports single emojis and combined emojis (like 👨‍👩‍👧‍👦 with ZWJ sequences).
func (v *EmojiValidator) IsValidEmoji(s string) bool {
	if len(s) == 0 {
		return false
	}

	runes := []rune(s)
	if len(runes) == 0 {
		return false
	}

	// Check if the string contains at least one emoji rune
	hasEmoji := false
	for _, r := range runes {
		if isEmojiRune(r) {
			hasEmoji = true
			break
		}
	}

	if !hasEmoji {
		return false
	}

	// Validate that all runes are either emoji or valid emoji modifiers/joiners
	for _, r := range runes {
		if !isValidEmojiComponent(r) {
			return false
		}
	}

	return true
}

// isEmojiRune checks if a rune is an emoji character
func isEmojiRune(r rune) bool {
	// Common emoji ranges
	switch {
	// Emoticons
	case r >= 0x1F600 && r <= 0x1F64F:
		return true
	// Miscellaneous Symbols and Pictographs
	case r >= 0x1F300 && r <= 0x1F5FF:
		return true
	// Transport and Map Symbols
	case r >= 0x1F680 && r <= 0x1F6FF:
		return true
	// Supplemental Symbols and Pictographs
	case r >= 0x1F900 && r <= 0x1F9FF:
		return true
	// Symbols and Pictographs Extended-A
	case r >= 0x1FA00 && r <= 0x1FA6F:
		return true
	// Symbols and Pictographs Extended-B
	case r >= 0x1FA70 && r <= 0x1FAFF:
		return true
	// Dingbats
	case r >= 0x2700 && r <= 0x27BF:
		return true
	// Miscellaneous Symbols
	case r >= 0x2600 && r <= 0x26FF:
		return true
	// Enclosed Alphanumeric Supplement (includes some emoji)
	case r >= 0x1F100 && r <= 0x1F1FF:
		return true
	// Playing Cards
	case r >= 0x1F0A0 && r <= 0x1F0FF:
		return true
	// Mahjong Tiles
	case r >= 0x1F000 && r <= 0x1F02F:
		return true
	// Domino Tiles
	case r >= 0x1F030 && r <= 0x1F09F:
		return true
	// Chess symbols
	case r >= 0x2654 && r <= 0x265F:
		return true
	// Common single-character emojis
	case r == 0x2764: // ❤ Red Heart
		return true
	case r == 0x2665: // ♥ Heart Suit
		return true
	case r == 0x2763: // ❣ Heart Exclamation
		return true
	case r == 0x2B50: // ⭐ Star
		return true
	case r == 0x2B55: // ⭕ Circle
		return true
	case r == 0x2139: // ℹ Information
		return true
	case r == 0x2122: // ™ Trade Mark
		return true
	case r == 0x00A9: // © Copyright
		return true
	case r == 0x00AE: // ® Registered
		return true
	case r == 0x203C: // ‼ Double Exclamation
		return true
	case r == 0x2049: // ⁉ Exclamation Question
		return true
	case r == 0x2934: // ⤴ Arrow
		return true
	case r == 0x2935: // ⤵ Arrow
		return true
	case r == 0x3030: // 〰 Wavy Dash
		return true
	case r == 0x303D: // 〽 Part Alternation Mark
		return true
	case r == 0x3297: // ㊗ Circled Ideograph Congratulation
		return true
	case r == 0x3299: // ㊙ Circled Ideograph Secret
		return true
	// Arrows
	case r >= 0x2190 && r <= 0x21FF:
		return true
	// Number signs with keycap
	case r >= 0x0023 && r <= 0x0039:
		return true
	}
	return false
}

// isValidEmojiComponent checks if a rune is a valid component of an emoji sequence
func isValidEmojiComponent(r rune) bool {
	// Check if it's an emoji itself
	if isEmojiRune(r) {
		return true
	}

	// Zero Width Joiner (used in combined emojis like family emojis)
	if r == 0x200D {
		return true
	}

	// Variation Selectors (VS15 for text, VS16 for emoji presentation)
	if r == 0xFE0E || r == 0xFE0F {
		return true
	}

	// Skin tone modifiers (Fitzpatrick scale)
	if r >= 0x1F3FB && r <= 0x1F3FF {
		return true
	}

	// Regional Indicator Symbols (for flags)
	if r >= 0x1F1E0 && r <= 0x1F1FF {
		return true
	}

	// Combining Enclosing Keycap
	if r == 0x20E3 {
		return true
	}

	// Tag characters (used in flag sequences like England, Scotland, Wales)
	if r >= 0xE0020 && r <= 0xE007F {
		return true
	}

	// Hair components
	if r >= 0x1F9B0 && r <= 0x1F9B3 {
		return true
	}

	// Gender symbols used in emoji sequences
	if r == 0x2640 || r == 0x2642 {
		return true
	}

	// Medical symbol
	if r == 0x2695 {
		return true
	}

	// Other symbols commonly used in emoji sequences
	if unicode.Is(unicode.So, r) {
		return true
	}

	return false
}
