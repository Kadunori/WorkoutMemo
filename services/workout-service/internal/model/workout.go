package model

import (
	"time"

	"github.com/google/uuid"
)

type WorkoutSession struct {
	ID          uuid.UUID    `json:"id"`
	UserID      uuid.UUID    `json:"user_id"`
	MuscleGroup string       `json:"muscle_group"` // chest/back/delts/legs/arms
	Date        time.Time    `json:"date"`
	Sets        []WorkoutSet `json:"sets,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
}

type WorkoutSet struct {
	ID           uuid.UUID `json:"id"`
	SessionID    uuid.UUID `json:"session_id"`
	ExerciseName string    `json:"exercise_name"`
	Equipment    string    `json:"equipment"` // bb/db/smith/machine/cable/bw/iso
	SetNumber    int       `json:"set_number"`
	Weight       float64   `json:"weight"`
	Reps         int       `json:"reps"`
	RIR          *int      `json:"rir,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type CreateSessionRequest struct {
	MuscleGroup string     `json:"muscle_group" binding:"required"`
	Date        *time.Time `json:"date"`
}

type AddSetRequest struct {
	ExerciseName string  `json:"exercise_name" binding:"required"`
	Equipment    string  `json:"equipment"`
	SetNumber    int     `json:"set_number"    binding:"required,min=1"`
	Weight       float64 `json:"weight"        binding:"min=0"`
	Reps         int     `json:"reps"          binding:"required,min=1"`
	RIR          *int    `json:"rir"`
}

type UpdateSetRequest struct {
	Weight float64 `json:"weight" binding:"min=0"`
	Reps   int     `json:"reps"   binding:"min=1"`
	RIR    *int    `json:"rir"`
}
