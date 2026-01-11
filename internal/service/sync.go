package service

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"matter-core/internal/model"
	"matter-core/internal/repository"
)

type SyncService struct {
	meiliRepo *repository.MeiliRepo
}

func NewSyncService(meiliRepo *repository.MeiliRepo) *SyncService {
	return &SyncService{meiliRepo: meiliRepo}
}

// SyncEntryAsync 异步同步 entry 到搜索引擎，带重试机制
func (s *SyncService) SyncEntryAsync(entry *model.Entry) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic in SyncEntryAsync: %v", r)
			}
		}()
		s.syncWithRetry(entry, 3)
	}()
}

func (s *SyncService) syncWithRetry(entry *model.Entry, maxRetries int) {
	var err error
	for i := 0; i < maxRetries; i++ {
		if err = s.SyncEntry(entry); err == nil {
			return
		}
		log.Printf("failed to sync entry %s (attempt %d/%d): %v", entry.ID.Hex(), i+1, maxRetries, err)
		time.Sleep(time.Duration(i+1) * time.Second) // exponential backoff
	}
	log.Printf("giving up syncing entry %s after %d attempts", entry.ID.Hex(), maxRetries)
}

func (s *SyncService) SyncEntry(entry *model.Entry) error {
	doc := s.entryToSearchDoc(entry)
	return s.meiliRepo.IndexDocument(doc)
}

// DeleteEntryAsync 异步删除搜索索引
func (s *SyncService) DeleteEntryAsync(id string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic in DeleteEntryAsync: %v", r)
			}
		}()
		if err := s.DeleteEntry(id); err != nil {
			log.Printf("failed to delete entry %s from search index: %v", id, err)
		}
	}()
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
