// Package simple 以獨立的 HTTP 入口提供短期 Cloudflare 配對及文字傳輸。
package simple

import (
	"bytes"

	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/kuanfu0430/tailclip/internal/api"
	"github.com/kuanfu0430/tailclip/internal/clipboard"
	"github.com/kuanfu0430/tailclip/internal/config"
	"github.com/kuanfu0430/tailclip/internal/tunnel"
)

type Pairing struct {
	Version   int       `json:"version"`
	Transport string    `json:"transport"`
	BaseURL   string    `json:"base_url"`
	Ticket    string    `json:"ticket"`
	ExpiresAt time.Time `json:"-"`
}

func (p Pairing) URL() string { return strings.TrimSuffix(p.BaseURL, "/v1") + "/setup/" + p.Ticket }

type runtime struct {
	server *http.Server
	wire   tunnel.Handle
}

func (r *runtime) Close() { _ = r.server.Close(); r.wire.Close() }

type Service struct {
	lifecycle                         sync.Mutex
	mu                                sync.Mutex
	deviceName                        string
	backend                           clipboard.Backend
	active, ready, paired             bool
	host, session, credential, ticket string
	expires                           time.Time
	attempts                          int
	attemptWindow                     time.Time
	runtime                           *runtime
	now                               func() time.Time
	start                             tunnel.Starter
	api                               http.Handler
}

type fixedConfig struct{ config.Config }

