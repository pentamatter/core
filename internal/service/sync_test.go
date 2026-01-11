package service

import (
	"matter-core/internal/model"
	"matter-core/internal/testutil"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestStripMarkdown(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "headers",
			input: "# Title\n## Subtitle",
			want:  "Title\nSubtitle",
		},
		{
			name:  "bold",
			input: "This is **bold** text",
			want:  "This is bold text",
		},
		{
			name:  "italic",
			input: "This is *italic* text",
			want:  "This is italic text",
		},
		{
			name:  "links",
			input: "Check [this link](https://example.com)",
			want:  "Check this link",
		},
		{
			name:  "inline code",
			input: "Use `code` here",
			want:  "Use  here",
		},
		{
			name:  "plain text",
			input: "Just plain text",
			want:  "Just plain text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdown() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractStrings(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  int // expected number of strings
	}{
		{
			name:  "string",
			input: "hello",
			want:  1,
		},
		{
			name:  "array",
			input: []any{"one", "two", "three"},
			want:  3,
		},
		{
			name:  "map",
			input: map[string]any{"a": "one", "b": "two"},
			want:  2,
		},
		{
			name:  "nested",
			input: map[string]any{"a": []any{"one", "two"}},
			want:  2,
		},
		{
			name:  "number",
			input: 42,
			want:  1,
		},
		{
			name:  "nil",
			input: nil,
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractStrings(tt.input)
			if len(got) != tt.want {
				t.Errorf("extractStrings() returned %d strings, want %d", len(got), tt.want)
			}
		})
	}
}

func TestSyncService_EntryToSearchDoc(t *testing.T) {
	// Create a mock sync service (without real Meilisearch)
	svc := &SyncService{meiliRepo: nil}

	entry := &model.Entry{
		ID:        primitive.NewObjectID(),
		SchemaKey: "post",
		Base: model.BaseMeta{
			Title: "Test Post",
		},
		Body: "# Hello\n\nThis is **bold** content.",
		Attributes: map[string]any{
			"summary": "A test post",
			"tags":    []any{"go", "testing"},
		},
	}

	doc := svc.entryToSearchDoc(entry)

	if doc.ID != entry.ID.Hex() {
		t.Errorf("ID mismatch: got %s, want %s", doc.ID, entry.ID.Hex())
	}

	if doc.Title != entry.Base.Title {
		t.Errorf("Title mismatch: got %s, want %s", doc.Title, entry.Base.Title)
	}

	if doc.SchemaKey != entry.SchemaKey {
		t.Errorf("SchemaKey mismatch: got %s, want %s", doc.SchemaKey, entry.SchemaKey)
	}

	// Body should have markdown stripped
	if doc.Body == entry.Body {
		t.Error("Expected body to have markdown stripped")
	}

	// AllText should contain attribute values
	if doc.AllText == "" {
		t.Error("Expected AllText to be non-empty")
	}
}

func TestSyncService_ExtractTextFromAttributes(t *testing.T) {
	svc := &SyncService{meiliRepo: nil}

	attrs := map[string]any{
		"title":   "Hello World",
		"tags":    []any{"go", "testing"},
		"meta":    map[string]any{"author": "John"},
		"count":   42,
		"enabled": true,
	}

	text := svc.extractTextFromAttributes(attrs)

	if text == "" {
		t.Error("Expected non-empty text")
	}
}

// Integration tests with real Meilisearch

func TestSyncService_Integration(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)

	if !env.HasMeili() {
		t.Skip("Skipping test: Meilisearch not available")
	}

	svc := NewSyncService(env.MeiliRepo)

	t.Run("SyncEntry", func(t *testing.T) {
		entry := &model.Entry{
			ID:        primitive.NewObjectID(),
			SchemaKey: "post",
			Base: model.BaseMeta{
				Title: "Integration Test Post",
				Slug:  "integration-test",
			},
			Body: "This is a test post for integration testing",
			Attributes: map[string]any{
				"summary": "Test summary",
			},
		}

		err := svc.SyncEntry(entry)
		if err != nil {
			t.Fatalf("SyncEntry failed: %v", err)
		}

		// Wait for indexing
		time.Sleep(500 * time.Millisecond)

		// Verify by searching
		ids, _, err := env.MeiliRepo.Search("Integration Test", "", 10, 0)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		found := false
		for _, id := range ids {
			if id == entry.ID.Hex() {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find synced entry in search results")
		}
	})

	t.Run("DeleteEntry", func(t *testing.T) {
		entry := &model.Entry{
			ID:        primitive.NewObjectID(),
			SchemaKey: "post",
			Base: model.BaseMeta{
				Title: "To Delete Entry",
				Slug:  "to-delete",
			},
			Body: "This entry will be deleted",
		}

		// Sync first
		_ = svc.SyncEntry(entry)
		time.Sleep(500 * time.Millisecond)

		// Delete
		err := svc.DeleteEntry(entry.ID.Hex())
		if err != nil {
			t.Fatalf("DeleteEntry failed: %v", err)
		}

		// Wait for deletion
		time.Sleep(500 * time.Millisecond)

		// Verify deletion
		ids, _, _ := env.MeiliRepo.Search("To Delete Entry", "", 10, 0)
		for _, id := range ids {
			if id == entry.ID.Hex() {
				t.Error("Deleted entry should not appear in search results")
			}
		}
	})
}
