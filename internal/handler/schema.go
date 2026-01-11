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

type SchemaHandler struct {
	mongoRepo *repository.MongoRepo
}

func NewSchemaHandler(mongoRepo *repository.MongoRepo) *SchemaHandler {
	return &SchemaHandler{mongoRepo: mongoRepo}
}

type CreateSchemaRequest struct {
	Key    string              `json:"key" binding:"required,max=50,alphanum"`
	Name   string              `json:"name" binding:"required,max=100"`
	Fields []model.FieldSchema `json:"fields" binding:"required"`
}

func (h *SchemaHandler) Create(c *gin.Context) {
	var req CreateSchemaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Check if schema with this key exists
	existing, err := h.mongoRepo.GetLatestSchema(ctx, req.Key)
	version := 1
	if err == nil && existing != nil {
		version = existing.Version + 1
	} else if err != nil && err != mongo.ErrNoDocuments {
		utils.InternalError(c, "failed to check existing schema")
		return
	}

	schema := &model.Schema{
		Key:       req.Key,
		Version:   version,
		Name:      req.Name,
		Fields:    req.Fields,
		CreatedAt: time.Now(),
	}

	if err := h.mongoRepo.CreateSchema(ctx, schema); err != nil {
		utils.InternalError(c, "failed to create schema")
		return
	}

	utils.Created(c, schema)
}

func (h *SchemaHandler) Get(c *gin.Context) {
	key := c.Param("key")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	schema, err := h.mongoRepo.GetLatestSchema(ctx, key)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFound(c, "schema not found")
			return
		}
		utils.InternalError(c, "failed to get schema")
		return
	}

	utils.Success(c, schema)
}

func (h *SchemaHandler) List(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	schemas, err := h.mongoRepo.ListSchemas(ctx)
	if err != nil {
		utils.InternalError(c, "failed to list schemas")
		return
	}

	utils.Success(c, schemas)
}

func (h *SchemaHandler) Delete(c *gin.Context) {
	key := c.Param("key")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Check if schema exists
	_, err := h.mongoRepo.GetLatestSchema(ctx, key)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFound(c, "schema not found")
			return
		}
		utils.InternalError(c, "failed to get schema")
		return
	}

	// Delete all versions of this schema
	if err := h.mongoRepo.DeleteSchemasByKey(ctx, key); err != nil {
		utils.InternalError(c, "failed to delete schema")
		return
	}

	utils.Success(c, nil)
}
