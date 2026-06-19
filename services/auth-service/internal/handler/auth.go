package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/kmac/workoutmemo/auth-service/internal/model"
	"github.com/kmac/workoutmemo/auth-service/internal/service"
)

type AuthHandler struct {
	svc       *service.AuthService
	jwtSecret []byte
}

func NewAuthHandler(svc *service.AuthService, jwtSecret string) *AuthHandler {
	return &AuthHandler{svc: svc, jwtSecret: []byte(jwtSecret)}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := h.svc.Register(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Validate はトークンを検証し user_id / email を返す。他サービスからも呼ばれる。
func (h *AuthHandler) Validate(c *gin.Context) {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}
	tokenStr := strings.TrimPrefix(header, "Bearer ")
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, gin.Error{}
		}
		return h.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	claims := token.Claims.(jwt.MapClaims)
	c.JSON(http.StatusOK, model.ValidateResponse{
		UserID: claims["sub"].(string),
		Email:  claims["email"].(string),
	})
}
