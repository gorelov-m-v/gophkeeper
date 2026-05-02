package tui

import (
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/user/gophkeeper/internal/client/crypto"
	"github.com/user/gophkeeper/internal/client/grpcclient"
	"github.com/user/gophkeeper/internal/client/session"
	"github.com/user/gophkeeper/internal/client/store"
)

type screen int

const (
	screenLogin screen = iota
	screenMasterPassword
	screenList
	screenAdd
	screenDetail
)

// AppModel is the top-level bubbletea model that manages screen navigation.
type AppModel struct {
	client        grpcclient.GophKeeperClient
	cache         *store.FileStore
	session       *session.Session
	enc           *crypto.Encryptor
	configDir     string
	currentScreen screen
	loginModel    loginModel
	masterPwModel masterPwModel
	listModel     listModel
	addModel      addModel
	detailModel   detailModel
	err           error
}

// NewApp creates a new AppModel initialized at the login screen.
func NewApp(client grpcclient.GophKeeperClient, cache *store.FileStore, sess *session.Session, enc *crypto.Encryptor, configDir string) AppModel {
	model := AppModel{
		client:     client,
		cache:      cache,
		session:    sess,
		enc:        enc,
		configDir:  configDir,
		loginModel: newLoginModel(),
	}

	if sess.AccessToken() != "" && sess.UserEmail() != "" {
		model.currentScreen = screenMasterPassword
		model.masterPwModel = newMasterPwModel(sess.UserEmail())
		return model
	}

	model.currentScreen = screenLogin
	return model
}

// Init implements tea.Model.
func (m AppModel) Init() tea.Cmd {
	switch m.currentScreen {
	case screenMasterPassword:
		return m.masterPwModel.Init()
	case screenList:
		return m.listModel.Init()
	case screenAdd:
		return m.addModel.Init()
	case screenDetail:
		return m.detailModel.Init()
	default:
		return m.loginModel.Init()
	}
}

// Update implements tea.Model.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	switch m.currentScreen {
	case screenLogin:
		updated, cmd := m.loginModel.Update(msg)
		m.loginModel = updated
		if m.loginModel.done {
			m.loginModel.done = false
			err := m.loginModel.submit(m.client)
			if err != nil {
				m.loginModel.err = err.Error()
				return m, nil
			}
			m.session.SetUserEmail(m.loginModel.Email())
			m.masterPwModel = newMasterPwModel(m.loginModel.Email())
			m.currentScreen = screenMasterPassword
			return m, m.masterPwModel.Init()
		}
		return m, cmd

	case screenMasterPassword:
		updated, cmd := m.masterPwModel.Update(msg)
		m.masterPwModel = updated
		if m.masterPwModel.done {
			m.masterPwModel.done = false
			err := m.masterPwModel.submit(m.session, m.enc)
			if err != nil {
				m.masterPwModel.err = err.Error()
				return m, nil
			}
			if err := m.session.SaveToDisk(filepath.Join(m.configDir, "session.json")); err != nil {
				m.masterPwModel.err = err.Error()
				return m, nil
			}
			m.listModel = newListModel(m.client, m.cache, m.session)
			m.currentScreen = screenList
			return m, m.listModel.Init()
		}
		return m, cmd

	case screenList:
		updated, cmd := m.listModel.Update(msg)
		m.listModel = updated
		if m.listModel.wantAdd {
			m.listModel.wantAdd = false
			m.addModel = newAddModel()
			m.currentScreen = screenAdd
			return m, m.addModel.Init()
		}
		if m.listModel.wantDetail {
			m.listModel.wantDetail = false
			if m.listModel.selectedSecret != nil {
				m.detailModel = newDetailModel(*m.listModel.selectedSecret, m.session, m.enc)
				m.currentScreen = screenDetail
				return m, m.detailModel.Init()
			}
		}
		return m, cmd

	case screenAdd:
		updated, cmd := m.addModel.Update(msg)
		m.addModel = updated
		if m.addModel.done {
			m.addModel.done = false
			err := m.addModel.submit(m.client, m.cache, m.session, m.enc)
			if err != nil {
				m.addModel.err = err.Error()
				return m, nil
			}
			m.listModel = newListModel(m.client, m.cache, m.session)
			m.currentScreen = screenList
			return m, m.listModel.Init()
		}
		if m.addModel.cancelled {
			m.addModel.cancelled = false
			m.listModel = newListModel(m.client, m.cache, m.session)
			m.currentScreen = screenList
			return m, m.listModel.Init()
		}
		return m, cmd

	case screenDetail:
		updated, cmd := m.detailModel.Update(msg)
		m.detailModel = updated
		if m.detailModel.back {
			m.detailModel.back = false
			m.listModel = newListModel(m.client, m.cache, m.session)
			m.currentScreen = screenList
			return m, m.listModel.Init()
		}
		return m, cmd
	}

	return m, nil
}

// View implements tea.Model.
func (m AppModel) View() string {
	switch m.currentScreen {
	case screenLogin:
		return m.loginModel.View()
	case screenMasterPassword:
		return m.masterPwModel.View()
	case screenList:
		return m.listModel.View()
	case screenAdd:
		return m.addModel.View()
	case screenDetail:
		return m.detailModel.View()
	default:
		return "Unknown screen"
	}
}
