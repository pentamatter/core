package handler

import (
	"context"
	"strconv"
	"time"

	"matter-core/internal/model"
	"matter-core/internal/repository"
	"matter-core/internal/service"
	"matter-core/pkg/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type EntryHandler struct {
	mongoRepo *repository.MongoRepo
	meiliRepo *repository.MeiliRepo
	validator *service.SchemaValidator
	syncSvc   *service.SyncService
}

func NewEntryHandler(
	mongoRepo *repository.MongoRepo,
	meiliRepo *repository.MeiliRepo,
	validator *service.SchemaValidator,
	syncSvc *service.SyncService,
) *EntryHandler {
	return &EntryHandler{
		mongoRepo: mongoRepo,
		meiliRepo: meiliRepo,
		validator: validator,
		syncSvc:   syncSvc,
	}
}

type CreateEntryRequest struct {
	SchemaKey  string                 `json:"schema_key" binding:"required"`
	Title      string                 `json:"title" binding:"required"`
	Body       string                 `json:"body"`
	Attributes map[string]interface{} `json:"attributes"`
}

func (h *EntryHandler) Create(c *gin.Context) {
	var req CreateEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	userID, _ := c.Get("user_id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	schema, err := h.mongoRepo.GetLatestSchema(ctx, req.SchemaKey)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFound(c, "schema not found")
			return
		}
		utils.InternalError(c, "failed to get schema")
		return
	}

	if req.Attributes == nil {
		req.Attributes = make(map[string]interface{})
	}

	if err := h.validator.ValidateEntry(*schema, req.Attributes); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	entry := &model.Entry{
		SchemaID:      schema.ID,
		SchemaKey:     schema.Key,
		SchemaVersion: schema.Version,
		AuthorID:      userID.(string),
		Base: model.BaseMeta{
			Title: req.Title,
		},
		Body:       req.Body,
		Attributes: req.Attributes,
	}

	if err := h.mongoRepo.CreateEntry(ctx, entry); err != nil {
		utils.InternalError(c, "failed to create entry")
		return
	}

	// Async sync to Meilisearch
	go func() {
		_ = h.syncSvc.SyncEntry(entry)
	}()

	utils.Created(c, entry)
}

type UpdateEntryRequest struct {
	Title      string                 `json:"title"`
	Body       string                 `json:"body"`
	Attributes map[string]interface{} `json:"attributes"`
}

func (h *EntryHandler) Update(c *gin.Context) {
	id := c.Param("id")
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		utils.BadRequest(c, "invalid entry id")
		return
	}

	var req UpdateEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	entry, err := h.mongoRepo.GetEntryByID(ctx, oid)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFound(c, "entry not found")
			return
		}
		utils.InternalError(c, "failed to get entry")
		return
	}

	// Check ownership or admin
	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")
	if entry.AuthorID != userID.(string) && userRole != "admin" {
		utils.Forbidden(c, "not authorized to update this entry")
		return
	}

	if req.Title != "" {
		entry.Base.Title = req.Title
	}
	if req.Body != "" {
		entry.Body = req.Body
	}
	if req.Attributes != nil {
		schema, err := h.mongoRepo.GetSchemaByID(ctx, entry.SchemaID)
		if err != nil {
			utils.InternalError(c, "failed to get schema")
			return
		}
		if err := h.validator.ValidateEntry(*schema, req.Attributes); err != nil {
			utils.BadRequest(c, err.Error())
			return
		}
		entry.Attributes = req.Attributes
	}

	if err := h.mongoRepo.UpdateEntry(ctx, entry); err != nil {
		utils.InternalError(c, "failed to update entry")
		return
	}

	go func() {
		_ = h.syncSvc.SyncEntry(entry)
	}()

	utils.Success(c, entry)
}

func (h *EntryHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		utils.BadRequest(c, "invalid entry id")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	entry, err := h.mongoRepo.GetEntryByID(ctx, oid)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFound(c, "entry not found")
			return
		}
		utils.InternalError(c, "failed to get entry")
		return
	}

	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")
	if entry.AuthorID != userID.(string) && userRole != "admin" {
		utils.Forbidden(c, "not authorized to delete this entry")
		return
	}

	if err := h.mongoRepo.DeleteEntry(ctx, oid); err != nil {
		utils.InternalError(c, "failed to delete entry")
		return
	}

	go func() {
		_ = h.syncSvc.DeleteEntry(id)
	}()

	utils.Success(c, nil)
}

func (h *EntryHandler) Get(c *gin.Context) {
	id := c.Param("id")
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		utils.BadRequest(c, "invalid entry id")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	entry, err := h.mongoRepo.GetEntryByID(ctx, oid)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFound(c, "entry not found")
			return
		}
		utils.InternalError(c, "failed to get entry")
		return
	}

	utils.Success(c, entry)
}

func (h *EntryHandler) List(c *gin.Context) {
	query := c.Query("q")
	schemaKey := c.Query("schema_key")
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.ParseInt(limitStr, 10, 64)
	offset, _ := strconv.ParseInt(offsetStr, 10, 64)

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var entries []model.Entry

	if query != "" && h.meiliRepo != nil {
		// Search via Meilisearch
		ids, err := h.meiliRepo.Search(query, schemaKey, limit, offset)
		if err != nil {
			utils.InternalError(c, "search failed")
			return
		}

		if len(ids) > 0 {
			oids := make([]primitive.ObjectID, 0, len(ids))
			for _, id := range ids {
				if oid, err := primitive.ObjectIDFromHex(id); err == nil {
					oids = append(oids, oid)
				}
			}
			entries, err = h.mongoRepo.GetEntriesByIDs(ctx, oids)
			if err != nil {
				utils.InternalError(c, "failed to get entries")
				return
			}
		}
	} else {
		// Direct MongoDB query
		var err error
		entries, err = h.mongoRepo.ListEntries(ctx, schemaKey, limit, offset)
		if err != nil {
			utils.InternalError(c, "failed to list entries")
			return
		}
	}

	utils.Success(c, entries)
}
