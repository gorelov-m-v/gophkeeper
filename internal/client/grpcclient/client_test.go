package grpcclient

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/gophkeeper/internal/client/session"
	"github.com/user/gophkeeper/pkg/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const bufSize = 1024 * 1024

type fakeAuthServer struct {
	gen.UnimplementedAuthServiceServer
}

func (f *fakeAuthServer) Register(_ context.Context, req *gen.RegisterRequest) (*gen.RegisterResponse, error) {
	return &gen.RegisterResponse{
		UserId:       fakeSecretID,
		AccessToken:  "access-token-for-" + req.GetEmail(),
		RefreshToken: "refresh-token-for-" + req.GetEmail(),
	}, nil
}

func (f *fakeAuthServer) Login(_ context.Context, req *gen.LoginRequest) (*gen.LoginResponse, error) {
	return &gen.LoginResponse{
		UserId:       fakeSecretID,
		AccessToken:  "access-token-for-" + req.GetEmail(),
		RefreshToken: "refresh-token-for-" + req.GetEmail(),
	}, nil
}

func (f *fakeAuthServer) RefreshToken(_ context.Context, _ *gen.RefreshTokenRequest) (*gen.RefreshTokenResponse, error) {
	return &gen.RefreshTokenResponse{
		UserId:       fakeSecretID,
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
	}, nil
}

type fakeSecretsServer struct {
	gen.UnimplementedSecretsServiceServer
}

var fakeSecretID = uuid.New().String()

func (f *fakeSecretsServer) CreateSecret(_ context.Context, req *gen.CreateSecretRequest) (*gen.CreateSecretResponse, error) {
	return &gen.CreateSecretResponse{
		Secret: &gen.SecretItem{
			Id:        fakeSecretID,
			Name:      req.GetName(),
			Kind:      req.GetKind(),
			Payload:   req.GetPayload(),
			Version:   1,
			CreatedAt: timestamppb.Now(),
			UpdatedAt: timestamppb.Now(),
		},
	}, nil
}

func (f *fakeSecretsServer) GetSecret(_ context.Context, req *gen.GetSecretRequest) (*gen.GetSecretResponse, error) {
	return &gen.GetSecretResponse{
		Secret: &gen.SecretItem{
			Id:        req.GetId(),
			Name:      "test-secret",
			Kind:      gen.DataKind(1),
			Payload:   []byte("payload"),
			Version:   1,
			CreatedAt: timestamppb.Now(),
			UpdatedAt: timestamppb.Now(),
		},
	}, nil
}

func (f *fakeSecretsServer) ListSecrets(_ context.Context, _ *gen.ListSecretsRequest) (*gen.ListSecretsResponse, error) {
	return &gen.ListSecretsResponse{
		Secrets: []*gen.SecretItem{
			{
				Id:        fakeSecretID,
				Name:      "secret-1",
				Kind:      gen.DataKind(1),
				Payload:   []byte("p1"),
				Version:   1,
				CreatedAt: timestamppb.Now(),
				UpdatedAt: timestamppb.Now(),
			},
		},
	}, nil
}

func (f *fakeSecretsServer) UpdateSecret(_ context.Context, req *gen.UpdateSecretRequest) (*gen.UpdateSecretResponse, error) {
	return &gen.UpdateSecretResponse{
		Secret: &gen.SecretItem{
			Id:        req.GetId(),
			Name:      req.GetName(),
			Kind:      req.GetKind(),
			Payload:   req.GetPayload(),
			Version:   req.GetVersion() + 1,
			CreatedAt: timestamppb.Now(),
			UpdatedAt: timestamppb.Now(),
		},
	}, nil
}

func (f *fakeSecretsServer) DeleteSecret(_ context.Context, _ *gen.DeleteSecretRequest) (*gen.DeleteSecretResponse, error) {
	return &gen.DeleteSecretResponse{}, nil
}

