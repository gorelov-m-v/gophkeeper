package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/user/gophkeeper/internal/client/grpcclient"
)

type loginModel struct {
	emailInput    textinput.Model
	passwordInput textinput.Model
	focused       int
	isRegister    bool
	done          bool
	err           string
}

func newLoginModel() loginModel {
	email := textinput.New()
	email.Placeholder = "email@example.com"
	email.Focus()
	email.CharLimit = 128
	email.Width = 40

	password := textinput.New()
	password.Placeholder = "password"
	password.EchoMode = textinput.EchoPassword
	password.CharLimit = 128
	password.Width = 40

	return loginModel{
		emailInput:    email,
		passwordInput: password,
	}
}

// Init returns the initial command for the login screen.
func (m loginModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages for the login screen.
func (m loginModel) Update(msg tea.Msg) (loginModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		m.err = ""
		switch keyMsg.String() {
		case "tab", "shift+tab":
			if m.focused == 0 {
				m.focused = 1
				m.emailInput.Blur()
				m.passwordInput.Focus()
			} else {
				m.focused = 0
				m.passwordInput.Blur()
				m.emailInput.Focus()
			}
			return m, nil
		case "ctrl+r":
			m.isRegister = !m.isRegister
			return m, nil
		case "enter":
			if m.emailInput.Value() == "" || m.passwordInput.Value() == "" {
				m.err = "email and password are required"
				return m, nil
			}
			m.done = true
			return m, nil
		}
	}

	var cmd tea.Cmd
	if m.focused == 0 {
		m.emailInput, cmd = m.emailInput.Update(msg)
	} else {
		m.passwordInput, cmd = m.passwordInput.Update(msg)
	}
	return m, cmd
}

// Email returns the email entered by the user.
func (m *loginModel) Email() string {
	return m.emailInput.Value()
}

func (m *loginModel) submit(client grpcclient.GophKeeperClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	email := m.emailInput.Value()
	password := m.passwordInput.Value()

	if m.isRegister {
		if err := client.Register(ctx, email, password); err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}
	} else {
		if err := client.Login(ctx, email, password); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
	}
	return nil
}

// View renders the login screen.
func (m loginModel) View() string {
	var b strings.Builder

	mode := "Login"
	if m.isRegister {
		mode = "Register"
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("GophKeeper - %s", mode)))
	b.WriteString("\n\n")

	b.WriteString(focusedStyle.Render("Email:"))
	b.WriteString("\n")
	b.WriteString(m.emailInput.View())
	b.WriteString("\n\n")

	b.WriteString(focusedStyle.Render("Password:"))
	b.WriteString("\n")
	b.WriteString(m.passwordInput.View())
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(errorStyle.Render(m.err))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("tab: switch field | ctrl+r: toggle register/login | enter: submit | ctrl+c: quit"))

	return b.String()
}
