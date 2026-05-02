package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/gophkeeper/internal/client/common"
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

func newTestSession(t *testing.T) (*session.Session, *crypto.Encryptor) {
	t.Helper()
	sess := session.NewSession()
	enc := crypto.NewEncryptor()
	salt := enc.DeriveUserSalt("user@example.com")
	key := enc.DeriveKey("master-password", salt)
	sess.SetUserEmail("user@example.com")
	sess.SetUserID("user-1")
	sess.SetTokens("access", "refresh")
	sess.SetSalt(salt)
	sess.SetEncryptionKey(key)
	return sess, enc
}

func TestNewAppStartsAtMasterPasswordWhenSessionExists(t *testing.T) {
	sess, enc := newTestSession(t)
	cache := store.NewFileStore(t.TempDir())

	app := NewApp(&mockClient{}, cache, sess, enc, t.TempDir())
	if app.currentScreen != screenMasterPassword {
		t.Fatalf("currentScreen = %v, want %v", app.currentScreen, screenMasterPassword)
	}
}

func TestNewAppStartsAtLoginWithoutSession(t *testing.T) {
	cache := store.NewFileStore(t.TempDir())
	app := NewApp(&mockClient{}, cache, session.NewSession(), crypto.NewEncryptor(), t.TempDir())
	if app.currentScreen != screenLogin {
		t.Fatalf("currentScreen = %v, want %v", app.currentScreen, screenLogin)
	}
	if app.Init() == nil {
		t.Fatal("Init() returned nil command")
	}
	if !strings.Contains(app.View(), "Login") {
		t.Fatalf("View() = %q, want Login", app.View())
	}
}

func TestAppScreenTransitions(t *testing.T) {
	sess := session.NewSession()
	sess.SetTokens("access", "refresh")
	sess.SetUserID("user-1")
	enc := crypto.NewEncryptor()
	cache := store.NewFileStore(t.TempDir())
	app := NewApp(&mockClient{}, cache, sess, enc, t.TempDir())

	app.loginModel.emailInput.SetValue("user@example.com")
	app.loginModel.passwordInput.SetValue("account-pass")
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = model.(AppModel)
	if app.currentScreen != screenMasterPassword {
		t.Fatalf("after login currentScreen = %v, want %v", app.currentScreen, screenMasterPassword)
	}

	app.masterPwModel.input.SetValue("master-pass")
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = model.(AppModel)
	if app.currentScreen != screenList {
		t.Fatalf("after master password currentScreen = %v, want %v", app.currentScreen, screenList)
	}
	if cmd == nil {
		t.Fatal("list init command is nil")
	}

	model, cmd = app.Update(secretsLoadedMsg{secrets: []store.SecretRecord{{ID: "secret-1", Name: "note", Kind: int32(gen.DataKind_DATA_KIND_TEXT)}}})
	app = model.(AppModel)
	if app.currentScreen != screenList || cmd != nil {
		t.Fatalf("after list load screen=%v cmd=%v, want list/nil", app.currentScreen, cmd)
	}

	model, cmd = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	app = model.(AppModel)
	if app.currentScreen != screenAdd || cmd == nil {
		t.Fatalf("after n screen=%v cmd=%v, want add/non-nil", app.currentScreen, cmd)
	}

	model, cmd = app.Update(tea.KeyMsg{Type: tea.KeyEscape})
	app = model.(AppModel)
	if app.currentScreen != screenList || cmd == nil {
		t.Fatalf("after add escape screen=%v cmd=%v, want list/non-nil", app.currentScreen, cmd)
	}
}

func TestAppCtrlCQuits(t *testing.T) {
	sess, enc := newTestSession(t)
	cache := store.NewFileStore(t.TempDir())
	app := NewApp(&mockClient{}, cache, sess, enc, t.TempDir())

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if model == nil {
		t.Fatal("model is nil")
	}
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("cmd() = %T, want tea.QuitMsg", cmd())
	}
}

