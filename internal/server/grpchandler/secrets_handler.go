package grpchandler

import (
	"context"
	"errors"

	"strings"

	"github.com/google/uuid"
	"github.com/user/gophkeeper/internal/server/interceptor"
	"github.com/user/gophkeeper/internal/server/repository"
	"github.com/user/gophkeeper/internal/server/service"
	"github.com/user/gophkeeper/pkg/gen"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// SecretsHandler implements the gRPC SecretsService server.
type SecretsHandler struct {
	gen.UnimplementedSecretsServiceServer
	secretsService *service.SecretsService
}

// NewSecretsHandler creates a new SecretsHandler with the given secrets service.
func NewSecretsHandler(secretsService *service.SecretsService) *SecretsHandler {
	return &SecretsHandler{secretsService: secretsService}
}

// CreateSecret stores a new encrypted secret.
func (h *SecretsHandler) CreateSecret(ctx context.Context, req *gen.CreateSecretRequest) (*gen.CreateSecretResponse, error) {
	userID, ok := interceptor.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user identity")
	}

	if strings.TrimSpace(req.GetName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "secret name must not be empty")
	}
	if req.GetKind() == gen.DataKind_DATA_KIND_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "secret kind must be specified")
	}

	row, err := h.secretsService.Create(ctx, userID, req.GetName(), int16(req.GetKind()), req.GetPayload())
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create secret")
	}

	return &gen.CreateSecretResponse{Secret: secretRowToProto(row)}, nil
}

// GetSecret retrieves a single secret by ID.
func (h *SecretsHandler) GetSecret(ctx context.Context, req *gen.GetSecretRequest) (*gen.GetSecretResponse, error) {
	userID, ok := interceptor.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user identity")
	}

	secretID, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid secret ID")
	}

	row, err := h.secretsService.GetByID(ctx, userID, secretID)
	if err != nil {
		if errors.Is(err, repository.ErrSecretNotFound) {
			return nil, status.Error(codes.NotFound, "secret not found")
		}
		return nil, status.Error(codes.Internal, "failed to get secret")
	}

	return &gen.GetSecretResponse{Secret: secretRowToProto(row)}, nil
}

// ListSecrets returns all secrets for the authenticated user.
func (h *SecretsHandler) ListSecrets(ctx context.Context, _ *gen.ListSecretsRequest) (*gen.ListSecretsResponse, error) {
	userID, ok := interceptor.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user identity")
	}

	rows, err := h.secretsService.List(ctx, userID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list secrets")
	}

	items := make([]*gen.SecretItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, secretRowToProto(r))
	}

	return &gen.ListSecretsResponse{Secrets: items}, nil
}

// UpdateSecret overwrites a secret with optimistic locking.
func (h *SecretsHandler) UpdateSecret(ctx context.Context, req *gen.UpdateSecretRequest) (*gen.UpdateSecretResponse, error) {
	userID, ok := interceptor.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user identity")
	}

	secretID, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid secret ID")
	}

	if strings.TrimSpace(req.GetName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "secret name must not be empty")
	}
	if req.GetKind() == gen.DataKind_DATA_KIND_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "secret kind must be specified")
	}

	row, err := h.secretsService.Update(ctx, userID, secretID, req.GetName(), int16(req.GetKind()), req.GetPayload(), req.GetVersion())
	if err != nil {
		if errors.Is(err, repository.ErrVersionConflict) {
			return nil, status.Error(codes.Aborted, "version conflict")
		}
		return nil, status.Error(codes.Internal, "failed to update secret")
	}

	return &gen.UpdateSecretResponse{Secret: secretRowToProto(row)}, nil
}

// DeleteSecret soft-deletes a secret.
func (h *SecretsHandler) DeleteSecret(ctx context.Context, req *gen.DeleteSecretRequest) (*gen.DeleteSecretResponse, error) {
	userID, ok := interceptor.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user identity")
	}

	secretID, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid secret ID")
	}

	if err := h.secretsService.SoftDelete(ctx, userID, secretID, req.GetVersion()); err != nil {
		if errors.Is(err, repository.ErrVersionConflict) {
			return nil, status.Error(codes.Aborted, "version conflict")
		}
		return nil, status.Error(codes.Internal, "failed to delete secret")
	}

	return &gen.DeleteSecretResponse{}, nil
}

// SyncSecrets returns secrets modified after a given timestamp.
func (h *SecretsHandler) SyncSecrets(ctx context.Context, req *gen.SyncSecretsRequest) (*gen.SyncSecretsResponse, error) {
	userID, ok := interceptor.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user identity")
	}

	if req.GetSince() == nil {
		return nil, status.Error(codes.InvalidArgument, "since timestamp is required")
	}

	since := req.GetSince().AsTime()

	rows, err := h.secretsService.ListModifiedSince(ctx, userID, since)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to sync secrets")
	}

	items := make([]*gen.SecretItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, secretRowToProto(r))
	}

	return &gen.SyncSecretsResponse{
		Secrets:    items,
		ServerTime: timestamppb.Now(),
	}, nil
}

func secretRowToProto(row *repository.SecretRow) *gen.SecretItem {
	return &gen.SecretItem{
		Id:        row.ID.String(),
		Name:      row.Name,
		Kind:      gen.DataKind(row.Kind),
		Payload:   row.Payload,
		CreatedAt: timestamppb.New(row.CreatedAt),
		UpdatedAt: timestamppb.New(row.UpdatedAt),
		Version:   row.Version,
		Deleted:   row.Deleted,
	}
}
