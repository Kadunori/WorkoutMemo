package service

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/kmac/workoutmemo/auth-service/internal/model"
	"github.com/kmac/workoutmemo/auth-service/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	repo      *repository.UserRepository
	jwtSecret []byte
}

func NewAuthService(repo *repository.UserRepository, jwtSecret string) *AuthService {
	return &AuthService{repo: repo, jwtSecret: []byte(jwtSecret)}
}

func (s *AuthService) Register(ctx context.Context, email, password string) (*model.AuthResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	user, err := s.repo.Create(ctx, email, string(hash))
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	token, err := s.issueToken(user)
	if err != nil {
		return nil, err
	}
	return &model.AuthResponse{Token: token, User: *user}, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*model.AuthResponse, error) {
	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	token, err := s.issueToken(user)
	if err != nil {
		return nil, err
	}
	return &model.AuthResponse{Token: token, User: *user}, nil
}

func (s *AuthService) issueToken(user *model.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID.String(),
		"email": user.Email,
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}
