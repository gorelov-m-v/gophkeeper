package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/user/gophkeeper/internal/client/grpcclient"
	"github.com/user/gophkeeper/internal/client/session"
	"github.com/user/gophkeeper/internal/client/store"
	"github.com/user/gophkeeper/pkg/gen"
)

type secretsLoadedMsg struct {
	secrets []store.SecretRecord
	err     error
}

type syncResultMsg struct {
	updated int
	deleted int
	err     error
}

type deleteResultMsg struct {
	err error
}

type listModel struct {
	client         grpcclient.GophKeeperClient
	cache          *store.FileStore
	session        *session.Session
	table          table.Model
	secrets        []store.SecretRecord
	wantAdd        bool
	wantDetail     bool
	selectedSecret *store.SecretRecord
	confirmDelete  bool
	err            string
	statusMsg      string
}

func newListModel(client grpcclient.GophKeeperClient, cache *store.FileStore, sess *session.Session) listModel {
	columns := []table.Column{
		{Title: "Name", Width: 25},
		{Title: "Type", Width: 10},
		{Title: "Updated", Width: 20},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")).BorderBottom(true).Bold(true)
	s.Selected = s.Selected.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Bold(false)
	t.SetStyles(s)

	return listModel{
		client:  client,
		cache:   cache,
		session: sess,
		table:   t,
	}
}

func (m listModel) loadSecrets() tea.Msg {
	userID, err := currentUserID(m.session)
	if err != nil {
		return secretsLoadedMsg{err: err}
	}

	secrets, err := m.cache.List(userID)
	if err == store.ErrCacheNotInitialized {
		return secretsLoadedMsg{secrets: []store.SecretRecord{}}
	}

	return secretsLoadedMsg{secrets: secrets, err: err}
}

func (m listModel) syncSecrets() tea.Msg {
	userID, err := currentUserID(m.session)
	if err != nil {
		return syncResultMsg{err: err}
	}

	since, err := m.cache.LoadCursor(userID)
	if err != nil {
		return syncResultMsg{err: fmt.Errorf("read sync cursor: %w", err)}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	secrets, serverTime, err := m.client.SyncSecrets(ctx, since)
	if err != nil {
		return syncResultMsg{err: fmt.Errorf("sync failed: %w", err)}
	}

	updated := 0
	deleted := 0
	for _, secret := range secrets {
		if secret.GetDeleted() {
			deleted++
			if err := m.cache.Delete(userID, secret.GetId()); err != nil && err != store.ErrCacheNotInitialized {
				return syncResultMsg{err: fmt.Errorf("delete local cache entry: %w", err)}
			}
			continue
		}

		updated++
		if err := m.cache.Upsert(userID, secretRecordFromProto(secret)); err != nil {
			return syncResultMsg{err: fmt.Errorf("update local cache: %w", err)}
		}
	}

	if err := m.cache.SaveCursor(userID, serverTime); err != nil {
		return syncResultMsg{err: fmt.Errorf("save sync cursor: %w", err)}
	}

	return syncResultMsg{updated: updated, deleted: deleted}
}

// Init returns the command that loads secrets from the server.
func (m listModel) Init() tea.Cmd {
	return m.loadSecrets
}

func kindString(k gen.DataKind) string {
	switch k {
	case gen.DataKind_DATA_KIND_LOGIN:
		return "login"
	case gen.DataKind_DATA_KIND_TEXT:
		return "text"
	case gen.DataKind_DATA_KIND_BINARY:
		return "binary"
	case gen.DataKind_DATA_KIND_CARD:
		return "card"
	default:
		return "unknown"
	}
}

func (m *listModel) refreshRows() {
	rows := make([]table.Row, 0, len(m.secrets))
	for _, s := range m.secrets {
		updated := s.UpdatedAt.Format(time.DateTime)
		if s.UpdatedAt.IsZero() {
			updated = ""
		}
		rows = append(rows, table.Row{
			s.Name,
			kindString(gen.DataKind(s.Kind)),
			updated,
		})
	}
	m.table.SetRows(rows)
}

// Update handles messages for the list screen.
func (m listModel) Update(msg tea.Msg) (listModel, tea.Cmd) {
	switch msg := msg.(type) {
	case secretsLoadedMsg:
		if msg.err != nil {
			m.err = fmt.Sprintf("failed to load local cache: %v", msg.err)
			return m, nil
		}
		m.secrets = msg.secrets
		m.err = ""
		m.refreshRows()
		return m, nil

	case syncResultMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}

		m.err = ""
		if msg.updated == 0 && msg.deleted == 0 {
			m.statusMsg = "Already up to date"
		} else {
			m.statusMsg = fmt.Sprintf("Sync complete: %d updated, %d deleted", msg.updated, msg.deleted)
		}
		return m, m.loadSecrets

	case deleteResultMsg:
		if msg.err != nil {
			m.err = fmt.Sprintf("delete failed: %v", msg.err)
			return m, nil
		}
		m.statusMsg = "Secret deleted"
		return m, m.loadSecrets

	case tea.KeyMsg:
		m.statusMsg = ""
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "n":
			m.confirmDelete = false
			m.wantAdd = true
			return m, nil
		case "enter":
			m.confirmDelete = false
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.secrets) {
				secret := m.secrets[idx]
				m.selectedSecret = &secret
				m.wantDetail = true
			}
			return m, nil
		case "d":
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.secrets) {
				if !m.confirmDelete {
					m.confirmDelete = true
					m.statusMsg = "Press d again to confirm delete"
					return m, nil
				}
				m.confirmDelete = false
				secret := m.secrets[idx]
				userID, err := currentUserID(m.session)
				if err != nil {
					m.err = err.Error()
					return m, nil
				}
				return m, func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()

					err := m.client.DeleteSecret(ctx, secret.ID, secret.Version)
					if err != nil {
						return deleteResultMsg{err: err}
					}
					if err := m.cache.Delete(userID, secret.ID); err != nil {
						return deleteResultMsg{err: err}
					}
					return deleteResultMsg{err: err}
				}
			}
			return m, nil
		case "s":
			m.confirmDelete = false
			return m, m.syncSecrets
		default:
			m.confirmDelete = false
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the list screen.
func (m listModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Your Secrets"))
	b.WriteString("\n")

	if m.err != "" {
		b.WriteString(errorStyle.Render(m.err))
		b.WriteString("\n")
	}

	if m.statusMsg != "" {
		b.WriteString(successStyle.Render(m.statusMsg))
		b.WriteString("\n")
	}

	b.WriteString(m.table.View())
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("n: new | enter: view | d: delete (confirm) | s: sync | q: quit"))

	return b.String()
}
