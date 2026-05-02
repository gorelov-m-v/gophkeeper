package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/user/gophkeeper/internal/retry"
)

// ErrSecretNotFound is returned when no secret matches the given criteria.
var ErrSecretNotFound = errors.New("secret not found")

// ErrVersionConflict is returned when an optimistic locking conflict occurs.
var ErrVersionConflict = errors.New("version conflict")

// SecretRow represents a row from the secrets table.
type SecretRow struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	Kind      int16
	Payload   []byte
	Version   int64
	Deleted   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SecretRepo provides database operations for secrets.
type SecretRepo struct {
	pool DBTX
}

// NewSecretRepo creates a new SecretRepo backed by the given connection pool.
func NewSecretRepo(pool DBTX) *SecretRepo {
	return &SecretRepo{pool: pool}
}

// Create inserts a new secret and returns the full row.
func (r *SecretRepo) Create(ctx context.Context, userID uuid.UUID, name string, kind int16, payload []byte) (*SecretRow, error) {
	var secret SecretRow
	err := retry.Do(func() error {
		return r.pool.QueryRow(ctx,
			`INSERT INTO secrets (user_id, name, kind, payload)
			 VALUES ($1, $2, $3, $4)
			 RETURNING id, user_id, name, kind, payload, version, deleted, created_at, updated_at`,
			userID, name, kind, payload,
		).Scan(&secret.ID, &secret.UserID, &secret.Name, &secret.Kind, &secret.Payload, &secret.Version, &secret.Deleted, &secret.CreatedAt, &secret.UpdatedAt)
	})
	if err != nil {
		return nil, err
	}
	return &secret, nil
}

// GetByID retrieves a single secret by ID scoped to a user.
func (r *SecretRepo) GetByID(ctx context.Context, userID, secretID uuid.UUID) (*SecretRow, error) {
	var secret SecretRow
	err := retry.Do(func() error {
		return r.pool.QueryRow(ctx,
			`SELECT id, user_id, name, kind, payload, version, deleted, created_at, updated_at
			 FROM secrets WHERE id = $1 AND user_id = $2 AND NOT deleted`,
			secretID, userID,
		).Scan(&secret.ID, &secret.UserID, &secret.Name, &secret.Kind, &secret.Payload, &secret.Version, &secret.Deleted, &secret.CreatedAt, &secret.UpdatedAt)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSecretNotFound
		}
		return nil, err
	}
	return &secret, nil
}

// List returns all non-deleted secrets for a user ordered by creation time.
func (r *SecretRepo) List(ctx context.Context, userID uuid.UUID) ([]*SecretRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, name, kind, payload, version, deleted, created_at, updated_at
		 FROM secrets WHERE user_id = $1 AND NOT deleted ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSecretRows(rows)
}

// Update modifies a secret with optimistic locking and returns the updated row.
func (r *SecretRepo) Update(ctx context.Context, userID, secretID uuid.UUID, name string, kind int16, payload []byte, expectedVersion int64) (*SecretRow, error) {
	var secret SecretRow
	err := retry.Do(func() error {
		return r.pool.QueryRow(ctx,
			`UPDATE secrets
			 SET name = $1, kind = $2, payload = $3, version = version + 1, updated_at = now()
			 WHERE id = $4 AND user_id = $5 AND version = $6 AND NOT deleted
			 RETURNING id, user_id, name, kind, payload, version, deleted, created_at, updated_at`,
			name, kind, payload, secretID, userID, expectedVersion,
		).Scan(&secret.ID, &secret.UserID, &secret.Name, &secret.Kind, &secret.Payload, &secret.Version, &secret.Deleted, &secret.CreatedAt, &secret.UpdatedAt)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVersionConflict
		}
		return nil, err
	}
	return &secret, nil
}

// SoftDelete marks a secret as deleted with optimistic locking.
func (r *SecretRepo) SoftDelete(ctx context.Context, userID, secretID uuid.UUID, expectedVersion int64) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE secrets SET deleted = TRUE, version = version + 1, updated_at = now()
		 WHERE id = $1 AND user_id = $2 AND version = $3 AND NOT deleted`,
		secretID, userID, expectedVersion,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrVersionConflict
	}
	return nil
}

// ListModifiedSince returns all secrets modified after the given timestamp, including deleted ones.
func (r *SecretRepo) ListModifiedSince(ctx context.Context, userID uuid.UUID, since time.Time) ([]*SecretRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, name, kind, payload, version, deleted, created_at, updated_at
		 FROM secrets WHERE user_id = $1 AND updated_at > $2 ORDER BY updated_at`,
		userID, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSecretRows(rows)
}

func scanSecretRows(rows pgx.Rows) ([]*SecretRow, error) {
	var secrets []*SecretRow
	for rows.Next() {
		var secret SecretRow
		if err := rows.Scan(&secret.ID, &secret.UserID, &secret.Name, &secret.Kind, &secret.Payload, &secret.Version, &secret.Deleted, &secret.CreatedAt, &secret.UpdatedAt); err != nil {
			return nil, err
		}
		secrets = append(secrets, &secret)
	}
	return secrets, rows.Err()
}
