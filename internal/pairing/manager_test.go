package pairing

import (
	"encoding/base64"
	"testing"
	"time"
)

func validData() Data {
	return Data{
		Version:         1,
		BaseURL:         "https://work-pc.example.ts.net/tailblink/v1",
		Token:           "secret",
		DeviceName:      "工作電腦",
		TailscaleDevice: "work-pc",
	}
}

func TestSessionEntropyAndExpiry(t *testing.T) {
	m := NewManager()
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	session, err := m.Create(validData())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(session.Nonce)
	if err != nil || len(raw) != 24 {
		t.Fatalf("nonce 必須有 192-bit，得到 %d bytes", len(raw))
	}
	if _, ok := m.Get(session.Nonce); !ok {
		t.Fatal("有效 session 應可讀取")
	}
	now = now.Add(SessionLifetime + time.Second)
	if _, ok := m.Get(session.Nonce); ok {
		t.Fatal("過期 session 不應可讀取")
	}
}

func TestRejectUnsafeBaseURL(t *testing.T) {
	data := validData()
	for _, rawURL := range []string{
		"http://work-pc.example.ts.net/tailblink/v1",
		"https://example.com/tailblink/v1",
		"https://work-pc.example.ts.net/other",
		"https://work-pc.example.ts.net/tailblink/v1?token=x",
	} {
		data.BaseURL = rawURL
		if err := ValidateData(data); err == nil {
			t.Fatalf("應拒絕 %s", rawURL)
		}
	}
}
