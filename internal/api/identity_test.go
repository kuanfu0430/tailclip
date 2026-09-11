package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kuanfu0430/tailblink/internal/config"
)

func TestIdentityAuthorizationDoesNotFallBackOrReadClipboard(t *testing.T) {
	token, _ := config.GenerateToken()
	for _, tc := range []struct {
		name   string
		mutate func(*http.Request)
		status int
	}{
		{"同帳號", func(*http.Request) {}, 200},
		{"不同帳號", func(r *http.Request) { r.Header.Set("Tailscale-User-Login", "other@example.com") }, 403},
		{"缺身分且有舊憑證", func(r *http.Request) {
			r.Header.Del("Tailscale-User-Login")
			r.Header.Set("Authorization", "Bearer "+token)
		}, 401},
		{"重複身分", func(r *http.Request) { r.Header.Add("Tailscale-User-Login", "owner@example.com") }, 401},
		{"跨站", func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }, 403},
		{"瀏覽器", func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") }, 403},
		{"直接連線", func(r *http.Request) { r.RemoteAddr = "100.64.0.1:1111" }, 403},
		{"本機Host", func(r *http.Request) { r.Host = "127.0.0.1:17733" }, 403},
		{"未知版本", func(r *http.Request) { r.Header.Set(ShortcutClientHeader, "v3") }, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := &countingBackend{}
			h := New(Options{Config: staticConfig{config.Config{Version: 1, DeviceName: "pc", PairingToken: token}}, Clipboard: backend, Owner: func(context.Context) (string, string, error) { return "owner@example.com", "pc.example.ts.net", nil }}).Handler()
			r := httptest.NewRequest("GET", "https://pc.example.ts.net/v1/status", nil)
			r.RemoteAddr = "127.0.0.1:1234"
			r.Header.Set(ShortcutClientHeader, ShortcutClientVersion)
			r.Header.Set("Tailscale-User-Login", "owner@example.com")
			tc.mutate(r)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if tc.status != 200 && backend.calls != 0 {
				t.Fatal("拒絕請求仍觸碰剪貼簿")
			}
		})
	}
}

func TestIdentityOwnerRefreshedAndEncodedLogin(t *testing.T) {
	backend := &countingBackend{}
	fail := false
	h := New(Options{Config: staticConfig{}, Clipboard: backend, Owner: func(context.Context) (string, string, error) {
		if fail {
			return "", "", errors.New("offline")
		}
		return "使用者@example.com", "pc.example.ts.net", nil
	}}).Handler()
	for _, want := range []int{200, 403} {
		r := httptest.NewRequest("GET", "https://pc.example.ts.net/v1/status", nil)
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set(ShortcutClientHeader, ShortcutClientVersion)
		r.Header.Set("Tailscale-User-Login", "=?utf-8?q?=E4=BD=BF=E7=94=A8=E8=80=85@example.com?=")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		fail = true
	}
	if backend.calls != 1 {
		t.Fatal("帳號失效後仍被快取授權")
	}
}
