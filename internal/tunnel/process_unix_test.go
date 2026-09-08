//go:build !windows

package tunnel

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProcessLifecycle(t *testing.T) {
	script := filepath.Join(t.TempDir(), "fake-cloudflared")
	// exec 讓 shell 被同一 PID 的長睡眠取代，關閉程序不遺留子程序。
	os.WriteFile(script, []byte("#!/bin/sh\necho https://test-process.trycloudflare.com\nexec sleep 60\n"), 0700)
	h, err := startBinary(context.Background(), script, "127.0.0.1:123")
	if err != nil {
		t.Fatal(err)
	}
	if h.URL() != "https://test-process.trycloudflare.com" {
		t.Fatal("網址不符")
	}
	h.Close()
	h.Close()
	select {
	case <-h.Done():
	case <-time.After(time.Second):
		t.Fatal("關閉未結束")
	}
	os.WriteFile(script, []byte("#!/bin/sh\nexec sleep 60\n"), 0700)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := startBinary(ctx, script, "127.0.0.1:123"); err == nil {
		t.Fatal("未在期限內取消")
	}
}
