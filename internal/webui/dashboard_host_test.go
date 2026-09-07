package webui

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHumanConfirmationMayOutlastWriteTimeout(t *testing.T) {
	host := NewDashboardHost("127.0.0.1:0")
	provider := func(context.Context, bool) (DashboardData, error) { return DashboardData{Mode: "choose"}, nil }
	host.entries["test"] = dashboardEntry{provider: provider, data: DashboardData{Configure: func(context.Context, string) error { time.Sleep(100 * time.Millisecond); return nil }}}
	server := httptest.NewUnstartedServer(host)
	server.Config.WriteTimeout = 25 * time.Millisecond
	server.Start()
	t.Cleanup(func() { server.Close(); host.Close() })
	client := &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Post(server.URL+"/dashboard/test/action", "application/x-www-form-urlencoded", strings.NewReader("action=tailscale"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 303 {
		t.Fatal(resp.StatusCode)
	}
}

func TestRefreshUsesCurrentMessage(t *testing.T) {
	host := NewDashboardHost("127.0.0.1:0")
	t.Cleanup(func() { host.Close() })
	var message atomic.Value
	message.Store("正在啟動連線")
	provider := func(context.Context, bool) (DashboardData, error) {
		return DashboardData{Mode: "choose", ConnectionMessage: message.Load().(string), Configure: func(context.Context, string) error { return nil }}, nil
	}
	url, err := host.Open(context.Background(), provider)
	if err != nil {
		t.Fatal(err)
	}
	message.Store("最新連線狀態")
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "正在啟動連線") || !strings.Contains(string(body), "最新連線狀態") {
		t.Fatal("最新訊息被舊快照覆蓋")
	}
}

func TestConnectionActionRejectsCrossOriginAndRefreshes(t *testing.T) {
	host := NewDashboardHost("127.0.0.1:0")
	t.Cleanup(func() { host.Close() })
	var paired atomic.Bool
	var actions atomic.Int32
	provider := func(context.Context, bool) (DashboardData, error) {
		return DashboardData{Mode: "simple", SimplePaired: paired.Load(), Configure: func(context.Context, string) error { actions.Add(1); paired.Store(true); return nil }}, nil
	}
	url, err := host.Open(context.Background(), provider)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, origin := range []string{"https://untrusted.example", ""} {
		r, _ := http.NewRequest("POST", url+"/action", strings.NewReader("action=pair"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		resp, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		want := 303
		if origin != "" {
			want = 403
		}
		if resp.StatusCode != want {
			t.Fatal(resp.StatusCode)
		}
	}
	if actions.Load() != 1 {
		t.Fatal("跨來源操作被執行")
	}
	paired.Store(false)
	resp, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "尚未連接手機") {
		t.Fatal("仍顯示舊配對狀態")
	}
}

func TestDashboardHostOpensOnDemandAndRotates(t *testing.T) {
	host := NewDashboardHost("127.0.0.1:0")
	t.Cleanup(func() { _ = host.Close() })
	var rotations atomic.Int32
	provider := func(_ context.Context, rotate bool) (DashboardData, error) {
		if rotate {
			rotations.Add(1)
		}
		return DashboardData{
			DeviceName: "工作電腦", DNSName: "work.example.ts.net",
			PairingURL: "https://work.example.ts.net/tailclip/setup/secret",
			ExpiresAt:  time.Now().Add(time.Minute), ClipboardAvailable: true, ShortcutsReady: true,
		}, nil
	}
	url, err := host.Open(context.Background(), provider)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "掃一次") {
		t.Fatalf("status=%d body=%s", response.StatusCode, body)
	}

	request, _ := http.NewRequest(http.MethodPost, url+"/rotate", nil)
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther || rotations.Load() != 1 {
		t.Fatalf("status=%d rotations=%d", response.StatusCode, rotations.Load())
	}
	if response.Header.Get("Location") == "" || response.Header.Get("Location") == strings.TrimPrefix(url, "http://127.0.0.1:0") {
		t.Fatal("輪替後應使用新的不可猜測頁面")
	}
}
