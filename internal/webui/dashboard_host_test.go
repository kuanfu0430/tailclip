package webui

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

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
