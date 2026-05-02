CREATE TABLE secrets (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       VARCHAR(512) NOT NULL,
    kind       SMALLINT    NOT NULL,
    payload    BYTEA       NOT NULL,
    version    BIGINT      NOT NULL DEFAULT 1,
    deleted    BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_secrets_user_id ON secrets(user_id);
CREATE INDEX idx_secrets_updated_at ON secrets(updated_at);
CREATE UNIQUE INDEX idx_secrets_user_name ON secrets(user_id, name) WHERE NOT deleted;
