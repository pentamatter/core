package repository

import (
	"encoding/json"
	"fmt"
	"regexp"

	"matter-core/internal/model"

	"github.com/meilisearch/meilisearch-go"
)

var schemaKeyRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func isValidSchemaKey(key string) bool {
	return len(key) <= 50 && schemaKeyRegex.MatchString(key)
}

type MeiliRepo struct {
	client meilisearch.ServiceManager
	index  meilisearch.IndexManager
}

func NewMeiliRepo(host, apiKey string) (*MeiliRepo, error) {
	client := meilisearch.New(host, meilisearch.WithAPIKey(apiKey))

	index := client.Index("entries")

	// Configure searchable and filterable attributes
	searchable := []string{"title", "body", "all_text", "schema_key"}
	_, err := index.UpdateSearchableAttributes(&searchable)
	if err != nil {
		return nil, err
	}

	filterable := []interface{}{"schema_key"}
	_, err = index.UpdateFilterableAttributes(&filterable)
	if err != nil {
		return nil, err
	}

	return &MeiliRepo{
		client: client,
		index:  index,
	}, nil
}

func (r *MeiliRepo) IndexDocument(doc model.SearchDocument) error {
	pk := "id"
	_, err := r.index.AddDocuments([]model.SearchDocument{doc}, &meilisearch.DocumentOptions{
		PrimaryKey: &pk,
	})
	return err
}

func (r *MeiliRepo) DeleteDocument(id string) error {
	_, err := r.index.DeleteDocument(id, nil)
	return err
}

func (r *MeiliRepo) Search(query string, schemaKey string, limit, offset int64) ([]string, int64, error) {
	searchReq := &meilisearch.SearchRequest{
		Limit:  limit,
		Offset: offset,
	}

	if schemaKey != "" {
		// Sanitize schemaKey to prevent filter injection
		// Only allow alphanumeric, underscore, and hyphen
		if !isValidSchemaKey(schemaKey) {
			return nil, 0, fmt.Errorf("invalid schema_key format")
		}
		searchReq.Filter = fmt.Sprintf("schema_key = \"%s\"", schemaKey)
	}

	result, err := r.index.Search(query, searchReq)
	if err != nil {
		return nil, 0, err
	}

	ids := make([]string, 0, len(result.Hits))
	for _, hit := range result.Hits {
		if idRaw, ok := hit["id"]; ok {
			var id string
			if err := json.Unmarshal(idRaw, &id); err == nil {
				ids = append(ids, id)
			}
		}
	}
	return ids, result.EstimatedTotalHits, nil
}
