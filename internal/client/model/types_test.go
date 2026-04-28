package model

import (
	"encoding/json"
	"strings"
	"testing"
)

type failingBody struct{}

func (failingBody) MarshalJSON() ([]byte, error) {
	return nil, errMarshalBody
}

type testError string

func (e testError) Error() string {
	return string(e)
}

const errMarshalBody = testError("marshal body failed")

func TestNewSecretEnvelopeAndDecode(t *testing.T) {
	encoded, err := NewSecretEnvelope("bank card", CardData{
		Number:   "4111111111111111",
		Holder:   "Jane Doe",
		CVV:      "123",
		ExpMonth: 12,
		ExpYear:  2030,
	})
	if err != nil {
		t.Fatalf("NewSecretEnvelope() error = %v", err)
	}

	envelope, err := DecodeSecretEnvelope(encoded)
	if err != nil {
		t.Fatalf("DecodeSecretEnvelope() error = %v", err)
	}
	if envelope.Version != SecretEnvelopeVersion {
		t.Fatalf("Version = %d, want %d", envelope.Version, SecretEnvelopeVersion)
	}
	if envelope.Meta != "bank card" {
		t.Fatalf("Meta = %q, want %q", envelope.Meta, "bank card")
	}

	var card CardData
	if err := json.Unmarshal(envelope.Body, &card); err != nil {
		t.Fatalf("Unmarshal body error = %v", err)
	}
	if card.Holder != "Jane Doe" {
		t.Fatalf("Holder = %q, want %q", card.Holder, "Jane Doe")
	}
}

func TestNewSecretEnvelopeBodyMarshalError(t *testing.T) {
	_, err := NewSecretEnvelope("meta", failingBody{})
	if err == nil {
		t.Fatal("NewSecretEnvelope() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "marshal body") {
		t.Fatalf("error = %q, want marshal body context", err.Error())
	}
}

func TestDecodeSecretEnvelopeErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{
			name: "invalid json",
			data: []byte("{bad json"),
			want: "unmarshal envelope",
		},
		{
			name: "unsupported version",
			data: []byte(`{"version":99,"body":{}}`),
			want: "unsupported envelope version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeSecretEnvelope(tt.data)
			if err == nil {
				t.Fatal("DecodeSecretEnvelope() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}
