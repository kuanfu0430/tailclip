//go:build darwin || linux || windows

package clipboard

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"testing"
	"time"
)

func TestNativeSessionKeepsOneOSThread(t *testing.T) {
	for range 100 {
		var owner uintptr
		check := func() {
			for range 10 {
				runtime.Gosched()
				time.Sleep(time.Microsecond)
				if got := testThreadID(); got != owner {
					t.Fatalf("OS thread 從 %d 變成 %d", owner, got)
				}
			}
		}
		session := nativeSession{
			createOwner: func() (uintptr, error) { owner = testThreadID(); return owner, nil },
			open: func(window uintptr) error {
				if window != owner {
					t.Fatal("owner 不符")
				}
				check()
				return nil
			},
			close:        func() error { check(); return nil },
			destroyOwner: func(uintptr) error { check(); return nil },
		}
		if err := session.run(context.Background(), func() error { check(); return nil }); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNativeSessionAlwaysCleansUp(t *testing.T) {
	failure := errors.New("injected failure")
	for _, stage := range []string{"success", "owner", "open", "operation", "close", "destroy", "cancel-after-open", "panic"} {
		t.Run(stage, func(t *testing.T) {
			var events []string
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			step := func(name string) error {
				events = append(events, name)
				if stage == name {
					return failure
				}
				return nil
			}
			session := nativeSession{
				createOwner: func() (uintptr, error) { return 42, step("owner") },
				open: func(uintptr) error {
					if stage == "cancel-after-open" {
						cancel()
					}
					return step("open")
				},
				close:        func() error { return step("close") },
				destroyOwner: func(uintptr) error { return step("destroy") },
			}
			var err error
			var recovered any
			func() {
				defer func() { recovered = recover() }()
				err = session.run(ctx, func() error {
					if stage == "panic" {
						events = append(events, "operation")
						panic("test panic")
					}
					return step("operation")
				})
			}()
			want := []string{"owner", "open", "operation", "close", "destroy"}
			switch stage {
			case "owner":
				want = []string{"owner"}
			case "open":
				want = []string{"owner", "open", "destroy"}
			case "cancel-after-open":
				want = []string{"owner", "open", "close", "destroy"}
			}
			if !reflect.DeepEqual(events, want) {
				t.Fatalf("events=%v want=%v", events, want)
			}
			switch stage {
			case "success":
				if err != nil {
					t.Fatal(err)
				}
			case "cancel-after-open":
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case "panic":
				if recovered != "test panic" {
					t.Fatalf("panic=%v", recovered)
				}
			default:
				if !errors.Is(err, failure) {
					t.Fatalf("清理／操作錯誤遺失：%v", err)
				}
			}
		})
	}
}

func TestCleanupFailureIsNotReportedAsEmptyClipboard(t *testing.T) {
	failure := errors.New("close failed")
	err := sessionCleanupError(ErrNoText, failure)
	if errors.Is(err, ErrNoText) || !errors.Is(err, failure) {
		t.Fatal(err)
	}
}

func TestRetryOpenClipboard(t *testing.T) {
	t.Run("temporary-busy", func(t *testing.T) {
		calls := 0
		err := retryOpenClipboard(context.Background(), time.Second, func() error {
			calls++
			if calls < 3 {
				return ErrBusy
			}
			return nil
		})
		if err != nil || calls != 3 {
			t.Fatalf("calls=%d err=%v", calls, err)
		}
	})
	t.Run("other-error-not-retried", func(t *testing.T) {
		calls := 0
		failure := errors.New("not a lock failure")
		err := retryOpenClipboard(context.Background(), time.Second, func() error { calls++; return failure })
		if calls != 1 || !errors.Is(err, failure) {
			t.Fatalf("calls=%d err=%v", calls, err)
		}
	})
	t.Run("cancel-before-open", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := retryOpenClipboard(ctx, time.Second, func() error { t.Fatal("取消後仍嘗試 Open"); return nil })
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	})
	t.Run("cancel-during-wait", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := retryOpenClipboard(ctx, time.Second, func() error { cancel(); return ErrBusy })
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	})
	t.Run("deadline-caps-backoff", func(t *testing.T) {
		start := time.Now()
		err := retryOpenClipboard(context.Background(), 35*time.Millisecond, func() error { return ErrBusy })
		if !errors.Is(err, ErrBusy) {
			t.Fatal(err)
		}
		if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
			t.Fatalf("退避超出限時：%v", elapsed)
		}
	})
}
