//go:build windows

package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/kuanfu0430/tailclip/internal/config"
	"github.com/kuanfu0430/tailclip/internal/tailscale"
	"github.com/kuanfu0430/tailclip/internal/tunnel"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	mbOK              = 0x00000000
	mbYesNo           = 0x00000004
	mbIconInformation = 0x00000040
	mbIconError       = 0x00000010
	idYes             = 6
	createNoWindow    = 0x08000000
	autostartKeyPath  = `Software\Microsoft\Windows\CurrentVersion\Run`
	autostartValue    = "TailClip"
)

var messageBox = windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")

type windowsTailscaleClient interface {
	Status(context.Context) (tailscale.Status, error)
	ServeConfigured(context.Context) (bool, error)
}

type windowsInstaller struct {
	installPath     func() (string, error)
	copyExecutable  func(string, string) error
	ensureAutostart func(string) error
	ensureAgent     func(context.Context, string) error
	newTailscale    func() (windowsTailscaleClient, error)
	confirmHTTPS    func(string) (bool, error)
	configureServe  func(context.Context) error
	runElevated     func(string, string) error
	openSettings    func(context.Context, string) error
	skipTailscale   func() (bool, error)
}

func defaultWindowsInstaller() windowsInstaller {
	return windowsInstaller{
		installPath:     windowsInstallPath,
		copyExecutable:  installWindowsFiles,
		ensureAutostart: ensureAutostart,
		ensureAgent:     ensureAgent,
		newTailscale: func() (windowsTailscaleClient, error) {
			return tailscale.New()
		},
		confirmHTTPS:   confirmHTTPS,
		configureServe: configureServe,
		runElevated:    runElevated,
		openSettings:   openSettings,
		skipTailscale: func() (bool, error) {
			path, err := config.DefaultPath()
			if err != nil {
				return false, err
			}
			cfg, _, err := config.LoadOrCreate(path)
			return cfg.Mode() != "tailscale", err
		},
	}
}

func installWindowsFiles(source, target string) error {
	if err := tunnel.InstallCompanion(filepath.Dir(source), filepath.Dir(target)); err != nil {
		return err
	}
	return copyExecutable(source, target)
}

func defaultAction(ctx context.Context, executable string) error {
	return defaultWindowsInstaller().run(ctx, executable)
}

func (installer windowsInstaller) run(ctx context.Context, executable string) error {
	target, err := installer.installPath()
	if err != nil {
		return err
	}
	current, err := filepath.Abs(executable)
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Clean(current), filepath.Clean(target)) {
		if err := installer.copyExecutable(current, target); err != nil {
			return err
		}
	}

	// 保留由檔案總管啟動的前景程序，避免安裝交給隱藏子程序後看似沒有反應。
	finishErr := installer.finish(ctx, target)
	cleanupErr := removeExecutableBackup(target)
	if finishErr != nil {
		return finishErr
	}
	return cleanupErr
}

func finishInstall(ctx context.Context, executable string) error {
	return defaultWindowsInstaller().finish(ctx, executable)
}

func (installer windowsInstaller) finish(ctx context.Context, executable string) error {
	if err := installer.ensureAutostart(executable); err != nil {
		return err
	}
	if err := installer.ensureAgent(ctx, executable); err != nil {
		return err
	}
	if installer.skipTailscale != nil {
		skip, err := installer.skipTailscale()
		if err != nil {
			return err
		}
		if skip {
			return installer.openSettings(ctx, executable)
		}
	}
	if err := installer.prepare(ctx, executable); err != nil {
		return err
	}
	return installer.openSettings(ctx, executable)
}

// 共用既有的名稱告知及按需 UAC 流程。
func prepareTailnet(ctx context.Context) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	return defaultWindowsInstaller().prepare(ctx, executable)
}

func (installer windowsInstaller) prepare(ctx context.Context, executable string) error {
	client, err := installer.newTailscale()
	if err != nil {
		return err
	}
	status, err := client.Status(ctx)
	if err != nil {
		return err
	}
	configured, err := client.ServeConfigured(ctx)
	if err != nil {
		return err
	}
	if configured {
		return nil
	}

	accepted, err := installer.confirmHTTPS(status.Self.DNSName)
	if err != nil {
		return err
	}
	if !accepted {
		return errors.New("已取消 HTTPS 設定；再次雙擊 TailClip 即可繼續")
	}
	if directErr := installer.configureServe(ctx); directErr != nil {
		if errors.Is(directErr, tailscale.ErrNotInstalled) ||
			errors.Is(directErr, tailscale.ErrNotRunning) ||
			errors.Is(directErr, tailscale.ErrServeConflict) ||
			errors.Is(directErr, context.Canceled) || errors.Is(directErr, context.DeadlineExceeded) {
			return directErr
		}
		if err := installer.runElevated(executable, "serve-install"); err != nil {
			return fmt.Errorf("目前使用者設定 Tailscale Serve 失敗（%v）；系統管理員權限重試也未完成: %w", directErr, err)
		}
	}
	return nil
}

