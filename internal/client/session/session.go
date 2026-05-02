// Package session holds authentication state and encryption key for the client.
package session

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"sync"
)

// Session stores the current authentication tokens and encryption key.
type Session struct {
	accessToken   string
	refreshToken  string
	encryptionKey []byte
	salt          []byte
	userEmail     string
	userID        string
	mu            sync.RWMutex
}

type diskSession struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Salt         string `json:"salt"`
	UserEmail    string `json:"user_email"`
	UserID       string `json:"user_id"`
}

// NewSession returns a new empty Session.
func NewSession() *Session {
	return &Session{}
}

// SetTokens stores the access and refresh tokens.
func (s *Session) SetTokens(access, refresh string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accessToken = access
	s.refreshToken = refresh
}

// SetUserEmail stores the authenticated user email.
func (s *Session) SetUserEmail(email string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.userEmail = email
}

// UserEmail returns the authenticated user email.
func (s *Session) UserEmail() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.userEmail
}

// SetUserID stores the authenticated user identifier.
func (s *Session) SetUserID(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.userID = userID
}

// UserID returns the authenticated user identifier.
func (s *Session) UserID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.userID
}

// AccessToken returns the current access token.
func (s *Session) AccessToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.accessToken
}

// RefreshToken returns the current refresh token.
func (s *Session) RefreshToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.refreshToken
}

// SetEncryptionKey stores the derived encryption key.
func (s *Session) SetEncryptionKey(key []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.encryptionKey = key
}

// EncryptionKey returns the current encryption key.
func (s *Session) EncryptionKey() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.encryptionKey
}

// SetSalt stores the key derivation salt.
func (s *Session) SetSalt(salt []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.salt = salt
}

// Salt returns the current key derivation salt.
func (s *Session) Salt() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.salt
}

// SaveToDisk writes the session tokens and salt to a JSON file.
// The encryption key is never persisted.
func (s *Session) SaveToDisk(path string) error {
	s.mu.RLock()
	ds := diskSession{
		AccessToken:  s.accessToken,
		RefreshToken: s.refreshToken,
		Salt:         base64.StdEncoding.EncodeToString(s.salt),
		UserEmail:    s.userEmail,
		UserID:       s.userID,
	}
	s.mu.RUnlock()

	data, err := json.Marshal(ds)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// LoadFromDisk reads session tokens and salt from a JSON file.
// The encryption key must be derived separately after loading.
func (s *Session) LoadFromDisk(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var ds diskSession
	if err := json.Unmarshal(data, &ds); err != nil {
		return err
	}

	salt, err := base64.StdEncoding.DecodeString(ds.Salt)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.accessToken = ds.AccessToken
	s.refreshToken = ds.RefreshToken
	s.salt = salt
	s.userEmail = ds.UserEmail
	s.userID = ds.UserID

	return nil
}
