package clipboard

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"
)

const maxOpenWait = time.Second

// nativeSession 將依賴 OS 執行緒的擁有者視窗、Open、操作及 Close 包成單一交易。
// 呼叫端仍須序列化交易；函式注入讓相同生命週期能在無 Windows 的環境回歸測試。
type nativeSession struct {
	createOwner  func() (uintptr, error)
	destroyOwner func(uintptr) error
	open         func(uintptr) error
	close        func() error
}

func (s nativeSession) run(ctx context.Context, operation func() error) (err error) {
	// Mutex 只能避免同時操作，無法避免 Go 將 goroutine 移到別條 OS thread。
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err = ctx.Err(); err != nil {
		return err
	}
	owner, err := s.createOwner()
	if err != nil {
		return err
	}
	defer func() { err = sessionCleanupError(err, s.destroyOwner(owner)) }()
	if err = retryOpenClipboard(ctx, maxOpenWait, func() error { return s.open(owner) }); err != nil {
		return err
	}
	defer func() { err = sessionCleanupError(err, s.close()) }()
	if err = ctx.Err(); err != nil {
		return err
	}
	return operation()
}

func sessionCleanupError(operationErr, cleanupErr error) error {
	if cleanupErr == nil {
		return operationErr
	}
	// 清理失敗優先；不可讓 ErrNoText/ErrBusy 令 API 把未釋放資源當成正常空值。
	return fmt.Errorf("剪貼簿交易清理失敗（原操作：%v）：%w", operationErr, cleanupErr)
}

func retryOpenClipboard(ctx context.Context, limit time.Duration, open func() error) error {
	deadline := time.Now().Add(limit)
	delay := 10 * time.Millisecond
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !time.Now().Before(deadline) {
			return ErrBusy
		}
		if err := open(); !errors.Is(err, ErrBusy) {
			return err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return ErrBusy
		}
		timer := time.NewTimer(min(delay, remaining))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		delay = min(delay*2, 160*time.Millisecond)
	}
}
