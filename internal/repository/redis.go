package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

// RedisRepo handles Redis operations for user reactions
type RedisRepo struct {
	client *redis.Client
}

// NewRedisRepo creates a new Redis repository with connection pool
func NewRedisRepo(addr, password string, db int) (*RedisRepo, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisRepo{client: client}, nil
}

// Close closes the Redis connection
func (r *RedisRepo) Close() error {
	return r.client.Close()
}

// Ping checks if Redis connection is alive
func (r *RedisRepo) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// userReactionKey returns the Redis key for user reactions
// Format: user_reactions:{user_id}
func userReactionKey(userID string) string {
	return fmt.Sprintf("user_reactions:%s", userID)
}

// reactionValue returns the value format for a reaction
// Format: {target_type}:{target_id}:{emoji}
func reactionValue(targetType, targetID, emoji string) string {
	return fmt.Sprintf("%s:%s:%s", targetType, targetID, emoji)
}

// parseReactionValue parses a reaction value into its components
func parseReactionValue(value string) (targetType, targetID, emoji string, ok bool) {
	parts := strings.SplitN(value, ":", 3)
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

// AddUserReaction adds a user reaction to the Set.
// Returns (true, nil) if newly added, (false, nil) if already existed.
func (r *RedisRepo) AddUserReaction(ctx context.Context, userID, targetType, targetID, emoji string) (bool, error) {
	key := userReactionKey(userID)
	value := reactionValue(targetType, targetID, emoji)
	added, err := r.client.SAdd(ctx, key, value).Result()
	if err != nil {
		return false, err
	}
	return added > 0, nil
}

// RemoveUserReaction removes a user reaction from the Set.
// Returns (true, nil) if removed, (false, nil) if not found.
func (r *RedisRepo) RemoveUserReaction(ctx context.Context, userID, targetType, targetID, emoji string) (bool, error) {
	key := userReactionKey(userID)
	value := reactionValue(targetType, targetID, emoji)
	removed, err := r.client.SRem(ctx, key, value).Result()
	if err != nil {
		return false, err
	}
	return removed > 0, nil
}

// HasUserReaction checks if a user has added a specific reaction
func (r *RedisRepo) HasUserReaction(ctx context.Context, userID, targetType, targetID, emoji string) (bool, error) {
	key := userReactionKey(userID)
	value := reactionValue(targetType, targetID, emoji)
	return r.client.SIsMember(ctx, key, value).Result()
}

// GetUserReactionsForTarget returns all emojis a user has added to a specific target.
// Uses SSCAN with pattern matching to avoid loading all members into memory.
func (r *RedisRepo) GetUserReactionsForTarget(ctx context.Context, userID, targetType, targetID string) ([]string, error) {
	key := userReactionKey(userID)
	pattern := fmt.Sprintf("%s:%s:*", targetType, targetID)
	var emojis []string
	var cursor uint64
	for {
		members, nextCursor, err := r.client.SScan(ctx, key, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		for _, member := range members {
			if _, _, emoji, ok := parseReactionValue(member); ok {
				emojis = append(emojis, emoji)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return emojis, nil
}

// ReactionTarget represents a target for batch queries
type ReactionTarget struct {
	Type string
	ID   string
}

// GetUserReactionsForTargets returns user reactions for multiple targets.
// Returns a map where key is "{target_type}:{target_id}" and value is list of emojis.
// Uses SSCAN per target to avoid loading the entire set into memory.
func (r *RedisRepo) GetUserReactionsForTargets(ctx context.Context, userID string, targets []ReactionTarget) (map[string][]string, error) {
	key := userReactionKey(userID)
	result := make(map[string][]string)

	for _, t := range targets {
		pattern := fmt.Sprintf("%s:%s:*", t.Type, t.ID)
		targetKey := fmt.Sprintf("%s:%s", t.Type, t.ID)
		var cursor uint64
		for {
			members, nextCursor, err := r.client.SScan(ctx, key, cursor, pattern, 100).Result()
			if err != nil {
				return nil, err
			}
			for _, member := range members {
				if _, _, emoji, ok := parseReactionValue(member); ok {
					result[targetKey] = append(result[targetKey], emoji)
				}
			}
			cursor = nextCursor
			if cursor == 0 {
				break
			}
		}
	}

	return result, nil
}
