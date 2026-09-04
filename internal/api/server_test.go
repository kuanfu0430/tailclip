package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kuanfu0430/tailclip/internal/clipboard"
	"github.com/kuanfu0430/tailclip/internal/config"
)

type staticConfig struct{ cfg config.Config }

func (s staticConfig) Snapshot() config.Config { return s.cfg }

func testServer(t *testing.T, backend clipboard.Backend, logger *slog.Logger, limit int) http.Handler {
	t.Helper()
	token, err := config.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	return New(Options{
		Config: staticConfig{config.Config{
			Version:         1,
			DeviceName:      "工作電腦",
			TailscaleDevice: "work-pc",
			PairingToken:    token,
		}},
		Clipboard: backend,
		Logger:    logger,
		RateLimit: limit,
	}).Handler()
}

func request(t *testing.T, handler http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func tokenFromHandler(t *testing.T) (http.Handler, string, *clipboard.Memory) {
	t.Helper()
	token, err := config.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	memory := clipboard.NewMemory()
	handler := New(Options{
		Config:    staticConfig{config.Config{Version: 1, DeviceName: "工作電腦", TailscaleDevice: "work-pc", PairingToken: token}},
		Clipboard: memory,
	}).Handler()
	return handler, token, memory
}

func TestHealthDoesNotTouchClipboard(t *testing.T) {
	backend := &countingBackend{}
	handler := testServer(t, backend, nil, 60)
	recorder := request(t, handler, http.MethodGet, "/v1/health", "", "")
	if recorder.Code != http.StatusOK || backend.calls != 0 {
		t.Fatalf("health status=%d clipboard calls=%d", recorder.Code, backend.calls)
	}
}

func TestAuthSendAndPullUnicode(t *testing.T) {
	handler, token, memory := tokenFromHandler(t)
	if got := request(t, handler, http.MethodGet, "/v1/status", "wrong", ""); got.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token status=%d", got.Code)
	}
	text := "中文\nEmoji 🐾\r\nline"
	body, _ := json.Marshal(map[string]string{"text": text})
	if got := request(t, handler, http.MethodPost, "/v1/clipboard/text", token, string(body)); got.Code != http.StatusOK {
		t.Fatalf("send status=%d body=%s", got.Code, got.Body.String())
	}
	stored, err := memory.ReadText(context.Background())
	if err != nil || stored != text {
		t.Fatalf("stored=%q err=%v", stored, err)
	}
	got := request(t, handler, http.MethodGet, "/v1/clipboard/text", token, "")
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), "Emoji") {
		t.Fatalf("pull status=%d body=%s", got.Code, got.Body.String())
	}
}

func TestEmptyInvalidAndSizeBoundaries(t *testing.T) {
	handler, token, _ := tokenFromHandler(t)
	tests := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{"empty", `{"text":""}`, http.StatusBadRequest, "invalid_text"},
		{"nul", `{"text":"a\u0000b"}`, http.StatusBadRequest, "invalid_text"},
		{"unknown", `{"text":"a","extra":true}`, http.StatusBadRequest, "invalid_request"},
		{"malformed", `{"text":`, http.StatusBadRequest, "invalid_request"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := request(t, handler, http.MethodPost, "/v1/clipboard/text", token, tc.body)
			if got.Code != tc.status || !strings.Contains(got.Body.String(), tc.code) {
				t.Fatalf("status=%d body=%s", got.Code, got.Body.String())
			}
		})
	}

	atLimit, _ := json.Marshal(map[string]string{"text": strings.Repeat("a", MaxTextBytes)})
	if got := request(t, handler, http.MethodPost, "/v1/clipboard/text", token, string(atLimit)); got.Code != http.StatusOK {
		t.Fatalf("1 MiB 應成功：%d %s", got.Code, got.Body.String())
	}
	escapedAtLimit, _ := json.Marshal(map[string]string{"text": strings.Repeat("\n", MaxTextBytes)})
	if got := request(t, handler, http.MethodPost, "/v1/clipboard/text", token, string(escapedAtLimit)); got.Code != http.StatusOK {
		t.Fatalf("JSON 跳脫後超過 1 MiB、解碼後仍為 1 MiB 應成功：%d %s", got.Code, got.Body.String())
	}
	over, _ := json.Marshal(map[string]string{"text": strings.Repeat("a", MaxTextBytes+1)})
	if got := request(t, handler, http.MethodPost, "/v1/clipboard/text", token, string(over)); got.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("超過 1 MiB 應拒絕：%d %s", got.Code, got.Body.String())
	}
}

