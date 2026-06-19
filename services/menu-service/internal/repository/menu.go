package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmac/workoutmemo/menu-service/internal/model"
)

type MenuRepository struct {
	db *pgxpool.Pool
}

func NewMenuRepository(db *pgxpool.Pool) *MenuRepository {
	return &MenuRepository{db: db}
}

func (r *MenuRepository) ListDefaults(ctx context.Context, muscleGroup string) ([]model.Exercise, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, muscle_group, equipment, is_default
		 FROM exercises
		 WHERE is_default = true
		   AND ($1 = '' OR muscle_group = $1)
		 ORDER BY muscle_group, equipment, name`,
		muscleGroup,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Exercise
	for rows.Next() {
		var e model.Exercise
		if err := rows.Scan(&e.ID, &e.Name, &e.MuscleGroup, &e.Equipment, &e.IsDefault); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, nil
}

func (r *MenuRepository) ListUserExercises(ctx context.Context, userID uuid.UUID, muscleGroup string) ([]model.UserExercise, error) {
	rows, err := r.db.Query(ctx,
		`SELECT ue.id, ue.user_id, e.id, e.name, e.muscle_group, e.equipment, e.is_default, ue.sort_order
		 FROM user_exercises ue
		 JOIN exercises e ON e.id = ue.exercise_id
		 WHERE ue.user_id = $1
		   AND ($2 = '' OR e.muscle_group = $2)
		 ORDER BY ue.sort_order`,
		userID, muscleGroup,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.UserExercise
	for rows.Next() {
		var ue model.UserExercise
		if err := rows.Scan(&ue.ID, &ue.UserID,
			&ue.Exercise.ID, &ue.Exercise.Name, &ue.Exercise.MuscleGroup, &ue.Exercise.Equipment, &ue.Exercise.IsDefault,
			&ue.SortOrder); err != nil {
			return nil, err
		}
		list = append(list, ue)
	}
	return list, nil
}

// CreateCustomAndAdd はカスタム種目を exercises に追加し、ユーザーのリストにも追加する。
func (r *MenuRepository) CreateCustomAndAdd(ctx context.Context, userID uuid.UUID, name, muscleGroup, equipment string) (*model.UserExercise, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var exID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO exercises (id, name, muscle_group, equipment, is_default)
		 VALUES ($1, $2, $3, $4, false)
		 ON CONFLICT (name, muscle_group, equipment) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id`,
		uuid.New(), name, muscleGroup, equipment,
	).Scan(&exID)
	if err != nil {
		return nil, err
	}

	var maxOrder int
	_ = tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(sort_order), 0) FROM user_exercises WHERE user_id = $1`, userID,
	).Scan(&maxOrder)

	ue := &model.UserExercise{}
	err = tx.QueryRow(ctx,
		`INSERT INTO user_exercises (id, user_id, exercise_id, sort_order)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, sort_order`,
		uuid.New(), userID, exID, maxOrder+1,
	).Scan(&ue.ID, &ue.UserID, &ue.SortOrder)
	if err != nil {
		return nil, err
	}
	ue.Exercise = model.Exercise{ID: exID, Name: name, MuscleGroup: muscleGroup, Equipment: equipment}
	return ue, tx.Commit(ctx)
}

// AddExisting は既存の exercises をユーザーのリストに追加する。
func (r *MenuRepository) AddExisting(ctx context.Context, userID, exerciseID uuid.UUID) (*model.UserExercise, error) {
	var maxOrder int
	_ = r.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(sort_order), 0) FROM user_exercises WHERE user_id = $1`, userID,
	).Scan(&maxOrder)

	ue := &model.UserExercise{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO user_exercises (id, user_id, exercise_id, sort_order)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_id, exercise_id) DO NOTHING
		 RETURNING id, user_id, sort_order`,
		uuid.New(), userID, exerciseID, maxOrder+1,
	).Scan(&ue.ID, &ue.UserID, &ue.SortOrder)
	if err != nil {
		return nil, err
	}
	return ue, nil
}

func (r *MenuRepository) RemoveUserExercise(ctx context.Context, id, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM user_exercises WHERE id = $1 AND user_id = $2`, id, userID)
	return err
}

func (r *MenuRepository) Reorder(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for i, id := range ids {
		if _, err := tx.Exec(ctx,
			`UPDATE user_exercises SET sort_order = $1 WHERE id = $2 AND user_id = $3`,
			i+1, id, userID,
		); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