func (f fixedConfig) Snapshot() config.Config { return f.Config }
func New(device string, backend clipboard.Backend) *Service {
	return &Service{deviceName: device, backend: backend, now: time.Now, start: tunnel.Start}
}
func (s *Service) setCredentialLocked(token string) {
	s.credential = token
	s.api = api.New(api.Options{Config: fixedConfig{config.Config{Version: 1, DeviceName: s.deviceName, PairingToken: token}}, Clipboard: s.backend}).Handler()
}
func (s *Service) Snapshot() config.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return config.Config{Version: 1, DeviceName: s.deviceName, PairingToken: s.credential}
}
func (s *Service) SetActive(active bool) { s.mu.Lock(); s.active = active; s.mu.Unlock() }
func (s *Service) Paired() bool          { s.mu.Lock(); defer s.mu.Unlock(); return s.ready && s.paired }
func (s *Service) Ready() bool           { s.mu.Lock(); defer s.mu.Unlock(); return s.ready }
func (s *Service) PairingPending() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ready && s.ticket != "" && s.now().Before(s.expires)
}
func (s *Service) NewPairing() (Pairing, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready || s.runtime == nil {
		return Pairing{}, errors.New("臨時隧道尚未就緒，請按重新連接")
	}
	ticket, err := config.GenerateToken()
	if err != nil {
		return Pairing{}, err
	}
	s.ticket = ticket
	s.expires = s.now().Add(5 * time.Minute)
	return Pairing{1, "cloudflare", "https://" + s.host + "/v1", ticket, s.expires}, nil
}
func (s *Service) Revoke() error {
	token, err := config.GenerateToken()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setCredentialLocked(token)
	s.paired = false
	s.ticket = ""
	s.expires = time.Time{}
	return nil
}
func (s *Service) clearLocked() {
	s.ready = false
	s.paired = false
	s.credential = ""
	s.ticket = ""
	s.host = ""
	s.session = ""
	s.expires = time.Time{}
}
func (s *Service) Start(ctx context.Context) error {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()
	s.mu.Lock()
	if s.runtime != nil && s.ready {
		select {
		case <-s.runtime.wire.Done():
		default:
			s.mu.Unlock()
			return nil
		}
	}
	previous := s.runtime
	s.runtime = nil
	s.clearLocked()
	s.mu.Unlock()
	if previous != nil {
		previous.Close()
	}
	session, err := config.GenerateToken()
	if err != nil {
		return err
	}
	token, err := config.GenerateToken()
	if err != nil {
		return err
	}
	// 埠 0 由 OS 原子選擇空閒埠，避免探測與綁定之間的競爭。
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return errors.New("無法建立簡易連線的本機埠")
	}
	server := &http.Server{Handler: s, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 12 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	s.mu.Lock()
	s.session = session
	s.setCredentialLocked(token)
	s.mu.Unlock()
	go func() { _ = server.Serve(listener) }()
	wire, err := s.start(ctx, listener.Addr().String())
	if err != nil {
		server.Close()
		s.mu.Lock()
		s.clearLocked()
		s.mu.Unlock()
		return err
	}
	rt := &runtime{server, wire}
	parsed, err := url.Parse(wire.URL())
	if err != nil || parsed.Scheme != "https" || !validHost(parsed.Host) || parsed.Path != "" || parsed.RawQuery != "" || parsed.User != nil || parsed.Fragment != "" {
		rt.Close()
		s.mu.Lock()
		s.clearLocked()
		s.mu.Unlock()
		return errors.New("Cloudflare 回傳的臨時網址無效")
	}
	s.mu.Lock()
	s.host = parsed.Host
	s.runtime = rt
	s.mu.Unlock()
	// 外部 HTTPS 必須回到本次本機服務，拒絕重新導向及其他端點的假健康回應。
	if err = s.verify(ctx, wire.URL(), session, wire.Done()); err != nil {
		rt.Close()
		s.mu.Lock()
		if s.runtime == rt {
			s.runtime = nil
			s.clearLocked()
		}
		s.mu.Unlock()
		return err
	}
	s.mu.Lock()
	s.ready = true
	s.mu.Unlock()
	go func() {
		<-wire.Done()
		server.Close()
		s.mu.Lock()
		if s.runtime == rt {
			s.clearLocked()
		}
		s.mu.Unlock()
	}()
	return nil
}
func validHost(host string) bool {
	const suffix = ".trycloudflare.com"
	if !strings.HasSuffix(host, suffix) {
		return false
	}
	label := strings.TrimSuffix(host, suffix)
	if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for _, r := range label {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}
func (s *Service) verify(ctx context.Context, base, session string, done <-chan struct{}) error {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/health", nil)
		resp, err := client.Do(req)
		if err == nil {
			var body struct {
				Session string `json:"session"`
				Service string `json:"service"`
			}
			err = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&body)
			resp.Body.Close()
			if resp.StatusCode == 200 && err == nil && body.Service == "tailclip-simple" && body.Session == session {
				return nil
			}
		}
		timer := time.NewTimer(400 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.New("臨時網址尚無法連線，請檢查網路後重新連接")
		case <-done:
			timer.Stop()
			return errors.New("臨時隧道已結束，請重新連接")
		case <-timer.C:
		}
	}
}
func (s *Service) Close() {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()
	s.mu.Lock()
	rt := s.runtime
	s.runtime = nil
	s.active = false
	s.clearLocked()
	s.mu.Unlock()
	if rt != nil {
		rt.Close()
	}
}
func simpleJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
func simpleError(w http.ResponseWriter, code int, message string) {
	simpleJSON(w, code, map[string]any{"ok": false, "error": map[string]string{"code": "simple_unavailable", "message": message}})
}

