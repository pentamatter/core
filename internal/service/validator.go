package service

import (
	"context"
	"fmt"
	"time"

	"matter-core/internal/model"
	"matter-core/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type SchemaValidator struct {
	mongoRepo *repository.MongoRepo
}

func NewSchemaValidator(mongoRepo *repository.MongoRepo) *SchemaValidator {
	return &SchemaValidator{mongoRepo: mongoRepo}
}

func (v *SchemaValidator) ValidateEntry(schema model.Schema, data map[string]any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return v.validateFields(ctx, schema.Fields, data)
}

func (v *SchemaValidator) validateFields(ctx context.Context, fields []model.FieldSchema, data map[string]any) error {
	for _, field := range fields {
		value, exists := data[field.Key]

		if field.Required && !exists {
			return fmt.Errorf("required field '%s' is missing", field.Key)
		}

		if !exists {
			continue
		}

		if err := v.validateFieldType(ctx, field, value); err != nil {
			return err
		}
	}
	return nil
}

func (v *SchemaValidator) validateFieldType(ctx context.Context, field model.FieldSchema, value interface{}) error {
	if value == nil {
		if field.Required {
			return fmt.Errorf("field '%s' cannot be null", field.Key)
		}
		return nil
	}

	switch field.Type {
	case model.TypeString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field '%s' must be a string", field.Key)
		}

	case model.TypeNumber:
		switch value.(type) {
		case float64, float32, int, int32, int64:
			// valid
		default:
			return fmt.Errorf("field '%s' must be a number", field.Key)
		}

	case model.TypeBool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("field '%s' must be a boolean", field.Key)
		}

	case model.TypeDate:
		switch val := value.(type) {
		case string:
			if _, err := time.Parse(time.RFC3339, val); err != nil {
				return fmt.Errorf("field '%s' must be a valid date (RFC3339)", field.Key)
			}
		case time.Time:
			// valid
		default:
			return fmt.Errorf("field '%s' must be a date", field.Key)
		}

	case model.TypeObject:
		obj, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("field '%s' must be an object", field.Key)
		}
		if len(field.Children) > 0 {
			if err := v.validateFields(ctx, field.Children, obj); err != nil {
				return err
			}
		}

	case model.TypeArray:
		arr, ok := value.([]any)
		if !ok {
			return fmt.Errorf("field '%s' must be an array", field.Key)
		}
		if field.ItemType != nil {
			for i, item := range arr {
				if err := v.validateFieldType(ctx, *field.ItemType, item); err != nil {
					return fmt.Errorf("field '%s[%d]': %w", field.Key, i, err)
				}
			}
		}

	case model.TypeTaxonomy:
		if err := v.validateTaxonomyField(ctx, field, value); err != nil {
			return err
		}
	}

	return nil
}

func (v *SchemaValidator) validateTaxonomyField(ctx context.Context, field model.FieldSchema, value interface{}) error {
	validateTermID := func(termIDStr string) error {
		termID, err := primitive.ObjectIDFromHex(termIDStr)
		if err != nil {
			return fmt.Errorf("field '%s': invalid term ID format", field.Key)
		}
		term, err := v.mongoRepo.GetTermByID(ctx, termID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return fmt.Errorf("field '%s': term '%s' not found", field.Key, termIDStr)
			}
			return fmt.Errorf("field '%s': failed to validate term", field.Key)
		}
		if field.TaxonomyKey != "" && term.TaxonomyKey != field.TaxonomyKey {
			return fmt.Errorf("field '%s': term '%s' belongs to wrong taxonomy", field.Key, termIDStr)
		}
		return nil
	}

	if field.AllowMultiple {
		arr, ok := value.([]any)
		if !ok {
			return fmt.Errorf("field '%s' must be an array of term IDs", field.Key)
		}
		for _, item := range arr {
			termIDStr, ok := item.(string)
			if !ok {
				return fmt.Errorf("field '%s' must contain string term IDs", field.Key)
			}
			if err := validateTermID(termIDStr); err != nil {
				return err
			}
		}
	} else {
		termIDStr, ok := value.(string)
		if !ok {
			return fmt.Errorf("field '%s' must be a term ID string", field.Key)
		}
		if err := validateTermID(termIDStr); err != nil {
			return err
		}
	}
	return nil
}

// ExtractTermIDs 从 attributes 中提取所有 taxonomy 类型字段的 term ID
func (v *SchemaValidator) ExtractTermIDs(schema model.Schema, data map[string]any) []primitive.ObjectID {
	var termIDs []primitive.ObjectID
	seen := make(map[primitive.ObjectID]bool)

	v.extractTermIDsFromFields(schema.Fields, data, &termIDs, seen)
	return termIDs
}

func (v *SchemaValidator) extractTermIDsFromFields(fields []model.FieldSchema, data map[string]any, termIDs *[]primitive.ObjectID, seen map[primitive.ObjectID]bool) {
	for _, field := range fields {
		value, exists := data[field.Key]
		if !exists || value == nil {
			continue
		}

		switch field.Type {
		case model.TypeTaxonomy:
			v.extractTermIDsFromTaxonomyField(field, value, termIDs, seen)
		case model.TypeObject:
			if obj, ok := value.(map[string]any); ok && len(field.Children) > 0 {
				v.extractTermIDsFromFields(field.Children, obj, termIDs, seen)
			}
		case model.TypeArray:
			if arr, ok := value.([]any); ok && field.ItemType != nil && field.ItemType.Type == model.TypeObject {
				for _, item := range arr {
					if obj, ok := item.(map[string]any); ok {
						v.extractTermIDsFromFields(field.ItemType.Children, obj, termIDs, seen)
					}
				}
			}
		}
	}
}

func (v *SchemaValidator) extractTermIDsFromTaxonomyField(field model.FieldSchema, value interface{}, termIDs *[]primitive.ObjectID, seen map[primitive.ObjectID]bool) {
	addTermID := func(termIDStr string) {
		if oid, err := primitive.ObjectIDFromHex(termIDStr); err == nil {
			if !seen[oid] {
				seen[oid] = true
				*termIDs = append(*termIDs, oid)
			}
		}
	}

	if field.AllowMultiple {
		if arr, ok := value.([]any); ok {
			for _, item := range arr {
				if termIDStr, ok := item.(string); ok {
					addTermID(termIDStr)
				}
			}
		}
	} else {
		if termIDStr, ok := value.(string); ok {
			addTermID(termIDStr)
		}
	}
}
