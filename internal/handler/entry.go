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
	SchemaKey  string         `json:"schema_key" binding:"required"`
	Title      string         `json:"title" binding:"required,max=200"`
	Slug       string         `json:"slug" binding:"max=200"`
	Body       string         `json:"body" binding:"max=100000"`
	Draft      bool           `json:"draft"`
	Attributes map[string]any `json:"attributes"`
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
			Slug:  req.Slug,
			Draft: req.Draft,
		},
		Body:       req.Body,
		Attributes: req.Attributes,
	}

	if err := h.mongoRepo.CreateEntry(ctx, entry); err != nil {
		utils.InternalError(c, "failed to create entry")
		return
	}

	// Async sync to Meilisearch with retry
	if h.syncSvc != nil {
		h.syncSvc.SyncEntryAsync(entry)
	}

	utils.Created(c, entry)
}

type UpdateEntryRequest struct {
	Title      *string        `json:"title" binding:"omitempty,max=200"`
	Slug       *string        `json:"slug" binding:"omitempty,max=200"`
	Body       *string        `json:"body" binding:"omitempty,max=100000"`
	Draft      *bool          `json:"draft"`
	Attributes map[string]any `json:"attributes"`
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

	// Use pointer to distinguish between "not provided" and "set to empty"
	if req.Title != nil {
		entry.Base.Title = *req.Title
	}
	if req.Slug != nil {
		entry.Base.Slug = *req.Slug
	}
	if req.Body != nil {
		entry.Body = *req.Body
	}
	if req.Draft != nil {
		entry.Base.Draft = *req.Draft
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

	if h.syncSvc != nil {
		h.syncSvc.SyncEntryAsync(entry)
	}

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

	if h.syncSvc != nil {
		h.syncSvc.DeleteEntryAsync(id)
	}

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
	draftParam := c.Query("draft")
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.ParseInt(limitStr, 10, 64)
	offset, _ := strconv.ParseInt(offsetStr, 10, 64)

	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	// 处理 draft 过滤
	var draft *bool
	userRole, _ := c.Get("user_role")
	if draftParam != "" {
		// 只有管理员可以查看草稿
		if userRole == "admin" {
			d := draftParam == "true"
			draft = &d
		}
	} else {
		// 默认只显示已发布的文章（非管理员）
		if userRole != "admin" {
			d := false
			draft = &d
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var entries []model.Entry
	var total int64

	if query != "" && h.meiliRepo != nil {
		// Search via Meilisearch
		ids, searchTotal, err := h.meiliRepo.Search(query, schemaKey, limit, offset)
		if err != nil {
			utils.InternalError(c, "search failed")
			return
		}
		total = searchTotal

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
			// 过滤草稿（搜索结果需要二次过滤）
			if draft != nil && !*draft {
				filtered := make([]model.Entry, 0)
				for _, e := range entries {
					if !e.Base.Draft {
						filtered = append(filtered, e)
					}
				}
				entries = filtered
			}
		} else {
			entries = []model.Entry{}
		}
	} else {
		// Direct MongoDB query
		var err error
		entries, err = h.mongoRepo.ListEntries(ctx, schemaKey, draft, limit, offset)
		if err != nil {
			utils.InternalError(c, "failed to list entries")
			return
		}
		total, err = h.mongoRepo.CountEntries(ctx, schemaKey, draft)
		if err != nil {
			utils.InternalError(c, "failed to count entries")
			return
		}
	}

	// Always return array, never nil
	if entries == nil {
		entries = []model.Entry{}
	}

	utils.SuccessWithPagination(c, entries, total, limit, offset)
}
