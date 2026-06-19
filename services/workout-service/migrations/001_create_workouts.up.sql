CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS workout_sessions (
    id           UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id      UUID         NOT NULL,
    muscle_group VARCHAR(50)  NOT NULL,
    date         DATE         NOT NULL DEFAULT CURRENT_DATE,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_date ON workout_sessions(user_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_user_muscle ON workout_sessions(user_id, muscle_group);

CREATE TABLE IF NOT EXISTS workout_sets (
    id            UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id    UUID         NOT NULL REFERENCES workout_sessions(id) ON DELETE CASCADE,
    exercise_name VARCHAR(255) NOT NULL,
    equipment     VARCHAR(50)  NOT NULL DEFAULT '',
    set_number    SMALLINT     NOT NULL,
    weight        NUMERIC(6,2) NOT NULL DEFAULT 0,
    reps          SMALLINT     NOT NULL,
    rir           SMALLINT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sets_session ON workout_sets(session_id);
-- 前回重量検索用（exercise_name + set_number で最新レコードを引く）
CREATE INDEX IF NOT EXISTS idx_sets_exercise ON workout_sets(exercise_name, set_number, created_at DESC);
