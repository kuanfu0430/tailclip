//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmClose              = 0x0010
	wmDestroy            = 0x0002
	wmNull               = 0x0000
	wmApp                = 0x8000
	wmLButtonDoubleClick = 0x0203
	wmRightButtonUp      = 0x0205
	wmContextMenu        = 0x007B

	trayCallbackMessage = wmApp + 1
	trayIconID          = 1
	trayMenuOpen        = 1001
	trayMenuAutostart   = 1002
	trayMenuExit        = 1003

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004
	nimAdd     = 0x00000000
	nimDelete  = 0x00000002

	mfString    = 0x00000000
	mfChecked   = 0x00000008
	mfSeparator = 0x00000800

	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100

	idiApplication = 32512
)

var (
	trayUser32                 = windows.NewLazySystemDLL("user32.dll")
	trayShell32                = windows.NewLazySystemDLL("shell32.dll")
	procRegisterClassExW       = trayUser32.NewProc("RegisterClassExW")
	procUnregisterClassW       = trayUser32.NewProc("UnregisterClassW")
	procCreateWindowExW        = trayUser32.NewProc("CreateWindowExW")
	procDefWindowProcW         = trayUser32.NewProc("DefWindowProcW")
	procDestroyWindow          = trayUser32.NewProc("DestroyWindow")
	procGetMessageW            = trayUser32.NewProc("GetMessageW")
	procTranslateMessage       = trayUser32.NewProc("TranslateMessage")
	procDispatchMessageW       = trayUser32.NewProc("DispatchMessageW")
	procPostMessageW           = trayUser32.NewProc("PostMessageW")
	procPostQuitMessage        = trayUser32.NewProc("PostQuitMessage")
	procLoadIconW              = trayUser32.NewProc("LoadIconW")
	procRegisterWindowMessageW = trayUser32.NewProc("RegisterWindowMessageW")
	procCreatePopupMenu        = trayUser32.NewProc("CreatePopupMenu")
	procAppendMenuW            = trayUser32.NewProc("AppendMenuW")
	procTrackPopupMenu         = trayUser32.NewProc("TrackPopupMenu")
	procDestroyMenu            = trayUser32.NewProc("DestroyMenu")
	procGetCursorPos           = trayUser32.NewProc("GetCursorPos")
	procSetForegroundWindow    = trayUser32.NewProc("SetForegroundWindow")
	procShellNotifyIconW       = trayShell32.NewProc("Shell_NotifyIconW")
	procGetModuleHandleW       = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW")
	trayWindowProcedure        = windows.NewCallback(trayWindowProc)
	trayInstances              sync.Map
)

type trayPoint struct {
	X int32
	Y int32
}

type trayMessage struct {
	Window  uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   trayPoint
}

type trayWindowClass struct {
	Size        uint32
	Style       uint32
	WindowProc  uintptr
	ClassExtra  int32
	WindowExtra int32
	Instance    uintptr
	Icon        uintptr
	Cursor      uintptr
	Background  uintptr
	MenuName    *uint16
	ClassName   *uint16
	SmallIcon   uintptr
}

type trayNotifyIconData struct {
	Size             uint32
	Window           uintptr
	ID               uint32
	Flags            uint32
	Callback         uint32
	Icon             uintptr
	Tip              [128]uint16
	State            uint32
	StateMask        uint32
	Info             [256]uint16
	TimeoutOrVersion uint32
	InfoTitle        [64]uint16
	InfoFlags        uint32
	GUID             windows.GUID
	BalloonIcon      uintptr
}

type windowsTray struct {
	executable      string
	shutdown        context.CancelFunc
	window          atomic.Uintptr
	openingSettings atomic.Bool
	taskbarCreated  uint32
	icon            uintptr
}

func startDesktopUI(ctx context.Context, executable string, shutdown context.CancelFunc) (desktopUI, error) {
	tray := &windowsTray{executable: executable, shutdown: shutdown}
	ready := make(chan error, 1)
	done := make(chan error, 1)
	closed := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		err := tray.run(ctx, ready)
		done <- err
		close(closed)
	}()
	if err := <-ready; err != nil {
		return desktopUI{}, err
	}
	return desktopUI{
		done: done,
		close: func() {
			tray.requestClose()
			select {
			case <-closed:
			case <-time.After(2 * time.Second):
			}
		},
	}, nil
}

