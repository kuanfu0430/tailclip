package simple

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/kuanfu0430/tailblink/internal/tunnel"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kuanfu0430/tailblink/internal/clipboard"
	"github.com/kuanfu0430/tailblink/internal/config"
)

type fakeTunnel struct{ done chan struct{} }

func (f *fakeTunnel) URL() string           { return "https://fixture.trycloudflare.com" }
func (f *fakeTunnel) Done() <-chan struct{} { return f.done }
func (f *fakeTunnel) Close() {
	select {
	case <-f.done:
	default:
		close(f.done)
	}
}
func testService(t *testing.T) *Service {
	t.Helper()
	s := New("測試電腦", clipboard.NewMemory())
	s.active = true
	s.ready = true
	s.host = "fixture.trycloudflare.com"
	s.session = "fixture-session"
	token, _ := config.GenerateToken()
	s.setCredentialLocked(token)
	s.runtime = &runtime{&http.Server{}, &fakeTunnel{make(chan struct{})}}
	t.Cleanup(s.Close)
	return s
}
func simpleRequest(s *Service, path, token, body string) *httptest.ResponseRecorder {
	method := "GET"
	if body != "" {
		method = "POST"
	}
	r := httptest.NewRequest(method, "https://fixture.trycloudflare.com"+path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func pair(t *testing.T, s *Service) string {
	t.Helper()
	p, err := s.NewPairing()
	if err != nil {
		t.Fatal(err)
	}
	body := `{"version":"1","ticket":"` + p.Ticket + `"}`
	w := simpleRequest(s, "/v1/pair", "", body)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var result struct {
		Token string `json:"token"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	if !config.ValidToken(result.Token) {
		t.Fatal("無效 token")
	}
	if simpleRequest(s, "/v1/pair", "", body).Code != 401 {
		t.Fatal("票券重放")
	}
	return result.Token
}
func TestPairingTransferRevocationAndIsolation(t *testing.T) {
	s := testService(t)
	if simpleRequest(s, "/v1/clipboard/text", "", "{}").Code != 401 {
		t.Fatal("未配對仍可用")
	}
	first := pair(t, s)
	text := "中文 🐾\n第二行"
	data, _ := json.Marshal(map[string]string{"text": text})
	if w := simpleRequest(s, "/v1/clipboard/text", first, string(data)); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := simpleRequest(s, "/v1/clipboard/text", first, ""); !strings.Contains(w.Body.String(), "中文") {
		t.Fatal("讀回不符")
	}
	// 新票券尚未使用前，舊配對繼續可用。
	s.NewPairing()
	if simpleRequest(s, "/v1/status", first, "").Code != 200 {
		t.Fatal("產生 QR 提早撤銷")
	}
	second := pair(t, s)
	if first == second || simpleRequest(s, "/v1/status", first, "").Code != 401 {
		t.Fatal("未取代舊憑證")
	}
	for _, p := range []string{"/local/shutdown", "/local/open-setup", "/dashboard/x", "/setup/invalid", "/anything"} {
		if simpleRequest(s, p, second, "{}").Code != 404 {
			t.Fatal("管理路由外洩", p)
		}
	}
	for name, value := range map[string]string{"Origin": "https://evil.example", "Sec-Fetch-Site": "same-origin", "X-TailBlink-Client": "shortcuts-v2"} {
		r := httptest.NewRequest("GET", "https://fixture.trycloudflare.com/v1/status", nil)
		r.Header.Set("Authorization", "Bearer "+second)
		r.Header.Set(name, value)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("跨站或 A 身分放行", name)
		}
	}
	r := httptest.NewRequest("GET", "https://other.trycloudflare.com/v1/status", nil)
	r.Header.Set("Authorization", "Bearer "+second)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("錯誤 Host 放行")
	}
	s.Revoke()
	if s.Paired() || s.PairingPending() || simpleRequest(s, "/v1/status", second, "").Code != 401 {
		t.Fatal("撤銷無效")
	}
	s.Close()
	if s.Ready() || s.Snapshot().PairingToken != "" {
		t.Fatal("關閉未清除秘密")
	}
}
func TestTicketAtomicExpiryAndLimit(t *testing.T) {
	s := testService(t)
	p, _ := s.NewPairing()
	body := `{"version":1,"ticket":"` + p.Ticket + `"}`
	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if simpleRequest(s, "/v1/pair", "", body).Code == 200 {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatal("票券非原子")
	}
	p, _ = s.NewPairing()
	s.expires = s.now().Add(-time.Second)
	if simpleRequest(s, "/v1/pair", "", `{"version":1,"ticket":"`+p.Ticket+`"}`).Code != 401 {
		t.Fatal("接受過期票券")
	}
	for range 30 {
		simpleRequest(s, "/v1/pair", "", `{}`)
	}
	if simpleRequest(s, "/v1/pair", "", `{}`).Code != 429 {
		t.Fatal("配對無限流")
	}
}
func TestSetupAndDownloads(t *testing.T) {
	s := testService(t)
	p, _ := s.NewPairing()
	w := simpleRequest(s, "/setup/"+p.Ticket, "", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "shortcuts://run-shortcut?") || strings.Contains(w.Body.String(), "#ZgotmplZ") {
		t.Fatal("手機配對入口無效")
	}
	if strings.Contains(w.Body.String(), s.credential) {
		t.Fatal("頁面外洩 Bearer")
	}
	for _, d := range []string{"Send", "Pull"} {
		w := simpleRequest(s, "/shortcuts/TailBlink-Simple-"+d+".shortcut", "", "")
		if w.Code != 200 || !strings.HasPrefix(w.Body.String(), "AEA1") {
			t.Fatal("未含已簽署捷徑")
		}
	}
	pair(t, s)
	if simpleRequest(s, "/setup/"+p.Ticket, "", "").Code != 404 {
		t.Fatal("已使用頁面仍可用")
	}
}
func TestHostValidation(t *testing.T) {
	for _, h := range []string{"x.trycloudflare.com.evil.com", "x.trycloudflare.com:443", "-x.trycloudflare.com", "x.y.trycloudflare.com", "trycloudflare.com", "x@x.trycloudflare.com"} {
		if validHost(h) {
			t.Fatal(h)
		}
	}
	if !validHost("quiet-river.trycloudflare.com") {
		t.Fatal("拒絕正常網址")
	}
}

func TestWrongCredentialsDoNotConsumeTransferQuota(t *testing.T) {
	s := testService(t)
	token := pair(t, s)
	for range 100 {
		if simpleRequest(s, "/v1/status", "invalid", "").Code != 401 {
			t.Fatal("錯憑證未拒絕")
		}
	}
	if simpleRequest(s, "/v1/status", token, "").Code != 200 {
		t.Fatal("錯憑證耗盡合法額度")
	}
}
func TestSlowRequestDoesNotBlockRevocation(t *testing.T) {
	s := testService(t)
	p, _ := s.NewPairing()
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	r := httptest.NewRequest("POST", "https://fixture.trycloudflare.com/v1/pair", reader)
	done := make(chan struct{})
	go func() { s.ServeHTTP(httptest.NewRecorder(), r); close(done) }()
	// 送出部分 JSON 並停住，確認讀網路期間不持狀態鎖。
	if _, err := writer.Write([]byte(`{"version":1,"ticket":"`)); err != nil {
		t.Fatal(err)
	}
	revoked := make(chan struct{})
	go func() { s.Revoke(); close(revoked) }()
	select {
	case <-revoked:
	case <-time.After(time.Second):
		t.Fatal("慢 body 阻塞撤銷")
	}
	writer.Write([]byte(p.Ticket + `"}`))
	writer.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("請求未結束")
	}
	if s.Paired() {
		t.Fatal("已撤銷票券仍配對成功")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestStartupAndUnexpectedExit(t *testing.T) {
	s := New("測試", clipboard.NewMemory())
	defer s.Close()
	wire := &fakeTunnel{make(chan struct{})}
	var address string
	s.start = func(_ context.Context, a string) (tunnel.Handle, error) { address = a; return wire, nil }
	original := http.DefaultTransport
	defer func() { http.DefaultTransport = original }()
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		clone := r.Clone(r.Context())
		clone.URL.Scheme = "http"
		clone.URL.Host = address
		clone.Host = "fixture.trycloudflare.com"
		return transport.RoundTrip(clone)
	})
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.SetActive(true)
	if !s.Ready() {
		t.Fatal("已驗證仍未就緒")
	}
	pair(t, s)
	wire.Close()
	deadline := time.Now().Add(time.Second)
	for s.Ready() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if s.Ready() || s.Paired() || s.PairingPending() || s.Snapshot().PairingToken != "" {
		t.Fatal("程序退出未撤銷")
	}
	if connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond); err == nil {
		connection.Close()
		t.Fatal("listener 殘留")
	}
}
func TestStartupFailureClearsSecrets(t *testing.T) {
	s := New("測試", clipboard.NewMemory())
	defer s.Close()
	s.start = func(context.Context, string) (tunnel.Handle, error) { return nil, errors.New("合成失敗") }
	if s.Start(context.Background()) == nil || s.Ready() || s.Snapshot().PairingToken != "" {
		t.Fatal("啟動失敗狀態錯誤")
	}
}
func TestHealthRejectsRedirectAndWrongSession(t *testing.T) {
	s := New("測試", clipboard.NewMemory())
	original := http.DefaultTransport
	defer func() { http.DefaultTransport = original }()
	for _, status := range []int{200, 302} {
		var requests int
		http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			requests++
			return &http.Response{StatusCode: status, Header: http.Header{"Location": []string{"https://evil.example"}}, Body: io.NopCloser(strings.NewReader(`{"service":"tailblink-simple","session":"wrong"}`)), Request: r}, nil
		})
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		err := s.verify(ctx, "https://fixture.trycloudflare.com", "expected", make(chan struct{}))
		cancel()
		if err == nil || requests != 1 {
			t.Fatal("假健康或重新導向被接受")
		}
	}
}
