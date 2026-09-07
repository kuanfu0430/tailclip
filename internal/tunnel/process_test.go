package tunnel

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOutputChunksAndBoundedLogs(t *testing.T) {
	w := &urlOutput{found: make(chan string, 1)}
	for range 10 {
		w.Write(make([]byte, 4096))
	}
	if len(w.buffer) > 8192 {
		t.Fatal("日誌無上限")
	}
	w.Write([]byte("https://quiet-river.trycloud"))
	w.Write([]byte("flare.com |"))
	if got := <-w.found; got != "https://quiet-river.trycloudflare.com" {
		t.Fatal(got)
	}
	w.Write([]byte("https://other.trycloudflare.com"))
	if len(w.found) != 0 || w.buffer != "" {
		t.Fatal("重複網址或日誌未清除")
	}
}
func TestRejectNonLoopbackAndInvalidBinary(t *testing.T) {
	for _, address := range []string{"0.0.0.0:123", "localhost:123", "192.168.1.1:123"} {
		if _, err := startBinary(context.Background(), "missing", address); err == nil {
			t.Fatal(address)
		}
	}
	if _, err := startBinary(context.Background(), filepath.Join(t.TempDir(), "missing"), "127.0.0.1:123"); err == nil {
		t.Fatal("缺檔成功")
	}
	b, err := CurrentBinary()
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, b.Filename)
	os.WriteFile(file, []byte("modified"), 0700)
	if Verify(file) == nil {
		t.Fatal("接受受損依賴")
	}
	if InstallCompanion(dir, t.TempDir()) == nil {
		t.Fatal("安裝未驗證依賴")
	}
}
