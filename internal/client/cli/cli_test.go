package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/user/gophkeeper/internal/client/common"
	clientconfig "github.com/user/gophkeeper/internal/client/config"
	"github.com/user/gophkeeper/internal/client/crypto"
	"github.com/user/gophkeeper/internal/client/model"
	"github.com/user/gophkeeper/internal/client/session"
	"github.com/user/gophkeeper/internal/client/store"
	"github.com/user/gophkeeper/pkg/gen"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockClient struct {
	registerFn     func(ctx context.Context, email, password string) error
	loginFn        func(ctx context.Context, email, password string) error
	refreshTokenFn func(ctx context.Context) error
	createSecretFn func(ctx context.Context, name string, kind gen.DataKind, payload []byte) (*gen.SecretItem, error)
	getSecretFn    func(ctx context.Context, id string) (*gen.SecretItem, error)
	listSecretsFn  func(ctx context.Context) ([]*gen.SecretItem, error)
	updateSecretFn func(ctx context.Context, id, name string, kind gen.DataKind, payload []byte, version int64) (*gen.SecretItem, error)
	deleteSecretFn func(ctx context.Context, id string, version int64) error
	syncSecretsFn  func(ctx context.Context, since time.Time) ([]*gen.SecretItem, time.Time, error)
	closeFn        func() error
}

func (m *mockClient) Register(ctx context.Context, email, password string) error {
	if m.registerFn != nil {
		return m.registerFn(ctx, email, password)
	}
	return nil
}

func (m *mockClient) Login(ctx context.Context, email, password string) error {
	if m.loginFn != nil {
		return m.loginFn(ctx, email, password)
	}
	return nil
}

func (m *mockClient) RefreshToken(ctx context.Context) error {
	if m.refreshTokenFn != nil {
		return m.refreshTokenFn(ctx)
	}
	return nil
}

func (m *mockClient) CreateSecret(ctx context.Context, name string, kind gen.DataKind, payload []byte) (*gen.SecretItem, error) {
	if m.createSecretFn != nil {
		return m.createSecretFn(ctx, name, kind, payload)
	}
	return nil, errors.New("not implemented")
}

