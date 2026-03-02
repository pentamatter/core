package repository_test

import (
	"testing"
	"time"

	"matter-core/internal/model"
	"matter-core/internal/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestMongoRepo_Schema(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)

	ctx := env.Context()

	t.Run("CreateAndGetSchema", func(t *testing.T) {
		schema := &model.Schema{
			Key:     "post",
			Version: 1,
			Name:    "Blog Post",
			Fields: []model.FieldSchema{
				{Key: "title", Label: "Title", Type: model.TypeString, Required: true},
				{Key: "content", Label: "Content", Type: model.TypeString},
			},
		}

		err := env.MongoRepo.CreateSchema(ctx, schema)
		if err != nil {
			t.Fatalf("CreateSchema failed: %v", err)
		}

		if schema.ID.IsZero() {
			t.Error("Expected schema ID to be set after creation")
		}

		// Get latest schema
		got, err := env.MongoRepo.GetLatestSchema(ctx, "post")
		if err != nil {
			t.Fatalf("GetLatestSchema failed: %v", err)
		}

		if got.Key != schema.Key || got.Version != schema.Version {
			t.Errorf("Schema mismatch: got %+v, want %+v", got, schema)
		}
	})

	t.Run("GetSchemaByID", func(t *testing.T) {
		schema := &model.Schema{
			Key:     "page",
			Version: 1,
			Name:    "Page",
			Fields:  []model.FieldSchema{},
		}
		_ = env.MongoRepo.CreateSchema(ctx, schema)

		got, err := env.MongoRepo.GetSchemaByID(ctx, schema.ID)
		if err != nil {
			t.Fatalf("GetSchemaByID failed: %v", err)
		}

		if got.ID != schema.ID {
			t.Errorf("ID mismatch: got %v, want %v", got.ID, schema.ID)
		}
	})

	t.Run("ListSchemas", func(t *testing.T) {
		// Create multiple versions
		for i := 1; i <= 3; i++ {
			_ = env.MongoRepo.CreateSchema(ctx, &model.Schema{
				Key:     "article",
				Version: i,
				Name:    "Article",
			})
		}

		schemas, err := env.MongoRepo.ListSchemas(ctx)
		if err != nil {
			t.Fatalf("ListSchemas failed: %v", err)
		}

		if len(schemas) == 0 {
			t.Error("Expected at least one schema")
		}
	})
}

