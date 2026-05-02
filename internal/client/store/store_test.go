package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testRecord(id, name string) SecretRecord {
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	return SecretRecord{
		ID:        id,
		Name:      name,
		Kind:      2,
		Payload:   []byte("ciphertext-" + id),
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestFileStoreCRUDAndPerUserIsolation(t *testing.T) {
	cache := NewFileStore(t.TempDir())
	first := testRecord("id-1", "alpha")
	second := testRecord("id-2", "beta")

	if err := cache.Upsert("user-1", first); err != nil {
		t.Fatalf("Upsert(first) error = %v", err)
	}
	if err := cache.Upsert("user-1", second); err != nil {
		t.Fatalf("Upsert(second) error = %v", err)
	}
	if err := cache.Upsert("user-2", testRecord("id-3", "alpha")); err != nil {
		t.Fatalf("Upsert(user-2) error = %v", err)
	}

	byName, err := cache.GetByName("user-1", "alpha")
	if err != nil {
		t.Fatalf("GetByName() error = %v", err)
	}
	if byName.ID != "id-1" {
		t.Fatalf("GetByName().ID = %q, want %q", byName.ID, "id-1")
	}

	byID, err := cache.GetByID("user-1", "id-2")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if byID.Name != "beta" {
		t.Fatalf("GetByID().Name = %q, want %q", byID.Name, "beta")
	}

	secrets, err := cache.List("user-1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(secrets) != 2 {
		t.Fatalf("List() len = %d, want 2", len(secrets))
	}

	if err := cache.Delete("user-1", "id-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := cache.GetByName("user-1", "alpha"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("GetByName(deleted) error = %v, want ErrSecretNotFound", err)
	}

	otherUser, err := cache.GetByName("user-2", "alpha")
	if err != nil {
		t.Fatalf("GetByName(user-2) error = %v", err)
	}
	if otherUser.ID != "id-3" {
		t.Fatalf("other user secret ID = %q, want %q", otherUser.ID, "id-3")
	}
}

func TestFileStoreEmptyCacheErrors(t *testing.T) {
	cache := NewFileStore(t.TempDir())

	if _, err := cache.List("missing-user"); !errors.Is(err, ErrCacheNotInitialized) {
		t.Fatalf("List() error = %v, want ErrCacheNotInitialized", err)
	}
	if _, err := cache.GetByID("missing-user", "id"); !errors.Is(err, ErrCacheNotInitialized) {
		t.Fatalf("GetByID() error = %v, want ErrCacheNotInitialized", err)
	}
	if err := cache.Delete("missing-user", "id"); !errors.Is(err, ErrCacheNotInitialized) {
		t.Fatalf("Delete() error = %v, want ErrCacheNotInitialized", err)
	}
}

func TestFileStoreMissingSecretErrors(t *testing.T) {
	cache := NewFileStore(t.TempDir())
	if err := cache.Upsert("user-1", testRecord("id-1", "alpha")); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	if _, err := cache.GetByName("user-1", "missing"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("GetByName() error = %v, want ErrSecretNotFound", err)
	}
	if _, err := cache.GetByID("user-1", "missing"); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("GetByID() error = %v, want ErrSecretNotFound", err)
	}
}

func TestFileStoreRejectsUnsafeUserID(t *testing.T) {
	cache := NewFileStore(t.TempDir())
	record := testRecord("id-1", "alpha")

	for _, userID := range []string{"", ".", "..", "../escape", `..\escape`, "nested/user"} {
		if err := cache.Upsert(userID, record); !errors.Is(err, ErrInvalidUserID) {
			t.Fatalf("Upsert(%q) error = %v, want ErrInvalidUserID", userID, err)
		}
		if _, err := cache.List(userID); !errors.Is(err, ErrInvalidUserID) {
			t.Fatalf("List(%q) error = %v, want ErrInvalidUserID", userID, err)
		}
		if err := cache.SaveCursor(userID, time.Now()); !errors.Is(err, ErrInvalidUserID) {
			t.Fatalf("SaveCursor(%q) error = %v, want ErrInvalidUserID", userID, err)
		}
	}
}

func TestFileStoreSyncCursor(t *testing.T) {
	cache := NewFileStore(t.TempDir())

	cursor, err := cache.LoadCursor("user-1")
	if err != nil {
		t.Fatalf("LoadCursor(empty) error = %v", err)
	}
	if !cursor.IsZero() {
		t.Fatalf("LoadCursor(empty) = %v, want zero time", cursor)
	}

	want := time.Date(2026, 4, 29, 13, 30, 0, 0, time.UTC)
	if err := cache.SaveCursor("user-1", want); err != nil {
		t.Fatalf("SaveCursor() error = %v", err)
	}

	got, err := cache.LoadCursor("user-1")
	if err != nil {
		t.Fatalf("LoadCursor(saved) error = %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("LoadCursor(saved) = %v, want %v", got, want)
	}
}

func TestFileStoreCorruptedFiles(t *testing.T) {
	root := t.TempDir()
	cache := NewFileStore(root)

	userDir := filepath.Join(root, "cache", "user-1")
	if err := os.MkdirAll(userDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "secrets.json"), []byte("{bad json"), 0600); err != nil {
		t.Fatalf("WriteFile(secrets) error = %v", err)
	}
	if _, err := cache.List("user-1"); err == nil {
		t.Fatal("List(corrupted) error = nil, want error")
	}

	if err := os.WriteFile(filepath.Join(userDir, "sync.json"), []byte("{bad json"), 0600); err != nil {
		t.Fatalf("WriteFile(sync) error = %v", err)
	}
	if _, err := cache.LoadCursor("user-1"); err == nil {
		t.Fatal("LoadCursor(corrupted) error = nil, want error")
	}
}
