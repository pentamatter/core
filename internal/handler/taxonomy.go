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
	Key            string `json:"key" binding:"required"`
	Name           string `json:"name" binding:"required"`
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
