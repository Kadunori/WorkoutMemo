CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS user_profiles (
    user_id    UUID         PRIMARY KEY,
    height     NUMERIC(5,1),
    unit       VARCHAR(5)   NOT NULL DEFAULT 'kg',
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS body_weight_records (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID         NOT NULL,
    weight      NUMERIC(5,1) NOT NULL,
    recorded_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_weight_user_date ON body_weight_records(user_id, recorded_at ASC);
