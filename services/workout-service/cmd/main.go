package main

import (
	"context"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmac/workoutmemo/workout-service/internal/handler"
	"github.com/kmac/workoutmemo/workout-service/internal/middleware"
	"github.com/kmac/workoutmemo/workout-service/internal/repository"
	"github.com/kmac/workoutmemo/workout-service/internal/service"
)

func main() {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	repo := repository.NewWorkoutRepository(pool)
	svc := service.NewWorkoutService(repo)
	h := handler.NewWorkoutHandler(svc)

	r := gin.Default()
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "service": "workout"}) })

	auth := r.Group("/", middleware.JWT(os.Getenv("JWT_SECRET")))
	auth.POST("/workouts", h.CreateSession)
	auth.GET("/workouts", h.ListSessions)
	auth.GET("/workouts/:id", h.GetSession)
	auth.DELETE("/workouts/:id", h.DeleteSession)
	auth.POST("/workouts/:id/sets", h.AddSet)
	auth.PUT("/workouts/:id/sets/:setId", h.UpdateSet)
	auth.DELETE("/workouts/:id/sets/:setId", h.DeleteSet)
	auth.GET("/workouts/last-set", h.GetLastSet) // 前回重量取得

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("workout-service :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server: %v", err)
	}
}
