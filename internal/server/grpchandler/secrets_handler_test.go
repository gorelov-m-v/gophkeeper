package grpchandler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/gophkeeper/internal/server/interceptor"
	"github.com/user/gophkeeper/internal/server/repository"
	"github.com/user/gophkeeper/internal/server/service"
	"github.com/user/gophkeeper/pkg/gen"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func newSecretsHandlerWithMock(t *testing.T) (*SecretsHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)

	secretRepo := repository.NewSecretRepo(mock)
	secretsSvc := service.NewSecretsService(secretRepo)
	handler := NewSecretsHandler(secretsSvc)

	return handler, mock
}

var secretColumns = []string{"id", "user_id", "name", "kind", "payload", "version", "deleted", "created_at", "updated_at"}

func TestCreateSecret_Success(t *testing.T) {
	handler, mock := newSecretsHandlerWithMock(t)
	defer mock.Close()

	userID := uuid.New()
	secretID := uuid.New()
	now := time.Now()
	ctx := interceptor.ContextWithUserID(context.Background(), userID)

	mock.ExpectQuery("INSERT INTO secrets").
		WithArgs(userID, "my-secret", int16(1), []byte("encrypted")).
		WillReturnRows(pgxmock.NewRows(secretColumns).
			AddRow(secretID, userID, "my-secret", int16(1), []byte("encrypted"), int64(1), false, now, now))

	resp, err := handler.CreateSecret(ctx, &gen.CreateSecretRequest{
		Name:    "my-secret",
		Kind:    1,
		Payload: []byte("encrypted"),
	})

	require.NoError(t, err)
	assert.Equal(t, secretID.String(), resp.GetSecret().GetId())
	assert.Equal(t, "my-secret", resp.GetSecret().GetName())
	assert.Equal(t, int64(1), resp.GetSecret().GetVersion())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetSecret_Success(t *testing.T) {
	handler, mock := newSecretsHandlerWithMock(t)
	defer mock.Close()

	userID := uuid.New()
	secretID := uuid.New()
	now := time.Now()
	ctx := interceptor.ContextWithUserID(context.Background(), userID)

	mock.ExpectQuery("SELECT id, user_id, name, kind, payload, version, deleted, created_at, updated_at").
		WithArgs(secretID, userID).
		WillReturnRows(pgxmock.NewRows(secretColumns).
			AddRow(secretID, userID, "my-secret", int16(1), []byte("data"), int64(1), false, now, now))

	resp, err := handler.GetSecret(ctx, &gen.GetSecretRequest{
		Id: secretID.String(),
	})

	require.NoError(t, err)
	assert.Equal(t, secretID.String(), resp.GetSecret().GetId())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetSecret_NotFound(t *testing.T) {
	handler, mock := newSecretsHandlerWithMock(t)
	defer mock.Close()

	userID := uuid.New()
	secretID := uuid.New()
	ctx := interceptor.ContextWithUserID(context.Background(), userID)

	mock.ExpectQuery("SELECT id, user_id, name, kind, payload, version, deleted, created_at, updated_at").
		WithArgs(secretID, userID).
		WillReturnRows(pgxmock.NewRows(secretColumns))

	_, err := handler.GetSecret(ctx, &gen.GetSecretRequest{
		Id: secretID.String(),
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListSecrets_Success(t *testing.T) {
	handler, mock := newSecretsHandlerWithMock(t)
	defer mock.Close()

	userID := uuid.New()
	now := time.Now()
	ctx := interceptor.ContextWithUserID(context.Background(), userID)

	id1 := uuid.New()
	id2 := uuid.New()

	mock.ExpectQuery("SELECT id, user_id, name, kind, payload, version, deleted, created_at, updated_at").
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows(secretColumns).
			AddRow(id1, userID, "secret-1", int16(1), []byte("d1"), int64(1), false, now, now).
			AddRow(id2, userID, "secret-2", int16(2), []byte("d2"), int64(1), false, now, now))

	resp, err := handler.ListSecrets(ctx, &gen.ListSecretsRequest{})

	require.NoError(t, err)
	assert.Len(t, resp.GetSecrets(), 2)
	assert.Equal(t, "secret-1", resp.GetSecrets()[0].GetName())
	assert.Equal(t, "secret-2", resp.GetSecrets()[1].GetName())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateSecret_Success(t *testing.T) {
	handler, mock := newSecretsHandlerWithMock(t)
	defer mock.Close()

	userID := uuid.New()
	secretID := uuid.New()
	now := time.Now()
	ctx := interceptor.ContextWithUserID(context.Background(), userID)

	mock.ExpectQuery("UPDATE secrets").
		WithArgs("updated-name", int16(1), []byte("new-payload"), secretID, userID, int64(1)).
		WillReturnRows(pgxmock.NewRows(secretColumns).
			AddRow(secretID, userID, "updated-name", int16(1), []byte("new-payload"), int64(2), false, now, now))

	resp, err := handler.UpdateSecret(ctx, &gen.UpdateSecretRequest{
		Id:      secretID.String(),
		Name:    "updated-name",
		Kind:    1,
		Payload: []byte("new-payload"),
		Version: 1,
	})

	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.GetSecret().GetVersion())
	assert.Equal(t, "updated-name", resp.GetSecret().GetName())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateSecret_VersionConflict(t *testing.T) {
	handler, mock := newSecretsHandlerWithMock(t)
	defer mock.Close()

	userID := uuid.New()
	secretID := uuid.New()
	ctx := interceptor.ContextWithUserID(context.Background(), userID)

	mock.ExpectQuery("UPDATE secrets").
		WithArgs("name", int16(1), []byte("data"), secretID, userID, int64(1)).
		WillReturnRows(pgxmock.NewRows(secretColumns))

	_, err := handler.UpdateSecret(ctx, &gen.UpdateSecretRequest{
		Id:      secretID.String(),
		Name:    "name",
		Kind:    1,
		Payload: []byte("data"),
		Version: 1,
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Aborted, st.Code())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteSecret_Success(t *testing.T) {
	handler, mock := newSecretsHandlerWithMock(t)
	defer mock.Close()

	userID := uuid.New()
	secretID := uuid.New()
	ctx := interceptor.ContextWithUserID(context.Background(), userID)

	mock.ExpectExec("UPDATE secrets SET deleted").
		WithArgs(secretID, userID, int64(1)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	_, err := handler.DeleteSecret(ctx, &gen.DeleteSecretRequest{
		Id:      secretID.String(),
		Version: 1,
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSyncSecrets_Success(t *testing.T) {
	handler, mock := newSecretsHandlerWithMock(t)
	defer mock.Close()

	userID := uuid.New()
	now := time.Now()
	since := now.Add(-1 * time.Hour)
	ctx := interceptor.ContextWithUserID(context.Background(), userID)

	id1 := uuid.New()

	mock.ExpectQuery("SELECT id, user_id, name, kind, payload, version, deleted, created_at, updated_at").
		WithArgs(userID, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(secretColumns).
			AddRow(id1, userID, "synced", int16(1), []byte("d"), int64(2), false, now, now))

	resp, err := handler.SyncSecrets(ctx, &gen.SyncSecretsRequest{
		Since: timestamppb.New(since),
	})

	require.NoError(t, err)
	assert.Len(t, resp.GetSecrets(), 1)
	assert.NotNil(t, resp.GetServerTime())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetSecret_InvalidUUID(t *testing.T) {
	handler, mock := newSecretsHandlerWithMock(t)
	defer mock.Close()

	userID := uuid.New()
	ctx := interceptor.ContextWithUserID(context.Background(), userID)

	_, err := handler.GetSecret(ctx, &gen.GetSecretRequest{
		Id: "not-a-uuid",
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestUpdateSecret_InvalidUUID(t *testing.T) {
	handler, mock := newSecretsHandlerWithMock(t)
	defer mock.Close()

	userID := uuid.New()
	ctx := interceptor.ContextWithUserID(context.Background(), userID)

	_, err := handler.UpdateSecret(ctx, &gen.UpdateSecretRequest{
		Id:   "not-a-uuid",
		Name: "x",
		Kind: 1,
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestDeleteSecret_InvalidUUID(t *testing.T) {
	handler, mock := newSecretsHandlerWithMock(t)
	defer mock.Close()

	userID := uuid.New()
	ctx := interceptor.ContextWithUserID(context.Background(), userID)

	_, err := handler.DeleteSecret(ctx, &gen.DeleteSecretRequest{
		Id:      "not-a-uuid",
		Version: 1,
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestListSecrets_Error(t *testing.T) {
	handler, mock := newSecretsHandlerWithMock(t)
	defer mock.Close()

	userID := uuid.New()
	ctx := interceptor.ContextWithUserID(context.Background(), userID)

	mock.ExpectQuery("SELECT id, user_id, name, kind, payload, version, deleted, created_at, updated_at").
		WithArgs(userID).
		WillReturnError(fmt.Errorf("db connection lost"))

	_, err := handler.ListSecrets(ctx, &gen.ListSecretsRequest{})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSyncSecrets_Error(t *testing.T) {
	handler, mock := newSecretsHandlerWithMock(t)
	defer mock.Close()

	userID := uuid.New()
	ctx := interceptor.ContextWithUserID(context.Background(), userID)
	since := time.Now().Add(-1 * time.Hour)

	mock.ExpectQuery("SELECT id, user_id, name, kind, payload, version, deleted, created_at, updated_at").
		WithArgs(userID, pgxmock.AnyArg()).
		WillReturnError(fmt.Errorf("db connection lost"))

	_, err := handler.SyncSecrets(ctx, &gen.SyncSecretsRequest{
		Since: timestamppb.New(since),
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateSecret_NoAuth(t *testing.T) {
	handler, mock := newSecretsHandlerWithMock(t)
	defer mock.Close()

	_, err := handler.CreateSecret(context.Background(), &gen.CreateSecretRequest{
		Name:    "my-secret",
		Kind:    1,
		Payload: []byte("encrypted"),
	})

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}
