package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/kmac/workoutmemo/workout-service/internal/model"
	"github.com/kmac/workoutmemo/workout-service/internal/service"
)

type WorkoutHandler struct {
	svc *service.WorkoutService
}

func NewWorkoutHandler(svc *service.WorkoutService) *WorkoutHandler {
	return &WorkoutHandler{svc: svc}
}

func userID(c *gin.Context) uuid.UUID {
	id, _ := uuid.Parse(c.GetString("user_id"))
	return id
}

func (h *WorkoutHandler) CreateSession(c *gin.Context) {
	var req model.CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sess, err := h.svc.CreateSession(c.Request.Context(), userID(c), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, sess)
}

func (h *WorkoutHandler) ListSessions(c *gin.Context) {
	var from, to *time.Time
	if s := c.Query("from"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err == nil {
			from = &t
		}
	}
	if s := c.Query("to"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err == nil {
			to = &t
		}
	}
	sessions, err := h.svc.ListSessions(c.Request.Context(), userID(c), c.Query("muscle_group"), from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sessions)
}

func (h *WorkoutHandler) GetSession(c *gin.Context) {
	sess, err := h.svc.GetSession(c.Request.Context(), userID(c), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if sess == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, sess)
}

func (h *WorkoutHandler) DeleteSession(c *gin.Context) {
	if err := h.svc.DeleteSession(c.Request.Context(), userID(c), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *WorkoutHandler) AddSet(c *gin.Context) {
	var req model.AddSetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s, err := h.svc.AddSet(c.Request.Context(), userID(c), c.Param("id"), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, s)
}

func (h *WorkoutHandler) UpdateSet(c *gin.Context) {
	var req model.UpdateSetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s, err := h.svc.UpdateSet(c.Request.Context(), c.Param("setId"), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *WorkoutHandler) DeleteSet(c *gin.Context) {
	if err := h.svc.DeleteSet(c.Request.Context(), c.Param("setId")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// GetLastSet は「前回重量の自動入力」用。クエリ: ?exercise=ベンチプレス&set=1
func (h *WorkoutHandler) GetLastSet(c *gin.Context) {
	setNum, _ := strconv.Atoi(c.Query("set"))
	if setNum < 1 {
		setNum = 1
	}
	s, err := h.svc.GetLastSet(c.Request.Context(), userID(c), c.Query("exercise"), setNum)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if s == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no previous record"})
		return
	}
	c.JSON(http.StatusOK, s)
}
