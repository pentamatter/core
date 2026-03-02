package repository_test

import (
	"context"
	"testing"
	"time"

	"matter-core/internal/testutil"
)

func TestRedisRepo_UserReaction(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)
	env.FlushRedis(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userID := "user123"
	targetType := "entry"
	targetID := "entry456"

	t.Run("AddUserReaction", func(t *testing.T) {
		added, err := env.RedisRepo.AddUserReaction(ctx, userID, targetType, targetID, "👍")
		if err != nil {
			t.Fatalf("AddUserReaction failed: %v", err)
		}
		if !added {
			t.Error("Expected reaction to be newly added")
		}

		// Verify it was added
		exists, err := env.RedisRepo.HasUserReaction(ctx, userID, targetType, targetID, "👍")
		if err != nil {
			t.Fatalf("HasUserReaction failed: %v", err)
		}
		if !exists {
			t.Error("Expected reaction to exist")
		}

		// Adding again should return false (already exists)
		added, err = env.RedisRepo.AddUserReaction(ctx, userID, targetType, targetID, "👍")
		if err != nil {
			t.Fatalf("AddUserReaction failed: %v", err)
		}
		if added {
			t.Error("Expected reaction to not be newly added (duplicate)")
		}
	})

	t.Run("HasUserReaction_NotExists", func(t *testing.T) {
		exists, err := env.RedisRepo.HasUserReaction(ctx, userID, targetType, targetID, "❌")
		if err != nil {
			t.Fatalf("HasUserReaction failed: %v", err)
		}
		if exists {
			t.Error("Expected reaction to not exist")
		}
	})

	t.Run("RemoveUserReaction", func(t *testing.T) {
		// Add first
		_, _ = env.RedisRepo.AddUserReaction(ctx, userID, targetType, targetID, "❤️")

		// Remove
		removed, err := env.RedisRepo.RemoveUserReaction(ctx, userID, targetType, targetID, "❤️")
		if err != nil {
			t.Fatalf("RemoveUserReaction failed: %v", err)
		}
		if !removed {
			t.Error("Expected reaction to be removed")
		}

		// Verify removed
		exists, _ := env.RedisRepo.HasUserReaction(ctx, userID, targetType, targetID, "❤️")
		if exists {
			t.Error("Expected reaction to be removed")
		}

		// Removing again should return false (not found)
		removed, err = env.RedisRepo.RemoveUserReaction(ctx, userID, targetType, targetID, "❤️")
		if err != nil {
			t.Fatalf("RemoveUserReaction failed: %v", err)
		}
		if removed {
			t.Error("Expected reaction to not be found (already removed)")
		}
	})

	t.Run("GetUserReactionsForTarget", func(t *testing.T) {
		// Add multiple reactions
		_, _ = env.RedisRepo.AddUserReaction(ctx, userID, targetType, targetID, "🎉")
		_, _ = env.RedisRepo.AddUserReaction(ctx, userID, targetType, targetID, "🚀")

		emojis, err := env.RedisRepo.GetUserReactionsForTarget(ctx, userID, targetType, targetID)
		if err != nil {
			t.Fatalf("GetUserReactionsForTarget failed: %v", err)
		}

		if len(emojis) < 2 {
			t.Errorf("Expected at least 2 emojis, got %d", len(emojis))
		}
	})

	t.Run("GetUserReactionsForTargets", func(t *testing.T) {
		// Add reactions to different targets
		_, _ = env.RedisRepo.AddUserReaction(ctx, userID, "entry", "target1", "👍")
		_, _ = env.RedisRepo.AddUserReaction(ctx, userID, "entry", "target2", "❤️")
		_, _ = env.RedisRepo.AddUserReaction(ctx, userID, "comment", "target3", "🎉")

		targets := []testutil.ReactionTarget{
			{Type: "entry", ID: "target1"},
			{Type: "entry", ID: "target2"},
			{Type: "comment", ID: "target3"},
		}

		result, err := env.RedisRepo.GetUserReactionsForTargets(ctx, userID, targets)
		if err != nil {
			t.Fatalf("GetUserReactionsForTargets failed: %v", err)
		}

		if len(result) < 3 {
			t.Errorf("Expected 3 targets in result, got %d", len(result))
		}

		if emojis, ok := result["entry:target1"]; !ok || len(emojis) == 0 {
			t.Error("Expected emojis for entry:target1")
		}
	})
}

func TestRedisRepo_Ping(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := env.RedisRepo.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}
