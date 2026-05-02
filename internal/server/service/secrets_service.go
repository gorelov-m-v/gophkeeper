package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/user/gophkeeper/internal/server/repository"
)

// SecretRepository describes secret persistence behavior required by SecretsService.
type SecretRepository interface {
	Create(ctx context.Context, userID uuid.UUID, name string, kind int16, payload []byte) (*repository.SecretRow, error)
	GetByID(ctx context.Context, userID, secretID uuid.UUID) (*repository.SecretRow, error)
	List(ctx context.Context, userID uuid.UUID) ([]*repository.SecretRow, error)
	Update(ctx context.Context, userID, secretID uuid.UUID, name string, kind int16, payload []byte, expectedVersion int64) (*repository.SecretRow, error)
	SoftDelete(ctx context.Context, userID, secretID uuid.UUID, expectedVersion int64) error
	ListModifiedSince(ctx context.Context, userID uuid.UUID, since time.Time) ([]*repository.SecretRow, error)
}

// SecretsService provides business logic for secret management.
type SecretsService struct {
	secretRepo SecretRepository
}

// NewSecretsService creates a new SecretsService with the given repository.
func NewSecretsService(secretRepo SecretRepository) *SecretsService {
	return &SecretsService{secretRepo: secretRepo}
}

// Create stores a new secret for the given user.
func (s *SecretsService) Create(ctx context.Context, userID uuid.UUID, name string, kind int16, payload []byte) (*repository.SecretRow, error) {
	return s.secretRepo.Create(ctx, userID, name, kind, payload)
}

// GetByID retrieves a single secret by ID for the given user.
func (s *SecretsService) GetByID(ctx context.Context, userID, secretID uuid.UUID) (*repository.SecretRow, error) {
	return s.secretRepo.GetByID(ctx, userID, secretID)
}

// List returns all non-deleted secrets for the given user.
func (s *SecretsService) List(ctx context.Context, userID uuid.UUID) ([]*repository.SecretRow, error) {
	return s.secretRepo.List(ctx, userID)
}

// Update modifies a secret with optimistic locking.
func (s *SecretsService) Update(ctx context.Context, userID, secretID uuid.UUID, name string, kind int16, payload []byte, expectedVersion int64) (*repository.SecretRow, error) {
	return s.secretRepo.Update(ctx, userID, secretID, name, kind, payload, expectedVersion)
}

// SoftDelete marks a secret as deleted with optimistic locking.
func (s *SecretsService) SoftDelete(ctx context.Context, userID, secretID uuid.UUID, expectedVersion int64) error {
	return s.secretRepo.SoftDelete(ctx, userID, secretID, expectedVersion)
}

// ListModifiedSince returns secrets modified after the given timestamp.
func (s *SecretsService) ListModifiedSince(ctx context.Context, userID uuid.UUID, since time.Time) ([]*repository.SecretRow, error) {
	return s.secretRepo.ListModifiedSince(ctx, userID, since)
}
