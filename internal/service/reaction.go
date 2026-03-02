package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"matter-core/internal/model"
	"matter-core/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Common errors for reaction operations
var (
	ErrInvalidEmoji      = errors.New("invalid emoji")
	ErrInvalidTargetType = errors.New("invalid target_type")
	ErrTargetNotFound    = errors.New("target not found")
	ErrReactionExists    = errors.New("reaction already exists")
	ErrReactionNotFound  = errors.New("reaction not found")
	ErrTooManyTargets    = errors.New("too many targets (max 100)")
)

// ReactionService handles reaction business logic
type ReactionService struct {
	mongoRepo      *repository.MongoRepo
	redisRepo      *repository.RedisRepo
	emojiValidator *EmojiValidator
}

// NewReactionService creates a new ReactionService instance
func NewReactionService(mongoRepo *repository.MongoRepo, redisRepo *repository.RedisRepo) *ReactionService {
	return &ReactionService{
		mongoRepo:      mongoRepo,
		redisRepo:      redisRepo,
		emojiValidator: NewEmojiValidator(),
	}
}

// validateTargetType validates and converts target type string
func validateTargetType(targetType string) (model.TargetType, error) {
	switch targetType {
	case string(model.TargetEntry):
		return model.TargetEntry, nil
	case string(model.TargetComment):
		return model.TargetComment, nil
	default:
		return "", ErrInvalidTargetType
	}
}

// validateTargetExists checks if the target (entry or comment) exists
func (s *ReactionService) validateTargetExists(ctx context.Context, targetType model.TargetType, targetID primitive.ObjectID) error {
	switch targetType {
	case model.TargetEntry:
		entry, err := s.mongoRepo.GetEntryByID(ctx, targetID)
		if err != nil || entry == nil {
			return ErrTargetNotFound
		}
	case model.TargetComment:
		comment, err := s.mongoRepo.GetCommentByID(ctx, targetID)
		if err != nil || comment == nil {
			return ErrTargetNotFound
		}
	default:
		return ErrInvalidTargetType
	}
	return nil
}

// AddReaction adds a reaction from a user to a target
// Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.7, 7.1, 7.2
func (s *ReactionService) AddReaction(ctx context.Context, userID, targetType, targetID, emoji string) (*model.ReactionResponse, error) {
	// Validate emoji
	if !s.emojiValidator.IsValidEmoji(emoji) {
		return nil, ErrInvalidEmoji
	}

	// Validate target type
	tt, err := validateTargetType(targetType)
	if err != nil {
		return nil, err
	}

	// Parse target ID
	tid, err := primitive.ObjectIDFromHex(targetID)
	if err != nil {
		return nil, ErrTargetNotFound
	}

	// Validate target exists
	if err := s.validateTargetExists(ctx, tt, tid); err != nil {
		return nil, err
	}

	// Step 1: Atomically add to Redis — SADD returns whether it was newly added
	added, err := s.redisRepo.AddUserReaction(ctx, userID, targetType, targetID, emoji)
	if err != nil {
		return nil, fmt.Errorf("failed to add user reaction to Redis: %w", err)
	}
	if !added {
		return nil, ErrReactionExists
	}

	// Step 2: Update MongoDB
	if err := s.mongoRepo.IncrementReaction(ctx, tt, tid, emoji); err != nil {
		// Rollback Redis on MongoDB failure
		if _, rollbackErr := s.redisRepo.RemoveUserReaction(ctx, userID, targetType, targetID, emoji); rollbackErr != nil {
			log.Printf("failed to rollback Redis after MongoDB failure: %v", rollbackErr)
		}
		return nil, fmt.Errorf("failed to increment reaction in MongoDB: %w", err)
	}

	// Return updated reaction response
	return s.GetReactions(ctx, userID, targetType, targetID)
}

// RemoveReaction removes a reaction from a user to a target
// Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 7.3, 7.4
func (s *ReactionService) RemoveReaction(ctx context.Context, userID, targetType, targetID, emoji string) (*model.ReactionResponse, error) {
	// Validate target type
	tt, err := validateTargetType(targetType)
	if err != nil {
		return nil, err
	}

	// Parse target ID
	tid, err := primitive.ObjectIDFromHex(targetID)
	if err != nil {
		return nil, ErrTargetNotFound
	}

	// Validate target exists
	if err := s.validateTargetExists(ctx, tt, tid); err != nil {
		return nil, err
	}

	// Step 1: Atomically remove from Redis — SREM returns whether it was actually removed
	removed, err := s.redisRepo.RemoveUserReaction(ctx, userID, targetType, targetID, emoji)
	if err != nil {
		return nil, fmt.Errorf("failed to remove user reaction from Redis: %w", err)
	}
	if !removed {
		return nil, ErrReactionNotFound
	}

	// Step 2: Update MongoDB
	// DecrementReaction handles removing the emoji key when count reaches 0
	if err := s.mongoRepo.DecrementReaction(ctx, tt, tid, emoji); err != nil {
		// Rollback Redis on MongoDB failure
		if _, rollbackErr := s.redisRepo.AddUserReaction(ctx, userID, targetType, targetID, emoji); rollbackErr != nil {
			log.Printf("failed to rollback Redis after MongoDB failure: %v", rollbackErr)
		}
		return nil, fmt.Errorf("failed to decrement reaction in MongoDB: %w", err)
	}

	// Return updated reaction response
	return s.GetReactions(ctx, userID, targetType, targetID)
}

