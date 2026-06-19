package main

import (
	"context"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmac/workoutmemo/menu-service/internal/handler"
	"github.com/kmac/workoutmemo/menu-service/internal/middleware"
)

func main() {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	h := handler.NewMenuHandler(pool)

	r := gin.Default()
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "service": "menu"}) })

	r.GET("/menus/defaults", h.ListDefaults) // 認証不要（デフォルト一覧は公開）

	auth := r.Group("/", middleware.JWT(os.Getenv("JWT_SECRET")))
	auth.GET("/menus", h.ListUserExercises)
	auth.POST("/menus", h.AddExercise)
	auth.DELETE("/menus/:id", h.RemoveExercise)
	auth.PUT("/menus/order", h.Reorder)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("menu-service :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server: %v", err)
	}
}
