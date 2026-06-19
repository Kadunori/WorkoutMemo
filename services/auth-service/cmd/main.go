package main

import (
	"context"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmac/workoutmemo/auth-service/internal/handler"
	"github.com/kmac/workoutmemo/auth-service/internal/repository"
	"github.com/kmac/workoutmemo/auth-service/internal/service"
)

func main() {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	repo := repository.NewUserRepository(pool)
	svc := service.NewAuthService(repo, jwtSecret)
	h := handler.NewAuthHandler(svc, jwtSecret)

	r := gin.Default()
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "service": "auth"}) })
	r.POST("/auth/register", h.Register)
	r.POST("/auth/login", h.Login)
	r.GET("/auth/validate", h.Validate)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("auth-service :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server: %v", err)
	}
}