func (f *fakeSecretsServer) SyncSecrets(_ context.Context, _ *gen.SyncSecretsRequest) (*gen.SyncSecretsResponse, error) {
	return &gen.SyncSecretsResponse{
		Secrets: []*gen.SecretItem{
			{
				Id:        fakeSecretID,
				Name:      "synced-secret",
				Kind:      gen.DataKind(1),
				Payload:   []byte("synced"),
				Version:   2,
				CreatedAt: timestamppb.Now(),
				UpdatedAt: timestamppb.Now(),
			},
		},
		ServerTime: timestamppb.Now(),
	}, nil
}

type fakeAuthClient struct {
	refreshFn func(ctx context.Context, in *gen.RefreshTokenRequest, opts ...grpc.CallOption) (*gen.RefreshTokenResponse, error)
}

func (f fakeAuthClient) Register(context.Context, *gen.RegisterRequest, ...grpc.CallOption) (*gen.RegisterResponse, error) {
	return nil, errors.New("not implemented")
}

func (f fakeAuthClient) Login(context.Context, *gen.LoginRequest, ...grpc.CallOption) (*gen.LoginResponse, error) {
	return nil, errors.New("not implemented")
}

func (f fakeAuthClient) RefreshToken(ctx context.Context, in *gen.RefreshTokenRequest, opts ...grpc.CallOption) (*gen.RefreshTokenResponse, error) {
	return f.refreshFn(ctx, in, opts...)
}

type fakeSecretsClient struct {
	updateFn func(ctx context.Context, in *gen.UpdateSecretRequest, opts ...grpc.CallOption) (*gen.UpdateSecretResponse, error)
	deleteFn func(ctx context.Context, in *gen.DeleteSecretRequest, opts ...grpc.CallOption) (*gen.DeleteSecretResponse, error)
}

func (f fakeSecretsClient) CreateSecret(context.Context, *gen.CreateSecretRequest, ...grpc.CallOption) (*gen.CreateSecretResponse, error) {
	return nil, errors.New("not implemented")
}

func (f fakeSecretsClient) GetSecret(context.Context, *gen.GetSecretRequest, ...grpc.CallOption) (*gen.GetSecretResponse, error) {
	return nil, errors.New("not implemented")
}

func (f fakeSecretsClient) ListSecrets(context.Context, *gen.ListSecretsRequest, ...grpc.CallOption) (*gen.ListSecretsResponse, error) {
	return nil, errors.New("not implemented")
}

func (f fakeSecretsClient) UpdateSecret(ctx context.Context, in *gen.UpdateSecretRequest, opts ...grpc.CallOption) (*gen.UpdateSecretResponse, error) {
	return f.updateFn(ctx, in, opts...)
}

func (f fakeSecretsClient) DeleteSecret(ctx context.Context, in *gen.DeleteSecretRequest, opts ...grpc.CallOption) (*gen.DeleteSecretResponse, error) {
	return f.deleteFn(ctx, in, opts...)
}

func (f fakeSecretsClient) SyncSecrets(context.Context, *gen.SyncSecretsRequest, ...grpc.CallOption) (*gen.SyncSecretsResponse, error) {
	return nil, errors.New("not implemented")
}

func startFakeServer(t *testing.T) *bufconn.Listener {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	gen.RegisterAuthServiceServer(srv, &fakeAuthServer{})
	gen.RegisterSecretsServiceServer(srv, &fakeSecretsServer{})

	go func() {
		if err := srv.Serve(lis); err != nil {
			log.Printf("fake server exited: %v", err)
		}
	}()

	t.Cleanup(func() {
		srv.GracefulStop()
	})

	return lis
}

func newTestClient(t *testing.T, lis *bufconn.Listener) *Client {
	t.Helper()
	sess := session.NewSession()

	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.DialContext(ctx)
	}

	c := &Client{session: sess, logger: zap.NewNop()}
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(c.authInterceptor),
	)
	require.NoError(t, err)

	c.conn = conn
	c.authClient = gen.NewAuthServiceClient(conn)
	c.secretsClient = gen.NewSecretsServiceClient(conn)

	t.Cleanup(func() {
		conn.Close()
	})

	return c
}

