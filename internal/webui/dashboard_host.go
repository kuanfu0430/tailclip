package webui

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const dashboardIdleTimeout = 20 * time.Minute

type DashboardProvider func(context.Context, bool) (DashboardData, error)

type dashboardEntry struct {
	data     DashboardData
	provider DashboardProvider
}

// DashboardHost 只在使用者要求設定頁後才綁定 loopback，閒置後自動關閉。
type DashboardHost struct {
	address string

	mu       sync.Mutex
	listener net.Listener
	server   *http.Server
	timer    *time.Timer
	entries  map[string]dashboardEntry
}

func NewDashboardHost(address string) *DashboardHost {
	return &DashboardHost{address: address, entries: make(map[string]dashboardEntry)}
}

func (h *DashboardHost) Open(ctx context.Context, provider DashboardProvider) (string, error) {
	data, err := provider(ctx, false)
	if err != nil {
		return "", err
	}
	id, err := randomID()
	if err != nil {
		return "", err
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.listener == nil {
		listener, err := net.Listen("tcp", h.address)
		if err != nil {
			return "", fmt.Errorf("本機設定頁無法使用 %s: %w", h.address, err)
		}
		h.listener = listener
		h.server = &http.Server{
			Handler:           h,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       30 * time.Second,
			MaxHeaderBytes:    16 << 10,
		}
		go func(server *http.Server, active net.Listener) {
			_ = server.Serve(active)
		}(h.server, listener)
	}

	data.RotatePath = "/dashboard/" + id + "/rotate"
	h.entries[id] = dashboardEntry{data: data, provider: provider}
	h.resetTimerLocked()
	return "http://" + h.listener.Addr().String() + "/dashboard/" + id, nil
}

func (h *DashboardHost) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setPrivateHeaders(w)
	if !requestIsLoopback(r) {
		http.Error(w, "只允許從這台電腦開啟。", http.StatusForbidden)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "dashboard" {
		http.NotFound(w, r)
		return
	}

	h.mu.Lock()
	entry, ok := h.entries[parts[1]]
	if ok {
		h.resetTimerLocked()
	}
	h.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}

	if len(parts) == 2 && r.Method == http.MethodGet {
		if err := RenderDashboard(w, entry.data); err != nil {
			http.Error(w, "無法顯示設定頁。", http.StatusInternalServerError)
		}
		return
	}
	if len(parts) == 3 && parts[2] == "rotate" && r.Method == http.MethodPost {
		h.rotate(w, r, parts[1], entry)
		return
	}
	http.Error(w, "不支援的操作。", http.StatusMethodNotAllowed)
}

func (h *DashboardHost) rotate(w http.ResponseWriter, r *http.Request, oldID string, entry dashboardEntry) {
	data, err := entry.provider(r.Context(), true)
	if err != nil {
		http.Error(w, "無法撤銷舊配對，設定未變更。", http.StatusInternalServerError)
		return
	}
	newID, err := randomID()
	if err != nil {
		http.Error(w, "無法建立新的設定頁。", http.StatusInternalServerError)
		return
	}
	data.RotatePath = "/dashboard/" + newID + "/rotate"

	h.mu.Lock()
	delete(h.entries, oldID)
	h.entries[newID] = dashboardEntry{data: data, provider: entry.provider}
	h.resetTimerLocked()
	h.mu.Unlock()
	http.Redirect(w, r, "/dashboard/"+newID, http.StatusSeeOther)
}

func (h *DashboardHost) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.closeLocked()
}

func (h *DashboardHost) resetTimerLocked() {
	if h.timer != nil {
		h.timer.Stop()
	}
	h.timer = time.AfterFunc(dashboardIdleTimeout, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		_ = h.closeLocked()
	})
}

func (h *DashboardHost) closeLocked() error {
	if h.timer != nil {
		h.timer.Stop()
		h.timer = nil
	}
	if h.server == nil {
		return nil
	}
	server := h.server
	h.server = nil
	h.listener = nil
	h.entries = make(map[string]dashboardEntry)
	return server.Close()
}

func randomID() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("無法建立本機設定連結: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func requestIsLoopback(r *http.Request) bool {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || !net.ParseIP(remoteHost).IsLoopback() {
		return false
	}
	host := r.Host
	if parsedHost, _, splitErr := net.SplitHostPort(r.Host); splitErr == nil {
		host = parsedHost
	}
	return net.ParseIP(strings.Trim(host, "[]")).IsLoopback()
}
