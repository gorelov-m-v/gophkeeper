// Package crypto provides AES-256-GCM encryption with Argon2 key derivation.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
)

// Encryptor performs stateless encryption and key derivation operations.
type Encryptor struct{}

// NewEncryptor returns a new Encryptor instance.
func NewEncryptor() *Encryptor {
	return &Encryptor{}
}

// DeriveKey derives a 32-byte encryption key from a master password and salt
// using Argon2id with time=3, memory=64MB, threads=4.
func (e *Encryptor) DeriveKey(masterPassword string, salt []byte) []byte {
	return argon2.IDKey([]byte(masterPassword), salt, 3, 64*1024, 4, 32)
}

// GenerateSalt returns 16 cryptographically random bytes suitable for key derivation.
func (e *Encryptor) GenerateSalt() ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// DeriveUserSalt returns a deterministic 16-byte salt derived from the given
// email address using SHA-256 with a fixed domain separator. This ensures the
// same email always produces the same salt on any device.
func (e *Encryptor) DeriveUserSalt(email string) []byte {
	h := sha256.Sum256([]byte("gophkeeper-salt:" + email))
	return h[:16]
}

// Encrypt encrypts plaintext using AES-256-GCM with the provided key.
// The returned ciphertext has the nonce prepended.
func (e *Encryptor) Encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts ciphertext produced by Encrypt using AES-256-GCM.
// It expects the nonce to be prepended to the ciphertext.
func (e *Encryptor) Decrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce := ciphertext[:nonceSize]
	data := ciphertext[nonceSize:]

	return gcm.Open(nil, nonce, data, nil)
}
