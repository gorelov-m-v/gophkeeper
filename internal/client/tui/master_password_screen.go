package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/user/gophkeeper/internal/client/crypto"
	"github.com/user/gophkeeper/internal/client/session"
)

type masterPwModel struct {
	input textinput.Model
	email string
	done  bool
	err   string
}

func newMasterPwModel(email string) masterPwModel {
	ti := textinput.New()
	ti.Placeholder = "master password"
	ti.EchoMode = textinput.EchoPassword
	ti.Focus()
	ti.CharLimit = 128
	ti.Width = 40

	return masterPwModel{input: ti, email: email}
}

// Init returns the initial command for the master password screen.
func (m masterPwModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages for the master password screen.
func (m masterPwModel) Update(msg tea.Msg) (masterPwModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		m.err = ""
		if keyMsg.String() == "enter" {
			if m.input.Value() == "" {
				m.err = "master password is required"
				return m, nil
			}
			m.done = true
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *masterPwModel) submit(sess *session.Session, enc *crypto.Encryptor) error {
	salt := enc.DeriveUserSalt(m.email)
	key := enc.DeriveKey(m.input.Value(), salt)
	sess.SetSalt(salt)
	sess.SetEncryptionKey(key)
	return nil
}

// View renders the master password screen.
func (m masterPwModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Enter Master Password"))
	b.WriteString("\n\n")
	b.WriteString(focusedStyle.Render("Master Password:"))
	b.WriteString("\n")
	b.WriteString(m.input.View())
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(errorStyle.Render(m.err))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("enter: submit | ctrl+c: quit"))

	return b.String()
}
