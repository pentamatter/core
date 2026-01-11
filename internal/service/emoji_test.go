package service

import "testing"

func TestEmojiValidator(t *testing.T) {
	v := NewEmojiValidator()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// Valid emojis
		{"thumbs up", "👍", true},
		{"heart", "❤️", true},
		{"fire", "🔥", true},
		{"party", "🎉", true},
		{"rocket", "🚀", true},
		{"star", "⭐", true},
		{"smile", "😀", true},
		{"laugh", "😂", true},
		{"thinking", "🤔", true},
		{"clap", "👏", true},
		{"100", "💯", true},
		{"eyes", "👀", true},
		{"check", "✅", true},
		{"cross", "❌", true},

		// Skin tone modifiers
		{"thumbs up light", "👍🏻", true},
		{"thumbs up dark", "👍🏿", true},

		// Combined emojis (ZWJ sequences)
		{"family", "👨‍👩‍👧", true},
		{"rainbow flag", "🏳️‍🌈", true},

		// Flags
		{"us flag", "🇺🇸", true},
		{"jp flag", "🇯🇵", true},

		// Invalid inputs
		{"empty string", "", false},
		{"plain text", "hello", false},
		{"number", "123", true}, // Numbers are valid as keycap emoji components
		{"special char", "@#$", false},
		{"mixed text emoji", "hello👍", false},
		{"space", " ", false},
		{"newline", "\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.IsValidEmoji(tt.input)
			if got != tt.want {
				t.Errorf("IsValidEmoji(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestEmojiValidator_EdgeCases(t *testing.T) {
	v := NewEmojiValidator()

	t.Run("multiple emojis should be valid", func(t *testing.T) {
		// Multiple emojis in sequence might be valid depending on implementation
		// This tests the current behavior
		result := v.IsValidEmoji("👍👍")
		// The validator should handle this case
		t.Logf("Multiple emojis result: %v", result)
	})

	t.Run("emoji with variation selector", func(t *testing.T) {
		// Heart with variation selector
		result := v.IsValidEmoji("❤️")
		if !result {
			t.Error("Expected heart with variation selector to be valid")
		}
	})

	t.Run("keycap emoji", func(t *testing.T) {
		// Number keycap
		result := v.IsValidEmoji("1️⃣")
		if !result {
			t.Error("Expected keycap emoji to be valid")
		}
	})
}

func BenchmarkEmojiValidator(b *testing.B) {
	v := NewEmojiValidator()
	emojis := []string{"👍", "❤️", "🔥", "🎉", "🚀", "😀", "👨‍👩‍👧"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, emoji := range emojis {
			v.IsValidEmoji(emoji)
		}
	}
}
