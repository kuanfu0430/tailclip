package simple

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kuanfu0430/tailclip/internal/clipboard"
	qrcode "github.com/skip2/go-qrcode"
	"github.com/tailscale/tailcat"
	"tailscale.com/derp/derpserver"
	"tailscale.com/tailcfg"
	"tailscale.com/types/key"
)

// 使用本機 TLS DERP 與真實 WireGuard 通道；不碰公共中繼或使用者剪貼簿。
func TestEncryptedTransportRestart(t *testing.T) {
	t.Setenv("IN_TS_TEST", "true")
	quiet := func(string, ...any) {}
	relay := derpserver.New(key.NewNode(), quiet)
	https := httptest.NewTLSServer(derpserver.Handler(relay))
	t.Cleanup(func() { relay.Close(); https.Close() })
	region := &tailcfg.DERPRegion{RegionID: 1, RegionCode: "local-test", Nodes: []*tailcfg.DERPNode{{Name: "test", RegionID: 1, HostName: "127.0.0.1", IPv4: "127.0.0.1", IPv6: "none", DERPPort: https.Listener.Addr().(*net.TCPAddr).Port, STUNPort: -1, InsecureForTests: true}}}
	s := testService(t)
	s.state.Identity.Public.Region = []*tailcfg.DERPRegion{region}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	pairing, err := s.NewPairing()
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(pairing)
	if _, err := qrcode.Encode(string(payload), qrcode.Medium, 360); err != nil {
		t.Fatal("配對資料無法產生 QR", err)
	}
	clientKey := key.NewNode()
	newClient := func() (*http.Client, func()) {
		wire := tailcat.NewClient(tailcat.Addr(pairing.Address))
		wire.Key = clientKey
		wire.Logf = quiet
		tr := &http.Transport{DisableKeepAlives: true, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) { return wire.DialTCPPort(ctx, Port) }}
		return &http.Client{Transport: tr, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, func() { tr.CloseIdleConnections(); wire.Close() }
	}
	client, closeClient := newClient()
	defer func() { closeClient() }()
	call := func(path, token, body string) (int, []byte) {
		t.Helper()
		method := "GET"
		if body != "" {
			method = "POST"
		}
		r, err := http.NewRequestWithContext(ctx, method, "http://tailclip"+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", "application/json")
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, data
	}
	if code, _ := call("/v1/status", "", ""); code != 401 {
		t.Fatal(code)
	}
	body, _ := json.Marshal(map[string]any{"version": 1, "ticket": pairing.Ticket})
	code, data := call("/v1/pair", "", string(body))
	if code != 200 {
		t.Fatalf("配對=%d", code)
	}
	var result struct {
		Token string `json:"token"`
	}
	if json.Unmarshal(data, &result) != nil {
		t.Fatal("回應格式")
	}
	if code, _ := call("/v1/clipboard/text", result.Token, `{"text":"加密通道 🐾"}`); code != 200 {
		t.Fatal(code)
	}
	if code, data := call("/v1/clipboard/text", result.Token, ""); code != 200 || !strings.Contains(string(data), "加密通道") {
		t.Fatal("讀回不符")
	}
	if code, _ := call("/local/shutdown", result.Token, "{}"); code != 404 {
		t.Fatal("管理端點外洩")
	}
	closeClient()
	s.Close()
	restored, err := Open(s.path, "pc", clipboard.NewMemory())
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if err := restored.Start(ctx); err != nil {
		t.Fatal(err)
	}
	restored.SetActive(true)
	if string(restored.runtime.server.TailcatAddr()) != pairing.Address {
		t.Fatal("正常重啟改變地址")
	}
	client, closeClient = newClient()
	if code, _ := call("/v1/status", result.Token, ""); code != 200 {
		t.Fatal("原配對無法重連", code)
	}
	if err := restored.Revoke(); err != nil {
		t.Fatal(err)
	}
	if code, _ := call("/v1/status", result.Token, ""); code != 401 {
		t.Fatal("撤銷後仍放行", code)
	}
}
