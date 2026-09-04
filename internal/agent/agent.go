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
	Store            *config.Store
	Clipboard        clipboard.Backend
	Tailnet          Tailnet
	Shortcuts        webui.ShortcutAssets
	Logger           *slog.Logger
	DashboardAddress string
}

type Agent struct {
	store     *config.Store
	clipboard clipboard.Backend
	tailnet   Tailnet
	shortcuts webui.ShortcutAssets
	logger    *slog.Logger
	pairing   *pairing.Manager
	dashboard *webui.DashboardHost
	handler   http.Handler
	stop      chan struct{}
	stopOnce  sync.Once
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
		dashboard: webui.NewDashboardHost(dashboardAddress),
		stop:      make(chan struct{}),
	}

	mux := http.NewServeMux()
	mux.Handle("/setup/", webui.NewPublicHandler(manager, options.Shortcuts))
	mux.HandleFunc("/local/open-setup", agent.openSetup)
	mux.HandleFunc("/local/shutdown", agent.shutdown)
	mux.Handle("/", api.New(api.Options{
		Config: options.Store, Clipboard: agent.clipboard, Logger: logger,
	}).Handler())
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
	status, err := a.tailnet.Status(ctx)
	if err != nil {
		return webui.DashboardData{}, err
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
	}, nil
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
