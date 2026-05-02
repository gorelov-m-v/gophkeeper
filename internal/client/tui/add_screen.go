package tui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/user/gophkeeper/internal/client/common"
	"github.com/user/gophkeeper/internal/client/crypto"
	"github.com/user/gophkeeper/internal/client/grpcclient"
	"github.com/user/gophkeeper/internal/client/model"
	"github.com/user/gophkeeper/internal/client/session"
	"github.com/user/gophkeeper/internal/client/store"
	"github.com/user/gophkeeper/pkg/gen"
)

var secretTypes = []string{"login", "text", "binary", "card"}

type addModel struct {
	typeIndex int
	inputs    []textinput.Model
	labels    []string
	focused   int
	done      bool
	cancelled bool
	err       string
}

func newAddModel() addModel {
	m := addModel{}
	m.buildInputs()
	return m
}

func (m *addModel) buildInputs() {
	m.focused = 0
	switch secretTypes[m.typeIndex] {
	case "login":
		m.labels = []string{"Name", "Metadata", "Username", "Password", "URL"}
		m.inputs = makeInputs(5)
		m.inputs[3].EchoMode = textinput.EchoPassword
	case "text":
		m.labels = []string{"Name", "Metadata", "Content"}
		m.inputs = makeInputs(3)
	case "binary":
		m.labels = []string{"Name", "Metadata", "File path"}
		m.inputs = makeInputs(3)
	case "card":
		m.labels = []string{"Name", "Metadata", "Card Number", "Holder", "Exp Month", "Exp Year", "CVV"}
		m.inputs = makeInputs(7)
		m.inputs[6].EchoMode = textinput.EchoPassword
	}
	if len(m.inputs) > 0 {
		m.inputs[0].Focus()
	}
}

func makeInputs(n int) []textinput.Model {
	inputs := make([]textinput.Model, n)
	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].CharLimit = 256
		inputs[i].Width = 40
	}
	return inputs
}

// Init returns the initial command for the add screen.
func (m addModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages for the add screen.
func (m addModel) Update(msg tea.Msg) (addModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		m.err = ""
		switch keyMsg.String() {
		case "esc":
			m.cancelled = true
			return m, nil
		case "ctrl+t":
			m.typeIndex = (m.typeIndex + 1) % len(secretTypes)
			m.buildInputs()
			return m, nil
		case "tab", "shift+tab":
			if keyMsg.String() == "tab" {
				m.focused = (m.focused + 1) % len(m.inputs)
			} else {
				m.focused = (m.focused - 1 + len(m.inputs)) % len(m.inputs)
			}
			for i := range m.inputs {
				if i == m.focused {
					m.inputs[i].Focus()
				} else {
					m.inputs[i].Blur()
				}
			}
			return m, nil
		case "enter":
			if m.inputs[0].Value() == "" {
				m.err = "name is required"
				return m, nil
			}
			m.done = true
			return m, nil
		}
	}

	if m.focused >= 0 && m.focused < len(m.inputs) {
		var cmd tea.Cmd
		m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *addModel) submit(client grpcclient.GophKeeperClient, cache store.SecretStore, sess *session.Session, enc *crypto.Encryptor) error {
	userID, err := common.CurrentUserID(sess, tuiLoginHint)
	if err != nil {
		return err
	}

	name := m.inputs[0].Value()
	meta := m.inputs[1].Value()
	typeName := secretTypes[m.typeIndex]

	var body any
	var kind gen.DataKind

	switch typeName {
	case "login":
		kind = gen.DataKind_DATA_KIND_LOGIN
		body = model.LoginData{
			Username: m.inputs[2].Value(),
			Password: m.inputs[3].Value(),
			URL:      m.inputs[4].Value(),
		}
	case "text":
		kind = gen.DataKind_DATA_KIND_TEXT
		body = model.TextData{
			Content: m.inputs[2].Value(),
		}
	case "binary":
		kind = gen.DataKind_DATA_KIND_BINARY
		filePath := m.inputs[2].Value()
		content, readErr := os.ReadFile(filePath)
		if readErr != nil {
			return fmt.Errorf("failed to read file: %w", readErr)
		}
		body = model.BinaryData{
			Filename: filePath,
			Content:  content,
		}
	case "card":
		kind = gen.DataKind_DATA_KIND_CARD
		expMonth, _ := strconv.Atoi(m.inputs[4].Value())
		expYear, _ := strconv.Atoi(m.inputs[5].Value())
		body = model.CardData{
			Number:   m.inputs[2].Value(),
			Holder:   m.inputs[3].Value(),
			ExpMonth: expMonth,
			ExpYear:  expYear,
			CVV:      m.inputs[6].Value(),
		}
	default:
		return fmt.Errorf("unknown type: %s", typeName)
	}

	encrypted, err := common.EncodeEnvelope(meta, body, sess, enc, tuiLoginHint)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	secret, err := client.CreateSecret(ctx, name, kind, encrypted)
	if err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}

	if err := cache.Upsert(userID, common.SecretRecordFromProto(secret)); err != nil {
		return fmt.Errorf("failed to update local cache: %w", err)
	}

	return nil
}

// View renders the add screen.
func (m addModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("Add Secret (%s)", secretTypes[m.typeIndex])))
	b.WriteString("\n\n")

	for i, input := range m.inputs {
		style := blurredStyle
		if i == m.focused {
			style = focusedStyle
		}
		b.WriteString(style.Render(m.labels[i] + ":"))
		b.WriteString("\n")
		b.WriteString(input.View())
		b.WriteString("\n\n")
	}

	if m.err != "" {
		b.WriteString(errorStyle.Render(m.err))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("tab: next field | ctrl+t: change type | enter: save | esc: cancel"))

	return b.String()
}