func (tray *windowsTray) run(ctx context.Context, ready chan<- error) error {
	instance, _, callErr := procGetModuleHandleW.Call(0)
	if instance == 0 {
		err := windowsCallError("無法取得 TailClip 程序模組", callErr)
		ready <- err
		return err
	}
	className, err := windows.UTF16PtrFromString(fmt.Sprintf("TailClipTrayWindow.%d", os.Getpid()))
	if err != nil {
		ready <- err
		return err
	}
	windowName, err := windows.UTF16PtrFromString("TailClip")
	if err != nil {
		ready <- err
		return err
	}
	windowClass := trayWindowClass{
		Size:       uint32(unsafe.Sizeof(trayWindowClass{})),
		WindowProc: trayWindowProcedure,
		Instance:   instance,
		ClassName:  className,
	}
	atom, _, callErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&windowClass)))
	if atom == 0 {
		err := windowsCallError("無法註冊 TailClip 通知區視窗", callErr)
		ready <- err
		return err
	}
	defer procUnregisterClassW.Call(uintptr(unsafe.Pointer(className)), instance)

	window, _, callErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		0,
		0, 0, 0, 0,
		0, 0, instance, 0,
	)
	if window == 0 {
		err := windowsCallError("無法建立 TailClip 通知區視窗", callErr)
		ready <- err
		return err
	}
	tray.window.Store(window)
	trayInstances.Store(window, tray)
	defer func() {
		if current := tray.window.Load(); current != 0 {
			procDestroyWindow.Call(current)
		}
		trayInstances.Delete(window)
		tray.window.Store(0)
	}()

	icon, _, callErr := procLoadIconW.Call(0, idiApplication)
	if icon == 0 {
		err := windowsCallError("無法載入 TailClip 通知區圖示", callErr)
		ready <- err
		return err
	}
	tray.icon = icon
	taskbarCreatedName, _ := windows.UTF16PtrFromString("TaskbarCreated")
	message, _, callErr := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(taskbarCreatedName)))
	if message == 0 {
		err := windowsCallError("無法監聽 Windows 通知區重啟", callErr)
		ready <- err
		return err
	}
	tray.taskbarCreated = uint32(message)
	if err := tray.addIcon(); err != nil {
		ready <- err
		return err
	}
	ready <- nil

	watcherDone := make(chan struct{})
	defer close(watcherDone)
	go func() {
		select {
		case <-ctx.Done():
			tray.requestClose()
		case <-watcherDone:
		}
	}()

	var messageData trayMessage
	for {
		result, _, callErr := procGetMessageW.Call(uintptr(unsafe.Pointer(&messageData)), 0, 0, 0)
		switch int32(result) {
		case -1:
			return windowsCallError("TailClip 通知區訊息迴圈失敗", callErr)
		case 0:
			return nil
		default:
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&messageData)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&messageData)))
		}
	}
}

func (tray *windowsTray) addIcon() error {
	window := tray.window.Load()
	if window == 0 {
		return errors.New("TailClip 通知區視窗尚未就緒")
	}
	data := trayNotifyIconData{
		Size:     uint32(unsafe.Sizeof(trayNotifyIconData{})),
		Window:   window,
		ID:       trayIconID,
		Flags:    nifMessage | nifIcon | nifTip,
		Callback: trayCallbackMessage,
		Icon:     tray.icon,
	}
	tip, _ := windows.UTF16FromString("TailClip（按兩下開啟）")
	copy(data.Tip[:], tip)
	result, _, callErr := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&data)))
	if result == 0 {
		return windowsCallError("無法加入 TailClip 通知區圖示", callErr)
	}
	return nil
}

func (tray *windowsTray) removeIcon() {
	window := tray.window.Load()
	if window == 0 {
		return
	}
	data := trayNotifyIconData{Size: uint32(unsafe.Sizeof(trayNotifyIconData{})), Window: window, ID: trayIconID}
	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&data)))
}

