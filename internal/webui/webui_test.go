package webui

import (
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kuanfu0430/tailclip/internal/pairing"
)

func newSession(t *testing.T) (*pairing.Manager, pairing.Session) {
	t.Helper()
	manager := pairing.NewManager()
	session, err := manager.Create(pairing.Data{
		Version: 1, BaseURL: "https://work.example.ts.net/tailclip/v1", Token: "top-secret-token",
		DeviceName: "工作電腦", TailscaleDevice: "work",
	})
	if err != nil {
		t.Fatal(err)
	}
	return manager, session
}

func TestPairingPageHeadersAndPayload(t *testing.T) {
	manager, session := newSession(t)
	handler := NewPublicHandler(manager, ShortcutAssets{Send: []byte("send"), Pull: []byte("pull")})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/setup/"+session.Nonce, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	for _, header := range []string{"Cache-Control", "Content-Security-Policy", "Referrer-Policy"} {
		if recorder.Header().Get(header) == "" {
			t.Fatalf("缺少 %s", header)
		}
	}
	if strings.Contains(recorder.Body.String(), "top-secret-token") {
		t.Fatal("配對資料應編碼後再嵌入 HTML，避免直接洩漏至 DOM 文字")
	}
	if !strings.Contains(recorder.Body.String(), `id="payload"`) {
		t.Fatal("配對頁缺少 payload")
	}
	if !strings.Contains(recorder.Body.String(), session.Nonce+"/TailClip-Send.shortcut") {
		t.Fatal("捷徑下載連結必須保留目前 nonce")
	}
}

func TestExpiredAndShortcutDownload(t *testing.T) {
	manager, session := newSession(t)
	handler := NewPublicHandler(manager, ShortcutAssets{Send: []byte("send"), Pull: []byte("pull")})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/setup/not-found", nil))
	if recorder.Code != http.StatusNotFound || strings.Contains(recorder.Body.String(), "not-found") {
		t.Fatalf("未知 nonce 回應不正確: %d %s", recorder.Code, recorder.Body.String())
	}

	for _, test := range []struct{ route, filename, body string }{
		{"TailClip-Send.shortcut", "TailClip：傳送.shortcut", "send"},
		{"TailClip-Pull.shortcut", "TailClip：取回.shortcut", "pull"},
	} {
		recorder = httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/setup/"+session.Nonce+"/"+test.route, nil))
		if recorder.Code != http.StatusOK || recorder.Body.String() != test.body || recorder.Header().Get("Cache-Control") == "" {
			t.Fatalf("捷徑下載不正確: %d %q", recorder.Code, recorder.Body.String())
		}
		mediaType, params, err := mime.ParseMediaType(recorder.Header().Get("Content-Disposition"))
		if err != nil || mediaType != "attachment" || params["filename"] != test.filename {
			t.Fatalf("匯入名稱必須與配對頁一致: %v %q", err, recorder.Header().Get("Content-Disposition"))
		}
	}
}

func TestRenderDashboardDoesNotEmbedPairingSecret(t *testing.T) {
	recorder := httptest.NewRecorder()
	secret := "pairing-secret-must-not-appear"
	err := RenderDashboard(recorder, DashboardData{
		DeviceName: "工作電腦", DNSName: "work.example.ts.net",
		PairingURL: "https://work.example.ts.net/tailclip/setup/" + secret,
		ExpiresAt:  time.Now().Add(time.Minute), ClipboardAvailable: true, ShortcutsReady: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(recorder.Body.String(), secret) {
		t.Fatal("本機頁不應以文字嵌入 pairing URL；QR data 可包含，但不該直接可搜尋")
	}
	if !strings.Contains(recorder.Body.String(), `src="data:image/png;base64,`) {
		t.Fatal("設定頁應嵌入可顯示的 PNG QR")
	}
	if strings.Contains(recorder.Body.String(), "#ZgotmplZ") {
		t.Fatal("QR data URL 不應被 html/template 安全過濾")
	}
	if strings.Contains(recorder.Body.String(), "vv0.") {
		t.Fatal("版本標籤不應重複 v 前綴")
	}
}
