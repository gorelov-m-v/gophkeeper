package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/user/gophkeeper/internal/client/crypto"
	"github.com/user/gophkeeper/internal/client/session"
	"github.com/user/gophkeeper/internal/client/store"
	"github.com/user/gophkeeper/pkg/gen"
)

type detailModel struct {
	secret store.SecretRecord
	fields []fieldPair
	back   bool
	err    string
}

type fieldPair struct {
	label string
	value string
}

func newDetailModel(secret store.SecretRecord, sess *session.Session, enc *crypto.Encryptor) detailModel {
	m := detailModel{secret: secret}
	m.fields = m.decryptAndParse(sess, enc)
	return m
}

func (m *detailModel) decryptAndParse(sess *session.Session, enc *crypto.Encryptor) []fieldPair {
	fields := []fieldPair{
		{label: "Name", value: m.secret.Name},
		{label: "Type", value: kindString(gen.DataKind(m.secret.Kind))},
		{label: "ID", value: m.secret.ID},
		{label: "Version", value: fmt.Sprintf("%d", m.secret.Version)},
	}

	envelope, err := decodeEnvelope(m.secret.Payload, sess, enc)
	if err != nil {
		m.err = err.Error()
		return fields
	}

	if envelope.Meta != "" {
		fields = append(fields, fieldPair{label: "Metadata", value: envelope.Meta})
	}

	switch gen.DataKind(m.secret.Kind) {
	case gen.DataKind_DATA_KIND_LOGIN:
		var data LoginData
		if err := json.Unmarshal(envelope.Body, &data); err != nil {
			m.err = fmt.Sprintf("failed to parse login data: %v", err)
			return fields
		}
		fields = append(fields,
			fieldPair{label: "Username", value: data.Username},
			fieldPair{label: "Password", value: data.Password},
			fieldPair{label: "URL", value: data.URL},
		)

	case gen.DataKind_DATA_KIND_TEXT:
		var data TextData
		if err := json.Unmarshal(envelope.Body, &data); err != nil {
			m.err = fmt.Sprintf("failed to parse text data: %v", err)
			return fields
		}
		fields = append(fields, fieldPair{label: "Content", value: data.Content})

	case gen.DataKind_DATA_KIND_BINARY:
		var data BinaryData
		if err := json.Unmarshal(envelope.Body, &data); err != nil {
			m.err = fmt.Sprintf("failed to parse binary data: %v", err)
			return fields
		}
		fields = append(fields,
			fieldPair{label: "Filename", value: data.Filename},
			fieldPair{label: "Size", value: fmt.Sprintf("%d bytes", len(data.Content))},
		)

	case gen.DataKind_DATA_KIND_CARD:
		var data CardData
		if err := json.Unmarshal(envelope.Body, &data); err != nil {
			m.err = fmt.Sprintf("failed to parse card data: %v", err)
			return fields
		}
		fields = append(fields,
			fieldPair{label: "Card Number", value: data.Number},
			fieldPair{label: "Holder", value: data.Holder},
			fieldPair{label: "Exp Month", value: fmt.Sprintf("%d", data.ExpMonth)},
			fieldPair{label: "Exp Year", value: fmt.Sprintf("%d", data.ExpYear)},
			fieldPair{label: "CVV", value: data.CVV},
		)
	}

	return fields
}

// Init returns the initial command for the detail screen.
func (m detailModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the detail screen.
func (m detailModel) Update(msg tea.Msg) (detailModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "esc" || keyMsg.String() == "q" {
			m.back = true
			return m, nil
		}
	}
	return m, nil
}

// View renders the detail screen.
func (m detailModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Secret Details"))
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(errorStyle.Render(m.err))
		b.WriteString("\n\n")
	}

	for _, f := range m.fields {
		b.WriteString(focusedStyle.Render(f.label + ":"))
		b.WriteString("  ")
		b.WriteString(f.value)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("esc/q: back to list"))

	return b.String()
}