// 先收齊受限 body，再用記憶體回應維持配對／撤銷與剪貼簿操作的原子性。
// 狀態鎖內不讀寫網路，慢速用戶不能阻塞桌面管理。
type bufferedResponse struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (b *bufferedResponse) Header() http.Header { return b.header }
func (b *bufferedResponse) WriteHeader(code int) {
	if b.status == 0 {
		b.status = code
	}
}
func (b *bufferedResponse) Write(data []byte) (int, error) {
	if b.status == 0 {
		b.status = 200
	}
	return b.body.Write(data)
}
func (s *Service) authorizedLocked(r *http.Request) bool {
	expected := "Bearer " + s.credential
	actual := r.Header.Get("Authorization")
	return s.paired && s.credential != "" && len(r.Header.Values("Authorization")) == 1 && len(actual) == len(expected) && subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	if r.Method == "POST" && (r.URL.Path == "/v1/pair" || r.URL.Path == "/v1/clipboard/text") {
		s.mu.Lock()
		allowed := s.active && s.ready && r.Host == s.host && (r.URL.Path == "/v1/pair" || s.authorizedLocked(r))
		s.mu.Unlock()
		if !allowed {
			simpleError(w, 401, "配對已失效，請重新掃描電腦 QR")
			return
		}
		limit := int64(4096)
		if r.URL.Path == "/v1/clipboard/text" {
			limit = api.MaxTextBytes*6 + 4096
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
		if err != nil {
			simpleError(w, 413, "請求過大或尚未完整傳送")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
	}
	response := &bufferedResponse{header: make(http.Header)}
	s.serveBuffered(response, r)
	for key, values := range response.header {
		w.Header()[key] = values
	}
	if response.status == 0 {
		response.status = 200
	}
	w.WriteHeader(response.status)
	_, _ = w.Write(response.body.Bytes())
}
func (s *Service) serveBuffered(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.host == "" || r.Host != s.host {
		simpleError(w, 403, "臨時網址不符")
		return
	}
	if r.URL.Path == "/v1/health" && r.Method == "GET" {
		simpleJSON(w, 200, map[string]any{"service": "tailclip-simple", "session": s.session})
		return
	}
	if !s.active || !s.ready {
		simpleError(w, 503, "臨時隧道尚未啟用或已結束")
		return
	}
	if r.Method == "GET" && (strings.HasPrefix(r.URL.Path, "/setup/") || strings.HasPrefix(r.URL.Path, "/shortcuts/")) {
		s.pageLocked(w, r)
		return
	}
	if len(r.Header.Values("Origin")) != 0 || len(r.Header.Values("Sec-Fetch-Site")) != 0 || len(r.Header.Values(api.ShortcutClientHeader)) != 0 {
		simpleError(w, 403, "請使用簡易捷徑操作")
		return
	}
	if r.URL.Path == "/v1/pair" {
		s.pairLocked(w, r)
		return
	}
	if r.URL.Path != "/v1/status" && r.URL.Path != "/v1/clipboard/text" {
		simpleError(w, 404, "不支援此操作")
		return
	}
	if !s.authorizedLocked(r) {
		simpleError(w, 401, "配對已失效，請重新掃描電腦 QR")
		return
	}
	s.api.ServeHTTP(w, r)
}
func (s *Service) pairLocked(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		simpleError(w, 405, "此操作只接受 POST")
		return
	}
	if s.now().Sub(s.attemptWindow) >= time.Minute {
		s.attemptWindow = s.now()
		s.attempts = 0
	}
	s.attempts++
	if s.attempts > 30 {
		simpleError(w, 429, "配對嘗試太頻繁，請稍後再試")
		return
	}
	var input struct {
		Version json.Number `json:"version"`
		Ticket  string      `json:"ticket"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		simpleError(w, 400, "配對資料無效")
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || input.Version.String() != "1" {
		simpleError(w, 400, "配對版本不相容")
		return
	}
	if s.ticket == "" || !s.now().Before(s.expires) || len(input.Ticket) != len(s.ticket) || subtle.ConstantTimeCompare([]byte(input.Ticket), []byte(s.ticket)) != 1 {
		simpleError(w, 401, "配對已失效，請在電腦重新顯示 QR")
		return
	}
	token, err := config.GenerateToken()
	if err != nil {
		simpleError(w, 500, "無法建立配對")
		return
	}
	s.setCredentialLocked(token)
	s.paired = true
	s.ticket = ""
	s.expires = time.Time{}
	simpleJSON(w, 200, map[string]any{"ok": true, "version": 1, "transport": "cloudflare", "base_url": "https://" + s.host + "/v1", "token": token, "device_name": s.deviceName})
}
