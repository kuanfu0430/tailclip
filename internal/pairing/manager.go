package pairing

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"
)

const SessionLifetime = 15 * time.Minute

var ErrInvalidBaseURL = errors.New("配對網址必須是 HTTPS 的 ts.net 網址")

type Data struct {
	Version         int    `json:"version"`
	BaseURL         string `json:"base_url"`
	Token           string `json:"token"`
	DeviceName      string `json:"device_name"`
	TailscaleDevice string `json:"tailscale_device"`
}

type Session struct {
	Nonce     string
	ExpiresAt time.Time
	Data      Data
}

type Manager struct {
	mu       sync.Mutex
	sessions map[string]Session
	now      func() time.Time
}

func NewManager() *Manager {
	return &Manager{sessions: make(map[string]Session), now: time.Now}
}

func (m *Manager) Create(data Data) (Session, error) {
	if err := ValidateData(data); err != nil {
		return Session{}, err
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return Session{}, err
	}
	now := m.now()
	session := Session{
		Nonce:     base64.RawURLEncoding.EncodeToString(raw),
		ExpiresAt: now.Add(SessionLifetime),
		Data:      data,
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for nonce, existing := range m.sessions {
		if !existing.ExpiresAt.After(now) {
			delete(m.sessions, nonce)
		}
	}
	m.sessions[session.Nonce] = session
	return session, nil
}

func (m *Manager) Get(nonce string) (Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[nonce]
	if !ok {
		return Session{}, false
	}
	if !session.ExpiresAt.After(m.now()) {
		delete(m.sessions, nonce)
		return Session{}, false
	}
	return session, true
}

func (d Data) JSON() ([]byte, error) {
	return json.Marshal(d)
}

func ValidateData(data Data) error {
	if data.Version != 1 || data.Token == "" || strings.TrimSpace(data.DeviceName) == "" {
		return errors.New("配對資料不完整")
	}
	u, err := url.Parse(data.BaseURL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return ErrInvalidBaseURL
	}
	host := strings.ToLower(u.Hostname())
	if !strings.HasSuffix(host, ".ts.net") || u.Path != "/tailclip/v1" {
		return ErrInvalidBaseURL
	}
	return nil
}
