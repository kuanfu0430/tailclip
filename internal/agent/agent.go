package agent

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kuanfu0430/tailclip/internal/api"
	"github.com/kuanfu0430/tailclip/internal/clipboard"
	"github.com/kuanfu0430/tailclip/internal/config"
	"github.com/kuanfu0430/tailclip/internal/pairing"
	"github.com/kuanfu0430/tailclip/internal/simple"
	"github.com/kuanfu0430/tailclip/internal/tailscale"
	"github.com/kuanfu0430/tailclip/internal/webui"
)

const (
	ListenAddress    = "127.0.0.1:17733"
	DashboardAddress = "127.0.0.1:17734"
)

type Tailnet interface {
	Status(context.Context) (tailscale.Status, error)
}

type Options struct {
	PrepareTailnet   func(context.Context) error
	Store            *config.Store
	Clipboard        clipboard.Backend
	Tailnet          Tailnet
	Shortcuts        webui.ShortcutAssets
	Logger           *slog.Logger
	DashboardAddress string
}

type Agent struct {
	store          *config.Store
	clipboard      clipboard.Backend
	tailnet        Tailnet
	shortcuts      webui.ShortcutAssets
	logger         *slog.Logger
	pairing        *pairing.Manager
	dashboard      *webui.DashboardHost
	handler        http.Handler
	stop           chan struct{}
	stopOnce       sync.Once
	modeMu         sync.Mutex
	simple         *simple.Service
	simplePairing  simple.Pairing
	prepareTailnet func(context.Context) error
}

func New(options Options) (*Agent, error) {
	if options.Store == nil || options.Clipboard == nil || options.Tailnet == nil {
		return nil, errors.New("Agent 缺少必要元件")
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	dashboardAddress := options.DashboardAddress
	if dashboardAddress == "" {
		dashboardAddress = DashboardAddress
	}
	manager := pairing.NewManager()
	agent := &Agent{
		store: options.Store, clipboard: clipboard.NewSynchronized(options.Clipboard), tailnet: options.Tailnet,
		shortcuts: options.Shortcuts, logger: logger, pairing: manager,
		dashboard:      webui.NewDashboardHost(dashboardAddress),
		stop:           make(chan struct{}),
		prepareTailnet: options.PrepareTailnet,
	}

	mux := http.NewServeMux()
	publicSetup := webui.NewPublicHandler(manager, options.Shortcuts)
	mux.Handle("/setup/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if agent.store.Snapshot().Mode() != "tailscale" {
			http.NotFound(w, r)
			return
		}
		publicSetup.ServeHTTP(w, r)
	}))
	mux.HandleFunc("/local/open-setup", agent.openSetup)
	mux.HandleFunc("/local/shutdown", agent.shutdown)
	apiHandler := api.New(api.Options{
		Config: options.Store, Clipboard: agent.clipboard, Logger: logger,
		Owner: func(ctx context.Context) (string, string, error) {
			status, err := agent.tailnet.Status(ctx)
			if err != nil {
				return "", "", err
			}
			login, err := status.OwnerLogin()
			return login, status.Self.DNSName, err
		},
	}).Handler()
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" && agent.store.Snapshot().Mode() != "tailscale" {
			writeControlError(w, http.StatusForbidden, "目前未啟用 Tailscale 入口。")
			return
		}
		apiHandler.ServeHTTP(w, r)
	}))
	agent.handler = mux
	return agent, nil
}

func (a *Agent) Handler() http.Handler { return a.handler }

func (a *Agent) Run(ctx context.Context, address string) error {
	if address == "" {
		address = ListenAddress
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil || host != "127.0.0.1" {
		return errors.New("Agent 只能監聽固定 loopback 位址")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("Agent 無法使用 %s；請確認 port 是否已被其他程式占用: %w", address, err)
	}
	defer func() {
		a.modeMu.Lock()
		defer a.modeMu.Unlock()
		if a.simple != nil {
			a.simple.Close()
		}
	}()
	if a.store.Snapshot().Mode() == "simple" {
		go func() {
			if err := a.configureConnection(ctx, "simple"); err != nil {
				a.logger.Warn("simple_start_failed")
			}
		}()
	}
	server := &http.Server{
		Handler:           a.handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       12 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
		case <-a.stop:
		case <-done:
			return
		}
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}()
	a.logger.Info("agent_started", "address", address)
	err = server.Serve(listener)
	close(done)
	_ = a.dashboard.Close()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (a *Agent) shutdown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodPost || !webuiRequestIsLocal(r) || !a.controlAuthorized(r) {
		writeControlError(w, http.StatusForbidden, "只允許這台電腦停止 Agent。")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	a.stopOnce.Do(func() { close(a.stop) })
}

func (a *Agent) openSetup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodPost {
		writeControlError(w, http.StatusMethodNotAllowed, "這個操作只接受 POST。")
		return
	}
	if !webuiRequestIsLocal(r) || !a.controlAuthorized(r) {
		writeControlError(w, http.StatusForbidden, "只允許這台電腦開啟設定頁。")
		return
	}
	url, err := a.dashboard.Open(r.Context(), a.dashboardData)
	if err != nil {
		a.logger.Warn("dashboard_open_failed", "error_type", classifySetupError(err))
		writeControlError(w, http.StatusServiceUnavailable, userSetupError(err))
		return
	}
	a.logger.Info("dashboard_opened")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "url": url})
}

