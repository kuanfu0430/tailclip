//go:build windows

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kuanfu0430/tailclip/internal/tailscale"
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
