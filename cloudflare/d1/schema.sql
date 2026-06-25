-- Cloudflare D1 (SQLite) スキーマ
-- PostgreSQL 版 (k8s/postgres/init-configmap.yaml) の SQLite 移植
-- ID: TEXT (crypto.randomUUID() で Worker 内生成)
-- timestamp: TEXT (ISO 8601: "2024-01-15T09:00:00.000Z")
-- boolean: INTEGER (0/1)
-- decimal: REAL

-- ===== auth テーブル =====
CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,          -- PBKDF2:salt:hash 形式
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- ===== workout テーブル =====
CREATE TABLE IF NOT EXISTS workout_sessions (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL,
    muscle_group TEXT NOT NULL,
    date         TEXT NOT NULL DEFAULT (date('now')),
    created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_sessions_user_date   ON workout_sessions(user_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_user_muscle ON workout_sessions(user_id, muscle_group);

CREATE TABLE IF NOT EXISTS workout_sets (
    id            TEXT    PRIMARY KEY,
    session_id    TEXT    NOT NULL REFERENCES workout_sessions(id) ON DELETE CASCADE,
    exercise_name TEXT    NOT NULL,
    equipment     TEXT    NOT NULL DEFAULT '',
    set_number    INTEGER NOT NULL,
    weight        REAL    NOT NULL DEFAULT 0,
    reps          INTEGER NOT NULL,
    rir           INTEGER,                -- Reps in Reserve (nullable)
    created_at    TEXT    NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_sets_session  ON workout_sets(session_id);
CREATE INDEX IF NOT EXISTS idx_sets_exercise ON workout_sets(exercise_name, set_number, created_at DESC);

-- ===== user テーブル =====
CREATE TABLE IF NOT EXISTS user_profiles (
    user_id    TEXT PRIMARY KEY,
    height     REAL,
    unit       TEXT NOT NULL DEFAULT 'kg',
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS body_weight_records (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL,
    weight      REAL NOT NULL,
    recorded_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_weight_user_date ON body_weight_records(user_id, recorded_at ASC);

-- ===== menu テーブル =====
CREATE TABLE IF NOT EXISTS exercises (
    id           TEXT    PRIMARY KEY,
    name         TEXT    NOT NULL,
    muscle_group TEXT    NOT NULL,
    equipment    TEXT    NOT NULL DEFAULT '',
    is_default   INTEGER NOT NULL DEFAULT 0,  -- 0=false, 1=true
    UNIQUE(name, muscle_group, equipment)
);
CREATE INDEX IF NOT EXISTS idx_exercises_muscle ON exercises(muscle_group);

CREATE TABLE IF NOT EXISTS user_exercises (
    id          TEXT    PRIMARY KEY,
    user_id     TEXT    NOT NULL,
    exercise_id TEXT    NOT NULL REFERENCES exercises(id),
    sort_order  INTEGER NOT NULL DEFAULT 0,
    UNIQUE(user_id, exercise_id)
);
CREATE INDEX IF NOT EXISTS idx_user_exercises_user ON user_exercises(user_id, sort_order);