// GetReactions retrieves reaction statistics for a target
// Requirements: 3.1, 3.2, 3.3, 3.4, 3.5
func (s *ReactionService) GetReactions(ctx context.Context, userID, targetType, targetID string) (*model.ReactionResponse, error) {
	// Validate target type
	tt, err := validateTargetType(targetType)
	if err != nil {
		return nil, err
	}

	// Parse target ID
	tid, err := primitive.ObjectIDFromHex(targetID)
	if err != nil {
		return nil, ErrTargetNotFound
	}

	// Get reaction summary from MongoDB (Requirement 3.4)
	summary, err := s.mongoRepo.GetReactionSummary(ctx, tt, tid)
	if err != nil {
		return nil, fmt.Errorf("failed to get reaction summary: %w", err)
	}

	// Build response
	response := &model.ReactionResponse{
		Reactions:     make(map[string]int),
		UserReactions: []string{},
	}

	// If summary exists, copy reactions (Requirement 3.1)
	if summary != nil && summary.Reactions != nil {
		response.Reactions = summary.Reactions
	}

	// If user is logged in, get their reactions from Redis (Requirement 3.2, 3.5)
	if userID != "" {
		userReactions, err := s.redisRepo.GetUserReactionsForTarget(ctx, userID, targetType, targetID)
		if err != nil {
			log.Printf("failed to get user reactions from Redis: %v", err)
			// Continue without user reactions rather than failing
		} else if userReactions != nil {
			response.UserReactions = userReactions
		}
	}

	return response, nil
}

// ReactionTarget represents a target for batch queries
type ReactionTarget struct {
	Type string `json:"target_type"`
	ID   string `json:"target_id"`
}

// GetReactionsBatch retrieves reaction statistics for multiple targets
// Requirements: 4.1, 4.2, 4.3, 4.4
func (s *ReactionService) GetReactionsBatch(ctx context.Context, userID string, targets []ReactionTarget) (map[string]*model.ReactionResponse, error) {
	// Validate batch size (Requirement 4.3)
	if len(targets) > 100 {
		return nil, ErrTooManyTargets
	}

	if len(targets) == 0 {
		return make(map[string]*model.ReactionResponse), nil
	}

	// Convert targets to MongoDB format and validate
	mongoTargets := make([]repository.MongoReactionTarget, 0, len(targets))
	redisTargets := make([]repository.ReactionTarget, 0, len(targets))

	for _, t := range targets {
		tt, err := validateTargetType(t.Type)
		if err != nil {
			continue // Skip invalid target types
		}

		tid, err := primitive.ObjectIDFromHex(t.ID)
		if err != nil {
			continue // Skip invalid IDs
		}

		mongoTargets = append(mongoTargets, repository.MongoReactionTarget{
			Type: tt,
			ID:   tid,
		})
		redisTargets = append(redisTargets, repository.ReactionTarget{
			Type: t.Type,
			ID:   t.ID,
		})
	}

	// Get reaction summaries from MongoDB (Requirement 4.1)
	summaries, err := s.mongoRepo.GetReactionSummaries(ctx, mongoTargets)
	if err != nil {
		return nil, fmt.Errorf("failed to get reaction summaries: %w", err)
	}

	// Build summary map for quick lookup
	summaryMap := make(map[string]*model.ReactionSummary)
	for i := range summaries {
		key := fmt.Sprintf("%s:%s", summaries[i].TargetType, summaries[i].TargetID.Hex())
		summaryMap[key] = &summaries[i]
	}

	// Get user reactions from Redis if logged in (Requirement 4.2)
	var userReactionsMap map[string][]string
	if userID != "" {
		userReactionsMap, err = s.redisRepo.GetUserReactionsForTargets(ctx, userID, redisTargets)
		if err != nil {
			log.Printf("failed to get user reactions from Redis: %v", err)
			userReactionsMap = make(map[string][]string)
		}
	}

	// Build response map (supports mixed Entry and Comment types - Requirement 4.4)
	result := make(map[string]*model.ReactionResponse)
	for _, t := range targets {
		key := fmt.Sprintf("%s:%s", t.Type, t.ID)

		response := &model.ReactionResponse{
			Reactions:     make(map[string]int),
			UserReactions: []string{},
		}

		// Add reactions from summary
		if summary, ok := summaryMap[key]; ok && summary.Reactions != nil {
			response.Reactions = summary.Reactions
		}

		// Add user reactions
		if userReactions, ok := userReactionsMap[key]; ok {
			response.UserReactions = userReactions
		}

		result[key] = response
	}

	return result, nil
}