func TestInvalidUTF8AndEmptyPull(t *testing.T) {
	handler, token, _ := tokenFromHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/clipboard/text", bytes.NewReader([]byte{'{', '"', 't', 'e', 'x', 't', '"', ':', '"', 0xff, '"', '}'}))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "invalid_text") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	got := request(t, handler, http.MethodGet, "/v1/clipboard/text", token, "")
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"empty":true`) {
		t.Fatalf("empty pull status=%d body=%s", got.Code, got.Body.String())
	}
}

func TestRateLimitAndLogRedaction(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	token, err := config.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	secret := "不應出現在日誌的文字"
	handler := New(Options{
		Config:    staticConfig{config.Config{Version: 1, DeviceName: "pc", PairingToken: token}},
		Clipboard: clipboard.NewMemory(),
		Logger:    logger,
		RateLimit: 1,
	}).Handler()
	body, _ := json.Marshal(map[string]string{"text": secret})
	if got := request(t, handler, http.MethodPost, "/v1/clipboard/text", token, string(body)); got.Code != http.StatusOK {
		t.Fatal(got.Code)
	}
	if got := request(t, handler, http.MethodGet, "/v1/status", token, ""); got.Code != http.StatusTooManyRequests {
		t.Fatalf("rate limit status=%d", got.Code)
	}
	if strings.Contains(logs.String(), secret) || strings.Contains(logs.String(), token) {
		t.Fatalf("日誌含有秘密：%s", logs.String())
	}
}

func TestConcurrentWritesAreSerialized(t *testing.T) {
	backend := &concurrencyBackend{}
	handler, token := handlerWithBackend(t, clipboard.NewSynchronized(backend))
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			_ = request(t, handler, http.MethodPost, "/v1/clipboard/text", token, `{"text":"x"}`)
		})
	}
	wg.Wait()
	if backend.maxActive != 1 {
		t.Fatalf("同時寫入數 = %d，預期 1", backend.maxActive)
	}
}

func handlerWithBackend(t *testing.T, backend clipboard.Backend) (http.Handler, string) {
	t.Helper()
	token, err := config.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	return New(Options{
		Config:    staticConfig{config.Config{Version: 1, DeviceName: "pc", PairingToken: token}},
		Clipboard: backend,
		RateLimit: 100,
	}).Handler(), token
}

type countingBackend struct{ calls int }

func (b *countingBackend) Available(context.Context) error { b.calls++; return nil }
func (b *countingBackend) ReadText(context.Context) (string, error) {
	b.calls++
	return "", clipboard.ErrNoText
}
func (b *countingBackend) WriteText(context.Context, string) error { b.calls++; return nil }

type concurrencyBackend struct {
	mu        sync.Mutex
	active    int
	maxActive int
}

func (b *concurrencyBackend) Available(context.Context) error { return nil }
func (b *concurrencyBackend) ReadText(context.Context) (string, error) {
	return "", clipboard.ErrNoText
}
func (b *concurrencyBackend) WriteText(context.Context, string) error {
	b.mu.Lock()
	b.active++
	if b.active > b.maxActive {
		b.maxActive = b.active
	}
	b.mu.Unlock()
	time.Sleep(time.Millisecond)
	b.mu.Lock()
	b.active--
	b.mu.Unlock()
	return nil
}

type errorBackend struct{ err error }

func (b errorBackend) Available(context.Context) error { return b.err }
func (b errorBackend) ReadText(context.Context) (string, error) {
	return "", b.err
}
func (b errorBackend) WriteText(context.Context, string) error { return b.err }

func TestClipboardErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code string
	}{
		{clipboard.ErrBusy, "clipboard_busy"},
		{clipboard.ErrUnavailable, "clipboard_unavailable"},
		{errors.New("unknown"), "internal_error"},
	} {
		handler, token := handlerWithBackend(t, errorBackend{tc.err})
		got := request(t, handler, http.MethodGet, "/v1/clipboard/text", token, "")
		if !strings.Contains(got.Body.String(), tc.code) {
			t.Fatalf("err=%v body=%s", tc.err, got.Body.String())
		}
	}
}
