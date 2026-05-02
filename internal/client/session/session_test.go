package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetGetTokens(t *testing.T) {
	s := NewSession()
	s.SetTokens("access-tok", "refresh-tok")

	assert.Equal(t, "access-tok", s.AccessToken())
	assert.Equal(t, "refresh-tok", s.RefreshToken())
}

func TestSetGetEncryptionKey(t *testing.T) {
	s := NewSession()
	key := []byte("0123456789abcdef0123456789abcdef")
	s.SetEncryptionKey(key)

	assert.Equal(t, key, s.EncryptionKey())
}

func TestSaveToDiskLoadFromDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")

	s1 := NewSession()
	s1.SetTokens("access123", "refresh456")
	s1.SetSalt([]byte("abcdef1234567890"))

	err := s1.SaveToDisk(path)
	require.NoError(t, err)

	s2 := NewSession()
	err = s2.LoadFromDisk(path)
	require.NoError(t, err)

	assert.Equal(t, "access123", s2.AccessToken())
	assert.Equal(t, "refresh456", s2.RefreshToken())
	assert.Equal(t, []byte("abcdef1234567890"), s2.Salt())
	assert.Nil(t, s2.EncryptionKey())
}

func TestLoadFromDisk_CorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")

	err := os.WriteFile(path, []byte("{invalid json!!!"), 0600)
	require.NoError(t, err)

	s := NewSession()
	err = s.LoadFromDisk(path)
	assert.Error(t, err, "loading corrupted JSON should return an error")
}

func TestLoadFromDiskNotFound(t *testing.T) {
	s := NewSession()
	err := s.LoadFromDisk(filepath.Join(os.TempDir(), "nonexistent-session-file.json"))
	assert.Error(t, err)
}