func TestAddModelSubmitCreatesAndCachesSecret(t *testing.T) {
	sess, enc := newTestSession(t)
	cache := store.NewFileStore(t.TempDir())

	add := newAddModel()
	add.inputs[0].SetValue("note")
	add.inputs[1].SetValue("meta")
	add.inputs[2].SetValue("hello")
	add.typeIndex = 1
	add.buildInputs()
	add.inputs[0].SetValue("note")
	add.inputs[1].SetValue("meta")
	add.inputs[2].SetValue("hello")

	client := &mockClient{
		createSecretFn: func(_ context.Context, name string, kind gen.DataKind, payload []byte) (*gen.SecretItem, error) {
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

	if err := add.submit(client, cache, sess, enc); err != nil {
		t.Fatalf("submit() error = %v", err)
	}

	record, err := cache.GetByName(sess.UserID(), "note")
	if err != nil {
		t.Fatalf("GetByName() error = %v", err)
	}
	envelope, err := common.DecodeEnvelope(record.Payload, sess, enc, tuiLoginHint)
	if err != nil {
		t.Fatalf("decodeEnvelope() error = %v", err)
	}
	if envelope.Meta != "meta" {
		t.Fatalf("Meta = %q, want %q", envelope.Meta, "meta")
	}
}

func TestAddModelUpdateViewAndTypeInputs(t *testing.T) {
	add := newAddModel()
	if add.Init() == nil {
		t.Fatal("Init() returned nil command")
	}
	if !strings.Contains(add.View(), "Add Secret") {
		t.Fatalf("View() = %q, want Add Secret", add.View())
	}

	for range secretTypes {
		add.buildInputs()
		if len(add.inputs) == 0 {
			t.Fatal("buildInputs() produced no inputs")
		}
		add.typeIndex = (add.typeIndex + 1) % len(secretTypes)
	}

	add = newAddModel()
	add, _ = add.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if add.err != "name is required" {
		t.Fatalf("enter without name err=%q, want name is required", add.err)
	}

	add.inputs[0].SetValue("note")
	add, _ = add.Update(tea.KeyMsg{Type: tea.KeyTab})
	if add.focused != 1 {
		t.Fatalf("tab focused=%d, want 1", add.focused)
	}
	add, _ = add.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if add.focused != 0 {
		t.Fatalf("shift-tab focused=%d, want 0", add.focused)
	}
	add, _ = add.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	if add.typeIndex != 1 {
		t.Fatalf("ctrl+t typeIndex=%d, want 1", add.typeIndex)
	}
	add, _ = add.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if !add.cancelled {
		t.Fatal("escape did not cancel add model")
	}
}

func TestAddModelSubmitErrors(t *testing.T) {
	sess, enc := newTestSession(t)
	cache := store.NewFileStore(t.TempDir())
	add := newAddModel()
	add.typeIndex = 2
	add.buildInputs()
	add.inputs[0].SetValue("binary")
	add.inputs[1].SetValue("meta")
	add.inputs[2].SetValue("missing.bin")
	if err := add.submit(&mockClient{}, cache, sess, enc); err == nil || !strings.Contains(err.Error(), "failed to read file") {
		t.Fatalf("binary submit error = %v, want read file error", err)
	}

	add = newAddModel()
	add.inputs[0].SetValue("note")
	add.inputs[1].SetValue("meta")
	add.inputs[2].SetValue("hello")
	errClient := &mockClient{
		createSecretFn: func(context.Context, string, gen.DataKind, []byte) (*gen.SecretItem, error) {
			return nil, errors.New("server down")
		},
	}
	if err := add.submit(errClient, cache, sess, enc); err == nil || !strings.Contains(err.Error(), "failed to create secret") {
		t.Fatalf("submit server error = %v, want create error", err)
	}
}

func TestListModelLoadsSecretsFromLocalCache(t *testing.T) {
	sess, enc := newTestSession(t)
	_ = enc
	cache := store.NewFileStore(t.TempDir())

	if err := cache.Upsert(sess.UserID(), store.SecretRecord{
		ID:        "secret-1",
		Name:      "note",
		Kind:      int32(gen.DataKind_DATA_KIND_TEXT),
		Payload:   []byte("payload"),
		Version:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	model := newListModel(&mockClient{}, cache, sess)
	msg := model.loadSecrets()
	loaded, ok := msg.(secretsLoadedMsg)
	if !ok {
		t.Fatalf("loadSecrets() = %T, want secretsLoadedMsg", msg)
	}
	if loaded.err != nil {
		t.Fatalf("loaded.err = %v", loaded.err)
	}
	if len(loaded.secrets) != 1 || loaded.secrets[0].Name != "note" {
		t.Fatalf("loaded.secrets = %+v, want one secret named note", loaded.secrets)
	}
}

func TestKindStringAllKinds(t *testing.T) {
	tests := []struct {
		kind gen.DataKind
		want string
	}{
		{gen.DataKind_DATA_KIND_LOGIN, "login"},
		{gen.DataKind_DATA_KIND_TEXT, "text"},
		{gen.DataKind_DATA_KIND_BINARY, "binary"},
		{gen.DataKind_DATA_KIND_CARD, "card"},
		{gen.DataKind_DATA_KIND_UNSPECIFIED, "unknown"},
	}

	for _, tt := range tests {
		if got := kindString(tt.kind); got != tt.want {
			t.Fatalf("kindString(%v) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestListModelRefreshRowsAndKeyFlow(t *testing.T) {
	sess, _ := newTestSession(t)
	cache := store.NewFileStore(t.TempDir())
	model := newListModel(&mockClient{}, cache, sess)
	model.secrets = []store.SecretRecord{
		{
			ID:        "secret-1",
			Name:      "note",
			Kind:      int32(gen.DataKind_DATA_KIND_TEXT),
			Version:   1,
			UpdatedAt: time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
		},
	}

	model.refreshRows()
	if len(model.table.Rows()) != 1 {
		t.Fatalf("table rows = %d, want 1", len(model.table.Rows()))
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if !updated.wantAdd {
		t.Fatal("wantAdd = false, want true")
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !updated.wantDetail || updated.selectedSecret == nil || updated.selectedSecret.ID != "secret-1" {
		t.Fatalf("detail selection = %+v, want secret-1", updated.selectedSecret)
	}

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("sync key returned nil cmd")
	}
	if updated.confirmDelete {
		t.Fatal("confirmDelete = true after sync key, want false")
	}

	_, quit := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if quit == nil {
		t.Fatal("q key returned nil cmd")
	}
}

func TestListModelDeleteKeyFlow(t *testing.T) {
	sess, _ := newTestSession(t)
	cache := store.NewFileStore(t.TempDir())
	record := store.SecretRecord{
		ID:      "secret-1",
		Name:    "note",
		Kind:    int32(gen.DataKind_DATA_KIND_TEXT),
		Version: 3,
	}
	if err := cache.Upsert(sess.UserID(), record); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	deleted := false
	client := &mockClient{
		deleteSecretFn: func(_ context.Context, id string, version int64) error {
			if id != "secret-1" || version != 3 {
				t.Fatalf("DeleteSecret(%q, %d), want secret-1, 3", id, version)
			}
			deleted = true
			return nil
		},
	}

	model := newListModel(client, cache, sess)
	model.secrets = []store.SecretRecord{record}
	model.refreshRows()

	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd != nil {
		t.Fatal("first d returned cmd, want confirmation only")
	}
	if !model.confirmDelete {
		t.Fatal("confirmDelete = false, want true")
	}

	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd == nil {
		t.Fatal("second d returned nil cmd")
	}
	msg := cmd()
	result, ok := msg.(deleteResultMsg)
	if !ok {
		t.Fatalf("delete cmd msg = %T, want deleteResultMsg", msg)
	}
	if result.err != nil {
		t.Fatalf("delete result error = %v", result.err)
	}
	if !deleted {
		t.Fatal("DeleteSecret was not called")
	}
	if _, err := cache.GetByID(sess.UserID(), "secret-1"); !errors.Is(err, store.ErrSecretNotFound) {
		t.Fatalf("GetByID(deleted) error = %v, want ErrSecretNotFound", err)
	}
}

func TestListModelSyncSecretsUpdatesCache(t *testing.T) {
	sess, _ := newTestSession(t)
	cache := store.NewFileStore(t.TempDir())

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
				t.Fatalf("since = %v, want zero", since)
			}
			return []*gen.SecretItem{
				{
					Id:        "keep",
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

	model := newListModel(client, cache, sess)
	msg := model.syncSecrets()
	result, ok := msg.(syncResultMsg)
	if !ok {
		t.Fatalf("syncSecrets() = %T, want syncResultMsg", msg)
	}
	if result.err != nil {
		t.Fatalf("result.err = %v", result.err)
	}

	secrets, err := cache.List(sess.UserID())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(secrets) != 1 || secrets[0].Name != "keep" {
		t.Fatalf("List() = %+v, want one cached secret named keep", secrets)
	}
}

func TestListModelMessageBranchesAndView(t *testing.T) {
	sess, _ := newTestSession(t)
	cache := store.NewFileStore(t.TempDir())
	model := newListModel(&mockClient{}, cache, sess)

	if model.Init() == nil {
		t.Fatal("Init() returned nil command")
	}

	model, _ = model.Update(secretsLoadedMsg{err: errors.New("bad cache")})
	if !strings.Contains(model.err, "failed to load local cache") {
		t.Fatalf("secretsLoaded error = %q, want cache error", model.err)
	}

	model, cmd := model.Update(syncResultMsg{updated: 1, deleted: 2})
	if cmd == nil || !strings.Contains(model.statusMsg, "Sync complete") {
		t.Fatalf("sync success status=%q cmd=%v", model.statusMsg, cmd)
	}

	model, _ = model.Update(syncResultMsg{err: errors.New("network")})
	if model.err != "network" {
		t.Fatalf("sync error = %q, want network", model.err)
	}

	model, cmd = model.Update(deleteResultMsg{})
	if cmd == nil || model.statusMsg != "Secret deleted" {
		t.Fatalf("delete success status=%q cmd=%v", model.statusMsg, cmd)
	}

	model, _ = model.Update(deleteResultMsg{err: errors.New("denied")})
	if !strings.Contains(model.err, "delete failed") {
		t.Fatalf("delete error = %q, want delete failed", model.err)
	}

	if !strings.Contains(model.View(), "Your Secrets") {
		t.Fatalf("View() = %q, want title", model.View())
	}
}

func TestDetailModelDecryptsEnvelopeMetadata(t *testing.T) {
	sess, enc := newTestSession(t)
	payload, err := common.EncodeEnvelope("meta", model.TextData{Content: "hello"}, sess, enc, tuiLoginHint)
	if err != nil {
		t.Fatalf("encodeEnvelope() error = %v", err)
	}

	detail := newDetailModel(store.SecretRecord{
		ID:      "secret-1",
		Name:    "note",
		Kind:    int32(gen.DataKind_DATA_KIND_TEXT),
		Payload: payload,
		Version: 1,
	}, sess, enc)

	if detail.err != "" {
		t.Fatalf("detail.err = %q", detail.err)
	}

	foundMeta := false
	foundContent := false
	for _, field := range detail.fields {
		if field.label == "Metadata" && field.value == "meta" {
			foundMeta = true
		}
		if field.label == "Content" && field.value == "hello" {
			foundContent = true
		}
	}
	if !foundMeta || !foundContent {
		t.Fatalf("fields = %+v, want metadata and content", detail.fields)
	}
}

func TestDetailModelDecryptsOtherSecretTypes(t *testing.T) {
	tests := []struct {
		name      string
		kind      gen.DataKind
		body      any
		wantLabel string
		wantValue string
	}{
		{
			name:      "login",
			kind:      gen.DataKind_DATA_KIND_LOGIN,
			body:      model.LoginData{Username: "alice", Password: "secret", URL: "https://example.com"},
			wantLabel: "Username",
			wantValue: "alice",
		},
		{
			name:      "binary",
			kind:      gen.DataKind_DATA_KIND_BINARY,
			body:      model.BinaryData{Filename: "file.bin", Content: []byte("abc")},
			wantLabel: "Filename",
			wantValue: "file.bin",
		},
		{
			name:      "card",
			kind:      gen.DataKind_DATA_KIND_CARD,
			body:      model.CardData{Number: "4111111111111111", Holder: "Jane", CVV: "123", ExpMonth: 12, ExpYear: 2030},
			wantLabel: "Holder",
			wantValue: "Jane",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess, enc := newTestSession(t)
			payload, err := common.EncodeEnvelope("meta", tt.body, sess, enc, tuiLoginHint)
			if err != nil {
				t.Fatalf("encodeEnvelope() error = %v", err)
			}

			detail := newDetailModel(store.SecretRecord{
				ID:      "secret-" + tt.name,
				Name:    tt.name,
				Kind:    int32(tt.kind),
				Payload: payload,
				Version: 1,
			}, sess, enc)
			if detail.err != "" {
				t.Fatalf("detail.err = %q", detail.err)
			}

			found := false
			for _, field := range detail.fields {
				if field.label == tt.wantLabel && field.value == tt.wantValue {
					found = true
				}
			}
			if !found {
				t.Fatalf("fields = %+v, want %s=%s", detail.fields, tt.wantLabel, tt.wantValue)
			}
		})
	}
}

func TestDetailModelErrorAndBackKeys(t *testing.T) {
	sess, enc := newTestSession(t)
	detail := newDetailModel(store.SecretRecord{
		ID:      "bad",
		Name:    "bad",
		Kind:    int32(gen.DataKind_DATA_KIND_TEXT),
		Payload: []byte("not encrypted"),
		Version: 1,
	}, sess, enc)
	if detail.err == "" {
		t.Fatal("detail.err is empty, want decrypt error")
	}
	if !strings.Contains(detail.View(), "decryption failed") {
		t.Fatal("View() does not include decrypt error")
	}

	detail, _ = detail.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if !detail.back {
		t.Fatal("escape did not set back=true")
	}
}

func TestLoginAndMasterPasswordModels(t *testing.T) {
	login := newLoginModel()
	if login.Init() == nil {
		t.Fatal("login Init() returned nil command")
	}
	login, _ = login.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if login.err != "email and password are required" {
		t.Fatalf("login.err = %q, want required error", login.err)
	}
	login, _ = login.Update(tea.KeyMsg{Type: tea.KeyTab})
	if login.focused != 1 {
		t.Fatalf("login tab focused=%d, want 1", login.focused)
	}
	login, _ = login.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if login.focused != 0 {
		t.Fatalf("login shift-tab focused=%d, want 0", login.focused)
	}
	login, _ = login.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	if !login.isRegister {
		t.Fatal("ctrl+r did not toggle register mode")
	}
	if !strings.Contains(login.View(), "Register") {
		t.Fatalf("login View() = %q, want Register", login.View())
	}
	login.isRegister = false

	login.emailInput.SetValue("user@example.com")
	login.passwordInput.SetValue("account-pass")
	if login.Email() != "user@example.com" {
		t.Fatalf("Email() = %q, want user@example.com", login.Email())
	}
	if err := login.submit(&mockClient{}); err != nil {
		t.Fatalf("login submit error = %v", err)
	}

	login.isRegister = true
	errClient := &mockClient{
		registerFn: func(context.Context, string, string) error {
			return errors.New("duplicate")
		},
	}
	if err := login.submit(errClient); err == nil || !strings.Contains(err.Error(), "registration failed") {
		t.Fatalf("register submit error = %v, want registration failed", err)
	}

	master := newMasterPwModel("user@example.com")
	if master.Init() == nil {
		t.Fatal("master Init() returned nil command")
	}
	master, _ = master.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if master.err != "master password is required" {
		t.Fatalf("master.err = %q, want required error", master.err)
	}
	if !strings.Contains(master.View(), "Master Password") {
		t.Fatalf("master View() = %q, want Master Password", master.View())
	}

	master.input.SetValue("master-pass")
	sess := session.NewSession()
	enc := crypto.NewEncryptor()
	if err := master.submit(sess, enc); err != nil {
		t.Fatalf("master submit error = %v", err)
	}
	if len(sess.EncryptionKey()) == 0 || len(sess.Salt()) == 0 {
		t.Fatal("master submit did not derive key and salt")
	}
}
