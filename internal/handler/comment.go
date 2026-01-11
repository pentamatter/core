package handler

import (
	"context"
	"strconv"
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

	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	limit, _ := strconv.ParseInt(limitStr, 10, 64)
	offset, _ := strconv.ParseInt(offsetStr, 10, 64)

	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	comments, err := h.mongoRepo.GetCommentsByEntryPaginated(ctx, entryOID, limit, offset)
	if err != nil {
		utils.InternalError(c, "failed to list comments")
		return
	}

	total, err := h.mongoRepo.CountCommentsByEntry(ctx, entryOID)
	if err != nil {
		utils.InternalError(c, "failed to count comments")
		return
	}

	if comments == nil {
		comments = []model.CommentWithAuthor{}
	}

	utils.SuccessWithPagination(c, comments, total, limit, offset)
}

type UpdateCommentRequest struct {
	Content string `json:"content" binding:"required,min=1,max=5000"`
}

func (h *CommentHandler) Update(c *gin.Context) {
	id := c.Param("id")
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		utils.BadRequest(c, "invalid comment id")
		return
	}

	var req UpdateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	comment, err := h.mongoRepo.GetCommentByID(ctx, oid)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFound(c, "comment not found")
			return
		}
		utils.InternalError(c, "failed to get comment")
		return
	}

	// 只有作者可以编辑评论
	userID, _ := c.Get("user_id")
	if comment.AuthorID != userID.(string) {
		utils.Forbidden(c, "not authorized to update this comment")
		return
	}

	comment.Content = req.Content
	if err := h.mongoRepo.UpdateComment(ctx, comment); err != nil {
		utils.InternalError(c, "failed to update comment")
		return
	}

	utils.Success(c, comment)
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

	// Delete comment and its replies (if this is a root comment)
	if comment.RootID.IsZero() {
		// This is a root comment, delete all replies first
		if err := h.mongoRepo.DeleteCommentsByRootID(ctx, oid); err != nil {
			utils.InternalError(c, "failed to delete replies")
			return
		}
	}

	if err := h.mongoRepo.DeleteComment(ctx, oid); err != nil {
		utils.InternalError(c, "failed to delete comment")
		return
	}

	utils.Success(c, nil)
}