func uninstallAction(ctx context.Context, executable string) error {
	confirmed, err := message("解除安裝 TailClip", "將移除 TailClip 的自啟、/tailclip Serve path、設定與日誌。\n\n不會修改 Tailscale 的其他 Serve 設定。確定繼續嗎？", mbYesNo|mbIconInformation)
	if err != nil {
		return err
	}
	if confirmed != idYes {
		return nil
	}
	target, err := windowsInstallPath()
	if err != nil {
		return err
	}
	if localHealth(ctx) {
		_ = stopAgent(ctx)
		time.Sleep(300 * time.Millisecond)
	}
	helper := target
	if _, err := os.Stat(helper); err != nil {
		helper = executable
	}
	if _, err := tailscale.New(); !errors.Is(err, tailscale.ErrNotInstalled) {
		if err != nil {
			return err
		}
		if err := runElevated(helper, "serve-remove"); err != nil {
			return fmt.Errorf("Serve path 未移除，因此解除安裝已安全停止: %w", err)
		}
	}
	if err := removeAutostart(); err != nil {
		return err
	}
	if err := scheduleRemove(filepath.Dir(target)); err != nil {
		return err
	}
	_, _ = message("TailClip", "解除安裝已完成。TailClip 檔案會在視窗關閉後移除。", mbOK|mbIconInformation)
	return nil
}

func windowsInstallPath() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return "", errors.New("找不到 LOCALAPPDATA")
	}
	return filepath.Join(base, "TailClip", "TailClip.exe"), nil
}

func copyExecutable(source, target string) error {
	same, err := sameFileContent(source, target)
	if err == nil && same {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return fmt.Errorf("無法建立安裝目錄: %w", err)
	}
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("無法讀取 TailClip.exe: %w", err)
	}
	defer input.Close()
	temporary := target + ".new"
	output, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o700)
	if err != nil {
		return fmt.Errorf("無法準備安裝檔: %w", err)
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		_ = os.Remove(temporary)
		return fmt.Errorf("無法複製 TailClip.exe: %w", err)
	}
	if err := output.Sync(); err != nil {
		output.Close()
		_ = os.Remove(temporary)
		return err
	}
	if err := output.Close(); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	backup := target + ".old"
	targetExists := false
	if _, statErr := os.Stat(target); statErr == nil {
		targetExists = true
		_ = os.Remove(backup)
		if err := os.Rename(target, backup); err != nil {
			_ = os.Remove(temporary)
			return fmt.Errorf("無法更新安裝檔；請先結束正在執行的 TailClip 後再試: %w", err)
		}
	}
	if err := os.Rename(temporary, target); err != nil {
		if targetExists {
			_ = os.Rename(backup, target)
		}
		_ = os.Remove(temporary)
		return fmt.Errorf("無法套用安裝檔: %w", err)
	}
	_ = os.Remove(backup)
	return nil
}

func removeExecutableBackup(target string) error {
	backup := target + ".old"
	deadline := time.Now().Add(3 * time.Second)
	for {
		err := os.Remove(backup)
		if err == nil || errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("新版 TailClip 已啟動，但無法移除舊執行檔 %s: %w", backup, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func sameFileContent(left, right string) (bool, error) {
	leftData, err := os.ReadFile(left)
	if err != nil {
		return false, err
	}
	rightData, err := os.ReadFile(right)
	if err != nil {
		return false, err
	}
	return sha256.Sum256(leftData) == sha256.Sum256(rightData), nil
}

func ensureAutostart(executable string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, autostartKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("無法建立使用者自啟: %w", err)
	}
	defer key.Close()
	return key.SetStringValue(autostartValue, autostartCommand(executable))
}

func autostartEnabled(executable string) (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, autostartKeyPath, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("無法讀取使用者自啟設定: %w", err)
	}
	defer key.Close()
	value, _, err := key.GetStringValue(autostartValue)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("無法讀取使用者自啟設定: %w", err)
	}
	return strings.EqualFold(strings.TrimSpace(value), autostartCommand(executable)), nil
}

func autostartCommand(executable string) string {
	return syscall.EscapeArg(executable) + " agent"
}

func removeAutostart() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, autostartKeyPath, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("無法開啟使用者自啟設定: %w", err)
	}
	defer key.Close()
	if err := key.DeleteValue(autostartValue); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("無法移除使用者自啟: %w", err)
	}
	return nil
}

func runElevated(executable string, argument string) error {
	script := "$p=Start-Process -FilePath '" + powerShellLiteral(executable) + "' -ArgumentList @('" + powerShellLiteral(argument) + "') -Verb RunAs -Wait -PassThru; exit $p.ExitCode"
	command := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	if err := command.Run(); err != nil {
		return fmt.Errorf("系統管理員權限操作未完成: %w", err)
	}
	return nil
}

func scheduleRemove(directory string) error {
	script := "Start-Sleep -Seconds 1; Remove-Item -LiteralPath '" + powerShellLiteral(directory) + "' -Recurse -Force"
	command := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return command.Start()
}

func powerShellLiteral(value string) string { return strings.ReplaceAll(value, "'", "''") }

func confirmHTTPS(dnsName string) (bool, error) {
	text := "TailClip 將為下列私有 tailnet 位址啟用 HTTPS：\n\n" + dnsName + "\n\n完整名稱會出現在公開的 Certificate Transparency 紀錄，但服務仍只在你的 tailnet 內可達。是否繼續？"
	result, err := message("TailClip HTTPS 設定", text, mbYesNo|mbIconInformation)
	return result == idYes, err
}

func startDetached(executable string, args ...string) error {
	command := exec.Command(executable, args...)
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return command.Start()
}

func openURL(url string) error {
	command := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return command.Start()
}

func showFatal(err error) { _, _ = message("TailClip", err.Error(), mbOK|mbIconError) }

func message(title, text string, flags uintptr) (uintptr, error) {
	titlePointer, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return 0, err
	}
	textPointer, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return 0, err
	}
	result, _, callErr := messageBox.Call(0, uintptr(unsafe.Pointer(textPointer)), uintptr(unsafe.Pointer(titlePointer)), flags)
	if result == 0 {
		return 0, callErr
	}
	return result, nil
}
