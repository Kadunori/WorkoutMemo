package main

import (
	"context"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmac/workoutmemo/user-service/internal/handler"
	"github.com/kmac/workoutmemo/user-service/internal/middleware"
)

func main() {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	h := handler.NewUserHandler(pool)

	r := gin.Default()
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "service": "user"}) })

	auth := r.Group("/", middleware.JWT(os.Getenv("JWT_SECRET")))
	auth.GET("/users/profile", h.GetProfile)
	auth.PUT("/users/profile", h.UpdateProfile)
	auth.GET("/users/weight", h.ListWeights)
	auth.POST("/users/weight", h.AddWeight)
	auth.DELETE("/users/weight/:id", h.DeleteWeight)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("user-service :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server: %v", err)
	}
}
