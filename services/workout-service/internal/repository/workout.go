package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmac/workoutmemo/workout-service/internal/model"
)

type WorkoutRepository struct {
	db *pgxpool.Pool
}

func NewWorkoutRepository(db *pgxpool.Pool) *WorkoutRepository {
	return &WorkoutRepository{db: db}
}

func (r *WorkoutRepository) CreateSession(ctx context.Context, userID uuid.UUID, muscleGroup string, date time.Time) (*model.WorkoutSession, error) {
	s := &model.WorkoutSession{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO workout_sessions (id, user_id, muscle_group, date)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, muscle_group, date, created_at`,
		uuid.New(), userID, muscleGroup, date,
	).Scan(&s.ID, &s.UserID, &s.MuscleGroup, &s.Date, &s.CreatedAt)
	return s, err
}

func (r *WorkoutRepository) ListSessions(ctx context.Context, userID uuid.UUID, muscleGroup string, from, to *time.Time) ([]model.WorkoutSession, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, muscle_group, date, created_at
		 FROM workout_sessions
		 WHERE user_id = $1
		   AND ($2 = '' OR muscle_group = $2)
		   AND ($3::timestamptz IS NULL OR date >= $3)
		   AND ($4::timestamptz IS NULL OR date <= $4)
		 ORDER BY date DESC`,
		userID, muscleGroup, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []model.WorkoutSession
	for rows.Next() {
		var s model.WorkoutSession
		if err := rows.Scan(&s.ID, &s.UserID, &s.MuscleGroup, &s.Date, &s.CreatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (r *WorkoutRepository) GetSession(ctx context.Context, id, userID uuid.UUID) (*model.WorkoutSession, error) {
	s := &model.WorkoutSession{}
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, muscle_group, date, created_at
		 FROM workout_sessions WHERE id = $1 AND user_id = $2`,
		id, userID,
	).Scan(&s.ID, &s.UserID, &s.MuscleGroup, &s.Date, &s.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return s, err
}

func (r *WorkoutRepository) DeleteSession(ctx context.Context, id, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM workout_sessions WHERE id = $1 AND user_id = $2`, id, userID)
	return err
}

func (r *WorkoutRepository) AddSet(ctx context.Context, sessionID uuid.UUID, req *model.AddSetRequest) (*model.WorkoutSet, error) {
	s := &model.WorkoutSet{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO workout_sets (id, session_id, exercise_name, equipment, set_number, weight, reps, rir)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, session_id, exercise_name, equipment, set_number, weight, reps, rir, created_at`,
		uuid.New(), sessionID, req.ExerciseName, req.Equipment,
		req.SetNumber, req.Weight, req.Reps, req.RIR,
	).Scan(&s.ID, &s.SessionID, &s.ExerciseName, &s.Equipment,
		&s.SetNumber, &s.Weight, &s.Reps, &s.RIR, &s.CreatedAt)
	return s, err
}

func (r *WorkoutRepository) ListSets(ctx context.Context, sessionID uuid.UUID) ([]model.WorkoutSet, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, session_id, exercise_name, equipment, set_number, weight, reps, rir, created_at
		 FROM workout_sets WHERE session_id = $1
		 ORDER BY exercise_name, set_number`,
		sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sets []model.WorkoutSet
	for rows.Next() {
		var s model.WorkoutSet
		if err := rows.Scan(&s.ID, &s.SessionID, &s.ExerciseName, &s.Equipment,
			&s.SetNumber, &s.Weight, &s.Reps, &s.RIR, &s.CreatedAt); err != nil {
			return nil, err
		}
		sets = append(sets, s)
	}
	return sets, nil
}

func (r *WorkoutRepository) UpdateSet(ctx context.Context, id uuid.UUID, req *model.UpdateSetRequest) (*model.WorkoutSet, error) {
	s := &model.WorkoutSet{}
	err := r.db.QueryRow(ctx,
		`UPDATE workout_sets SET weight=$2, reps=$3, rir=$4
		 WHERE id=$1
		 RETURNING id, session_id, exercise_name, equipment, set_number, weight, reps, rir, created_at`,
		id, req.Weight, req.Reps, req.RIR,
	).Scan(&s.ID, &s.SessionID, &s.ExerciseName, &s.Equipment,
		&s.SetNumber, &s.Weight, &s.Reps, &s.RIR, &s.CreatedAt)
	return s, err
}

func (r *WorkoutRepository) DeleteSet(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM workout_sets WHERE id = $1`, id)
	return err
}

// GetLastSet は「前回重量の自動入力」機能のため、同種目・同セット番号の最新記録を返す。
func (r *WorkoutRepository) GetLastSet(ctx context.Context, userID uuid.UUID, exerciseName string, setNumber int) (*model.WorkoutSet, error) {
	s := &model.WorkoutSet{}
	err := r.db.QueryRow(ctx,
		`SELECT ws.id, ws.session_id, ws.exercise_name, ws.equipment,
		        ws.set_number, ws.weight, ws.reps, ws.rir, ws.created_at
		 FROM workout_sets ws
		 JOIN workout_sessions sess ON sess.id = ws.session_id
		 WHERE sess.user_id = $1
		   AND ws.exercise_name = $2
		   AND ws.set_number = $3
		 ORDER BY ws.created_at DESC
		 LIMIT 1`,
		userID, exerciseName, setNumber,
	).Scan(&s.ID, &s.SessionID, &s.ExerciseName, &s.Equipment,
		&s.SetNumber, &s.Weight, &s.Reps, &s.RIR, &s.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return s, err
}

// ListAllUserSets は export-service が CSV 生成に使う全件取得。
func (r *WorkoutRepository) ListAllUserSets(ctx context.Context, userID uuid.UUID) ([]model.WorkoutSet, error) {
	rows, err := r.db.Query(ctx,
		`SELECT ws.id, ws.session_id, ws.exercise_name, ws.equipment,
		        ws.set_number, ws.weight, ws.reps, ws.rir, ws.created_at
		 FROM workout_sets ws
		 JOIN workout_sessions sess ON sess.id = ws.session_id
		 WHERE sess.user_id = $1
		 ORDER BY ws.created_at DESC`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sets []model.WorkoutSet
	for rows.Next() {
		var s model.WorkoutSet
		if err := rows.Scan(&s.ID, &s.SessionID, &s.ExerciseName, &s.Equipment,
			&s.SetNumber, &s.Weight, &s.Reps, &s.RIR, &s.CreatedAt); err != nil {
			return nil, err
		}
		sets = append(sets, s)
	}
	return sets, nil
}
