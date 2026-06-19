package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmac/workoutmemo/menu-service/internal/model"
	"github.com/kmac/workoutmemo/menu-service/internal/repository"
)

type MenuHandler struct {
	repo *repository.MenuRepository
}

func NewMenuHandler(db *pgxpool.Pool) *MenuHandler {
	return &MenuHandler{repo: repository.NewMenuRepository(db)}
}

func userID(c *gin.Context) uuid.UUID {
	id, _ := uuid.Parse(c.GetString("user_id"))
	return id
}

func (h *MenuHandler) ListDefaults(c *gin.Context) {
	list, err := h.repo.ListDefaults(c.Request.Context(), c.Query("muscle_group"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *MenuHandler) ListUserExercises(c *gin.Context) {
	list, err := h.repo.ListUserExercises(c.Request.Context(), userID(c), c.Query("muscle_group"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *MenuHandler) AddExercise(c *gin.Context) {
	var req model.AddExerciseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ExerciseID != "" {
		// 既存種目をユーザーリストへ追加
		exID, err := uuid.Parse(req.ExerciseID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid exercise_id"})
			return
		}
		ue, err := h.repo.AddExisting(c.Request.Context(), userID(c), exID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, ue)
		return
	}
	// カスタム種目を新規作成してリストへ追加
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name or exercise_id required"})
		return
	}
	ue, err := h.repo.CreateCustomAndAdd(c.Request.Context(), userID(c), req.Name, req.MuscleGroup, req.Equipment)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ue)
}

func (h *MenuHandler) RemoveExercise(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.repo.RemoveUserExercise(c.Request.Context(), id, userID(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *MenuHandler) Reorder(c *gin.Context) {
	var req model.ReorderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, s := range req.IDs {
		id, err := uuid.Parse(s)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id: " + s})
			return
		}
		ids = append(ids, id)
	}
	if err := h.repo.Reorder(c.Request.Context(), userID(c), ids); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