func TestMongoRepo_Entry(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)

	ctx := env.Context()

	// Create a schema first
	schema := &model.Schema{Key: "post", Version: 1, Name: "Post"}
	_ = env.MongoRepo.CreateSchema(ctx, schema)

	t.Run("CreateAndGetEntry", func(t *testing.T) {
		entry := &model.Entry{
			SchemaID:      schema.ID,
			SchemaKey:     "post",
			SchemaVersion: 1,
			AuthorID:      "user123",
			Base: model.BaseMeta{
				Title: "Test Post",
				Slug:  "test-post",
				Draft: false,
			},
			Body:       "This is test content",
			Attributes: map[string]any{"featured": true},
		}

		err := env.MongoRepo.CreateEntry(ctx, entry)
		if err != nil {
			t.Fatalf("CreateEntry failed: %v", err)
		}

		if entry.ID.IsZero() {
			t.Error("Expected entry ID to be set")
		}

		got, err := env.MongoRepo.GetEntryByID(ctx, entry.ID)
		if err != nil {
			t.Fatalf("GetEntryByID failed: %v", err)
		}

		if got.Base.Title != entry.Base.Title {
			t.Errorf("Title mismatch: got %s, want %s", got.Base.Title, entry.Base.Title)
		}
	})

	t.Run("UpdateEntry", func(t *testing.T) {
		entry := &model.Entry{
			SchemaID:  schema.ID,
			SchemaKey: "post",
			Base:      model.BaseMeta{Title: "Original", Slug: "original"},
		}
		_ = env.MongoRepo.CreateEntry(ctx, entry)

		entry.Base.Title = "Updated"
		err := env.MongoRepo.UpdateEntry(ctx, entry)
		if err != nil {
			t.Fatalf("UpdateEntry failed: %v", err)
		}

		got, _ := env.MongoRepo.GetEntryByID(ctx, entry.ID)
		if got.Base.Title != "Updated" {
			t.Errorf("Expected title 'Updated', got %s", got.Base.Title)
		}
	})

	t.Run("ListEntries", func(t *testing.T) {
		// Create multiple entries
		for i := 0; i < 5; i++ {
			_ = env.MongoRepo.CreateEntry(ctx, &model.Entry{
				SchemaID:  schema.ID,
				SchemaKey: "post",
				Base:      model.BaseMeta{Title: "List Test", Slug: "list-test", Draft: false},
			})
		}

		entries, err := env.MongoRepo.ListEntries(ctx, "post", nil, "", 10, 0)
		if err != nil {
			t.Fatalf("ListEntries failed: %v", err)
		}

		if len(entries) < 5 {
			t.Errorf("Expected at least 5 entries, got %d", len(entries))
		}
	})

	t.Run("CountEntries", func(t *testing.T) {
		count, err := env.MongoRepo.CountEntries(ctx, "post", nil, "")
		if err != nil {
			t.Fatalf("CountEntries failed: %v", err)
		}

		if count == 0 {
			t.Error("Expected count > 0")
		}
	})

	t.Run("DeleteEntry", func(t *testing.T) {
		entry := &model.Entry{
			SchemaID:  schema.ID,
			SchemaKey: "post",
			Base:      model.BaseMeta{Title: "To Delete", Slug: "to-delete"},
		}
		_ = env.MongoRepo.CreateEntry(ctx, entry)

		err := env.MongoRepo.DeleteEntry(ctx, entry.ID)
		if err != nil {
			t.Fatalf("DeleteEntry failed: %v", err)
		}

		_, err = env.MongoRepo.GetEntryByID(ctx, entry.ID)
		if err == nil {
			t.Error("Expected error when getting deleted entry")
		}
	})
}

func TestMongoRepo_User(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)

	ctx := env.Context()

	t.Run("CreateAndGetUser", func(t *testing.T) {
		user := &model.User{
			Role:     "user",
			Nickname: "testuser",
			Email:    "test@example.com",
			Avatar:   "https://example.com/avatar.png",
			Socials: []model.SocialBind{
				{Provider: "github", ProviderUserID: "12345", Name: "testuser"},
			},
		}

		err := env.MongoRepo.CreateUser(ctx, user)
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		got, err := env.MongoRepo.GetUserByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetUserByID failed: %v", err)
		}

		if got.Nickname != user.Nickname {
			t.Errorf("Nickname mismatch: got %s, want %s", got.Nickname, user.Nickname)
		}
	})

	t.Run("GetUserBySocial", func(t *testing.T) {
		user := &model.User{
			Role:     "user",
			Nickname: "socialuser",
			Email:    "social@example.com",
			Socials: []model.SocialBind{
				{Provider: "google", ProviderUserID: "google123", Name: "socialuser"},
			},
		}
		_ = env.MongoRepo.CreateUser(ctx, user)

		got, err := env.MongoRepo.GetUserBySocial(ctx, "google", "google123")
		if err != nil {
			t.Fatalf("GetUserBySocial failed: %v", err)
		}

		if got.ID != user.ID {
			t.Errorf("User ID mismatch")
		}
	})

	t.Run("GetUserByEmail", func(t *testing.T) {
		user := &model.User{
			Role:     "user",
			Nickname: "emailuser",
			Email:    "unique@example.com",
		}
		_ = env.MongoRepo.CreateUser(ctx, user)

		got, err := env.MongoRepo.GetUserByEmail(ctx, "unique@example.com")
		if err != nil {
			t.Fatalf("GetUserByEmail failed: %v", err)
		}

		if got.ID != user.ID {
			t.Errorf("User ID mismatch")
		}
	})
}