func TestRegister(t *testing.T) {
	lis := startFakeServer(t)
	c := newTestClient(t, lis)

	err := c.Register(context.Background(), "user@example.com", "password")
	require.NoError(t, err)
	assert.Equal(t, "access-token-for-user@example.com", c.session.AccessToken())
	assert.Equal(t, "refresh-token-for-user@example.com", c.session.RefreshToken())
}

func TestLoadTransportCredentials(t *testing.T) {
	creds, err := loadTransportCredentials(true, "")
	require.NoError(t, err)
	assert.Equal(t, "insecure", creds.Info().SecurityProtocol)

	creds, err = loadTransportCredentials(false, "")
	require.NoError(t, err)
	assert.Equal(t, "tls", creds.Info().SecurityProtocol)

	_, err = loadTransportCredentials(false, "missing-ca.pem")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read CA file")

	caPath := t.TempDir() + "/bad-ca.pem"
	require.NoError(t, os.WriteFile(caPath, []byte("not a pem cert"), 0600))
	_, err = loadTransportCredentials(false, caPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "append CA certificates")
}

func TestNewClientInsecure(t *testing.T) {
	sess := session.NewSession()
	c, err := NewClient("passthrough:///localhost:1", sess, true, "", zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, c.authClient)
	require.NotNil(t, c.secretsClient)
	require.NoError(t, c.Close())
}

func TestLogin(t *testing.T) {
	lis := startFakeServer(t)
	c := newTestClient(t, lis)

	err := c.Login(context.Background(), "user@example.com", "password")
	require.NoError(t, err)
	assert.Equal(t, "access-token-for-user@example.com", c.session.AccessToken())
	assert.Equal(t, "refresh-token-for-user@example.com", c.session.RefreshToken())
}

func TestCreateSecret(t *testing.T) {
	lis := startFakeServer(t)
	c := newTestClient(t, lis)

	secret, err := c.CreateSecret(context.Background(), "my-secret", gen.DataKind(1), []byte("payload"))
	require.NoError(t, err)
	assert.Equal(t, fakeSecretID, secret.GetId())
	assert.Equal(t, "my-secret", secret.GetName())
	assert.Equal(t, int64(1), secret.GetVersion())
}

func TestGetSecret(t *testing.T) {
	lis := startFakeServer(t)
	c := newTestClient(t, lis)

	secretID := uuid.New().String()
	secret, err := c.GetSecret(context.Background(), secretID)
	require.NoError(t, err)
	assert.Equal(t, secretID, secret.GetId())
	assert.Equal(t, "test-secret", secret.GetName())
}

func TestListSecrets(t *testing.T) {
	lis := startFakeServer(t)
	c := newTestClient(t, lis)

	secrets, err := c.ListSecrets(context.Background())
	require.NoError(t, err)
	assert.Len(t, secrets, 1)
	assert.Equal(t, "secret-1", secrets[0].GetName())
}

func TestRefreshToken(t *testing.T) {
	lis := startFakeServer(t)
	c := newTestClient(t, lis)

	c.session.SetTokens("old-access", "old-refresh")

	err := c.RefreshToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "new-access-token", c.session.AccessToken())
	assert.Equal(t, "new-refresh-token", c.session.RefreshToken())
}

func TestUpdateSecret(t *testing.T) {
	lis := startFakeServer(t)
	c := newTestClient(t, lis)

	secretID := uuid.New().String()
	secret, err := c.UpdateSecret(context.Background(), secretID, "updated", gen.DataKind(1), []byte("new-data"), 1)
	require.NoError(t, err)
	assert.Equal(t, secretID, secret.GetId())
	assert.Equal(t, "updated", secret.GetName())
	assert.Equal(t, int64(2), secret.GetVersion())
}

func TestDeleteSecret(t *testing.T) {
	lis := startFakeServer(t)
	c := newTestClient(t, lis)

	err := c.DeleteSecret(context.Background(), uuid.New().String(), 1)
	require.NoError(t, err)
}