func (tray *windowsTray) requestClose() {
	if window := tray.window.Load(); window != 0 {
		procPostMessageW.Call(window, wmClose, 0, 0)
	}
}

func trayWindowProc(window uintptr, message uint32, wParam, lParam uintptr) uintptr {
	value, ok := trayInstances.Load(window)
	if !ok {
		result, _, _ := procDefWindowProcW.Call(window, uintptr(message), wParam, lParam)
		return result
	}
	tray := value.(*windowsTray)
	if message == tray.taskbarCreated {
		if err := tray.addIcon(); err != nil {
			showFatal(err)
		}
		return 0
	}
	switch message {
	case trayCallbackMessage:
		switch uint32(lParam) {
		case wmLButtonDoubleClick:
			tray.openSetupPage()
		case wmRightButtonUp, wmContextMenu:
			tray.showMenu()
		}
		return 0
	case wmClose:
		procDestroyWindow.Call(window)
		return 0
	case wmDestroy:
		tray.removeIcon()
		trayInstances.Delete(window)
		tray.window.Store(0)
		procPostQuitMessage.Call(0)
		return 0
	default:
		result, _, _ := procDefWindowProcW.Call(window, uintptr(message), wParam, lParam)
		return result
	}
}

func (tray *windowsTray) showMenu() {
	autostart, err := autostartEnabled(tray.executable)
	if err != nil {
		showFatal(err)
		return
	}
	menu, _, callErr := procCreatePopupMenu.Call()
	if menu == 0 {
		showFatal(windowsCallError("無法建立 TailClip 選單", callErr))
		return
	}
	defer procDestroyMenu.Call(menu)
	if err := appendTrayMenu(menu, mfString, trayMenuOpen, "開啟連線與配對頁面"); err != nil {
		showFatal(err)
		return
	}
	autostartFlags := uintptr(mfString)
	if autostart {
		autostartFlags |= mfChecked
	}
	if err := appendTrayMenu(menu, autostartFlags, trayMenuAutostart, "登入 Windows 後自動啟動"); err != nil {
		showFatal(err)
		return
	}
	if err := appendTrayMenu(menu, mfSeparator, 0, ""); err != nil {
		showFatal(err)
		return
	}
	if err := appendTrayMenu(menu, mfString, trayMenuExit, "結束 TailClip"); err != nil {
		showFatal(err)
		return
	}
	var point trayPoint
	result, _, callErr := procGetCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	if result == 0 {
		showFatal(windowsCallError("無法取得滑鼠位置", callErr))
		return
	}
	window := tray.window.Load()
	procSetForegroundWindow.Call(window)
	command, _, _ := procTrackPopupMenu.Call(menu, tpmRightButton|tpmReturnCmd, uintptr(point.X), uintptr(point.Y), 0, window, 0)
	procPostMessageW.Call(window, wmNull, 0, 0)
	switch command {
	case trayMenuOpen:
		tray.openSetupPage()
	case trayMenuAutostart:
		if autostart {
			err = removeAutostart()
		} else {
			err = ensureAutostart(tray.executable)
		}
		if err != nil {
			showFatal(err)
		}
	case trayMenuExit:
		tray.shutdown()
		tray.requestClose()
	}
}

func appendTrayMenu(menu, flags, id uintptr, label string) error {
	var labelPointer *uint16
	if label != "" {
		var err error
		labelPointer, err = windows.UTF16PtrFromString(label)
		if err != nil {
			return err
		}
	}
	result, _, callErr := procAppendMenuW.Call(menu, flags, id, uintptr(unsafe.Pointer(labelPointer)))
	if result == 0 {
		return windowsCallError("無法建立 TailClip 選單項目", callErr)
	}
	return nil
}

func (tray *windowsTray) openSetupPage() {
	if !tray.openingSettings.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer tray.openingSettings.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := openSettings(ctx, tray.executable); err != nil {
			showFatal(err)
		}
	}()
}

func windowsCallError(operation string, err error) error {
	if err == nil || err == syscall.Errno(0) {
		return errors.New(operation)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
