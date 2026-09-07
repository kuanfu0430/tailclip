package simple

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/kuanfu0430/tailclip/internal/api"
	"github.com/kuanfu0430/tailclip/internal/clipboard"
	"github.com/kuanfu0430/tailclip/internal/config"
)

type Pairing struct {
	Version   int       `json:"version"`
	Transport string    `json:"transport"`
	Address   string    `json:"address"`
	Ticket    string    `json:"ticket"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Service struct {
	mu               sync.Mutex
	path, deviceName string
	state            savedState
	ticket           string
	expires          time.Time
	active           bool
	now              func() time.Time
	api              http.Handler
	lifecycle        sync.Mutex
	runtime          *transport
}

func Open(path, deviceName string, backend clipboard.Backend) (*Service, error) {
	state, err := readState(path)
	if err != nil {
		return nil, err
	}
	if err = saveState(path, state); err != nil {
		return nil, err
	}
	s := &Service{path: path, deviceName: deviceName, state: state, now: time.Now}
	s.api = api.New(api.Options{Config: s, Clipboard: backend}).Handler()
	return s, nil
}

func (s *Service) Snapshot() config.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return config.Config{Version: 1, DeviceName: s.deviceName, PairingToken: s.state.Credential}
}

func (s *Service) Paired() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.state.Paired }
func (s *Service) PairingPending() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ticket != "" && s.now().Before(s.expires)
}
func (s *Service) SetActive(active bool) { s.mu.Lock(); s.active = active; s.mu.Unlock() }

func (s *Service) NewPairing() (Pairing, error) {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()
	if s.runtime == nil {
		return Pairing{}, errors.New("簡易連線尚未啟動，請按重新連接")
	}
	ticket, err := config.GenerateToken()
	if err != nil {
		return Pairing{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ticket, s.expires = ticket, s.now().Add(5*time.Minute)
	return Pairing{1, "tailcat", string(s.runtime.server.TailcatAddr()), ticket, s.expires}, nil
}

func (s *Service) Revoke() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.state
	token, err := config.GenerateToken()
	if err != nil {
		return err
	}
	next.Credential, next.Paired = token, false
	if err = saveState(s.path, next); err != nil {
		return err
	}
	s.state, s.ticket, s.expires = next, "", time.Time{}
	return nil
}

func simpleJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
func simpleError(w http.ResponseWriter, code int, message string) {
	simpleJSON(w, code, map[string]any{"ok": false, "error": map[string]string{"code": "simple_unavailable", "message": message}})
}

// ServeHTTP 僅掛在 Tailcat 私有通道，不轉發 Agent 的本機管理路由。
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	active := s.active
	s.mu.Unlock()
	if !active {
		simpleError(w, 503, "目前未啟用簡易連線")
		return
	}
	if len(r.Header.Values("Origin")) != 0 || len(r.Header.Values("Sec-Fetch-Site")) != 0 {
		simpleError(w, 403, "請使用 TailClip 客戶端")
		return
	}
	if r.URL.Path == "/v1/pair" {
		s.pair(w, r)
		return
	}
	if r.URL.Path != "/v1/health" && r.URL.Path != "/v1/status" && r.URL.Path != "/v1/clipboard/text" {
		simpleError(w, 404, "不支援此操作")
		return
	}
	if r.URL.Path != "/v1/health" && !s.Paired() {
		simpleError(w, 401, "手機尚未配對或已解除連接")
		return
	}
	s.api.ServeHTTP(w, r)
}

func (s *Service) pair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		simpleError(w, 405, "此操作只接受 POST")
		return
	}
	var input struct {
		Version int    `json:"version"`
		Ticket  string `json:"ticket"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		simpleError(w, 400, "配對資料無效")
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || input.Version != 1 {
		simpleError(w, 400, "配對版本不相容")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ticket == "" || !s.now().Before(s.expires) || len(input.Ticket) != len(s.ticket) || subtle.ConstantTimeCompare([]byte(input.Ticket), []byte(s.ticket)) != 1 {
		simpleError(w, 401, "配對已失效，請在電腦重新顯示 QR")
		return
	}
	token, err := config.GenerateToken()
	if err != nil {
		simpleError(w, 500, "無法建立配對，請再試一次")
		return
	}
	next := s.state
	next.Credential, next.Paired = token, true
	if err = saveState(s.path, next); err != nil {
		simpleError(w, 500, "無法保存配對，原配對未變更")
		return
	}
	s.state, s.ticket, s.expires = next, "", time.Time{}
	simpleJSON(w, 200, map[string]any{"ok": true, "version": 1, "token": token, "device_name": s.deviceName})
}

func (s *Service) Start(ctx context.Context) error {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()
	if s.runtime != nil {
		return nil
	}
	s.mu.Lock()
	identity := *s.state.Identity
	s.mu.Unlock()
	runtime, err := startTransport(ctx, &identity, s)
	if err != nil {
		return errors.New("簡易連線暫時無法啟動，請檢查網路後重新連接")
	}
	s.mu.Lock()
	next := s.state
	next.Identity = &identity
	err = saveState(s.path, next)
	if err == nil {
		s.state = next
	}
	s.mu.Unlock()
	if err != nil {
		runtime.Close()
		return errors.New("無法保存簡易連線身分")
	}
	s.runtime = runtime
	return nil
}

func (s *Service) Close() {
	s.SetActive(false)
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()
	if s.runtime != nil {
		s.runtime.Close()
		s.runtime = nil
	}
	s.mu.Lock()
	s.ticket, s.expires = "", time.Time{}
	s.mu.Unlock()
}
