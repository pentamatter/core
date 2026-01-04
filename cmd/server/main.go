package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"matter-core/internal/config"
	"matter-core/internal/handler"
	"matter-core/internal/repository"
	"matter-core/internal/service"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	// Initialize MongoDB
	mongoRepo, err := repository.NewMongoRepo(cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = mongoRepo.Close(ctx)
	}()

	// Initialize Meilisearch (optional)
	var meiliRepo *repository.MeiliRepo
	if cfg.MeilisearchHost != "" {
		meiliRepo, err = repository.NewMeiliRepo(cfg.MeilisearchHost, cfg.MeilisearchKey)
		if err != nil {
			log.Printf("Warning: Failed to connect to Meilisearch: %v", err)
		}
	}

	// Initialize services
	validator := service.NewSchemaValidator(mongoRepo)
	var syncSvc *service.SyncService
	if meiliRepo != nil {
		syncSvc = service.NewSyncService(meiliRepo)
	}
	authService := service.NewAuthService(mongoRepo, cfg)

	// Initialize handlers
	schemaHandler := handler.NewSchemaHandler(mongoRepo)
	entryHandler := handler.NewEntryHandler(mongoRepo, meiliRepo, validator, syncSvc)
	authHandler := handler.NewAuthHandler(authService)
	taxonomyHandler := handler.NewTaxonomyHandler(mongoRepo)
	termHandler := handler.NewTermHandler(mongoRepo)
	commentHandler := handler.NewCommentHandler(mongoRepo)

	// Setup Gin router
	r := gin.Default()

	// API routes
	v1 := r.Group("/api/v1")
	{
		// Auth routes (public)
		auth := v1.Group("/auth")
		{
			auth.GET("/login/:provider", authHandler.Login)
			auth.GET("/callback/:provider", authHandler.Callback)
		}

		// Schema routes (admin only)
		schemas := v1.Group("/schemas")
		schemas.Use(handler.AuthMiddleware(authService), handler.AdminMiddleware())
		{
			schemas.POST("", schemaHandler.Create)
			schemas.GET("", schemaHandler.List)
			schemas.GET("/:key", schemaHandler.Get)
		}

		// Entry routes
		entries := v1.Group("/entries")
		{
			entries.GET("", handler.OptionalAuthMiddleware(authService), entryHandler.List)
			entries.GET("/:id", handler.OptionalAuthMiddleware(authService), entryHandler.Get)
			entries.POST("", handler.AuthMiddleware(authService), entryHandler.Create)
			entries.PUT("/:id", handler.AuthMiddleware(authService), entryHandler.Update)
			entries.DELETE("/:id", handler.AuthMiddleware(authService), entryHandler.Delete)
		}

		// Taxonomy routes (admin only for create)
		taxonomies := v1.Group("/taxonomies")
		{
			taxonomies.GET("", taxonomyHandler.List)
			taxonomies.GET("/:key", taxonomyHandler.Get)
			taxonomies.POST("", handler.AuthMiddleware(authService), handler.AdminMiddleware(), taxonomyHandler.Create)
		}

		// Term routes
		terms := v1.Group("/terms")
		{
			terms.GET("/taxonomy/:key", termHandler.ListByTaxonomy)
			terms.GET("/:id", termHandler.Get)
			terms.POST("", handler.AuthMiddleware(authService), handler.AdminMiddleware(), termHandler.Create)
		}

		// Comment routes
		comments := v1.Group("/comments")
		{
			comments.GET("/entry/:entry_id", commentHandler.ListByEntry)
			comments.POST("", handler.AuthMiddleware(authService), commentHandler.Create)
			comments.DELETE("/:id", handler.AuthMiddleware(authService), commentHandler.Delete)
		}

		// User profile
		v1.GET("/me", handler.AuthMiddleware(authService), authHandler.Me)
	}

	// Graceful shutdown
	go func() {
		if err := r.Run(":" + cfg.Port); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
}
