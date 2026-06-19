package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmac/workoutmemo/user-service/internal/model"
	"github.com/kmac/workoutmemo/user-service/internal/repository"
)

type UserHandler struct {
	repo *repository.UserRepository
}

func NewUserHandler(db *pgxpool.Pool) *UserHandler {
	return &UserHandler{repo: repository.NewUserRepository(db)}
}

func userID(c *gin.Context) uuid.UUID {
	id, _ := uuid.Parse(c.GetString("user_id"))
	return id
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	p, err := h.repo.GetProfile(c.Request.Context(), userID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if p == nil {
		c.JSON(http.StatusOK, model.UserProfile{UserID: userID(c), Unit: "kg"})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	var req model.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := h.repo.UpsertProfile(c.Request.Context(), userID(c), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *UserHandler) ListWeights(c *gin.Context) {
	var from *time.Time
	if s := c.Query("from"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err == nil {
			from = &t
		}
	}
	records, err := h.repo.ListWeights(c.Request.Context(), userID(c), from)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, records)
}

func (h *UserHandler) AddWeight(c *gin.Context) {
	var req model.AddWeightRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	at := time.Now().UTC()
	if req.RecordedAt != nil {
		at = *req.RecordedAt
	}
	rec, err := h.repo.AddWeight(c.Request.Context(), userID(c), req.Weight, at)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, rec)
}

func (h *UserHandler) DeleteWeight(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.repo.DeleteWeight(c.Request.Context(), id, userID(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
