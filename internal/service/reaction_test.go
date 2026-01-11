package service

import (
	"testing"

	"matter-core/internal/model"
	"matter-core/internal/testutil"
)

func TestReactionService(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)
	env.FlushRedis(t)

	ctx := env.Context()
	svc := NewReactionService(env.MongoRepo, env.RedisRepo)

	// Create test entry
	schema := &model.Schema{Key: "post", Version: 1, Name: "Post"}
	_ = env.MongoRepo.CreateSchema(ctx, schema)

	entry := &model.Entry{
		SchemaID:  schema.ID,
		SchemaKey: "post",
		Base:      model.BaseMeta{Title: "Test", Slug: "test"},
	}
	_ = env.MongoRepo.CreateEntry(ctx, entry)

	// Create test user
	user := &model.User{Nickname: "tester", Email: "tester@test.com"}
	_ = env.MongoRepo.CreateUser(ctx, user)
	userID := user.ID.Hex()
	entryID := entry.ID.Hex()

	t.Run("AddReaction", func(t *testing.T) {
		resp, err := svc.AddReaction(ctx, userID, "entry", entryID, "👍")
		if err != nil {
			t.Fatalf("AddReaction failed: %v", err)
		}

		if resp.Reactions["👍"] != 1 {
			t.Errorf("Expected 1 thumbs up, got %d", resp.Reactions["👍"])
		}

		// Check user reactions
		found := false
		for _, emoji := range resp.UserReactions {
			if emoji == "👍" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected user reaction to include 👍")
		}
	})

	t.Run("AddReaction_Duplicate", func(t *testing.T) {
		// First add
		_, _ = svc.AddReaction(ctx, userID, "entry", entryID, "❤️")

		// Second add should fail
		_, err := svc.AddReaction(ctx, userID, "entry", entryID, "❤️")
		if err != ErrReactionExists {
			t.Errorf("Expected ErrReactionExists, got %v", err)
		}
	})

	t.Run("AddReaction_InvalidEmoji", func(t *testing.T) {
		_, err := svc.AddReaction(ctx, userID, "entry", entryID, "not-an-emoji")
		if err != ErrInvalidEmoji {
			t.Errorf("Expected ErrInvalidEmoji, got %v", err)
		}
	})

	t.Run("AddReaction_InvalidTargetType", func(t *testing.T) {
		_, err := svc.AddReaction(ctx, userID, "invalid", entryID, "👍")
		if err != ErrInvalidTargetType {
			t.Errorf("Expected ErrInvalidTargetType, got %v", err)
		}
	})

	t.Run("AddReaction_TargetNotFound", func(t *testing.T) {
		_, err := svc.AddReaction(ctx, userID, "entry", "000000000000000000000000", "👍")
		if err != ErrTargetNotFound {
			t.Errorf("Expected ErrTargetNotFound, got %v", err)
		}
	})

	t.Run("RemoveReaction", func(t *testing.T) {
		// Add first
		_, _ = svc.AddReaction(ctx, userID, "entry", entryID, "🎉")

		// Remove
		resp, err := svc.RemoveReaction(ctx, userID, "entry", entryID, "🎉")
		if err != nil {
			t.Fatalf("RemoveReaction failed: %v", err)
		}

		// Check count is 0 or key doesn't exist
		if count, exists := resp.Reactions["🎉"]; exists && count > 0 {
			t.Errorf("Expected 0 party reactions, got %d", count)
		}
	})

	t.Run("RemoveReaction_NotFound", func(t *testing.T) {
		_, err := svc.RemoveReaction(ctx, userID, "entry", entryID, "🚀")
		if err != ErrReactionNotFound {
			t.Errorf("Expected ErrReactionNotFound, got %v", err)
		}
	})

	t.Run("GetReactions", func(t *testing.T) {
		// Add some reactions
		_, _ = svc.AddReaction(ctx, userID, "entry", entryID, "🔥")

		resp, err := svc.GetReactions(ctx, userID, "entry", entryID)
		if err != nil {
			t.Fatalf("GetReactions failed: %v", err)
		}

		if resp.Reactions == nil {
			t.Error("Expected reactions map to be non-nil")
		}
	})

	t.Run("GetReactions_Anonymous", func(t *testing.T) {
		resp, err := svc.GetReactions(ctx, "", "entry", entryID)
		if err != nil {
			t.Fatalf("GetReactions failed: %v", err)
		}

		// Anonymous user should have empty user reactions
		if len(resp.UserReactions) != 0 {
			t.Error("Expected empty user reactions for anonymous user")
		}
	})

	t.Run("GetReactionsBatch", func(t *testing.T) {
		// Create another entry
		entry2 := &model.Entry{
			SchemaID:  schema.ID,
			SchemaKey: "post",
			Base:      model.BaseMeta{Title: "Test2", Slug: "test2"},
		}
		_ = env.MongoRepo.CreateEntry(ctx, entry2)
		entry2ID := entry2.ID.Hex()

		// Add reactions to both
		_, _ = svc.AddReaction(ctx, userID, "entry", entry2ID, "👍")

		targets := []ReactionTarget{
			{Type: "entry", ID: entryID},
			{Type: "entry", ID: entry2ID},
		}

		result, err := svc.GetReactionsBatch(ctx, userID, targets)
		if err != nil {
			t.Fatalf("GetReactionsBatch failed: %v", err)
		}

		if len(result) != 2 {
			t.Errorf("Expected 2 results, got %d", len(result))
		}
	})

	t.Run("GetReactionsBatch_TooManyTargets", func(t *testing.T) {
		targets := make([]ReactionTarget, 101)
		for i := range targets {
			targets[i] = ReactionTarget{Type: "entry", ID: "fake"}
		}

		_, err := svc.GetReactionsBatch(ctx, userID, targets)
		if err != ErrTooManyTargets {
			t.Errorf("Expected ErrTooManyTargets, got %v", err)
		}
	})
}

