// Package common provides shared helpers for client interfaces.
package common

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/user/gophkeeper/internal/client/crypto"
	"github.com/user/gophkeeper/internal/client/model"
	"github.com/user/gophkeeper/internal/client/session"
	"github.com/user/gophkeeper/internal/client/store"
	"github.com/user/gophkeeper/pkg/gen"
)

// CurrentUserID returns the authenticated user ID from session state.
func CurrentUserID(sess *session.Session, loginHint string) (string, error) {
	userID := sess.UserID()
	if userID == "" {
		return "", fmt.Errorf("user session is missing, %s", loginHint)
	}
	return userID, nil
}

// SecretRecordFromProto converts a wire secret item into a local cache record.
func SecretRecordFromProto(secret *gen.SecretItem) store.SecretRecord {
	return store.SecretRecord{
		ID:      secret.GetId(),
		Name:    secret.GetName(),
		Kind:    int32(secret.GetKind()),
		Payload: secret.GetPayload(),
		Version: secret.GetVersion(),
		Deleted: secret.GetDeleted(),
		CreatedAt: func() (createdAt time.Time) {
			if secret.GetCreatedAt() != nil {
				createdAt = secret.GetCreatedAt().AsTime()
			}
			return createdAt
		}(),
		UpdatedAt: func() (updatedAt time.Time) {
			if secret.GetUpdatedAt() != nil {
				updatedAt = secret.GetUpdatedAt().AsTime()
			}
			return updatedAt
		}(),
	}
}

// DecodeEnvelope decrypts and parses an encrypted secret payload.
func DecodeEnvelope(ciphertext []byte, sess *session.Session, enc *crypto.Encryptor, loginHint string) (*model.SecretEnvelope, error) {
	if len(sess.EncryptionKey()) == 0 {
		return nil, fmt.Errorf("master password not set, %s", loginHint)
	}

	plaintext, err := enc.Decrypt(sess.EncryptionKey(), ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	envelope, err := model.DecodeSecretEnvelope(plaintext)
	if err != nil {
		return nil, fmt.Errorf("decode secret envelope: %w", err)
	}

	return envelope, nil
}

// EncodeEnvelope builds, serializes, and encrypts a secret envelope.
func EncodeEnvelope(meta string, body any, sess *session.Session, enc *crypto.Encryptor, loginHint string) ([]byte, error) {
	if len(sess.EncryptionKey()) == 0 {
		return nil, fmt.Errorf("master password not set, %s", loginHint)
	}

	plaintext, err := model.NewSecretEnvelope(meta, body)
	if err != nil {
		return nil, fmt.Errorf("build secret envelope: %w", err)
	}

	ciphertext, err := enc.Encrypt(sess.EncryptionKey(), plaintext)
	if err != nil {
		return nil, fmt.Errorf("encryption failed: %w", err)
	}

	return ciphertext, nil
}

// SortSecretsByName sorts secret records by display name.
func SortSecretsByName(secrets []store.SecretRecord) {
	slices.SortFunc(secrets, func(left, right store.SecretRecord) int {
		return cmp.Compare(left.Name, right.Name)
	})
}

// DecodeBody decodes a typed secret body from an envelope.
func DecodeBody[T any](envelope *model.SecretEnvelope) (*T, error) {
	var body T
	if err := json.Unmarshal(envelope.Body, &body); err != nil {
		return nil, err
	}
	return &body, nil
}