func TestMongoRepo_Taxonomy(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)

	ctx := env.Context()

	t.Run("CreateAndGetTaxonomy", func(t *testing.T) {
		tax := &model.Taxonomy{
			Key:            "category",
			Name:           "Category",
			IsHierarchical: true,
		}

		err := env.MongoRepo.CreateTaxonomy(ctx, tax)
		if err != nil {
			t.Fatalf("CreateTaxonomy failed: %v", err)
		}

		got, err := env.MongoRepo.GetTaxonomyByKey(ctx, "category")
		if err != nil {
			t.Fatalf("GetTaxonomyByKey failed: %v", err)
		}

		if got.Name != tax.Name {
			t.Errorf("Name mismatch: got %s, want %s", got.Name, tax.Name)
		}
	})

	t.Run("ListTaxonomies", func(t *testing.T) {
		_ = env.MongoRepo.CreateTaxonomy(ctx, &model.Taxonomy{Key: "tag", Name: "Tag"})

		taxonomies, err := env.MongoRepo.ListTaxonomies(ctx)
		if err != nil {
			t.Fatalf("ListTaxonomies failed: %v", err)
		}

		if len(taxonomies) == 0 {
			t.Error("Expected at least one taxonomy")
		}
	})
}

func TestMongoRepo_Term(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)

	ctx := env.Context()

	// Create taxonomy first
	tax := &model.Taxonomy{Key: "category", Name: "Category"}
	_ = env.MongoRepo.CreateTaxonomy(ctx, tax)

	t.Run("CreateAndGetTerm", func(t *testing.T) {
		term := &model.Term{
			TaxonomyKey: "category",
			Name:        "Technology",
			Slug:        "technology",
			Color:       "#3498db",
		}

		err := env.MongoRepo.CreateTerm(ctx, term)
		if err != nil {
			t.Fatalf("CreateTerm failed: %v", err)
		}

		got, err := env.MongoRepo.GetTermByID(ctx, term.ID)
		if err != nil {
			t.Fatalf("GetTermByID failed: %v", err)
		}

		if got.Name != term.Name {
			t.Errorf("Name mismatch: got %s, want %s", got.Name, term.Name)
		}
	})

	t.Run("GetTermBySlug", func(t *testing.T) {
		term := &model.Term{
			TaxonomyKey: "category",
			Name:        "Science",
			Slug:        "science",
		}
		_ = env.MongoRepo.CreateTerm(ctx, term)

		got, err := env.MongoRepo.GetTermBySlug(ctx, "category", "science")
		if err != nil {
			t.Fatalf("GetTermBySlug failed: %v", err)
		}

		if got.ID != term.ID {
			t.Errorf("Term ID mismatch")
		}
	})

	t.Run("GetTermsByTaxonomy", func(t *testing.T) {
		terms, err := env.MongoRepo.GetTermsByTaxonomy(ctx, "category")
		if err != nil {
			t.Fatalf("GetTermsByTaxonomy failed: %v", err)
		}

		if len(terms) == 0 {
			t.Error("Expected at least one term")
		}
	})
}

