package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var secretCols = []string{"id", "user_id", "name", "kind", "payload", "version", "deleted", "created_at", "updated_at"}

func TestCreateSecret(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewSecretRepo(mock)
	userID := uuid.New()
	secretID := uuid.New()
	now := time.Now()

	mock.ExpectQuery("INSERT INTO secrets").
		WithArgs(userID, "my-secret", int16(1), []byte("payload")).
		WillReturnRows(pgxmock.NewRows(secretCols).
			AddRow(secretID, userID, "my-secret", int16(1), []byte("payload"), int64(1), false, now, now))

	s, err := repo.Create(context.Background(), userID, "my-secret", 1, []byte("payload"))
	require.NoError(t, err)
	assert.Equal(t, secretID, s.ID)
	assert.Equal(t, "my-secret", s.Name)
	assert.Equal(t, int64(1), s.Version)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetByID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewSecretRepo(mock)
	userID := uuid.New()
	secretID := uuid.New()
	now := time.Now()

	mock.ExpectQuery("SELECT id, user_id, name, kind, payload, version, deleted, created_at, updated_at").
		WithArgs(secretID, userID).
		WillReturnRows(pgxmock.NewRows(secretCols).
			AddRow(secretID, userID, "test", int16(1), []byte("data"), int64(1), false, now, now))

	s, err := repo.GetByID(context.Background(), userID, secretID)
	require.NoError(t, err)
	assert.Equal(t, secretID, s.ID)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetByIDNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewSecretRepo(mock)
	userID := uuid.New()
	secretID := uuid.New()

	mock.ExpectQuery("SELECT id, user_id, name, kind, payload, version, deleted, created_at, updated_at").
		WithArgs(secretID, userID).
		WillReturnRows(pgxmock.NewRows(secretCols))

	_, err = repo.GetByID(context.Background(), userID, secretID)
	assert.ErrorIs(t, err, ErrSecretNotFound)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestList(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewSecretRepo(mock)
	userID := uuid.New()
	now := time.Now()

	mock.ExpectQuery("SELECT id, user_id, name, kind, payload, version, deleted, created_at, updated_at").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows(secretCols).
			AddRow(uuid.New(), userID, "secret1", int16(1), []byte("p1"), int64(1), false, now, now).
			AddRow(uuid.New(), userID, "secret2", int16(2), []byte("p2"), int64(1), false, now, now))

	secrets, err := repo.List(context.Background(), userID)
	require.NoError(t, err)
	assert.Len(t, secrets, 2)
	assert.Equal(t, "secret1", secrets[0].Name)
	assert.Equal(t, "secret2", secrets[1].Name)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewSecretRepo(mock)
	userID := uuid.New()
	secretID := uuid.New()
	now := time.Now()

	mock.ExpectQuery("UPDATE secrets").
		WithArgs("updated", int16(1), []byte("new-data"), secretID, userID, int64(1)).
		WillReturnRows(pgxmock.NewRows(secretCols).
			AddRow(secretID, userID, "updated", int16(1), []byte("new-data"), int64(2), false, now, now))

	s, err := repo.Update(context.Background(), userID, secretID, "updated", 1, []byte("new-data"), 1)
	require.NoError(t, err)
	assert.Equal(t, "updated", s.Name)
	assert.Equal(t, int64(2), s.Version)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateVersionConflict(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewSecretRepo(mock)
	userID := uuid.New()
	secretID := uuid.New()

	mock.ExpectQuery("UPDATE secrets").
		WithArgs("updated", int16(1), []byte("data"), secretID, userID, int64(1)).
		WillReturnRows(pgxmock.NewRows(secretCols))

	_, err = repo.Update(context.Background(), userID, secretID, "updated", 1, []byte("data"), 1)
	assert.ErrorIs(t, err, ErrVersionConflict)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSoftDelete(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewSecretRepo(mock)
	userID := uuid.New()
	secretID := uuid.New()

	mock.ExpectExec("UPDATE secrets SET deleted").
		WithArgs(secretID, userID, int64(1)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = repo.SoftDelete(context.Background(), userID, secretID, 1)
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSoftDeleteVersionConflict(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewSecretRepo(mock)
	userID := uuid.New()
	secretID := uuid.New()

	mock.ExpectExec("UPDATE secrets SET deleted").
		WithArgs(secretID, userID, int64(99)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err = repo.SoftDelete(context.Background(), userID, secretID, 99)
	assert.ErrorIs(t, err, ErrVersionConflict)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListModifiedSince(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewSecretRepo(mock)
	userID := uuid.New()
	since := time.Now().Add(-time.Hour)
	now := time.Now()

	mock.ExpectQuery("SELECT id, user_id, name, kind, payload, version, deleted, created_at, updated_at").
		WithArgs(userID, since).
		WillReturnRows(pgxmock.NewRows(secretCols).
			AddRow(uuid.New(), userID, "modified", int16(1), []byte("p"), int64(2), false, now, now))

	secrets, err := repo.ListModifiedSince(context.Background(), userID, since)
	require.NoError(t, err)
	assert.Len(t, secrets, 1)
	assert.Equal(t, "modified", secrets[0].Name)

	require.NoError(t, mock.ExpectationsWereMet())
}
