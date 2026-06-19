CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS exercises (
    id           UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    name         VARCHAR(255) NOT NULL,
    muscle_group VARCHAR(50)  NOT NULL,
    equipment    VARCHAR(50)  NOT NULL DEFAULT '',
    is_default   BOOLEAN      NOT NULL DEFAULT false,
    UNIQUE (name, muscle_group, equipment)
);

CREATE INDEX IF NOT EXISTS idx_exercises_muscle ON exercises(muscle_group);

CREATE TABLE IF NOT EXISTS user_exercises (
    id          UUID    PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID    NOT NULL,
    exercise_id UUID    NOT NULL REFERENCES exercises(id),
    sort_order  INTEGER NOT NULL DEFAULT 0,
    UNIQUE (user_id, exercise_id)
);

CREATE INDEX IF NOT EXISTS idx_user_exercises_user ON user_exercises(user_id, sort_order);
