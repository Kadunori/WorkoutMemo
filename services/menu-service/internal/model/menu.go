package model

import "github.com/google/uuid"

type Exercise struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	MuscleGroup string    `json:"muscle_group"` // chest/back/delts_front/...
	Equipment   string    `json:"equipment"`    // bb/db/smith/machine/cable/bw/iso
	IsDefault   bool      `json:"is_default"`
}

type UserExercise struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	Exercise   Exercise  `json:"exercise"`
	SortOrder  int       `json:"sort_order"`
}

type AddExerciseRequest struct {
	ExerciseID string `json:"exercise_id"` // 既存種目を追加
	// または新規カスタム種目
	Name        string `json:"name"`
	MuscleGroup string `json:"muscle_group"`
	Equipment   string `json:"equipment"`
}

type ReorderRequest struct {
	IDs []string `json:"ids" binding:"required"` // 新しい順序で user_exercise の id を列挙
}
