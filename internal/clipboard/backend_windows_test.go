//go:build windows

package clipboard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

// 只在可丟棄的 Windows 工作階段啟用，避免一般 go test 改寫使用者剪貼簿。
func TestWindowsClipboardIntegration(t *testing.T) {
	if os.Getenv("TAILBLINK_WINDOWS_CLIPBOARD_TEST") != "1" {
		t.Skip("需要隔離 Windows session 與 TAILBLINK_WINDOWS_CLIPBOARD_TEST=1")
	}
	backend := NewSynchronized(NewSystemBackend())
	ctx := context.Background()
	t.Cleanup(func() {
		if err := windowsClipboardSession.run(ctx, func() error {
			if result, _, err := procEmptyClipboard.Call(); result == 0 {
				return normalizeCallError(err)
			}
			return nil
		}); err != nil {
			t.Error(err)
		}
	})
	t.Run("repeated-unicode-write-read-with-status", func(t *testing.T) {
		for i := range 100 {
			text := fmt.Sprintf("原生 Windows 🐾 %d\r\n第二行", i)
			if err := backend.WriteText(ctx, text); err != nil {
				t.Fatalf("write %d: %v", i, err)
			}
			if err := backend.Available(ctx); err != nil {
				t.Fatalf("status %d: %v", i, err)
			}
			got, err := backend.ReadText(ctx)
			if err != nil || got != text {
				t.Fatalf("read %d: %q %v", i, got, err)
			}
		}
	})
	t.Run("concurrent-status-read-write", func(t *testing.T) {
		const text = "同時查詢與傳輸 🐾"
		if err := backend.WriteText(ctx, text); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		for i := range 90 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				switch i % 3 {
				case 0:
					if err := backend.Available(ctx); err != nil {
						t.Error(err)
					}
				case 1:
					if got, err := backend.ReadText(ctx); err != nil || got != text {
						t.Errorf("got=%q err=%v", got, err)
					}
				case 2:
					if err := backend.WriteText(ctx, text); err != nil {
						t.Error(err)
					}
				}
			}()
		}
		wg.Wait()
	})
	t.Run("foreign-thread-lock-and-release", func(t *testing.T) {
		release := holdWindowsClipboard(t)
		requestCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		if err := backend.Available(requestCtx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("cancelled wait: %v", err)
		}
		release()
		if err := backend.Available(ctx); err != nil {
			t.Fatalf("lock retained after cancellation: %v", err)
		}

		release = holdWindowsClipboard(t)
		timer := time.AfterFunc(50*time.Millisecond, release)
		defer timer.Stop()
		if err := backend.Available(ctx); err != nil {
			t.Fatalf("temporary lock did not recover: %v", err)
		}
		release()
	})
	t.Run("persistent-busy-is-bounded-and-recovers", func(t *testing.T) {
		release := holdWindowsClipboard(t)
		start := time.Now()
		if err := backend.Available(ctx); !errors.Is(err, ErrBusy) {
			t.Fatalf("busy: %v", err)
		}
		if elapsed := time.Since(start); elapsed > 1500*time.Millisecond {
			t.Fatalf("wait=%v", elapsed)
		}
		release()
		if err := backend.WriteText(ctx, "解除占用後恢復"); err != nil {
			t.Fatal(err)
		}
	})
}

func holdWindowsClipboard(t *testing.T) func() {
	t.Helper()
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- windowsClipboardSession.run(context.Background(), func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	select {
	case <-entered:
	case err := <-done:
		t.Fatalf("無法建立另一執行緒的剪貼簿占用：%v", err)
	}
	stop := sync.OnceFunc(func() {
		close(release)
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	t.Cleanup(stop)
	return stop
}
