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
	sessionStore := service.NewSessionStore(mongoRepo)

	// Initialize handlers
	schemaHandler := handler.NewSchemaHandler(mongoRepo)
	entryHandler := handler.NewEntryHandler(mongoRepo, meiliRepo, validator, syncSvc)
	authHandler := handler.NewAuthHandler(authService, sessionStore, cfg)
	taxonomyHandler := handler.NewTaxonomyHandler(mongoRepo)
	termHandler := handler.NewTermHandler(mongoRepo)
	commentHandler := handler.NewCommentHandler(mongoRepo)

	// Setup Gin router
	r := gin.Default()

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API routes
	v1 := r.Group("/api/v1")
	{
		// Auth routes
		auth := v1.Group("/auth")
		{
			auth.GET("/signin/:provider", authHandler.SignIn)
			auth.GET("/callback/:provider", authHandler.Callback)
			auth.GET("/session", handler.OptionalAuthMiddleware(sessionStore), authHandler.Session)
			auth.POST("/signout", authHandler.SignOut)
		}

		// Schema routes (admin only)
		schemas := v1.Group("/schemas")
		schemas.Use(handler.AuthMiddleware(sessionStore), handler.AdminMiddleware())
		{
			schemas.POST("", schemaHandler.Create)
			schemas.GET("", schemaHandler.List)
			schemas.GET("/:key", schemaHandler.Get)
			schemas.DELETE("/:key", schemaHandler.Delete)
		}

		// Entry routes
		entries := v1.Group("/entries")
		{
			entries.GET("", handler.OptionalAuthMiddleware(sessionStore), entryHandler.List)
			entries.GET("/:id", handler.OptionalAuthMiddleware(sessionStore), entryHandler.Get)
			entries.POST("", handler.AuthMiddleware(sessionStore), entryHandler.Create)
			entries.PUT("/:id", handler.AuthMiddleware(sessionStore), entryHandler.Update)
			entries.DELETE("/:id", handler.AuthMiddleware(sessionStore), entryHandler.Delete)
		}

		// Taxonomy routes
		taxonomies := v1.Group("/taxonomies")
		{
			taxonomies.GET("", taxonomyHandler.List)
			taxonomies.GET("/:key", taxonomyHandler.Get)
			taxonomies.POST("", handler.AuthMiddleware(sessionStore), handler.AdminMiddleware(), taxonomyHandler.Create)
			taxonomies.PUT("/:key", handler.AuthMiddleware(sessionStore), handler.AdminMiddleware(), taxonomyHandler.Update)
			taxonomies.DELETE("/:key", handler.AuthMiddleware(sessionStore), handler.AdminMiddleware(), taxonomyHandler.Delete)
		}

		// Term routes
		terms := v1.Group("/terms")
		{
			terms.GET("/taxonomy/:key", termHandler.ListByTaxonomy)
			terms.GET("/:id", termHandler.Get)
			terms.POST("", handler.AuthMiddleware(sessionStore), handler.AdminMiddleware(), termHandler.Create)
			terms.PUT("/:id", handler.AuthMiddleware(sessionStore), handler.AdminMiddleware(), termHandler.Update)
			terms.DELETE("/:id", handler.AuthMiddleware(sessionStore), handler.AdminMiddleware(), termHandler.Delete)
		}

		// Comment routes
		comments := v1.Group("/comments")
		{
			comments.GET("/entry/:entry_id", commentHandler.ListByEntry)
			comments.POST("", handler.AuthMiddleware(sessionStore), commentHandler.Create)
			comments.DELETE("/:id", handler.AuthMiddleware(sessionStore), commentHandler.Delete)
		}
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
