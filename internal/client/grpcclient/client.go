// Package grpcclient wraps gRPC service clients for the GophKeeper server.
package grpcclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/user/gophkeeper/internal/client/session"
	"github.com/user/gophkeeper/pkg/gen"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ErrSyncRequired indicates that the local cache is stale and must be refreshed.
var ErrSyncRequired = errors.New("version conflict, please run 'gophkeeper sync'")

// Client provides methods to interact with the GophKeeper gRPC server.
type Client struct {
	authClient    gen.AuthServiceClient
	secretsClient gen.SecretsServiceClient
	conn          *grpc.ClientConn
	session       *session.Session
	logger        *zap.Logger
}

// NewClient creates a new gRPC client connected to the given address.
func NewClient(addr string, sess *session.Session, insecureConn bool, caFile string, logger *zap.Logger) (*Client, error) {
	client := &Client{
		session: sess,
		logger:  logger,
	}

	transportCredentials, err := loadTransportCredentials(insecureConn, caFile)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(transportCredentials),
		grpc.WithUnaryInterceptor(client.authInterceptor),
	)
	if err != nil {
		return nil, err
	}

	client.conn = conn
	client.authClient = gen.NewAuthServiceClient(conn)
	client.secretsClient = gen.NewSecretsServiceClient(conn)

	return client, nil
}

func loadTransportCredentials(insecureConn bool, caFile string) (credentials.TransportCredentials, error) {
	if insecureConn {
		return grpcinsecure.NewCredentials(), nil
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if caFile != "" {
		caData, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}

		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(caData) {
			return nil, fmt.Errorf("append CA certificates")
		}
		tlsConfig.RootCAs = roots
	}

	return credentials.NewTLS(tlsConfig), nil
}

func (c *Client) authInterceptor(
	ctx context.Context,
	method string,
	req, reply any,
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	if token := c.session.AccessToken(); token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	}

	err := invoker(ctx, method, req, reply, cc, opts...)
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unauthenticated {
		return err
	}

	refreshToken := c.session.RefreshToken()
	if refreshToken == "" {
		return err
	}

	c.logger.Debug("refreshing access token after unauthenticated response", zap.String("method", method))

	refreshCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, refreshErr := c.authClient.RefreshToken(refreshCtx, &gen.RefreshTokenRequest{
		RefreshToken: refreshToken,
	})
	if refreshErr != nil {
		return err
	}

	c.session.SetTokens(response.GetAccessToken(), response.GetRefreshToken())
	c.session.SetUserID(response.GetUserId())

	retryCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var retryCancel context.CancelFunc
		retryCtx, retryCancel = context.WithTimeout(context.Background(), 30*time.Second)
		defer retryCancel()
	}

	retryCtx = metadata.AppendToOutgoingContext(retryCtx, "authorization", "Bearer "+response.GetAccessToken())
	return invoker(retryCtx, method, req, reply, cc, opts...)
}

// Close closes the underlying gRPC connection.
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// Register creates a new user account and stores the returned tokens.
func (c *Client) Register(ctx context.Context, email, password string) error {
	response, err := c.authClient.Register(ctx, &gen.RegisterRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		return err
	}

	c.session.SetTokens(response.GetAccessToken(), response.GetRefreshToken())
	c.session.SetUserID(response.GetUserId())
	return nil
}

// Login authenticates a user and stores the returned tokens.
func (c *Client) Login(ctx context.Context, email, password string) error {
	response, err := c.authClient.Login(ctx, &gen.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		return err
	}

	c.session.SetTokens(response.GetAccessToken(), response.GetRefreshToken())
	c.session.SetUserID(response.GetUserId())
	return nil
}

// RefreshToken exchanges the current refresh token for a new token pair.
func (c *Client) RefreshToken(_ context.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := c.authClient.RefreshToken(ctx, &gen.RefreshTokenRequest{
		RefreshToken: c.session.RefreshToken(),
	})
	if err != nil {
		return err
	}

	c.session.SetTokens(response.GetAccessToken(), response.GetRefreshToken())
	c.session.SetUserID(response.GetUserId())
	return nil
}

// CreateSecret stores a new encrypted secret on the server.
func (c *Client) CreateSecret(ctx context.Context, name string, kind gen.DataKind, payload []byte) (*gen.SecretItem, error) {
	response, err := c.secretsClient.CreateSecret(ctx, &gen.CreateSecretRequest{
		Name:    name,
		Kind:    kind,
		Payload: payload,
	})
	if err != nil {
		return nil, err
	}

	return response.GetSecret(), nil
}

// GetSecret retrieves a single secret by ID.
func (c *Client) GetSecret(ctx context.Context, id string) (*gen.SecretItem, error) {
	response, err := c.secretsClient.GetSecret(ctx, &gen.GetSecretRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return response.GetSecret(), nil
}

// ListSecrets returns all secrets for the authenticated user.
func (c *Client) ListSecrets(ctx context.Context) ([]*gen.SecretItem, error) {
	response, err := c.secretsClient.ListSecrets(ctx, &gen.ListSecretsRequest{})
	if err != nil {
		return nil, err
	}
	return response.GetSecrets(), nil
}

// UpdateSecret overwrites a secret with optimistic locking based on version.
func (c *Client) UpdateSecret(ctx context.Context, id, name string, kind gen.DataKind, payload []byte, version int64) (*gen.SecretItem, error) {
	response, err := c.secretsClient.UpdateSecret(ctx, &gen.UpdateSecretRequest{
		Id:      id,
		Name:    name,
		Kind:    kind,
		Payload: payload,
		Version: version,
	})
	if err != nil {
		if status.Code(err) == codes.Aborted {
			return nil, ErrSyncRequired
		}
		return nil, err
	}
	return response.GetSecret(), nil
}

// DeleteSecret soft-deletes a secret by ID with version check.
func (c *Client) DeleteSecret(ctx context.Context, id string, version int64) error {
	_, err := c.secretsClient.DeleteSecret(ctx, &gen.DeleteSecretRequest{
		Id:      id,
		Version: version,
	})
	if err != nil && status.Code(err) == codes.Aborted {
		return ErrSyncRequired
	}
	return err
}

// SyncSecrets returns secrets modified after the given timestamp along with the server time.
func (c *Client) SyncSecrets(ctx context.Context, since time.Time) ([]*gen.SecretItem, time.Time, error) {
	response, err := c.secretsClient.SyncSecrets(ctx, &gen.SyncSecretsRequest{
		Since: timestamppb.New(since),
	})
	if err != nil {
		return nil, time.Time{}, err
	}

	return response.GetSecrets(), response.GetServerTime().AsTime(), nil
}
