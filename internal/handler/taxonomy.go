package handler

import (
	"context"
	"time"

	"matter-core/internal/model"
	"matter-core/internal/repository"
	"matter-core/pkg/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

type TaxonomyHandler struct {
	mongoRepo *repository.MongoRepo
}

func NewTaxonomyHandler(mongoRepo *repository.MongoRepo) *TaxonomyHandler {
	return &TaxonomyHandler{mongoRepo: mongoRepo}
}

type CreateTaxonomyRequest struct {
	Key            string `json:"key" binding:"required,max=50,alphanum"`
	Name           string `json:"name" binding:"required,max=100"`
	IsHierarchical bool   `json:"is_hierarchical"`
}

func (h *TaxonomyHandler) Create(c *gin.Context) {
	var req CreateTaxonomyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	tax := &model.Taxonomy{
		Key:            req.Key,
		Name:           req.Name,
		IsHierarchical: req.IsHierarchical,
	}

	if err := h.mongoRepo.CreateTaxonomy(ctx, tax); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			utils.BadRequest(c, "taxonomy key already exists")
			return
		}
		utils.InternalError(c, "failed to create taxonomy")
		return
	}

	utils.Created(c, tax)
}

func (h *TaxonomyHandler) List(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	taxonomies, err := h.mongoRepo.ListTaxonomies(ctx)
	if err != nil {
		utils.InternalError(c, "failed to list taxonomies")
		return
	}

	utils.Success(c, taxonomies)
}

func (h *TaxonomyHandler) Get(c *gin.Context) {
	key := c.Param("key")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	tax, err := h.mongoRepo.GetTaxonomyByKey(ctx, key)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFound(c, "taxonomy not found")
			return
		}
		utils.InternalError(c, "failed to get taxonomy")
		return
	}

	utils.Success(c, tax)
}

type UpdateTaxonomyRequest struct {
	Name           string `json:"name" binding:"required,max=100"`
	IsHierarchical *bool  `json:"is_hierarchical"`
}

func (h *TaxonomyHandler) Update(c *gin.Context) {
	key := c.Param("key")

	var req UpdateTaxonomyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	tax, err := h.mongoRepo.GetTaxonomyByKey(ctx, key)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFound(c, "taxonomy not found")
			return
		}
		utils.InternalError(c, "failed to get taxonomy")
		return
	}

	tax.Name = req.Name
	if req.IsHierarchical != nil {
		tax.IsHierarchical = *req.IsHierarchical
	}

	if err := h.mongoRepo.UpdateTaxonomy(ctx, tax); err != nil {
		utils.InternalError(c, "failed to update taxonomy")
		return
	}

	utils.Success(c, tax)
}

func (h *TaxonomyHandler) Delete(c *gin.Context) {
	key := c.Param("key")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Check if taxonomy exists
	_, err := h.mongoRepo.GetTaxonomyByKey(ctx, key)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFound(c, "taxonomy not found")
			return
		}
		utils.InternalError(c, "failed to get taxonomy")
		return
	}

	// Delete all terms under this taxonomy
	if err := h.mongoRepo.DeleteTermsByTaxonomy(ctx, key); err != nil {
		utils.InternalError(c, "failed to delete terms")
		return
	}

	// Delete taxonomy
	if err := h.mongoRepo.DeleteTaxonomy(ctx, key); err != nil {
		utils.InternalError(c, "failed to delete taxonomy")
		return
	}

	utils.Success(c, nil)
}
