//go:build windows

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/kuanfu0430/tailclip/internal/tailscale"
	"github.com/kuanfu0430/tailclip/internal/tunnel"
)

type fakeWindowsTailscaleClient struct {
	status        tailscale.Status
	statusErr     error
	configured    bool
	configuredErr error
	calls         *[]string
}

func (client fakeWindowsTailscaleClient) Status(context.Context) (tailscale.Status, error) {
	*client.calls = append(*client.calls, "status")
	return client.status, client.statusErr
}

func (client fakeWindowsTailscaleClient) ServeConfigured(context.Context) (bool, error) {
	*client.calls = append(*client.calls, "serve-configured")
	return client.configured, client.configuredErr
}

func testWindowsInstaller(calls *[]string, client fakeWindowsTailscaleClient) windowsInstaller {
	client.calls = calls
	return windowsInstaller{
		ensureAutostart: func(string) error {
			*calls = append(*calls, "autostart")
			return nil
		},
		ensureAgent: func(context.Context, string) error {
			*calls = append(*calls, "agent")
			return nil
		},
		newTailscale: func() (windowsTailscaleClient, error) {
			*calls = append(*calls, "new-tailscale")
			return client, nil
		},
		confirmHTTPS: func(string) (bool, error) {
			*calls = append(*calls, "confirm")
			return true, nil
		},
		configureServe: func(context.Context) error {
			*calls = append(*calls, "configure-current-user")
			return nil
		},
		runElevated: func(string, string) error {
			*calls = append(*calls, "configure-elevated")
			return nil
		},
		openSettings: func(context.Context, string) error {
			*calls = append(*calls, "open-settings")
			return nil
		},
	}
}

func TestNewInstallationDoesNotRequireTailscale(t *testing.T) {
	var calls []string
	installer := testWindowsInstaller(&calls, fakeWindowsTailscaleClient{})
	installer.skipTailscale = func() (bool, error) { return true, nil }
	if err := installer.finish(context.Background(), "TailClip.exe"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"autostart", "agent", "open-settings"}) {
		t.Fatal(calls)
	}
}

func TestWindowsInstallerCopiesThenCompletesSynchronously(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "download", "TailClip.exe")
	target := filepath.Join(root, "local", "TailClip", "TailClip.exe")
	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("candidate"), 0o700); err != nil {
		t.Fatal(err)
	}

	var calls []string
	var status tailscale.Status
	status.Self.DNSName = "work.example.ts.net"
	installer := testWindowsInstaller(&calls, fakeWindowsTailscaleClient{status: status, configured: true})
	installer.installPath = func() (string, error) { return target, nil }
	installer.copyExecutable = func(source, target string) error {
		calls = append(calls, "copy")
		return copyExecutable(source, target)
	}
	installer.openSettings = func(_ context.Context, executable string) error {
		calls = append(calls, "open-settings")
		content, err := os.ReadFile(executable)
		if err != nil {
			return err
		}
		if string(content) != "candidate" {
			return errors.New("安裝完成前尚未複製執行檔")
		}
		return nil
	}

	if err := installer.run(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	want := []string{"copy", "autostart", "agent", "new-tailscale", "status", "serve-configured", "open-settings"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%v want=%v", calls, want)
	}
}

func TestWindowsInstallerRemovesUpgradeBackupAfterAgentTransition(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "download", "TailClip.exe")
	target := filepath.Join(root, "local", "TailClip", "TailClip.exe")
	backup := target + ".old"
	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("candidate"), 0o700); err != nil {
		t.Fatal(err)
	}

	var calls []string
	installer := testWindowsInstaller(&calls, fakeWindowsTailscaleClient{configured: true})
	installer.installPath = func() (string, error) { return target, nil }
	installer.copyExecutable = func(_, target string) error {
		calls = append(calls, "copy")
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte("candidate"), 0o700); err != nil {
			return err
		}
		return os.WriteFile(backup, []byte("old-agent"), 0o700)
	}
	installer.ensureAgent = func(context.Context, string) error {
		calls = append(calls, "agent")
		if _, err := os.Stat(backup); err != nil {
			return errors.New("舊執行檔在 Agent 切換前不應被清除")
		}
		return nil
	}

	if err := installer.run(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(backup); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("升級完成後仍殘留舊執行檔：%v", err)
	}
}

