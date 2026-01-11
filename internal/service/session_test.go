package service

import (
	"testing"
	"time"

	"matter-core/internal/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestSessionStore(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)

	ctx := env.Context()
	store := NewSessionStore(env.MongoRepo)

	t.Run("Create", func(t *testing.T) {
		userID := primitive.NewObjectID()
		token, err := store.Create(ctx, userID, "user", 24*time.Hour)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		if token == "" {
			t.Error("Expected non-empty token")
		}

		// Verify token length (64 hex chars for 32 bytes)
		if len(token) != 64 {
			t.Errorf("Expected token length 64, got %d", len(token))
		}
	})

	t.Run("Get", func(t *testing.T) {
		userID := primitive.NewObjectID()
		token, _ := store.Create(ctx, userID, "admin", time.Hour)

		session, err := store.Get(ctx, token)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if session.UserID != userID {
			t.Error("UserID mismatch")
		}
		if session.Role != "admin" {
			t.Errorf("Expected role 'admin', got %s", session.Role)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		userID := primitive.NewObjectID()
		token, _ := store.Create(ctx, userID, "user", time.Hour)

		err := store.Delete(ctx, token)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		_, err = store.Get(ctx, token)
		if err == nil {
			t.Error("Expected error when getting deleted session")
		}
	})

	t.Run("IsValid_ValidSession", func(t *testing.T) {
		userID := primitive.NewObjectID()
		token, _ := store.Create(ctx, userID, "user", time.Hour)

		session, valid := store.IsValid(ctx, token)
		if !valid {
			t.Error("Expected session to be valid")
		}
		if session == nil {
			t.Error("Expected session to be returned")
		}
	})

	t.Run("IsValid_InvalidToken", func(t *testing.T) {
		_, valid := store.IsValid(ctx, "invalid-token")
		if valid {
			t.Error("Expected session to be invalid")
		}
	})

	t.Run("IsValid_ExpiredSession", func(t *testing.T) {
		userID := primitive.NewObjectID()
		// Create with negative duration (already expired)
		token, _ := store.Create(ctx, userID, "user", -time.Hour)

		_, valid := store.IsValid(ctx, token)
		if valid {
			t.Error("Expected expired session to be invalid")
		}
	})
}

func TestGenerateToken(t *testing.T) {
	t.Run("GeneratesUniqueTokens", func(t *testing.T) {
		tokens := make(map[string]bool)
		for i := 0; i < 100; i++ {
			token, err := generateToken(32)
			if err != nil {
				t.Fatalf("generateToken failed: %v", err)
			}
			if tokens[token] {
				t.Error("Generated duplicate token")
			}
			tokens[token] = true
		}
	})

	t.Run("CorrectLength", func(t *testing.T) {
		token, _ := generateToken(16)
		if len(token) != 32 { // hex encoding doubles the length
			t.Errorf("Expected length 32, got %d", len(token))
		}
	})
}