func (m *mockClient) GetSecret(ctx context.Context, id string) (*gen.SecretItem, error) {
	if m.getSecretFn != nil {
		return m.getSecretFn(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (m *mockClient) ListSecrets(ctx context.Context) ([]*gen.SecretItem, error) {
	if m.listSecretsFn != nil {
		return m.listSecretsFn(ctx)
	}
	return nil, nil
}

func (m *mockClient) UpdateSecret(ctx context.Context, id, name string, kind gen.DataKind, payload []byte, version int64) (*gen.SecretItem, error) {
	if m.updateSecretFn != nil {
		return m.updateSecretFn(ctx, id, name, kind, payload, version)
	}
	return nil, errors.New("not implemented")
}

func (m *mockClient) DeleteSecret(ctx context.Context, id string, version int64) error {
	if m.deleteSecretFn != nil {
		return m.deleteSecretFn(ctx, id, version)
	}
	return nil
}

func (m *mockClient) SyncSecrets(ctx context.Context, since time.Time) ([]*gen.SecretItem, time.Time, error) {
	if m.syncSecretsFn != nil {
		return m.syncSecretsFn(ctx, since)
	}
	return nil, time.Time{}, nil
}

func (m *mockClient) Close() error {
	if m.closeFn != nil {
		return m.closeFn()
	}
	return nil
}

func newTestConfig(t *testing.T) *clientconfig.Config {
	t.Helper()
	return &clientconfig.Config{
		ServerAddress: "localhost:3200",
		ConfigDir:     t.TempDir(),
		Insecure:      true,
	}
}

func newTestSession(t *testing.T) (*session.Session, *crypto.Encryptor) {
	t.Helper()
	sess := session.NewSession()
	enc := crypto.NewEncryptor()
	salt := enc.DeriveUserSalt("user@example.com")
	key := enc.DeriveKey("master-password", salt)
	sess.SetUserEmail("user@example.com")
	sess.SetUserID("user-1")
	sess.SetSalt(salt)
	sess.SetEncryptionKey(key)
	return sess, enc
}

func newCache(t *testing.T, cfg *clientconfig.Config) *store.FileStore {
	t.Helper()
	return store.NewFileStore(cfg.ConfigDir)
}

func TestNewRootCmdContainsCommands(t *testing.T) {
	cfg := newTestConfig(t)
	sess, enc := newTestSession(t)
	cache := newCache(t, cfg)

	root := NewRootCmd(&mockClient{}, cache, sess, enc, cfg)
	if root.Use != "gophkeeper" {
		t.Fatalf("Use = %q, want %q", root.Use, "gophkeeper")
	}

	expected := map[string]bool{
		"version":  true,
		"register": true,
		"login":    true,
		"add":      true,
		"get":      true,
		"list":     true,
		"update":   true,
		"delete":   true,
		"sync":     true,
		"tui":      true,
	}

	for _, cmd := range root.Commands() {
		delete(expected, cmd.Name())
	}
	if len(expected) != 0 {
		t.Fatalf("missing commands: %v", expected)
	}
}

func TestAddCmdCreatesEncryptedSecretAndCachesIt(t *testing.T) {
	cfg := newTestConfig(t)
	sess, enc := newTestSession(t)
	cache := newCache(t, cfg)

	var capturedPayload []byte
	client := &mockClient{
		createSecretFn: func(_ context.Context, name string, kind gen.DataKind, payload []byte) (*gen.SecretItem, error) {
			capturedPayload = payload
			return &gen.SecretItem{
				Id:        "secret-1",
				Name:      name,
				Kind:      kind,
				Payload:   payload,
				Version:   1,
				CreatedAt: timestamppb.Now(),
				UpdatedAt: timestamppb.Now(),
			}, nil
		},
	}

	cmd := newAddCmd(client, cache, sess, enc, cfg)
	cmd.SetArgs([]string{
		"--type", "text",
		"--name", "note",
		"--text", "hello",
		"--meta", "private",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	envelope, err := common.DecodeEnvelope(capturedPayload, sess, enc, cliLoginHint)
	if err != nil {
		t.Fatalf("decodeEnvelope() error = %v", err)
	}
	if envelope.Meta != "private" {
		t.Fatalf("Meta = %q, want %q", envelope.Meta, "private")
	}

	record, err := cache.GetByName(sess.UserID(), "note")
	if err != nil {
		t.Fatalf("GetByName() error = %v", err)
	}
	if record.ID != "secret-1" {
		t.Fatalf("record.ID = %q, want %q", record.ID, "secret-1")
	}
}

func TestGetCmdReadsSecretFromLocalCache(t *testing.T) {
	cfg := newTestConfig(t)
	sess, enc := newTestSession(t)
	cache := newCache(t, cfg)

	payload, err := common.EncodeEnvelope("meta", model.TextData{Content: "hello"}, sess, enc, cliLoginHint)
	if err != nil {
		t.Fatalf("encodeEnvelope() error = %v", err)
	}
	if err := cache.Upsert(sess.UserID(), store.SecretRecord{
		ID:        "secret-1",
		Name:      "note",
		Kind:      int32(gen.DataKind_DATA_KIND_TEXT),
		Payload:   payload,
		Version:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	cmd := newGetCmd(cache, sess, enc, cfg)
	if cmd == nil {
		t.Fatal("newGetCmd() returned nil")
	}
	cmd.SetArgs([]string{"note"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestGetCmdReadsAllSecretTypesFromLocalCache(t *testing.T) {
	tests := []struct {
		name string
		kind gen.DataKind
		body any
	}{
		{
			name: "login",
			kind: gen.DataKind_DATA_KIND_LOGIN,
			body: model.LoginData{Username: "alice", Password: "secret", URL: "https://example.com"},
		},
		{
			name: "binary",
			kind: gen.DataKind_DATA_KIND_BINARY,
			body: model.BinaryData{Filename: "file.bin", Content: []byte("abc")},
		},
		{
			name: "card",
			kind: gen.DataKind_DATA_KIND_CARD,
			body: model.CardData{Number: "4111111111111111", Holder: "Jane", CVV: "123", ExpMonth: 12, ExpYear: 2030},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfig(t)
			sess, enc := newTestSession(t)
			cache := newCache(t, cfg)

			payload, err := common.EncodeEnvelope("meta", tt.body, sess, enc, cliLoginHint)
			if err != nil {
				t.Fatalf("encodeEnvelope() error = %v", err)
			}
			if err := cache.Upsert(sess.UserID(), store.SecretRecord{
				ID:        "secret-" + tt.name,
				Name:      tt.name,
				Kind:      int32(tt.kind),
				Payload:   payload,
				Version:   1,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}); err != nil {
				t.Fatalf("Upsert() error = %v", err)
			}

			cmd := newGetCmd(cache, sess, enc, cfg)
			cmd.SetArgs([]string{tt.name})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
		})
	}
}

func TestGetCmdRequiresCacheAndEncryptionKey(t *testing.T) {
	cfg := newTestConfig(t)
	sess := session.NewSession()
	sess.SetUserID("user-1")
	enc := crypto.NewEncryptor()
	cache := newCache(t, cfg)

	cmd := newGetCmd(cache, sess, enc, cfg)
	cmd.SetArgs([]string{"missing"})
	if err := cmd.Execute(); !errors.Is(err, store.ErrCacheNotInitialized) {
		t.Fatalf("Execute(empty cache) error = %v, want ErrCacheNotInitialized", err)
	}

	payload, err := common.EncodeEnvelope("meta", model.TextData{Content: "hello"}, newSessionWithKey(t), enc, cliLoginHint)
	if err != nil {
		t.Fatalf("encodeEnvelope() error = %v", err)
	}
	if err := cache.Upsert("user-1", store.SecretRecord{
		ID:      "secret-1",
		Name:    "note",
		Kind:    int32(gen.DataKind_DATA_KIND_TEXT),
		Payload: payload,
		Version: 1,
	}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	cmd = newGetCmd(cache, sess, enc, cfg)
	cmd.SetArgs([]string{"note"})
	err = cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "master password not set") {
		t.Fatalf("Execute(no key) error = %v, want master password error", err)
	}
}

func TestUpdateCmdUpdatesServerAndLocalCache(t *testing.T) {
	cfg := newTestConfig(t)
	sess, enc := newTestSession(t)
	cache := newCache(t, cfg)

	initialPayload, err := common.EncodeEnvelope("old-meta", model.TextData{Content: "before"}, sess, enc, cliLoginHint)
	if err != nil {
		t.Fatalf("encodeEnvelope() error = %v", err)
	}
	if err := cache.Upsert(sess.UserID(), store.SecretRecord{
		ID:        "secret-1",
		Name:      "note",
		Kind:      int32(gen.DataKind_DATA_KIND_TEXT),
		Payload:   initialPayload,
		Version:   4,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	client := &mockClient{
		updateSecretFn: func(_ context.Context, id, name string, kind gen.DataKind, payload []byte, version int64) (*gen.SecretItem, error) {
			if id != "secret-1" {
				t.Fatalf("id = %q, want %q", id, "secret-1")
			}
			if version != 4 {
				t.Fatalf("version = %d, want %d", version, 4)
			}
			return &gen.SecretItem{
				Id:        id,
				Name:      name,
				Kind:      kind,
				Payload:   payload,
				Version:   5,
				CreatedAt: timestamppb.Now(),
				UpdatedAt: timestamppb.Now(),
			}, nil
		},
	}

	cmd := newUpdateCmd(client, cache, sess, enc, cfg)
	cmd.SetArgs([]string{"note", "--type", "text", "--text", "after"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	record, err := cache.GetByName(sess.UserID(), "note")
	if err != nil {
		t.Fatalf("GetByName() error = %v", err)
	}
	envelope, err := common.DecodeEnvelope(record.Payload, sess, enc, cliLoginHint)
	if err != nil {
		t.Fatalf("decodeEnvelope() error = %v", err)
	}
	if envelope.Meta != "old-meta" {
		t.Fatalf("Meta = %q, want %q", envelope.Meta, "old-meta")
	}
}

func TestDeleteCmdRemovesSecretFromLocalCache(t *testing.T) {
	cfg := newTestConfig(t)
	sess, enc := newTestSession(t)
	cache := newCache(t, cfg)

	if err := cache.Upsert(sess.UserID(), store.SecretRecord{
		ID:        "secret-1",
		Name:      "note",
		Kind:      int32(gen.DataKind_DATA_KIND_TEXT),
		Payload:   []byte("payload"),
		Version:   3,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	client := &mockClient{
		deleteSecretFn: func(_ context.Context, id string, version int64) error {
			if id != "secret-1" {
				t.Fatalf("id = %q, want %q", id, "secret-1")
			}
			if version != 3 {
				t.Fatalf("version = %d, want %d", version, 3)
			}
			return nil
		},
	}

	cmd := newDeleteCmd(client, cache, sess, enc, cfg)
	cmd.SetArgs([]string{"note"})
	cmd.SetIn(strings.NewReader("y\n"))
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if _, err := cache.GetByName(sess.UserID(), "note"); !errors.Is(err, store.ErrSecretNotFound) {
		t.Fatalf("GetByName() error = %v, want ErrSecretNotFound", err)
	}
}

func TestSyncCmdAppliesDeltaToLocalCache(t *testing.T) {
	cfg := newTestConfig(t)
	sess, enc := newTestSession(t)
	cache := newCache(t, cfg)

	if err := cache.Upsert(sess.UserID(), store.SecretRecord{
		ID:        "delete-me",
		Name:      "delete-me",
		Kind:      int32(gen.DataKind_DATA_KIND_TEXT),
		Payload:   []byte("payload"),
		Version:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	client := &mockClient{
		syncSecretsFn: func(_ context.Context, since time.Time) ([]*gen.SecretItem, time.Time, error) {
			if !since.IsZero() {
				t.Fatalf("since = %v, want zero time on first sync", since)
			}
			return []*gen.SecretItem{
				{
					Id:        "secret-2",
					Name:      "keep",
					Kind:      gen.DataKind_DATA_KIND_TEXT,
					Payload:   []byte("cipher"),
					Version:   2,
					CreatedAt: timestamppb.Now(),
					UpdatedAt: timestamppb.Now(),
				},
				{
					Id:        "delete-me",
					Name:      "delete-me",
					Kind:      gen.DataKind_DATA_KIND_TEXT,
					Deleted:   true,
					Version:   2,
					CreatedAt: timestamppb.Now(),
					UpdatedAt: timestamppb.Now(),
				},
			}, time.Now(), nil
		},
	}

	cmd := newSyncCmd(client, cache, sess, enc, cfg)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	secrets, err := cache.List(sess.UserID())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(secrets) != 1 || secrets[0].Name != "keep" {
		t.Fatalf("List() = %+v, want one cached secret named keep", secrets)
	}

	cursor, err := cache.LoadCursor(sess.UserID())
	if err != nil {
		t.Fatalf("LoadCursor() error = %v", err)
	}
	if cursor.IsZero() {
		t.Fatal("cursor is zero after sync")
	}
}

func TestSyncCmdIgnoresDeleteForEmptyCache(t *testing.T) {
	cfg := newTestConfig(t)
	sess, enc := newTestSession(t)
	cache := newCache(t, cfg)

	client := &mockClient{
		syncSecretsFn: func(_ context.Context, _ time.Time) ([]*gen.SecretItem, time.Time, error) {
			return []*gen.SecretItem{
				{Id: "missing", Name: "missing", Deleted: true, Version: 1},
			}, time.Now(), nil
		},
	}

	cmd := newSyncCmd(client, cache, sess, enc, cfg)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestListCmdFiltersAndRejectsUnknownType(t *testing.T) {
	cfg := newTestConfig(t)
	sess, enc := newTestSession(t)
	cache := newCache(t, cfg)

	for i, kind := range []gen.DataKind{
		gen.DataKind_DATA_KIND_LOGIN,
		gen.DataKind_DATA_KIND_TEXT,
		gen.DataKind_DATA_KIND_BINARY,
		gen.DataKind_DATA_KIND_CARD,
	} {
		if err := cache.Upsert(sess.UserID(), store.SecretRecord{
			ID:        fmt.Sprintf("secret-%d", i),
			Name:      fmt.Sprintf("secret-%d", i),
			Kind:      int32(kind),
			Payload:   []byte("payload"),
			Version:   1,
			UpdatedAt: time.Now(),
		}); err != nil {
			t.Fatalf("Upsert() error = %v", err)
		}
	}

	for _, filter := range []string{"login", "text", "binary", "card"} {
		cmd := newListCmd(cache, sess, enc, cfg)
		cmd.SetArgs([]string{"--type", filter})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute(%s) error = %v", filter, err)
		}
	}

	cmd := newListCmd(cache, sess, enc, cfg)
	cmd.SetArgs([]string{"--type", "otp"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "unknown type") {
		t.Fatalf("Execute(unknown) error = %v, want unknown type", err)
	}
}

func TestRegisterCmdStoresSessionOnDisk(t *testing.T) {
	cfg := newTestConfig(t)
	sess := session.NewSession()
	enc := crypto.NewEncryptor()
	client := &mockClient{
		registerFn: func(_ context.Context, email, password string) error {
			if email != "user@example.com" || password != "account-pass" {
				t.Fatalf("Register() called with email=%q password=%q", email, password)
			}
			sess.SetUserID("user-1")
			return nil
		},
	}

	cmd := newRegisterCmd(client, sess, enc, cfg)
	cmd.SetArgs([]string{"--email", "user@example.com"})
	cmd.SetIn(strings.NewReader("account-pass\nmaster-pass\n"))
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	loaded := session.NewSession()
	if err := loaded.LoadFromDisk(filepath.Join(cfg.ConfigDir, "session.json")); err != nil {
		t.Fatalf("LoadFromDisk() error = %v", err)
	}
	if loaded.UserEmail() != "user@example.com" {
		t.Fatalf("UserEmail = %q, want %q", loaded.UserEmail(), "user@example.com")
	}
	if loaded.UserID() != "user-1" {
		t.Fatalf("UserID = %q, want %q", loaded.UserID(), "user-1")
	}
}

func TestLoginCmdStoresSessionOnDisk(t *testing.T) {
	cfg := newTestConfig(t)
	sess := session.NewSession()
	enc := crypto.NewEncryptor()
	client := &mockClient{
		loginFn: func(_ context.Context, email, password string) error {
			if email != "user@example.com" || password != "account-pass" {
				t.Fatalf("Login() called with email=%q password=%q", email, password)
			}
			sess.SetUserID("user-1")
			return nil
		},
	}

	cmd := newLoginCmd(client, sess, enc, cfg)
	cmd.SetArgs([]string{"--email", "user@example.com"})
	cmd.SetIn(strings.NewReader("account-pass\nmaster-pass\n"))
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	loaded := session.NewSession()
	if err := loaded.LoadFromDisk(filepath.Join(cfg.ConfigDir, "session.json")); err != nil {
		t.Fatalf("LoadFromDisk() error = %v", err)
	}
	if loaded.UserEmail() != "user@example.com" || loaded.UserID() != "user-1" {
		t.Fatalf("loaded session email=%q id=%q", loaded.UserEmail(), loaded.UserID())
	}
}

func TestLoginCmdReturnsClientError(t *testing.T) {
	cfg := newTestConfig(t)
	sess := session.NewSession()
	enc := crypto.NewEncryptor()
	client := &mockClient{
		loginFn: func(context.Context, string, string) error {
			return errors.New("bad credentials")
		},
	}

	cmd := newLoginCmd(client, sess, enc, cfg)
	cmd.SetArgs([]string{"--email", "user@example.com"})
	cmd.SetIn(strings.NewReader("account-pass\n"))
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "login failed") {
		t.Fatalf("Execute() error = %v, want login failed", err)
	}
}

func TestCurrentUserIDAndEnvelopeErrorBranches(t *testing.T) {
	sess := session.NewSession()
	if _, err := common.CurrentUserID(sess, cliLoginHint); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("currentUserID(empty) error = %v, want missing session", err)
	}

	enc := crypto.NewEncryptor()
	if _, err := common.EncodeEnvelope("meta", model.TextData{Content: "x"}, sess, enc, cliLoginHint); err == nil {
		t.Fatal("encodeEnvelope(no key) error = nil, want error")
	}

	sess.SetEncryptionKey([]byte("short"))
	if _, err := common.DecodeEnvelope([]byte("bad payload"), sess, enc, cliLoginHint); err == nil {
		t.Fatal("decodeEnvelope(invalid key/payload) error = nil, want error")
	}
}

func newSessionWithKey(t *testing.T) *session.Session {
	t.Helper()
	sess, _ := newTestSession(t)
	return sess
}
