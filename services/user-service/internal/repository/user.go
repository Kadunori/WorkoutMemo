package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmac/workoutmemo/user-service/internal/model"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetProfile(ctx context.Context, userID uuid.UUID) (*model.UserProfile, error) {
	p := &model.UserProfile{}
	err := r.db.QueryRow(ctx,
		`SELECT user_id, height, unit, updated_at FROM user_profiles WHERE user_id = $1`,
		userID,
	).Scan(&p.UserID, &p.Height, &p.Unit, &p.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (r *UserRepository) UpsertProfile(ctx context.Context, userID uuid.UUID, req *model.UpdateProfileRequest) (*model.UserProfile, error) {
	unit := req.Unit
	if unit == "" {
		unit = "kg"
	}
	p := &model.UserProfile{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO user_profiles (user_id, height, unit)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id) DO UPDATE
		   SET height = EXCLUDED.height,
		       unit   = EXCLUDED.unit,
		       updated_at = NOW()
		 RETURNING user_id, height, unit, updated_at`,
		userID, req.Height, unit,
	).Scan(&p.UserID, &p.Height, &p.Unit, &p.UpdatedAt)
	return p, err
}

func (r *UserRepository) AddWeight(ctx context.Context, userID uuid.UUID, weight float64, at time.Time) (*model.BodyWeightRecord, error) {
	rec := &model.BodyWeightRecord{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO body_weight_records (id, user_id, weight, recorded_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, weight, recorded_at`,
		uuid.New(), userID, weight, at,
	).Scan(&rec.ID, &rec.UserID, &rec.Weight, &rec.RecordedAt)
	return rec, err
}

func (r *UserRepository) ListWeights(ctx context.Context, userID uuid.UUID, from *time.Time) ([]model.BodyWeightRecord, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, weight, recorded_at
		 FROM body_weight_records
		 WHERE user_id = $1
		   AND ($2::timestamptz IS NULL OR recorded_at >= $2)
		 ORDER BY recorded_at ASC`,
		userID, from,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []model.BodyWeightRecord
	for rows.Next() {
		var rec model.BodyWeightRecord
		if err := rows.Scan(&rec.ID, &rec.UserID, &rec.Weight, &rec.RecordedAt); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, nil
}

func (r *UserRepository) DeleteWeight(ctx context.Context, id, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM body_weight_records WHERE id = $1 AND user_id = $2`, id, userID)
	return err
}
