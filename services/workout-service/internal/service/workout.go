package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kmac/workoutmemo/workout-service/internal/model"
	"github.com/kmac/workoutmemo/workout-service/internal/repository"
)

type WorkoutService struct {
	repo *repository.WorkoutRepository
}

func NewWorkoutService(repo *repository.WorkoutRepository) *WorkoutService {
	return &WorkoutService{repo: repo}
}

func (s *WorkoutService) CreateSession(ctx context.Context, userID uuid.UUID, req *model.CreateSessionRequest) (*model.WorkoutSession, error) {
	date := time.Now().UTC()
	if req.Date != nil {
		date = *req.Date
	}
	return s.repo.CreateSession(ctx, userID, req.MuscleGroup, date)
}

func (s *WorkoutService) ListSessions(ctx context.Context, userID uuid.UUID, muscleGroup string, from, to *time.Time) ([]model.WorkoutSession, error) {
	return s.repo.ListSessions(ctx, userID, muscleGroup, from, to)
}

func (s *WorkoutService) GetSession(ctx context.Context, userID uuid.UUID, sessionID string) (*model.WorkoutSession, error) {
	id, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, fmt.Errorf("invalid session id")
	}
	sess, err := s.repo.GetSession(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, nil
	}
	sets, err := s.repo.ListSets(ctx, id)
	if err != nil {
		return nil, err
	}
	sess.Sets = sets
	return sess, nil
}

func (s *WorkoutService) DeleteSession(ctx context.Context, userID uuid.UUID, sessionID string) error {
	id, err := uuid.Parse(sessionID)
	if err != nil {
		return fmt.Errorf("invalid session id")
	}
	return s.repo.DeleteSession(ctx, id, userID)
}

func (s *WorkoutService) AddSet(ctx context.Context, userID uuid.UUID, sessionID string, req *model.AddSetRequest) (*model.WorkoutSet, error) {
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, fmt.Errorf("invalid session id")
	}
	// セッションの所有権確認
	sess, err := s.repo.GetSession(ctx, sid, userID)
	if err != nil || sess == nil {
		return nil, fmt.Errorf("session not found")
	}
	return s.repo.AddSet(ctx, sid, req)
}

func (s *WorkoutService) UpdateSet(ctx context.Context, setID string, req *model.UpdateSetRequest) (*model.WorkoutSet, error) {
	id, err := uuid.Parse(setID)
	if err != nil {
		return nil, fmt.Errorf("invalid set id")
	}
	return s.repo.UpdateSet(ctx, id, req)
}

func (s *WorkoutService) DeleteSet(ctx context.Context, setID string) error {
	id, err := uuid.Parse(setID)
	if err != nil {
		return fmt.Errorf("invalid set id")
	}
	return s.repo.DeleteSet(ctx, id)
}

func (s *WorkoutService) GetLastSet(ctx context.Context, userID uuid.UUID, exerciseName string, setNumber int) (*model.WorkoutSet, error) {
	return s.repo.GetLastSet(ctx, userID, exerciseName, setNumber)
}
