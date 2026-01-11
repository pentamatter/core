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

type TermHandler struct {
	mongoRepo *repository.MongoRepo
}

func NewTermHandler(mongoRepo *repository.MongoRepo) *TermHandler {
	return &TermHandler{mongoRepo: mongoRepo}
}

type CreateTermRequest struct {
	TaxonomyKey string `json:"taxonomy_key" binding:"required,max=50"`
	Name        string `json:"name" binding:"required,max=100"`
	Slug        string `json:"slug" binding:"required,max=100"`
	Color       string `json:"color" binding:"max=20"`
	ParentID    string `json:"parent_id"`
}

func (h *TermHandler) Create(c *gin.Context) {
	var req CreateTermRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Verify taxonomy exists
	_, err := h.mongoRepo.GetTaxonomyByKey(ctx, req.TaxonomyKey)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFound(c, "taxonomy not found")
			return
		}
		utils.InternalError(c, "failed to verify taxonomy")
		return
	}

	term := &model.Term{
		TaxonomyKey: req.TaxonomyKey,
		Name:        req.Name,
		Slug:        req.Slug,
		Color:       req.Color,
	}

	if req.ParentID != "" {
		parentOID, err := primitive.ObjectIDFromHex(req.ParentID)
		if err != nil {
			utils.BadRequest(c, "invalid parent_id")
			return
		}
		term.ParentID = parentOID
	}

	if err := h.mongoRepo.CreateTerm(ctx, term); err != nil {
		utils.InternalError(c, "failed to create term")
		return
	}

	utils.Created(c, term)
}

func (h *TermHandler) ListByTaxonomy(c *gin.Context) {
	taxonomyKey := c.Param("key")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	terms, err := h.mongoRepo.GetTermsByTaxonomy(ctx, taxonomyKey)
	if err != nil {
		utils.InternalError(c, "failed to list terms")
		return
	}

	utils.Success(c, terms)
}

func (h *TermHandler) Get(c *gin.Context) {
	id := c.Param("id")
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		utils.BadRequest(c, "invalid term id")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	term, err := h.mongoRepo.GetTermByID(ctx, oid)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFound(c, "term not found")
			return
		}
		utils.InternalError(c, "failed to get term")
		return
	}

	utils.Success(c, term)
}

type UpdateTermRequest struct {
	Name     string `json:"name" binding:"required,max=100"`
	Slug     string `json:"slug" binding:"required,max=100,alphanumunicode"`
	Color    string `json:"color" binding:"max=20"`
	ParentID string `json:"parent_id"`
}

func (h *TermHandler) Update(c *gin.Context) {
	id := c.Param("id")
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		utils.BadRequest(c, "invalid term id")
		return
	}

	var req UpdateTermRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	term, err := h.mongoRepo.GetTermByID(ctx, oid)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFound(c, "term not found")
			return
		}
		utils.InternalError(c, "failed to get term")
		return
	}

	term.Name = req.Name
	term.Slug = req.Slug
	term.Color = req.Color

	if req.ParentID != "" {
		parentOID, err := primitive.ObjectIDFromHex(req.ParentID)
		if err != nil {
			utils.BadRequest(c, "invalid parent_id")
			return
		}
		term.ParentID = parentOID
	} else {
		term.ParentID = primitive.NilObjectID
	}

	if err := h.mongoRepo.UpdateTerm(ctx, term); err != nil {
		utils.InternalError(c, "failed to update term")
		return
	}

	utils.Success(c, term)
}

func (h *TermHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		utils.BadRequest(c, "invalid term id")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Check if term exists
	_, err = h.mongoRepo.GetTermByID(ctx, oid)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFound(c, "term not found")
			return
		}
		utils.InternalError(c, "failed to get term")
		return
	}

	if err := h.mongoRepo.DeleteTerm(ctx, oid); err != nil {
		utils.InternalError(c, "failed to delete term")
		return
	}

	utils.Success(c, nil)
}
