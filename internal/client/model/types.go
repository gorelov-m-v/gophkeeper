// Package model defines shared data types used by both the CLI and TUI clients.
package model

import (
	"encoding/json"
	"fmt"
)

// LoginData holds plaintext login credentials.
type LoginData struct {
	Username string `json:"username"`
	Password string `json:"password"`
	URL      string `json:"url,omitempty"`
}

// TextData holds plaintext text content.
type TextData struct {
	Content string `json:"content"`
}

// BinaryData holds a filename and its binary content.
type BinaryData struct {
	Filename string `json:"filename"`
	Content  []byte `json:"content"`
}

// CardData holds plaintext payment card information.
type CardData struct {
	Number   string `json:"number"`
	Holder   string `json:"holder"`
	CVV      string `json:"cvv"`
	ExpMonth int    `json:"exp_month"`
	ExpYear  int    `json:"exp_year"`
}

// SecretEnvelopeVersion is the current encrypted payload envelope version.
const SecretEnvelopeVersion = 1

// SecretEnvelope wraps a typed secret body and encrypted metadata.
type SecretEnvelope struct {
	Version int             `json:"version"`
	Meta    string          `json:"meta,omitempty"`
	Body    json.RawMessage `json:"body"`
}

// NewSecretEnvelope creates a versioned envelope from the given metadata and body.
func NewSecretEnvelope(meta string, body any) ([]byte, error) {
	encodedBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	encodedEnvelope, err := json.Marshal(SecretEnvelope{
		Version: SecretEnvelopeVersion,
		Meta:    meta,
		Body:    encodedBody,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal envelope: %w", err)
	}

	return encodedEnvelope, nil
}

// DecodeSecretEnvelope parses an encrypted envelope payload.
func DecodeSecretEnvelope(data []byte) (*SecretEnvelope, error) {
	var envelope SecretEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}

	if envelope.Version != SecretEnvelopeVersion {
		return nil, fmt.Errorf("unsupported envelope version %d", envelope.Version)
	}

	return &envelope, nil
}