func TestSyncSecrets(t *testing.T) {
	lis := startFakeServer(t)
	c := newTestClient(t, lis)

	secrets, serverTime, err := c.SyncSecrets(context.Background(), time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Len(t, secrets, 1)
	assert.Equal(t, "synced-secret", secrets[0].GetName())
	assert.False(t, serverTime.IsZero())
}

func TestClose(t *testing.T) {
	lis := startFakeServer(t)
	c := newTestClient(t, lis)

	err := c.Close()
	assert.NoError(t, err)
}

func TestCloseNilConnection(t *testing.T) {
	c := &Client{}
	assert.NoError(t, c.Close())
}

func TestAuthInterceptorRefreshSuccess(t *testing.T) {
	sess := session.NewSession()
	sess.SetTokens("old-access", "old-refresh")
	c := &Client{
		session: sess,
		logger:  zap.NewNop(),
		authClient: fakeAuthClient{
			refreshFn: func(_ context.Context, in *gen.RefreshTokenRequest, _ ...grpc.CallOption) (*gen.RefreshTokenResponse, error) {
				assert.Equal(t, "old-refresh", in.GetRefreshToken())
				return &gen.RefreshTokenResponse{
					AccessToken:  "new-access",
					RefreshToken: "new-refresh",
					UserId:       "user-1",
				}, nil
			},
		},
	}

	calls := 0
	err := c.authInterceptor(context.Background(), "/service/method", nil, nil, nil, func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		calls++
		if calls == 1 {
			return status.Error(codes.Unauthenticated, "expired")
		}
		values := ctx.Value("authorization")
		_ = values
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 2, calls)
	assert.Equal(t, "new-access", sess.AccessToken())
	assert.Equal(t, "new-refresh", sess.RefreshToken())
	assert.Equal(t, "user-1", sess.UserID())
}

func TestAuthInterceptorRefreshFailureReturnsOriginalError(t *testing.T) {
	sess := session.NewSession()
	sess.SetTokens("old-access", "old-refresh")
	c := &Client{
		session: sess,
		logger:  zap.NewNop(),
		authClient: fakeAuthClient{
			refreshFn: func(context.Context, *gen.RefreshTokenRequest, ...grpc.CallOption) (*gen.RefreshTokenResponse, error) {
				return nil, status.Error(codes.Unauthenticated, "bad refresh")
			},
		},
	}

	err := c.authInterceptor(context.Background(), "/service/method", nil, nil, nil, func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		return status.Error(codes.Unauthenticated, "expired")
	})

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.True(t, strings.Contains(err.Error(), "expired"))
}

func TestAuthInterceptorWithoutRefreshTokenReturnsOriginalError(t *testing.T) {
	c := &Client{session: session.NewSession(), logger: zap.NewNop()}

	err := c.authInterceptor(context.Background(), "/service/method", nil, nil, nil, func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		return status.Error(codes.Unauthenticated, "missing")
	})

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestUpdateAndDeleteVersionConflictMapping(t *testing.T) {
	c := &Client{
		session: session.NewSession(),
		logger:  zap.NewNop(),
		secretsClient: fakeSecretsClient{
			updateFn: func(context.Context, *gen.UpdateSecretRequest, ...grpc.CallOption) (*gen.UpdateSecretResponse, error) {
				return nil, status.Error(codes.Aborted, "version conflict")
			},
			deleteFn: func(context.Context, *gen.DeleteSecretRequest, ...grpc.CallOption) (*gen.DeleteSecretResponse, error) {
				return nil, status.Error(codes.Aborted, "version conflict")
			},
		},
	}

	_, err := c.UpdateSecret(context.Background(), "id", "name", gen.DataKind_DATA_KIND_TEXT, []byte("payload"), 1)
	assert.ErrorIs(t, err, ErrSyncRequired)
	assert.ErrorIs(t, c.DeleteSecret(context.Background(), "id", 1), ErrSyncRequired)
}
