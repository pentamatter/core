package service

import (
	"testing"

	"matter-core/internal/model"
	"matter-core/internal/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestSchemaValidator(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)

	validator := NewSchemaValidator(env.MongoRepo)

	t.Run("ValidateEntry_RequiredField", func(t *testing.T) {
		schema := model.Schema{
			Fields: []model.FieldSchema{
				{Key: "title", Type: model.TypeString, Required: true},
			},
		}

		// Missing required field
		err := validator.ValidateEntry(schema, map[string]any{})
		if err == nil {
			t.Error("Expected error for missing required field")
		}

		// With required field
		err = validator.ValidateEntry(schema, map[string]any{"title": "Test"})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("ValidateEntry_StringType", func(t *testing.T) {
		schema := model.Schema{
			Fields: []model.FieldSchema{
				{Key: "name", Type: model.TypeString},
			},
		}

		// Valid string
		err := validator.ValidateEntry(schema, map[string]any{"name": "John"})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		// Invalid type
		err = validator.ValidateEntry(schema, map[string]any{"name": 123})
		if err == nil {
			t.Error("Expected error for invalid type")
		}
	})

	t.Run("ValidateEntry_NumberType", func(t *testing.T) {
		schema := model.Schema{
			Fields: []model.FieldSchema{
				{Key: "count", Type: model.TypeNumber},
			},
		}

		// Valid numbers
		testCases := []any{float64(42), float32(3.14), int(10), int32(20), int64(30)}
		for _, val := range testCases {
			err := validator.ValidateEntry(schema, map[string]any{"count": val})
			if err != nil {
				t.Errorf("Unexpected error for %T: %v", val, err)
			}
		}

		// Invalid type
		err := validator.ValidateEntry(schema, map[string]any{"count": "not a number"})
		if err == nil {
			t.Error("Expected error for invalid type")
		}
	})

	t.Run("ValidateEntry_BoolType", func(t *testing.T) {
		schema := model.Schema{
			Fields: []model.FieldSchema{
				{Key: "active", Type: model.TypeBool},
			},
		}

		err := validator.ValidateEntry(schema, map[string]any{"active": true})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		err = validator.ValidateEntry(schema, map[string]any{"active": "true"})
		if err == nil {
			t.Error("Expected error for string instead of bool")
		}
	})

	t.Run("ValidateEntry_DateType", func(t *testing.T) {
		schema := model.Schema{
			Fields: []model.FieldSchema{
				{Key: "published", Type: model.TypeDate},
			},
		}

		// Valid RFC3339 date
		err := validator.ValidateEntry(schema, map[string]any{"published": "2024-01-15T10:30:00Z"})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		// Invalid date format
		err = validator.ValidateEntry(schema, map[string]any{"published": "2024-01-15"})
		if err == nil {
			t.Error("Expected error for invalid date format")
		}
	})

	t.Run("ValidateEntry_ObjectType", func(t *testing.T) {
		schema := model.Schema{
			Fields: []model.FieldSchema{
				{
					Key:  "meta",
					Type: model.TypeObject,
					Children: []model.FieldSchema{
						{Key: "author", Type: model.TypeString, Required: true},
					},
				},
			},
		}

		// Valid object
		err := validator.ValidateEntry(schema, map[string]any{
			"meta": map[string]any{"author": "John"},
		})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		// Missing required child
		err = validator.ValidateEntry(schema, map[string]any{
			"meta": map[string]any{},
		})
		if err == nil {
			t.Error("Expected error for missing required child field")
		}
	})

	t.Run("ValidateEntry_ArrayType", func(t *testing.T) {
		schema := model.Schema{
			Fields: []model.FieldSchema{
				{
					Key:      "tags",
					Type:     model.TypeArray,
					ItemType: &model.FieldSchema{Type: model.TypeString},
				},
			},
		}

		// Valid array
		err := validator.ValidateEntry(schema, map[string]any{
			"tags": []any{"go", "testing"},
		})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		// Invalid item type
		err = validator.ValidateEntry(schema, map[string]any{
			"tags": []any{"go", 123},
		})
		if err == nil {
			t.Error("Expected error for invalid array item type")
		}
	})
}

func TestSchemaValidator_Taxonomy(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)

	ctx := env.Context()
	validator := NewSchemaValidator(env.MongoRepo)

	// Create taxonomy and term
	tax := &model.Taxonomy{Key: "category", Name: "Category"}
	_ = env.MongoRepo.CreateTaxonomy(ctx, tax)

	term := &model.Term{TaxonomyKey: "category", Name: "Tech", Slug: "tech"}
	_ = env.MongoRepo.CreateTerm(ctx, term)

	t.Run("ValidateEntry_TaxonomyType_Single", func(t *testing.T) {
		schema := model.Schema{
			Fields: []model.FieldSchema{
				{Key: "category", Type: model.TypeTaxonomy, TaxonomyKey: "category"},
			},
		}

		// Valid term ID
		err := validator.ValidateEntry(schema, map[string]any{
			"category": term.ID.Hex(),
		})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		// Invalid term ID
		err = validator.ValidateEntry(schema, map[string]any{
			"category": primitive.NewObjectID().Hex(),
		})
		if err == nil {
			t.Error("Expected error for non-existent term")
		}
	})

	t.Run("ValidateEntry_TaxonomyType_Multiple", func(t *testing.T) {
		// Create another term
		term2 := &model.Term{TaxonomyKey: "category", Name: "Science", Slug: "science"}
		_ = env.MongoRepo.CreateTerm(ctx, term2)

		schema := model.Schema{
			Fields: []model.FieldSchema{
				{Key: "categories", Type: model.TypeTaxonomy, TaxonomyKey: "category", AllowMultiple: true},
			},
		}

		// Valid multiple terms
		err := validator.ValidateEntry(schema, map[string]any{
			"categories": []any{term.ID.Hex(), term2.ID.Hex()},
		})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})
}

func TestSchemaValidator_ExtractTermIDs(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup(t)

	ctx := env.Context()
	validator := NewSchemaValidator(env.MongoRepo)

	// Create terms
	term1 := &model.Term{TaxonomyKey: "category", Name: "Tech", Slug: "tech"}
	_ = env.MongoRepo.CreateTerm(ctx, term1)
	term2 := &model.Term{TaxonomyKey: "tag", Name: "Go", Slug: "go"}
	_ = env.MongoRepo.CreateTerm(ctx, term2)

	t.Run("ExtractTermIDs_Single", func(t *testing.T) {
		schema := model.Schema{
			Fields: []model.FieldSchema{
				{Key: "category", Type: model.TypeTaxonomy, TaxonomyKey: "category"},
			},
		}

		data := map[string]any{"category": term1.ID.Hex()}
		termIDs := validator.ExtractTermIDs(schema, data)

		if len(termIDs) != 1 {
			t.Errorf("Expected 1 term ID, got %d", len(termIDs))
		}
		if termIDs[0] != term1.ID {
			t.Error("Term ID mismatch")
		}
	})

	t.Run("ExtractTermIDs_Multiple", func(t *testing.T) {
		schema := model.Schema{
			Fields: []model.FieldSchema{
				{Key: "tags", Type: model.TypeTaxonomy, TaxonomyKey: "tag", AllowMultiple: true},
			},
		}

		data := map[string]any{"tags": []any{term1.ID.Hex(), term2.ID.Hex()}}
		termIDs := validator.ExtractTermIDs(schema, data)

		if len(termIDs) != 2 {
			t.Errorf("Expected 2 term IDs, got %d", len(termIDs))
		}
	})

	t.Run("ExtractTermIDs_Nested", func(t *testing.T) {
		schema := model.Schema{
			Fields: []model.FieldSchema{
				{
					Key:  "meta",
					Type: model.TypeObject,
					Children: []model.FieldSchema{
						{Key: "category", Type: model.TypeTaxonomy, TaxonomyKey: "category"},
					},
				},
			},
		}

		data := map[string]any{
			"meta": map[string]any{"category": term1.ID.Hex()},
		}
		termIDs := validator.ExtractTermIDs(schema, data)

		if len(termIDs) != 1 {
			t.Errorf("Expected 1 term ID, got %d", len(termIDs))
		}
	})
}
