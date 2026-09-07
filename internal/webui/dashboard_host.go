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
	data           DashboardData
	provider       DashboardProvider
	operationError string
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
			WriteTimeout:      35 * time.Second,
			IdleTimeout:       30 * time.Second,
			MaxHeaderBytes:    16 << 10,
		}
		go func(server *http.Server, active net.Listener) {
			_ = server.Serve(active)
		}(h.server, listener)
	}

	data.RotatePath = "/dashboard/" + id + "/rotate"
	data.ActionPath = "/dashboard/" + id + "/action"
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
		if r.Method == http.MethodGet {
			current := entry
			current.operationError = ""
			h.entries[parts[1]] = current
		}
	}
	h.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}

	if len(parts) == 2 && r.Method == http.MethodGet {
		// 重新整理可看見已配對、已撤銷及已到期的 QR，不沿用開頁時的快照。
		if entry.data.Configure != nil {
			data, err := entry.provider(r.Context(), false)
			if err != nil {
				http.Error(w, "無法更新連線狀態。", 503)
				return
			}
			data.ActionPath, data.RotatePath = entry.data.ActionPath, entry.data.RotatePath
			if entry.operationError != "" {
				data.ConnectionMessage = strings.TrimSpace(entry.operationError + " " + data.ConnectionMessage)
			}
			entry.data = data
		}
		if err := RenderDashboard(w, entry.data); err != nil {
			http.Error(w, "無法顯示設定頁。", http.StatusInternalServerError)
		}
		return
	}
	if r.Method == http.MethodPost && (len(r.Header.Values("Origin")) > 1 || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+r.Host)) {
		http.Error(w, "不允許跨網站變更設定。", http.StatusForbidden)
		return
	}
	if len(parts) == 3 && parts[2] == "action" && r.Method == http.MethodPost && entry.data.Configure != nil {
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "設定資料無效。", 400)
			return
		}
		action := r.PostForm.Get("action")
		if action == "tailscale" {
			// 本機原生確認／UAC 由使用者決定閱讀時間；各 CLI／HTTP 呼叫仍各有網路逾時。
			_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
		}
		err := entry.data.Configure(r.Context(), action)
		data, dataErr := entry.provider(r.Context(), false)
		if dataErr != nil {
			http.Error(w, "無法更新連線狀態。", 503)
			return
		}
		operationError := ""
		if err != nil {
			operationError = err.Error()
		}
		data.RotatePath = "/dashboard/" + parts[1] + "/rotate"
		data.ActionPath = "/dashboard/" + parts[1] + "/action"
		h.mu.Lock()
		h.entries[parts[1]] = dashboardEntry{data: data, provider: entry.provider, operationError: operationError}
		h.mu.Unlock()
		http.Redirect(w, r, "/dashboard/"+parts[1], http.StatusSeeOther)
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
	data.ActionPath = "/dashboard/" + newID + "/action"

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