func TestReactionService_Comment(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)
	env.FlushRedis(t)

	ctx := env.Context()
	svc := NewReactionService(env.MongoRepo, env.RedisRepo)

	// Create test entry and comment
	schema := &model.Schema{Key: "post", Version: 1, Name: "Post"}
	_ = env.MongoRepo.CreateSchema(ctx, schema)

	entry := &model.Entry{
		SchemaID:  schema.ID,
		SchemaKey: "post",
		Base:      model.BaseMeta{Title: "Test", Slug: "test"},
	}
	_ = env.MongoRepo.CreateEntry(ctx, entry)

	user := &model.User{Nickname: "tester", Email: "comment-tester@test.com"}
	_ = env.MongoRepo.CreateUser(ctx, user)

	comment := &model.Comment{
		EntryID:  entry.ID,
		AuthorID: user.ID.Hex(),
		Content:  "Test comment",
	}
	_ = env.MongoRepo.CreateComment(ctx, comment)

	userID := user.ID.Hex()
	commentID := comment.ID.Hex()

	t.Run("AddReactionToComment", func(t *testing.T) {
		resp, err := svc.AddReaction(ctx, userID, "comment", commentID, "👍")
		if err != nil {
			t.Fatalf("AddReaction to comment failed: %v", err)
		}

		if resp.Reactions["👍"] != 1 {
			t.Errorf("Expected 1 thumbs up, got %d", resp.Reactions["👍"])
		}
	})

	t.Run("RemoveReactionFromComment", func(t *testing.T) {
		_, _ = svc.AddReaction(ctx, userID, "comment", commentID, "❤️")

		resp, err := svc.RemoveReaction(ctx, userID, "comment", commentID, "❤️")
		if err != nil {
			t.Fatalf("RemoveReaction from comment failed: %v", err)
		}

		if count, exists := resp.Reactions["❤️"]; exists && count > 0 {
			t.Errorf("Expected 0 hearts, got %d", count)
		}
	})
}
