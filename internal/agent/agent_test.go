package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kuanfu0430/tailclip/internal/clipboard"
	"github.com/kuanfu0430/tailclip/internal/config"
	"github.com/kuanfu0430/tailclip/internal/tailscale"
	"github.com/kuanfu0430/tailclip/internal/webui"
)

type fakeTailnet struct{ status tailscale.Status }

type missingTailnet struct{}

func (missingTailnet) Status(context.Context) (tailscale.Status, error) {
	return tailscale.Status{}, tailscale.ErrNotInstalled
}

func TestModeIsolationAndLegacyPreservation(t *testing.T) {
	a, store := testAgent(t, nil)
	token := store.Snapshot().PairingToken
	a.tailnet = missingTailnet{}
	for _, mode := range []string{"choose", "simple"} {
		if err := store.Update(func(c *config.Config) error { c.ConnectionMode = mode; return nil }); err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, localControlRequest(store))
		if w.Code != 200 {
			t.Fatal("沒有 Tailscale 不能管理", w.Code)
		}
		for _, path := range []string{"/v1/status", "/v1/clipboard/text", "/setup/old"} {
			r := httptest.NewRequest("GET", path, nil)
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			a.Handler().ServeHTTP(w, r)
			if w.Code != 403 && w.Code != 404 {
				t.Fatal("舊入口未停用", path, w.Code)
			}
		}
		if err := a.configureConnection(context.Background(), "tailscale"); err == nil {
			t.Fatal("缺少 Tailscale 卻切換成功")
		}
		if store.Snapshot().Mode() != mode || store.Snapshot().PairingToken != token {
			t.Fatal("失敗切換修改設定")
		}
	}
	a.tailnet = fakeTailnet{}
	a.prepareTailnet = func(ctx context.Context) error {
		if _, limited := ctx.Deadline(); limited {
			t.Fatal("人工確認被套用網路逾時")
		}
		return nil
	}
	if err := a.configureConnection(context.Background(), "tailscale"); err != nil {
		t.Fatal(err)
	}
	if store.Snapshot().PairingToken != token || store.Snapshot().Mode() != "tailscale" {
		t.Fatal("切回 A 破壞配對")
	}
	r := httptest.NewRequest("GET", "/v1/status", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal("舊 token 失效", w.Code)
	}
}

func (f fakeTailnet) Status(context.Context) (tailscale.Status, error) { return f.status, nil }

func testAgent(t *testing.T, logs *bytes.Buffer) (*Agent, *config.Store) {
	t.Helper()
	store, _, err := config.OpenStore(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(c *config.Config) error { c.ConnectionMode = "tailscale"; return nil }); err != nil {
		t.Fatal(err)
	}
	var status tailscale.Status
	status.BackendState = "Running"
	status.Self.DNSName = "work.example.ts.net"
	status.Self.HostName = "work"
	var logger *slog.Logger
	if logs != nil {
		logger = slog.New(slog.NewJSONHandler(logs, nil))
	}
	agent, err := New(Options{
		Store: store, Clipboard: clipboard.NewMemory(), Tailnet: fakeTailnet{status}, Logger: logger,
		Shortcuts:        webui.ShortcutAssets{Send: []byte("send"), Pull: []byte("pull")},
		DashboardAddress: "127.0.0.1:0",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = agent.dashboard.Close() })
	return agent, store
}

func localControlRequest(store *config.Store) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/local/open-setup", nil)
	request.Host = "127.0.0.1:17733"
	request.RemoteAddr = "127.0.0.1:54321"
	request.Header.Set("Authorization", "Bearer "+store.Snapshot().PairingToken)
	return request
}

func TestHandlerHealthAndLocalControl(t *testing.T) {
	var logs bytes.Buffer
	agent, store := testAgent(t, &logs)
	health := httptest.NewRecorder()
	agent.Handler().ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/v1/health", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health=%d", health.Code)
	}

	response := httptest.NewRecorder()
	agent.Handler().ServeHTTP(response, localControlRequest(store))
	if response.Code != http.StatusOK {
		t.Fatalf("open setup=%d body=%s", response.Code, response.Body.String())
	}
	var result struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || !strings.HasPrefix(result.URL, "http://127.0.0.1:") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	page, err := http.Get(result.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(page.Body)
	page.Body.Close()
	if page.StatusCode != http.StatusOK || !strings.Contains(string(body), "掃一次") {
		t.Fatalf("page=%d body=%s", page.StatusCode, body)
	}
	if strings.Contains(logs.String(), result.URL) || strings.Contains(logs.String(), store.Snapshot().PairingToken) {
		t.Fatal("日誌不得包含設定 URL 或 token")
	}
}

func TestControlRejectsExternalOrInvalidToken(t *testing.T) {
	agent, store := testAgent(t, nil)
	for _, mutate := range []func(*http.Request){
		func(r *http.Request) { r.RemoteAddr = "100.64.0.2:1234" },
		func(r *http.Request) { r.Host = "work.example.ts.net" },
		func(r *http.Request) { r.Header.Set("Authorization", "Bearer wrong") },
	} {
		request := localControlRequest(store)
		mutate(request)
		response := httptest.NewRecorder()
		agent.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("status=%d", response.Code)
		}
	}
}

func TestDashboardRotationPersistsNewToken(t *testing.T) {
	agent, store := testAgent(t, nil)
	oldToken := store.Snapshot().PairingToken
	response := httptest.NewRecorder()
	agent.Handler().ServeHTTP(response, localControlRequest(store))
	var result struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &result)
	request, _ := http.NewRequest(http.MethodPost, result.URL+"/rotate", nil)
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	rotated, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	rotated.Body.Close()
	if rotated.StatusCode != http.StatusSeeOther || store.Snapshot().PairingToken == oldToken {
		t.Fatalf("status=%d token 未輪替", rotated.StatusCode)
	}
}
