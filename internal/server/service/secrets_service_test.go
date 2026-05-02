package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/gophkeeper/internal/server/repository"
)

var secretCols = []string{"id", "user_id", "name", "kind", "payload", "version", "deleted", "created_at", "updated_at"}

func newTestSecretsService(t *testing.T) (*SecretsService, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	secretRepo := repository.NewSecretRepo(mock)
	return NewSecretsService(secretRepo), mock
}

func TestCreateSecret(t *testing.T) {
	svc, mock := newTestSecretsService(t)
	defer mock.Close()

	userID := uuid.New()
	secretID := uuid.New()
	now := time.Now()

	mock.ExpectQuery("INSERT INTO secrets").
		WithArgs(userID, "my-secret", int16(1), []byte("payload")).
		WillReturnRows(pgxmock.NewRows(secretCols).
			AddRow(secretID, userID, "my-secret", int16(1), []byte("payload"), int64(1), false, now, now))

	s, err := svc.Create(context.Background(), userID, "my-secret", 1, []byte("payload"))
	require.NoError(t, err)
	assert.Equal(t, secretID, s.ID)
	assert.Equal(t, "my-secret", s.Name)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetSecret(t *testing.T) {
	svc, mock := newTestSecretsService(t)
	defer mock.Close()

	userID := uuid.New()
	secretID := uuid.New()
	now := time.Now()

	mock.ExpectQuery("SELECT id, user_id, name, kind, payload, version, deleted, created_at, updated_at").
		WithArgs(secretID, userID).
		WillReturnRows(pgxmock.NewRows(secretCols).
			AddRow(secretID, userID, "test", int16(1), []byte("data"), int64(1), false, now, now))

	s, err := svc.GetByID(context.Background(), userID, secretID)
	require.NoError(t, err)
	assert.Equal(t, secretID, s.ID)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListSecrets(t *testing.T) {
	svc, mock := newTestSecretsService(t)
	defer mock.Close()

	userID := uuid.New()
	now := time.Now()

	mock.ExpectQuery("SELECT id, user_id, name, kind, payload, version, deleted, created_at, updated_at").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows(secretCols).
			AddRow(uuid.New(), userID, "s1", int16(1), []byte("p1"), int64(1), false, now, now).
			AddRow(uuid.New(), userID, "s2", int16(2), []byte("p2"), int64(1), false, now, now))

	secrets, err := svc.List(context.Background(), userID)
	require.NoError(t, err)
	assert.Len(t, secrets, 2)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateSecret(t *testing.T) {
	svc, mock := newTestSecretsService(t)
	defer mock.Close()

	userID := uuid.New()
	secretID := uuid.New()
	now := time.Now()

	mock.ExpectQuery("UPDATE secrets").
		WithArgs("updated", int16(1), []byte("new"), secretID, userID, int64(1)).
		WillReturnRows(pgxmock.NewRows(secretCols).
			AddRow(secretID, userID, "updated", int16(1), []byte("new"), int64(2), false, now, now))

	s, err := svc.Update(context.Background(), userID, secretID, "updated", 1, []byte("new"), 1)
	require.NoError(t, err)
	assert.Equal(t, int64(2), s.Version)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteSecret(t *testing.T) {
	svc, mock := newTestSecretsService(t)
	defer mock.Close()

	userID := uuid.New()
	secretID := uuid.New()

	mock.ExpectExec("UPDATE secrets SET deleted").
		WithArgs(secretID, userID, int64(1)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := svc.SoftDelete(context.Background(), userID, secretID, 1)
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSyncSecrets(t *testing.T) {
	svc, mock := newTestSecretsService(t)
	defer mock.Close()

	userID := uuid.New()
	since := time.Now().Add(-time.Hour)
	now := time.Now()

	mock.ExpectQuery("SELECT id, user_id, name, kind, payload, version, deleted, created_at, updated_at").
		WithArgs(userID, since).
		WillReturnRows(pgxmock.NewRows(secretCols).
			AddRow(uuid.New(), userID, "synced", int16(1), []byte("p"), int64(3), false, now, now))

	secrets, err := svc.ListModifiedSince(context.Background(), userID, since)
	require.NoError(t, err)
	assert.Len(t, secrets, 1)

	require.NoError(t, mock.ExpectationsWereMet())
}
