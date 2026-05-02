package common

import (
	"strings"
	"testing"
	"time"

	"github.com/user/gophkeeper/internal/client/crypto"
	"github.com/user/gophkeeper/internal/client/model"
	"github.com/user/gophkeeper/internal/client/session"
	"github.com/user/gophkeeper/internal/client/store"
	"github.com/user/gophkeeper/pkg/gen"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestCurrentUserID(t *testing.T) {
	sess := session.NewSession()
	if _, err := CurrentUserID(sess, "please log in"); err == nil || !strings.Contains(err.Error(), "please log in") {
		t.Fatalf("CurrentUserID(empty) error = %v, want login hint", err)
	}

	sess.SetUserID("user-1")
	got, err := CurrentUserID(sess, "please log in")
	if err != nil {
		t.Fatalf("CurrentUserID() error = %v", err)
	}
	if got != "user-1" {
		t.Fatalf("CurrentUserID() = %q, want user-1", got)
	}
}

func TestEnvelopeHelpersAndDecodeBody(t *testing.T) {
	enc := crypto.NewEncryptor()
	sess := session.NewSession()
	key := enc.DeriveKey("master", []byte("0123456789abcdef"))
	sess.SetEncryptionKey(key)

	payload, err := EncodeEnvelope("meta", model.TextData{Content: "hello"}, sess, enc, "please log in")
	if err != nil {
		t.Fatalf("EncodeEnvelope() error = %v", err)
	}

	envelope, err := DecodeEnvelope(payload, sess, enc, "please log in")
	if err != nil {
		t.Fatalf("DecodeEnvelope() error = %v", err)
	}
	if envelope.Meta != "meta" {
		t.Fatalf("DecodeEnvelope().Meta = %q, want meta", envelope.Meta)
	}

	body, err := DecodeBody[model.TextData](envelope)
	if err != nil {
		t.Fatalf("DecodeBody() error = %v", err)
	}
	if body.Content != "hello" {
		t.Fatalf("DecodeBody().Content = %q, want hello", body.Content)
	}
}

func TestEnvelopeHelpersMissingKey(t *testing.T) {
	enc := crypto.NewEncryptor()
	sess := session.NewSession()

	if _, err := EncodeEnvelope("meta", model.TextData{Content: "hello"}, sess, enc, "please log in"); err == nil {
		t.Fatal("EncodeEnvelope(no key) error = nil, want error")
	}
	if _, err := DecodeEnvelope([]byte("ciphertext"), sess, enc, "please log in"); err == nil {
		t.Fatal("DecodeEnvelope(no key) error = nil, want error")
	}
}

func TestSecretRecordFromProto(t *testing.T) {
	createdAt := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)

	record := SecretRecordFromProto(&gen.SecretItem{
		Id:        "secret-id",
		Name:      "secret",
		Kind:      gen.DataKind_DATA_KIND_TEXT,
		Payload:   []byte("payload"),
		Version:   3,
		Deleted:   true,
		CreatedAt: timestamppb.New(createdAt),
		UpdatedAt: timestamppb.New(updatedAt),
	})

	if record.ID != "secret-id" || record.Name != "secret" || record.Kind != int32(gen.DataKind_DATA_KIND_TEXT) {
		t.Fatalf("SecretRecordFromProto() basic fields = %+v", record)
	}
	if !record.CreatedAt.Equal(createdAt) || !record.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("SecretRecordFromProto() times = %v/%v", record.CreatedAt, record.UpdatedAt)
	}
	if !record.Deleted || record.Version != 3 || string(record.Payload) != "payload" {
		t.Fatalf("SecretRecordFromProto() state fields = %+v", record)
	}
}

func TestSortSecretsByName(t *testing.T) {
	secrets := []store.SecretRecord{
		{Name: "zeta"},
		{Name: "alpha"},
		{Name: "beta"},
	}

	SortSecretsByName(secrets)

	if got := []string{secrets[0].Name, secrets[1].Name, secrets[2].Name}; strings.Join(got, ",") != "alpha,beta,zeta" {
		t.Fatalf("SortSecretsByName() = %v", got)
	}
}
