package handler

import (
	"context"
	"errors"
	"matter-core/internal/repository"
	"matter-core/internal/service"
	"matter-core/pkg/utils"
	"time"

	"github.com/gin-gonic/gin"
)

// ReactionHandler handles reaction-related HTTP requests
type ReactionHandler struct {
	reactionSvc *service.ReactionService
	mongoRepo   *repository.MongoRepo
}

// NewReactionHandler creates a new ReactionHandler instance
func NewReactionHandler(reactionSvc *service.ReactionService, mongoRepo *repository.MongoRepo) *ReactionHandler {
	return &ReactionHandler{
		reactionSvc: reactionSvc,
		mongoRepo:   mongoRepo,
	}
}

// Request/Response structures (Requirement 8.5)

// ReactionRequest represents the request body for add/remove reaction
type ReactionRequest struct {
	TargetID   string `json:"target_id" binding:"required"`
	TargetType string `json:"target_type" binding:"required"`
	Emoji      string `json:"emoji" binding:"required"`
}

// BatchTarget represents a single target in batch query
type BatchTarget struct {
	TargetID   string `json:"target_id" binding:"required"`
	TargetType string `json:"target_type" binding:"required"`
}

// BatchReactionRequest represents the request body for batch query
type BatchReactionRequest struct {
	Targets []BatchTarget `json:"targets" binding:"required"`
}

// Add handles POST /api/v1/reactions - adds a reaction to a target
// Requirements: 8.1, 8.5
func (h *ReactionHandler) Add(c *gin.Context) {
	var req ReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "unauthorized")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	response, err := h.reactionSvc.AddReaction(ctx, userID.(string), req.TargetType, req.TargetID, req.Emoji)
	if err != nil {
		handleReactionError(c, err)
		return
	}

	utils.Created(c, response)
}

// handleReactionError maps service errors to HTTP responses
func handleReactionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidEmoji):
		utils.BadRequest(c, "invalid emoji")
	case errors.Is(err, service.ErrInvalidTargetType):
		utils.BadRequest(c, "invalid target_type")
	case errors.Is(err, service.ErrTargetNotFound):
		utils.NotFound(c, "target not found")
	case errors.Is(err, service.ErrReactionExists):
		utils.Error(c, 409, "reaction already exists")
	case errors.Is(err, service.ErrReactionNotFound):
		utils.NotFound(c, "reaction not found")
	case errors.Is(err, service.ErrTooManyTargets):
		utils.BadRequest(c, "too many targets (max 100)")
	default:
		utils.InternalError(c, "internal server error")
	}
}

// Remove handles DELETE /api/v1/reactions - removes a reaction from a target
// Requirements: 8.2, 8.5
func (h *ReactionHandler) Remove(c *gin.Context) {
	var req ReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Unauthorized(c, "unauthorized")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	response, err := h.reactionSvc.RemoveReaction(ctx, userID.(string), req.TargetType, req.TargetID, req.Emoji)
	if err != nil {
		handleReactionError(c, err)
		return
	}

	utils.Success(c, response)
}

// Get handles GET /api/v1/reactions - retrieves reaction statistics for a target
// Requirements: 8.3
func (h *ReactionHandler) Get(c *gin.Context) {
	targetID := c.Query("target_id")
	targetType := c.Query("target_type")

	if targetID == "" {
		utils.BadRequest(c, "target_id is required")
		return
	}
	if targetType == "" {
		utils.BadRequest(c, "target_type is required")
		return
	}

	// Get user ID from context if available (optional auth)
	userID := ""
	if uid, exists := c.Get("user_id"); exists {
		userID = uid.(string)
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	response, err := h.reactionSvc.GetReactions(ctx, userID, targetType, targetID)
	if err != nil {
		handleReactionError(c, err)
		return
	}

	utils.Success(c, response)
}

// GetBatch handles POST /api/v1/reactions/batch - retrieves reaction statistics for multiple targets
// Requirements: 8.4
func (h *ReactionHandler) GetBatch(c *gin.Context) {
	var req BatchReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	// Get user ID from context if available (optional auth)
	userID := ""
	if uid, exists := c.Get("user_id"); exists {
		userID = uid.(string)
	}

	// Convert BatchTarget to service.ReactionTarget
	targets := make([]service.ReactionTarget, len(req.Targets))
	for i, t := range req.Targets {
		targets[i] = service.ReactionTarget{
			Type: t.TargetType,
			ID:   t.TargetID,
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	response, err := h.reactionSvc.GetReactionsBatch(ctx, userID, targets)
	if err != nil {
		handleReactionError(c, err)
		return
	}

	utils.Success(c, response)
}