func (a *Agent) dashboardData(ctx context.Context, rotate bool) (webui.DashboardData, error) {
	if a.store.Snapshot().Mode() != "tailscale" {
		return a.connectionData(), nil
	}
	status, err := a.tailnet.Status(ctx)
	if err != nil {
		data := a.connectionData()
		data.ConnectionMessage = userSetupError(err)
		return data, nil
	}
	tailscaleDevice := strings.TrimSpace(status.Self.HostName)
	if tailscaleDevice == "" {
		tailscaleDevice = strings.Split(status.Self.DNSName, ".")[0]
	}
	if err := a.store.Update(func(cfg *config.Config) error {
		cfg.TailscaleDevice = tailscaleDevice
		if rotate {
			return cfg.RotateToken()
		}
		return nil
	}); err != nil {
		return webui.DashboardData{}, err
	}
	cfg := a.store.Snapshot()
	session, err := a.pairing.Create(pairing.Data{
		Version: 1, BaseURL: tailscale.BaseURL(status), Token: cfg.PairingToken,
		DeviceName: cfg.DeviceName, TailscaleDevice: cfg.TailscaleDevice,
	})
	if err != nil {
		return webui.DashboardData{}, err
	}
	pairingURL := strings.TrimSuffix(tailscale.BaseURL(status), "/v1") + "/setup/" + session.Nonce
	return webui.DashboardData{
		DeviceName: cfg.DeviceName, DNSName: status.Self.DNSName, PairingURL: pairingURL,
		ExpiresAt: session.ExpiresAt, ClipboardAvailable: a.clipboard.Available(ctx) == nil,
		ShortcutsReady: a.shortcuts.Ready(),
		Mode:           "tailscale", Configure: a.configureConnection,
	}, nil
}

func (a *Agent) connectionData() webui.DashboardData {
	cfg := a.store.Snapshot()
	data := webui.DashboardData{DeviceName: cfg.DeviceName, Mode: cfg.Mode(), Configure: a.configureConnection}
	if !a.modeMu.TryLock() {
		data.ConnectionMessage = "正在啟動連線，請稍候重新開啟此頁。"
		return data
	}
	defer a.modeMu.Unlock()
	if a.simple != nil {
		data.SimplePaired = a.simple.Paired()
		if !a.simple.Ready() {
			data.ConnectionMessage = "臨時隧道尚未就緒或已結束，請按重新連接並掃描新 QR。"
		}
	}
	if a.simplePairing.Ticket != "" && a.simple != nil && a.simple.PairingPending() && time.Now().Before(a.simplePairing.ExpiresAt) {
		data.PairingURL, data.ExpiresAt = a.simplePairing.URL(), a.simplePairing.ExpiresAt
	}
	return data
}

func (a *Agent) configureConnection(ctx context.Context, action string) error {
	a.modeMu.Lock()
	defer a.modeMu.Unlock()
	switch action {
	case "tailscale":
		// 缺少 CLI 或未登入時保留原入口，絕不暗中切換。
		if _, err := a.tailnet.Status(ctx); err != nil {
			return errors.New(userSetupError(err))
		}
		if a.prepareTailnet != nil {
			if err := a.prepareTailnet(ctx); err != nil {
				return err
			}
		}
		if a.simple != nil {
			a.simple.SetActive(false)
		}
		if err := a.store.Update(func(c *config.Config) error { c.ConnectionMode = ""; return nil }); err != nil {
			if a.simple != nil && a.store.Snapshot().Mode() == "simple" {
				a.simple.SetActive(true)
			}
			return errors.New("無法保存連線方式")
		}
		if a.simple != nil {
			a.simple.Close()
		}
		a.simplePairing = simple.Pairing{}
	case "simple", "pair":
		ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
		if a.simple == nil {
			cfg := a.store.Snapshot()
			a.simple = simple.New(cfg.DeviceName, a.clipboard)
		}
		if err := a.simple.Start(ctx); err != nil {
			return err
		}
		if a.store.Snapshot().Mode() != "simple" {
			if err := a.store.Update(func(c *config.Config) error { c.ConnectionMode = "simple"; return nil }); err != nil {
				a.simple.Close()
				return err
			}
		}
		a.simple.SetActive(true)
		if action == "pair" || (!a.simple.Paired() && !a.simple.PairingPending()) {
			pairing, err := a.simple.NewPairing()
			if err != nil {
				return err
			}
			a.simplePairing = pairing
		}
	case "revoke":
		if a.simple != nil {
			if err := a.simple.Revoke(); err != nil {
				return err
			}
		}
		a.simplePairing = simple.Pairing{}
	default:
		return errors.New("不支援的連線操作")
	}
	return nil
}

func (a *Agent) controlAuthorized(r *http.Request) bool {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	provided := strings.TrimPrefix(header, prefix)
	expected := a.store.Snapshot().PairingToken
	return len(provided) == len(expected) && subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

func webuiRequestIsLocal(r *http.Request) bool {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || !net.ParseIP(remoteHost).IsLoopback() {
		return false
	}
	host := r.Host
	if parsed, _, splitErr := net.SplitHostPort(r.Host); splitErr == nil {
		host = parsed
	}
	return net.ParseIP(strings.Trim(host, "[]")).IsLoopback()
}

func writeControlError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok": false, "error": map[string]string{"code": "setup_unavailable", "message": message},
	})
}

func classifySetupError(err error) string {
	switch {
	case errors.Is(err, tailscale.ErrNotInstalled):
		return "tailscale_missing"
	case errors.Is(err, tailscale.ErrNotRunning):
		return "tailscale_offline"
	default:
		return "setup_unavailable"
	}
}

func userSetupError(err error) string {
	switch {
	case errors.Is(err, tailscale.ErrNotInstalled):
		return "找不到 Tailscale，請先安裝並登入。"
	case errors.Is(err, tailscale.ErrNotRunning):
		return "Tailscale 尚未連線，請先按下 Connect。"
	default:
		return "暫時無法開啟設定頁，請稍後再試。"
	}
}
