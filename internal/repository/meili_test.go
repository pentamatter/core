package repository_test

import (
	"testing"
	"time"

	"matter-core/internal/model"
	"matter-core/internal/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestMeiliRepo_IndexAndSearch(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)

	if !env.HasMeili() {
		t.Skip("Skipping test: Meilisearch not available")
	}

	t.Run("IndexDocument", func(t *testing.T) {
		doc := model.SearchDocument{
			ID:        primitive.NewObjectID().Hex(),
			Title:     "Test Document",
			Body:      "This is a test document body",
			SchemaKey: "post",
			AllText:   "Test Document This is a test document body",
		}

		err := env.MeiliRepo.IndexDocument(doc)
		if err != nil {
			t.Fatalf("IndexDocument failed: %v", err)
		}

		// Wait for indexing to complete
		time.Sleep(500 * time.Millisecond)
	})

	t.Run("Search_Basic", func(t *testing.T) {
		// Index a document first
		doc := model.SearchDocument{
			ID:        primitive.NewObjectID().Hex(),
			Title:     "Golang Tutorial",
			Body:      "Learn Go programming language",
			SchemaKey: "post",
			AllText:   "Golang Tutorial Learn Go programming language",
		}
		_ = env.MeiliRepo.IndexDocument(doc)
		time.Sleep(500 * time.Millisecond)

		ids, total, err := env.MeiliRepo.Search("Golang", "", 10, 0)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if total == 0 {
			t.Error("Expected at least one result")
		}

		if len(ids) == 0 {
			t.Error("Expected at least one ID in results")
		}
	})

	t.Run("Search_WithSchemaFilter", func(t *testing.T) {
		// Index documents with different schema keys
		doc1 := model.SearchDocument{
			ID:        primitive.NewObjectID().Hex(),
			Title:     "Article One",
			Body:      "Content for article",
			SchemaKey: "article",
			AllText:   "Article One Content for article",
		}
		doc2 := model.SearchDocument{
			ID:        primitive.NewObjectID().Hex(),
			Title:     "Page One",
			Body:      "Content for page",
			SchemaKey: "page",
			AllText:   "Page One Content for page",
		}
		_ = env.MeiliRepo.IndexDocument(doc1)
		_ = env.MeiliRepo.IndexDocument(doc2)
		time.Sleep(500 * time.Millisecond)

		// Search with schema filter
		ids, _, err := env.MeiliRepo.Search("Content", "article", 10, 0)
		if err != nil {
			t.Fatalf("Search with filter failed: %v", err)
		}

		// Should only return article, not page
		for _, id := range ids {
			if id == doc2.ID {
				t.Error("Expected page document to be filtered out")
			}
		}
	})

	t.Run("Search_Pagination", func(t *testing.T) {
		// Index multiple documents
		for i := 0; i < 5; i++ {
			doc := model.SearchDocument{
				ID:        primitive.NewObjectID().Hex(),
				Title:     "Pagination Test",
				Body:      "Document for pagination testing",
				SchemaKey: "test",
				AllText:   "Pagination Test Document for pagination testing",
			}
			_ = env.MeiliRepo.IndexDocument(doc)
		}
		time.Sleep(500 * time.Millisecond)

		// Get first page
		ids1, _, err := env.MeiliRepo.Search("Pagination", "", 2, 0)
		if err != nil {
			t.Fatalf("Search page 1 failed: %v", err)
		}

		// Get second page
		ids2, _, err := env.MeiliRepo.Search("Pagination", "", 2, 2)
		if err != nil {
			t.Fatalf("Search page 2 failed: %v", err)
		}

		// Results should be different
		if len(ids1) > 0 && len(ids2) > 0 && ids1[0] == ids2[0] {
			t.Error("Expected different results for different pages")
		}
	})

	t.Run("Search_InvalidSchemaKey", func(t *testing.T) {
		// Try to search with invalid schema key (SQL injection attempt)
		_, _, err := env.MeiliRepo.Search("test", "'; DROP TABLE entries; --", 10, 0)
		if err == nil {
			t.Error("Expected error for invalid schema key")
		}
	})

	t.Run("DeleteDocument", func(t *testing.T) {
		doc := model.SearchDocument{
			ID:        primitive.NewObjectID().Hex(),
			Title:     "To Be Deleted",
			Body:      "This document will be deleted",
			SchemaKey: "post",
			AllText:   "To Be Deleted This document will be deleted",
		}
		_ = env.MeiliRepo.IndexDocument(doc)
		time.Sleep(500 * time.Millisecond)

		err := env.MeiliRepo.DeleteDocument(doc.ID)
		if err != nil {
			t.Fatalf("DeleteDocument failed: %v", err)
		}

		// Wait for deletion to complete
		time.Sleep(500 * time.Millisecond)

		// Search should not find the deleted document
		ids, _, _ := env.MeiliRepo.Search("To Be Deleted", "", 10, 0)
		for _, id := range ids {
			if id == doc.ID {
				t.Error("Deleted document should not appear in search results")
			}
		}
	})
}

func TestIsValidSchemaKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"post", true},
		{"blog-post", true},
		{"blog_post", true},
		{"Post123", true},
		{"", false},
		{"a very long schema key that exceeds the maximum allowed length of fifty characters", false},
		{"invalid key", false},
		{"invalid@key", false},
		{"'; DROP TABLE", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := testutil.IsValidSchemaKey(tt.key)
			if got != tt.want {
				t.Errorf("IsValidSchemaKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}
