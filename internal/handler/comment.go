package handler

import (
	"context"
	"time"

	"matter-core/internal/model"
	"matter-core/internal/repository"
	"matter-core/pkg/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type CommentHandler struct {
	mongoRepo *repository.MongoRepo
}

func NewCommentHandler(mongoRepo *repository.MongoRepo) *CommentHandler {
	return &CommentHandler{mongoRepo: mongoRepo}
}

type CreateCommentRequest struct {
	EntryID    string `json:"entry_id" binding:"required"`
	Content    string `json:"content" binding:"required,min=1,max=5000"`
	ParentID   string `json:"parent_id"`
	ReplyToUID string `json:"reply_to_uid"`
}

func (h *CommentHandler) Create(c *gin.Context) {
	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	userID, _ := c.Get("user_id")

	entryOID, err := primitive.ObjectIDFromHex(req.EntryID)
	if err != nil {
		utils.BadRequest(c, "invalid entry_id")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Verify entry exists
	_, err = h.mongoRepo.GetEntryByID(ctx, entryOID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFound(c, "entry not found")
			return
		}
		utils.InternalError(c, "failed to verify entry")
		return
	}

	comment := &model.Comment{
		EntryID:    entryOID,
		AuthorID:   userID.(string),
		Content:    req.Content,
		ReplyToUID: req.ReplyToUID,
	}

	// Handle reply (two-level flat structure)
	if req.ParentID != "" {
		parentOID, err := primitive.ObjectIDFromHex(req.ParentID)
		if err != nil {
			utils.BadRequest(c, "invalid parent_id")
			return
		}

		// Get parent comment to determine root_id
		parentComment, err := h.mongoRepo.GetCommentByID(ctx, parentOID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				utils.NotFound(c, "parent comment not found")
				return
			}
			utils.InternalError(c, "failed to get parent comment")
			return
		}

		comment.ParentID = parentOID
		// For two-level flat: if parent is already a reply, use its root_id; otherwise parent is the root
		if parentComment.RootID.IsZero() {
			comment.RootID = parentOID
		} else {
			comment.RootID = parentComment.RootID
		}
	}

	if err := h.mongoRepo.CreateComment(ctx, comment); err != nil {
		utils.InternalError(c, "failed to create comment")
		return
	}

	utils.Created(c, comment)
}

func (h *CommentHandler) ListByEntry(c *gin.Context) {
	entryID := c.Param("entry_id")
	entryOID, err := primitive.ObjectIDFromHex(entryID)
	if err != nil {
		utils.BadRequest(c, "invalid entry_id")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	comments, err := h.mongoRepo.GetCommentsByEntry(ctx, entryOID)
	if err != nil {
		utils.InternalError(c, "failed to list comments")
		return
	}

	utils.Success(c, comments)
}

func (h *CommentHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		utils.BadRequest(c, "invalid comment id")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Get comment to check ownership
	comment, err := h.mongoRepo.GetCommentByID(ctx, oid)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFound(c, "comment not found")
			return
		}
		utils.InternalError(c, "failed to get comment")
		return
	}

	// Check ownership or admin
	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")
	if comment.AuthorID != userID.(string) && userRole != "admin" {
		utils.Forbidden(c, "not authorized to delete this comment")
		return
	}

	if err := h.mongoRepo.DeleteComment(ctx, oid); err != nil {
		utils.InternalError(c, "failed to delete comment")
		return
	}

	utils.Success(c, nil)
}