func TestMongoRepo_Comment(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)

	ctx := env.Context()

	// Create entry first
	schema := &model.Schema{Key: "post", Version: 1, Name: "Post"}
	_ = env.MongoRepo.CreateSchema(ctx, schema)

	entry := &model.Entry{
		SchemaID:  schema.ID,
		SchemaKey: "post",
		Base:      model.BaseMeta{Title: "Test", Slug: "test"},
	}
	_ = env.MongoRepo.CreateEntry(ctx, entry)

	// Create user
	user := &model.User{Nickname: "commenter", Email: "commenter@test.com"}
	_ = env.MongoRepo.CreateUser(ctx, user)

	t.Run("CreateAndGetComment", func(t *testing.T) {
		comment := &model.Comment{
			EntryID:  entry.ID,
			AuthorID: user.ID.Hex(),
			Content:  "This is a test comment",
		}

		err := env.MongoRepo.CreateComment(ctx, comment)
		if err != nil {
			t.Fatalf("CreateComment failed: %v", err)
		}

		got, err := env.MongoRepo.GetCommentByID(ctx, comment.ID)
		if err != nil {
			t.Fatalf("GetCommentByID failed: %v", err)
		}

		if got.Content != comment.Content {
			t.Errorf("Content mismatch: got %s, want %s", got.Content, comment.Content)
		}
	})

	t.Run("GetCommentsByEntry", func(t *testing.T) {
		// Create more comments
		for i := 0; i < 3; i++ {
			_ = env.MongoRepo.CreateComment(ctx, &model.Comment{
				EntryID:  entry.ID,
				AuthorID: user.ID.Hex(),
				Content:  "Comment",
			})
		}

		comments, err := env.MongoRepo.GetCommentsByEntry(ctx, entry.ID)
		if err != nil {
			t.Fatalf("GetCommentsByEntry failed: %v", err)
		}

		if len(comments) < 3 {
			t.Errorf("Expected at least 3 comments, got %d", len(comments))
		}
	})

	t.Run("CountCommentsByEntry", func(t *testing.T) {
		count, err := env.MongoRepo.CountCommentsByEntry(ctx, entry.ID)
		if err != nil {
			t.Fatalf("CountCommentsByEntry failed: %v", err)
		}

		if count == 0 {
			t.Error("Expected count > 0")
		}
	})

	t.Run("DeleteComment", func(t *testing.T) {
		comment := &model.Comment{
			EntryID:  entry.ID,
			AuthorID: user.ID.Hex(),
			Content:  "To delete",
		}
		_ = env.MongoRepo.CreateComment(ctx, comment)

		err := env.MongoRepo.DeleteComment(ctx, comment.ID)
		if err != nil {
			t.Fatalf("DeleteComment failed: %v", err)
		}

		_, err = env.MongoRepo.GetCommentByID(ctx, comment.ID)
		if err == nil {
			t.Error("Expected error when getting deleted comment")
		}
	})
}

func TestMongoRepo_Session(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)

	ctx := env.Context()

	t.Run("CreateAndGetSession", func(t *testing.T) {
		userID := primitive.NewObjectID()
		session := &model.Session{
			Token:     "test-token-123",
			UserID:    userID,
			Role:      "user",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		err := env.MongoRepo.CreateSession(ctx, session)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		got, err := env.MongoRepo.GetSessionByToken(ctx, "test-token-123")
		if err != nil {
			t.Fatalf("GetSessionByToken failed: %v", err)
		}

		if got.UserID != userID {
			t.Errorf("UserID mismatch")
		}
	})

	t.Run("DeleteSession", func(t *testing.T) {
		session := &model.Session{
			Token:     "to-delete-token",
			UserID:    primitive.NewObjectID(),
			Role:      "user",
			ExpiresAt: time.Now().Add(time.Hour),
		}
		_ = env.MongoRepo.CreateSession(ctx, session)

		err := env.MongoRepo.DeleteSession(ctx, "to-delete-token")
		if err != nil {
			t.Fatalf("DeleteSession failed: %v", err)
		}

		_, err = env.MongoRepo.GetSessionByToken(ctx, "to-delete-token")
		if err == nil {
			t.Error("Expected error when getting deleted session")
		}
	})

	t.Run("ExpiredSessionNotReturned", func(t *testing.T) {
		session := &model.Session{
			Token:     "expired-token",
			UserID:    primitive.NewObjectID(),
			Role:      "user",
			ExpiresAt: time.Now().Add(-time.Hour), // Already expired
		}
		_ = env.MongoRepo.CreateSession(ctx, session)

		_, err := env.MongoRepo.GetSessionByToken(ctx, "expired-token")
		if err == nil {
			t.Error("Expected error when getting expired session")
		}
	})
}

