package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	enc := NewEncryptor()
	key := enc.DeriveKey("password", []byte("0123456789abcdef"))
	plaintext := []byte("hello, world!")

	ciphertext, err := enc.Encrypt(key, plaintext)
	require.NoError(t, err)
	require.NotEmpty(t, ciphertext)

	decrypted, err := enc.Decrypt(key, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestDecryptWrongKey(t *testing.T) {
	enc := NewEncryptor()
	key1 := enc.DeriveKey("password1", []byte("0123456789abcdef"))
	key2 := enc.DeriveKey("password2", []byte("0123456789abcdef"))

	ciphertext, err := enc.Encrypt(key1, []byte("secret data"))
	require.NoError(t, err)

	_, err = enc.Decrypt(key2, ciphertext)
	assert.Error(t, err)
}

func TestDecryptTooShort(t *testing.T) {
	enc := NewEncryptor()
	key := enc.DeriveKey("password", []byte("0123456789abcdef"))

	_, err := enc.Decrypt(key, []byte{})
	assert.Error(t, err)

	_, err = enc.Decrypt(key, []byte{1, 2, 3})
	assert.Error(t, err)
}

func TestDeriveKey(t *testing.T) {
	enc := NewEncryptor()
	salt := []byte("0123456789abcdef")

	key1 := enc.DeriveKey("password", salt)
	key2 := enc.DeriveKey("password", salt)
	assert.Equal(t, key1, key2)

	key3 := enc.DeriveKey("password", []byte("fedcba9876543210"))
	assert.NotEqual(t, key1, key3)

	key4 := enc.DeriveKey("different", salt)
	assert.NotEqual(t, key1, key4)

	assert.Len(t, key1, 32)
}

func TestDeriveUserSalt_Deterministic(t *testing.T) {
	enc := NewEncryptor()
	salt1 := enc.DeriveUserSalt("user@example.com")
	salt2 := enc.DeriveUserSalt("user@example.com")
	assert.Equal(t, salt1, salt2, "same email must produce the same salt")
}

func TestDeriveUserSalt_DifferentEmails(t *testing.T) {
	enc := NewEncryptor()
	salt1 := enc.DeriveUserSalt("alice@example.com")
	salt2 := enc.DeriveUserSalt("bob@example.com")
	assert.NotEqual(t, salt1, salt2, "different emails must produce different salts")
}

func TestDeriveUserSalt_Length(t *testing.T) {
	enc := NewEncryptor()
	salt := enc.DeriveUserSalt("user@example.com")
	assert.Len(t, salt, 16, "derived salt must be 16 bytes")
}

func TestGenerateSalt(t *testing.T) {
	enc := NewEncryptor()

	salt1, err := enc.GenerateSalt()
	require.NoError(t, err)
	assert.Len(t, salt1, 16)

	salt2, err := enc.GenerateSalt()
	require.NoError(t, err)
	assert.Len(t, salt2, 16)

	assert.NotEqual(t, salt1, salt2)
}
