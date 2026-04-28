// Package store provides a file-backed local cache for synced secrets.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	// ErrCacheNotInitialized indicates that the local cache has not been created yet.
	ErrCacheNotInitialized = errors.New("local cache is empty, run 'gophkeeper sync'")
	// ErrSecretNotFound indicates that the secret does not exist in the local cache.
	ErrSecretNotFound = errors.New("secret not found in local cache")
)

// SecretRecord is the persisted local representation of a secret.
type SecretRecord struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Kind      int32     `json:"kind"`
	Payload   []byte    `json:"payload"`
	Version   int64     `json:"version"`
	Deleted   bool      `json:"deleted"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type cacheFile struct {
	Secrets map[string]SecretRecord `json:"secrets"`
}

type syncState struct {
	Cursor time.Time `json:"cursor"`
}

// SecretStore defines local secret cache operations.
type SecretStore interface {
	Upsert(userID string, secret SecretRecord) error
	GetByName(userID, name string) (*SecretRecord, error)
	GetByID(userID, id string) (*SecretRecord, error)
	List(userID string) ([]SecretRecord, error)
	Delete(userID, id string) error
}

// SyncStateStore defines persisted synchronization cursor operations.
type SyncStateStore interface {
	LoadCursor(userID string) (time.Time, error)
	SaveCursor(userID string, cursor time.Time) error
}

// FileStore stores secrets and sync state as JSON files under the config directory.
type FileStore struct {
	root string
	mu   sync.Mutex
}

// NewFileStore creates a new file-backed cache rooted at the given directory.
func NewFileStore(root string) *FileStore {
	return &FileStore{root: root}
}

// Upsert stores or replaces a secret in the local cache.
func (s *FileStore) Upsert(userID string, secret SecretRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cache, err := s.loadCacheLocked(userID, true)
	if err != nil {
		return err
	}

	cache.Secrets[secret.ID] = secret
	return s.saveCacheLocked(userID, cache)
}

// GetByName returns a secret by its name from the local cache.
func (s *FileStore) GetByName(userID, name string) (*SecretRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cache, err := s.loadCacheLocked(userID, false)
	if err != nil {
		return nil, err
	}

	for _, secret := range cache.Secrets {
		if secret.Name == name {
			record := secret
			return &record, nil
		}
	}

	return nil, ErrSecretNotFound
}

// GetByID returns a secret by its identifier from the local cache.
func (s *FileStore) GetByID(userID, id string) (*SecretRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cache, err := s.loadCacheLocked(userID, false)
	if err != nil {
		return nil, err
	}

	secret, ok := cache.Secrets[id]
	if !ok {
		return nil, ErrSecretNotFound
	}

	record := secret
	return &record, nil
}

// List returns all secrets from the local cache.
func (s *FileStore) List(userID string) ([]SecretRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cache, err := s.loadCacheLocked(userID, false)
	if err != nil {
		return nil, err
	}

	secrets := make([]SecretRecord, 0, len(cache.Secrets))
	for _, secret := range cache.Secrets {
		secrets = append(secrets, secret)
	}

	return secrets, nil
}

// Delete removes a secret from the local cache.
func (s *FileStore) Delete(userID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cache, err := s.loadCacheLocked(userID, false)
	if err != nil {
		return err
	}

	delete(cache.Secrets, id)
	return s.saveCacheLocked(userID, cache)
}

// LoadCursor reads the persisted sync cursor for a user.
func (s *FileStore) LoadCursor(userID string) (time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.syncPath(userID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("read sync state: %w", err)
	}

	var state syncState
	if err := json.Unmarshal(data, &state); err != nil {
		return time.Time{}, fmt.Errorf("unmarshal sync state: %w", err)
	}

	return state.Cursor, nil
}

// SaveCursor persists the sync cursor for a user.
func (s *FileStore) SaveCursor(userID string, cursor time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.userDir(userID), 0700); err != nil {
		return fmt.Errorf("create user cache dir: %w", err)
	}

	data, err := json.Marshal(syncState{Cursor: cursor})
	if err != nil {
		return fmt.Errorf("marshal sync state: %w", err)
	}

	if err := os.WriteFile(s.syncPath(userID), data, 0600); err != nil {
		return fmt.Errorf("write sync state: %w", err)
	}

	return nil
}

func (s *FileStore) loadCacheLocked(userID string, create bool) (*cacheFile, error) {
	data, err := os.ReadFile(s.cachePath(userID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if !create {
				return nil, ErrCacheNotInitialized
			}

			return &cacheFile{Secrets: make(map[string]SecretRecord)}, nil
		}
		return nil, fmt.Errorf("read cache: %w", err)
	}

	var cache cacheFile
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("unmarshal cache: %w", err)
	}

	if cache.Secrets == nil {
		cache.Secrets = make(map[string]SecretRecord)
	}

	return &cache, nil
}

func (s *FileStore) saveCacheLocked(userID string, cache *cacheFile) error {
	if err := os.MkdirAll(s.userDir(userID), 0700); err != nil {
		return fmt.Errorf("create user cache dir: %w", err)
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}

	if err := os.WriteFile(s.cachePath(userID), data, 0600); err != nil {
		return fmt.Errorf("write cache: %w", err)
	}

	return nil
}

func (s *FileStore) userDir(userID string) string {
	return filepath.Join(s.root, "cache", userID)
}

func (s *FileStore) cachePath(userID string) string {
	return filepath.Join(s.userDir(userID), "secrets.json")
}

func (s *FileStore) syncPath(userID string) string {
	return filepath.Join(s.userDir(userID), "sync.json")
}