func TestWindowsInstallerReusesExistingServe(t *testing.T) {
	var calls []string
	var status tailscale.Status
	status.Self.DNSName = "work.example.ts.net"
	installer := testWindowsInstaller(&calls, fakeWindowsTailscaleClient{status: status, configured: true})

	if err := installer.finish(context.Background(), `C:\TailClip\TailClip.exe`); err != nil {
		t.Fatal(err)
	}
	want := []string{"autostart", "agent", "new-tailscale", "status", "serve-configured", "open-settings"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%v want=%v", calls, want)
	}
}

func TestWindowsInstallerTriesCurrentUserBeforeElevation(t *testing.T) {
	var calls []string
	var status tailscale.Status
	status.Self.DNSName = "work.example.ts.net"
	installer := testWindowsInstaller(&calls, fakeWindowsTailscaleClient{status: status})

	if err := installer.finish(context.Background(), `C:\TailClip\TailClip.exe`); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"autostart", "agent", "new-tailscale", "status", "serve-configured",
		"confirm", "configure-current-user", "open-settings",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%v want=%v", calls, want)
	}
}

func TestWindowsInstallerFallsBackToElevation(t *testing.T) {
	var calls []string
	var status tailscale.Status
	status.Self.DNSName = "work.example.ts.net"
	installer := testWindowsInstaller(&calls, fakeWindowsTailscaleClient{status: status})
	installer.configureServe = func(context.Context) error {
		calls = append(calls, "configure-current-user")
		return errors.New("access denied")
	}

	if err := installer.finish(context.Background(), `C:\TailClip\TailClip.exe`); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"autostart", "agent", "new-tailscale", "status", "serve-configured",
		"confirm", "configure-current-user", "configure-elevated", "open-settings",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%v want=%v", calls, want)
	}
}

func TestWindowsInstallerDoesNotElevateKnownStateError(t *testing.T) {
	var calls []string
	var status tailscale.Status
	status.Self.DNSName = "work.example.ts.net"
	installer := testWindowsInstaller(&calls, fakeWindowsTailscaleClient{status: status})
	installer.configureServe = func(context.Context) error {
		calls = append(calls, "configure-current-user")
		return tailscale.ErrServeConflict
	}

	err := installer.finish(context.Background(), `C:\TailClip\TailClip.exe`)
	if !errors.Is(err, tailscale.ErrServeConflict) {
		t.Fatalf("err=%v", err)
	}
	for _, call := range calls {
		if call == "configure-elevated" || call == "open-settings" {
			t.Fatalf("已知狀態錯誤不應提升權限或開啟設定頁：%v", calls)
		}
	}
}

func TestAutostartCommandQuotesExecutablePath(t *testing.T) {
	got := autostartCommand(`C:\Users\Tester Name\AppData\Local\TailClip\TailClip.exe`)
	want := `"C:\Users\Tester Name\AppData\Local\TailClip\TailClip.exe" agent`
	if got != want {
		t.Fatalf("autostartCommand()=%q want=%q", got, want)
	}
}

// 使用完整解壓的真實發行包，驗證原生檔案安裝與 EXE 可執行性，不修改使用者設定。
func TestWindowsPackagedInstallIntegration(t *testing.T) {
	directory := os.Getenv("TAILCLIP_WINDOWS_PACKAGE_DIR")
	if directory == "" {
		t.Skip("需要 TAILCLIP_WINDOWS_PACKAGE_DIR 指向完整發行包")
	}
	source := filepath.Join(directory, "TailClip.exe")
	target := filepath.Join(t.TempDir(), "中文 使用者", "TailClip.exe")
	if err := installWindowsFiles(source, target); err != nil {
		t.Fatal(err)
	}
	if same, err := sameFileContent(source, target); err != nil || !same {
		t.Fatalf("安裝後 EXE 內容不符：%v", err)
	}
	companion, err := tunnel.CurrentBinary()
	if err != nil {
		t.Fatal(err)
	}
	dependency := filepath.Join(filepath.Dir(target), companion.Filename)
	if err := tunnel.Verify(dependency); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(dependency)
	if err != nil {
		t.Fatal(err)
	}
	if err := installWindowsFiles(source, target); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(dependency)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("再次安裝不應重寫有效 companion：%v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, target, "version").CombinedOutput(); err != nil || len(output) == 0 {
		t.Fatalf("安裝後 EXE 無法正常執行：%v", err)
	}
}
