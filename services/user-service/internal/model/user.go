package model

import (
	"time"

	"github.com/google/uuid"
)

type UserProfile struct {
	UserID    uuid.UUID `json:"user_id"`
	Height    *float64  `json:"height,omitempty"` // cm
	Unit      string    `json:"unit"`             // kg / lb
	UpdatedAt time.Time `json:"updated_at"`
}

type BodyWeightRecord struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	Weight     float64   `json:"weight"`
	RecordedAt time.Time `json:"recorded_at"`
}

type UpdateProfileRequest struct {
	Height *float64 `json:"height"`
	Unit   string   `json:"unit"`
}

type AddWeightRequest struct {
	Weight     float64    `json:"weight"      binding:"required,min=0"`
	RecordedAt *time.Time `json:"recorded_at"`
}
