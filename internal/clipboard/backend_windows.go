//go:build windows

package clipboard

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
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
	procCreateWindowExW   = user32.NewProc("CreateWindowExW")
	procDestroyWindow     = user32.NewProc("DestroyWindow")
)

type windowsBackend struct{}

func NewSystemBackend() Backend { return windowsBackend{} }

func (windowsBackend) Available(ctx context.Context) error {
	return windowsClipboardSession.run(ctx, func() error { return nil })
}

func (windowsBackend) ReadText(ctx context.Context) (string, error) {
	var units []uint16
	err := windowsClipboardSession.run(ctx, func() error {
		available, _, _ := procIsFormatAvailable.Call(cfUnicodeText)
		if available == 0 {
			return ErrNoText
		}
		handle, _, callErr := procGetClipboardData.Call(cfUnicodeText)
		if handle == 0 {
			return fmt.Errorf("讀取 Windows 剪貼簿失敗: %w", normalizeCallError(callErr))
		}
		size, _, callErr := procGlobalSize.Call(handle)
		if size == 0 {
			return fmt.Errorf("取得 Windows 剪貼簿大小失敗: %w", normalizeCallError(callErr))
		}
		if size < 2 {
			return ErrNoText
		}
		pointer, _, callErr := procGlobalLock.Call(handle)
		if pointer == 0 {
			return fmt.Errorf("鎖定 Windows 剪貼簿資料失敗: %w", normalizeCallError(callErr))
		}
		defer procGlobalUnlock.Call(handle)
		units = make([]uint16, int(size/2))
		procMoveMemory.Call(uintptr(unsafe.Pointer(&units[0])), pointer, uintptr(len(units)*2))
		return nil
	})
	if err != nil {
		return "", err
	}
	// 複製完即釋放剪貼簿，解碼不占用 Windows 的全域鎖。
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
	if err := ctx.Err(); err != nil {
		return err
	}
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
	return windowsClipboardSession.run(ctx, func() error {
		if result, _, callErr := procEmptyClipboard.Call(); result == 0 {
			return fmt.Errorf("清空 Windows 剪貼簿失敗: %w", normalizeCallError(callErr))
		}
		if result, _, callErr := procSetClipboardData.Call(cfUnicodeText, handle); result == 0 {
			return fmt.Errorf("寫入 Windows 剪貼簿失敗: %w", normalizeCallError(callErr))
		}
		owned = false // SetClipboardData 成功後由系統接管 handle。
		return nil
	})
}

var windowsClipboardSession = nativeSession{
	createOwner: createClipboardOwner,
	destroyOwner: func(owner uintptr) error {
		if result, _, err := procDestroyWindow.Call(owner); result == 0 {
			return fmt.Errorf("釋放 Windows 剪貼簿視窗失敗: %w", normalizeCallError(err))
		}
		return nil
	},
	open: func(owner uintptr) error {
		if result, _, err := procOpenClipboard.Call(owner); result == 0 {
			if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
				return ErrBusy
			}
			return fmt.Errorf("開啟 Windows 剪貼簿失敗: %w", normalizeCallError(err))
		}
		return nil
	},
	close: func() error {
		if result, _, err := procCloseClipboard.Call(); result == 0 {
			return fmt.Errorf("關閉 Windows 剪貼簿失敗: %w", normalizeCallError(err))
		}
		return nil
	},
}

func createClipboardOwner() (uintptr, error) {
	// 標準 STATIC class 的 message-only 視窗，無 UI、callback 或延遲呈現內容。
	// OpenClipboard(NULL) 無法提供 EmptyClipboard/SetClipboardData 所需的有效 owner。
	className, _ := windows.UTF16PtrFromString("STATIC")
	owner, _, err := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)), 0, 0,
		0, 0, 0, 0, ^uintptr(2), 0, 0, 0) // HWND_MESSAGE = -3
	if owner == 0 {
		return 0, fmt.Errorf("建立 Windows 剪貼簿視窗失敗: %w", normalizeCallError(err))
	}
	return owner, nil
}

func normalizeCallError(err error) error {
	if err == nil || errors.Is(err, syscall.Errno(0)) {
		return errors.New("未知 Win32 錯誤")
	}
	return err
}
