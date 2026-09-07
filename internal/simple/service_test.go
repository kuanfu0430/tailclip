package simple

import (
	"encoding/json"

	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kuanfu0430/tailclip/internal/clipboard"
	"github.com/kuanfu0430/tailclip/internal/config"
)

func testService(t *testing.T) *Service {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "simple.state"), "測試電腦", clipboard.NewMemory())
	if err != nil {
		t.Fatal(err)
	}
	s.SetActive(true)
	t.Cleanup(s.Close)
	return s
}
func simpleRequest(s *Service, path, token, body string) *httptest.ResponseRecorder {
	method := "GET"
	if body != "" {
		method = "POST"
	}
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func issueTestTicket(s *Service) string {
	ticket, _ := config.GenerateToken()
	s.mu.Lock()
	s.ticket = ticket
	s.expires = s.now().Add(time.Minute)
	s.mu.Unlock()
	return `{"version":1,"ticket":"` + ticket + `"}`
}
func TestPairingPersistenceRevocationAndRouteIsolation(t *testing.T) {
	s := testService(t)
	if w := simpleRequest(s, "/v1/clipboard/text", "", "{}"); w.Code != 401 {
		t.Fatal(w.Code)
	}
	body := issueTestTicket(s)
	w := simpleRequest(s, "/v1/pair", "", body)
	if w.Code != 200 {
		t.Fatalf("配對失敗 %s", w.Body.String())
	}
	var result struct {
		Token string `json:"token"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	if w := simpleRequest(s, "/v1/pair", "", body); w.Code != 401 {
		t.Fatal("票券可重放")
	}
	if w := simpleRequest(s, "/v1/clipboard/text", result.Token, `{"text":"中文 🐾\n第二行"}`); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := simpleRequest(s, "/v1/clipboard/text", result.Token, ""); !strings.Contains(w.Body.String(), "中文") {
		t.Fatal(w.Body.String())
	}
	for _, path := range []string{"/local/shutdown", "/local/open-setup", "/setup/x", "/anything"} {
		if w := simpleRequest(s, path, result.Token, "{}"); w.Code != 404 {
			t.Fatal("遠端路由未隔離", path, w.Code)
		}
	}
	reloaded, err := Open(s.path, "pc", clipboard.NewMemory())
	if err != nil {
		t.Fatal(err)
	}
	reloaded.SetActive(true)
	defer reloaded.Close()
	if !reloaded.Paired() || reloaded.Snapshot().PairingToken != result.Token {
		t.Fatal("重啟遺失配對")
	}
	if !reloaded.state.Identity.Private.Equal(s.state.Identity.Private) {
		t.Fatal("重啟更換身分")
	}
	if err := reloaded.Revoke(); err != nil {
		t.Fatal(err)
	}
	if w := simpleRequest(reloaded, "/v1/status", result.Token, ""); w.Code != 401 {
		t.Fatal("撤銷後仍可用")
	}
	reloaded.SetActive(false)
	if w := simpleRequest(reloaded, "/v1/health", "", ""); w.Code != 503 {
		t.Fatal("停用後仍開放")
	}
	bytes, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(bytes), "中文") {
		t.Fatal("剪貼簿被保存")
	}
	if runtime.GOOS == "windows" && strings.Contains(string(bytes), result.Token) {
		t.Fatal("Windows 憑證未加密")
	}
}
func TestTicketAtomicExpiryAndFailedSave(t *testing.T) {
	s := testService(t)
	body := issueTestTicket(s)
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
		t.Fatal("票券不具原子性", successes.Load())
	}
	body = issueTestTicket(s)
	s.expires = time.Now().Add(-time.Second)
	if w := simpleRequest(s, "/v1/pair", "", body); w.Code != 401 {
		t.Fatal("過期票券被接受")
	}
	previous := s.Snapshot().PairingToken
	body = issueTestTicket(s)
	s.path = filepath.Join(s.path, "not-a-directory")
	if w := simpleRequest(s, "/v1/pair", "", body); w.Code != 500 {
		t.Fatal("保存失敗卻回成功", w.Code)
	}
	if s.Snapshot().PairingToken != previous {
		t.Fatal("失敗仍覆蓋有效憑證")
	}
}

// 未連到任何網路也能完成 B 設定儲存，不讀取桌面真實剪貼簿。
func TestReadStateRejectsCorruption(t *testing.T) {
	s := testService(t)
	if err := os.WriteFile(s.path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(s.path, "pc", clipboard.NewMemory()); err == nil {
		t.Fatal("損壞身分被默默重設")
	}
}
