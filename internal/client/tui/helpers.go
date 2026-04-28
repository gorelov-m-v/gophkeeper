package tui

import (
	"fmt"
	"time"

	"github.com/user/gophkeeper/internal/client/crypto"
	"github.com/user/gophkeeper/internal/client/model"
	"github.com/user/gophkeeper/internal/client/session"
	"github.com/user/gophkeeper/internal/client/store"
	"github.com/user/gophkeeper/pkg/gen"
)

func currentUserID(sess *session.Session) (string, error) {
	userID := sess.UserID()
	if userID == "" {
		return "", fmt.Errorf("user session is missing, please log in first")
	}

	return userID, nil
}

func secretRecordFromProto(secret *gen.SecretItem) store.SecretRecord {
	return store.SecretRecord{
		ID:      secret.GetId(),
		Name:    secret.GetName(),
		Kind:    int32(secret.GetKind()),
		Payload: secret.GetPayload(),
		Version: secret.GetVersion(),
		Deleted: secret.GetDeleted(),
		CreatedAt: func() time.Time {
			if secret.GetCreatedAt() != nil {
				return secret.GetCreatedAt().AsTime()
			}
			return time.Time{}
		}(),
		UpdatedAt: func() time.Time {
			if secret.GetUpdatedAt() != nil {
				return secret.GetUpdatedAt().AsTime()
			}
			return time.Time{}
		}(),
	}
}

func decodeEnvelope(ciphertext []byte, sess *session.Session, enc *crypto.Encryptor) (*model.SecretEnvelope, error) {
	if len(sess.EncryptionKey()) == 0 {
		return nil, fmt.Errorf("master password not set, please log in first")
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

func encodeEnvelope(meta string, body any, sess *session.Session, enc *crypto.Encryptor) ([]byte, error) {
	if len(sess.EncryptionKey()) == 0 {
		return nil, fmt.Errorf("master password not set, please log in first")
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