func TestMongoRepo_Reaction(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)

	ctx := env.Context()

	// Create entry for reactions
	schema := &model.Schema{Key: "post", Version: 1, Name: "Post"}
	_ = env.MongoRepo.CreateSchema(ctx, schema)

	entry := &model.Entry{
		SchemaID:  schema.ID,
		SchemaKey: "post",
		Base:      model.BaseMeta{Title: "Reaction Test", Slug: "reaction-test"},
	}
	_ = env.MongoRepo.CreateEntry(ctx, entry)

	t.Run("IncrementReaction", func(t *testing.T) {
		err := env.MongoRepo.IncrementReaction(ctx, model.TargetEntry, entry.ID, "👍")
		if err != nil {
			t.Fatalf("IncrementReaction failed: %v", err)
		}

		// Increment again
		err = env.MongoRepo.IncrementReaction(ctx, model.TargetEntry, entry.ID, "👍")
		if err != nil {
			t.Fatalf("IncrementReaction second time failed: %v", err)
		}

		summary, err := env.MongoRepo.GetReactionSummary(ctx, model.TargetEntry, entry.ID)
		if err != nil {
			t.Fatalf("GetReactionSummary failed: %v", err)
		}

		if summary.Reactions["👍"] != 2 {
			t.Errorf("Expected 2 thumbs up, got %d", summary.Reactions["👍"])
		}
	})

	t.Run("DecrementReaction", func(t *testing.T) {
		// Add some reactions first
		_ = env.MongoRepo.IncrementReaction(ctx, model.TargetEntry, entry.ID, "❤️")
		_ = env.MongoRepo.IncrementReaction(ctx, model.TargetEntry, entry.ID, "❤️")

		err := env.MongoRepo.DecrementReaction(ctx, model.TargetEntry, entry.ID, "❤️")
		if err != nil {
			t.Fatalf("DecrementReaction failed: %v", err)
		}

		summary, _ := env.MongoRepo.GetReactionSummary(ctx, model.TargetEntry, entry.ID)
		if summary.Reactions["❤️"] != 1 {
			t.Errorf("Expected 1 heart, got %d", summary.Reactions["❤️"])
		}
	})

	t.Run("GetReactionSummaries", func(t *testing.T) {
		// Create another entry
		entry2 := &model.Entry{
			SchemaID:  schema.ID,
			SchemaKey: "post",
			Base:      model.BaseMeta{Title: "Another", Slug: "another"},
		}
		_ = env.MongoRepo.CreateEntry(ctx, entry2)
		_ = env.MongoRepo.IncrementReaction(ctx, model.TargetEntry, entry2.ID, "🎉")

		targets := []testutil.MongoReactionTarget{
			{Type: model.TargetEntry, ID: entry.ID},
			{Type: model.TargetEntry, ID: entry2.ID},
		}

		summaries, err := env.MongoRepo.GetReactionSummaries(ctx, targets)
		if err != nil {
			t.Fatalf("GetReactionSummaries failed: %v", err)
		}

		if len(summaries) < 2 {
			t.Errorf("Expected at least 2 summaries, got %d", len(summaries))
		}
	})
}

func TestMongoRepo_OAuthState(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)

	ctx := env.Context()

	t.Run("CreateAndGetOAuthState", func(t *testing.T) {
		state := &model.OAuthState{
			State:     "random-state-123",
			ExpiresAt: time.Now().Add(10 * time.Minute),
		}

		err := env.MongoRepo.CreateOAuthState(ctx, state)
		if err != nil {
			t.Fatalf("CreateOAuthState failed: %v", err)
		}

		got, err := env.MongoRepo.GetAndDeleteOAuthState(ctx, "random-state-123")
		if err != nil {
			t.Fatalf("GetAndDeleteOAuthState failed: %v", err)
		}

		if got.State != state.State {
			t.Errorf("State mismatch")
		}

		// Should not find it again (was deleted)
		_, err = env.MongoRepo.GetAndDeleteOAuthState(ctx, "random-state-123")
		if err == nil {
			t.Error("Expected error when getting deleted state")
		}
	})
}
