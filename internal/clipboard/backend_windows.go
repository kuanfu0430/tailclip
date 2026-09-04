//go:build windows

package clipboard

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
	maxOpenWait   = time.Second
)

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procOpenClipboard     = user32.NewProc("OpenClipboard")
	procCloseClipboard    = user32.NewProc("CloseClipboard")
	procEmptyClipboard    = user32.NewProc("EmptyClipboard")
	procGetClipboardData  = user32.NewProc("GetClipboardData")
	procSetClipboardData  = user32.NewProc("SetClipboardData")
	procIsFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procGlobalAlloc       = kernel32.NewProc("GlobalAlloc")
	procGlobalFree        = kernel32.NewProc("GlobalFree")
	procGlobalLock        = kernel32.NewProc("GlobalLock")
	procGlobalUnlock      = kernel32.NewProc("GlobalUnlock")
	procGlobalSize        = kernel32.NewProc("GlobalSize")
	procMoveMemory        = kernel32.NewProc("RtlMoveMemory")
)

type windowsBackend struct{}

func NewSystemBackend() Backend { return windowsBackend{} }

func (windowsBackend) Available(ctx context.Context) error {
	if err := openClipboard(ctx); err != nil {
		return err
	}
	closeClipboard()
	return nil
}

func (windowsBackend) ReadText(ctx context.Context) (string, error) {
	if err := openClipboard(ctx); err != nil {
		return "", err
	}
	defer closeClipboard()

	available, _, _ := procIsFormatAvailable.Call(cfUnicodeText)
	if available == 0 {
		return "", ErrNoText
	}
	handle, _, callErr := procGetClipboardData.Call(cfUnicodeText)
	if handle == 0 {
		return "", fmt.Errorf("讀取 Windows 剪貼簿失敗: %w", normalizeCallError(callErr))
	}
	size, _, callErr := procGlobalSize.Call(handle)
	if size < 2 {
		if size == 0 && !errors.Is(normalizeCallError(callErr), syscall.Errno(0)) {
			return "", fmt.Errorf("取得 Windows 剪貼簿大小失敗: %w", normalizeCallError(callErr))
		}
		return "", ErrNoText
	}
	pointer, _, callErr := procGlobalLock.Call(handle)
	if pointer == 0 {
		return "", fmt.Errorf("鎖定 Windows 剪貼簿資料失敗: %w", normalizeCallError(callErr))
	}
	defer procGlobalUnlock.Call(handle)

	units := make([]uint16, int(size/2))
	procMoveMemory.Call(uintptr(unsafe.Pointer(&units[0])), pointer, uintptr(len(units)*2))
	end := 0
	for end < len(units) && units[end] != 0 {
		end++
	}
	if end == 0 {
		return "", ErrNoText
	}
	return string(utf16.Decode(units[:end])), nil
}

func (windowsBackend) WriteText(ctx context.Context, text string) error {
	units := utf16.Encode([]rune(text))
	units = append(units, 0)
	size := uintptr(len(units) * 2)
	handle, _, callErr := procGlobalAlloc.Call(gmemMoveable, size)
	if handle == 0 {
		return fmt.Errorf("配置 Windows 剪貼簿記憶體失敗: %w", normalizeCallError(callErr))
	}
	owned := true
	defer func() {
		if owned {
			procGlobalFree.Call(handle)
		}
	}()

	pointer, _, callErr := procGlobalLock.Call(handle)
	if pointer == 0 {
		return fmt.Errorf("鎖定 Windows 剪貼簿記憶體失敗: %w", normalizeCallError(callErr))
	}
	procMoveMemory.Call(pointer, uintptr(unsafe.Pointer(&units[0])), size)
	procGlobalUnlock.Call(handle)

	// 先完整準備資料，再開啟及清空剪貼簿；記憶體配置失敗時不破壞原內容。
	if err := openClipboard(ctx); err != nil {
		return err
	}
	defer closeClipboard()
	if result, _, callErr := procEmptyClipboard.Call(); result == 0 {
		return fmt.Errorf("清空 Windows 剪貼簿失敗: %w", normalizeCallError(callErr))
	}

	if result, _, callErr := procSetClipboardData.Call(cfUnicodeText, handle); result == 0 {
		return fmt.Errorf("寫入 Windows 剪貼簿失敗: %w", normalizeCallError(callErr))
	}
	owned = false // SetClipboardData 成功後由系統接管 handle。
	return nil
}

func openClipboard(ctx context.Context) error {
	deadline := time.Now().Add(maxOpenWait)
	delay := 10 * time.Millisecond
	for {
		if result, _, _ := procOpenClipboard.Call(0); result != 0 {
			return nil
		}
		if !time.Now().Before(deadline) {
			return ErrBusy
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		if delay < 160*time.Millisecond {
			delay *= 2
		}
	}
}

func closeClipboard() {
	procCloseClipboard.Call()
}

func normalizeCallError(err error) error {
	if err == nil || errors.Is(err, syscall.Errno(0)) {
		return errors.New("未知 Win32 錯誤")
	}
	return err
}
