package service

import (
	"fmt"
	"regexp"
	"strings"

	"matter-core/internal/model"
	"matter-core/internal/repository"
)

type SyncService struct {
	meiliRepo *repository.MeiliRepo
}

func NewSyncService(meiliRepo *repository.MeiliRepo) *SyncService {
	return &SyncService{meiliRepo: meiliRepo}
}

func (s *SyncService) SyncEntry(entry *model.Entry) error {
	doc := s.entryToSearchDoc(entry)
	return s.meiliRepo.IndexDocument(doc)
}

func (s *SyncService) DeleteEntry(id string) error {
	return s.meiliRepo.DeleteDocument(id)
}

func (s *SyncService) entryToSearchDoc(entry *model.Entry) model.SearchDocument {
	allText := s.extractTextFromAttributes(entry.Attributes)

	return model.SearchDocument{
		ID:        entry.ID.Hex(),
		Title:     entry.Base.Title,
		Body:      stripMarkdown(entry.Body),
		SchemaKey: entry.SchemaKey,
		AllText:   allText,
	}
}

func (s *SyncService) extractTextFromAttributes(attrs map[string]any) string {
	var texts []string
	for _, v := range attrs {
		texts = append(texts, extractStrings(v)...)
	}
	return strings.Join(texts, " ")
}

func extractStrings(v any) []string {
	var result []string
	switch val := v.(type) {
	case string:
		result = append(result, val)
	case []any:
		for _, item := range val {
			result = append(result, extractStrings(item)...)
		}
	case map[string]any:
		for _, item := range val {
			result = append(result, extractStrings(item)...)
		}
	default:
		if val != nil {
			result = append(result, fmt.Sprintf("%v", val))
		}
	}
	return result
}

var mdPatterns = []*regexp.Regexp{
	regexp.MustCompile(`#{1,6}\s`),
	regexp.MustCompile(`\*\*([^*]+)\*\*`),
	regexp.MustCompile(`\*([^*]+)\*`),
	regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`),
	regexp.MustCompile("```[^`]*```"),
	regexp.MustCompile("`[^`]+`"),
}

func stripMarkdown(md string) string {
	result := md
	for _, pattern := range mdPatterns {
		result = pattern.ReplaceAllString(result, "$1")
	}
	return strings.TrimSpace(result)
}
