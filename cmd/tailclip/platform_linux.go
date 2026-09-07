//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/kuanfu0430/tailclip/internal/tailscale"
)

func prepareTailnet(ctx context.Context) error {
	client, err := tailscale.New()
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
	status, err := client.Status(ctx)
	if err != nil {
		return err
	}
	return fmt.Errorf("尚未設定 HTTPS。完整名稱 %s 將出現在公開憑證紀錄；同意後請在終端執行 sudo ~/.local/bin/tailclip serve-install，再回來選擇 Tailscale。", status.Self.DNSName)
}

func defaultAction(ctx context.Context, executable string) error {
	return openSettings(ctx, executable)
}

func finishInstall(context.Context, string) error {
	return errors.New("Debian／Ubuntu 請使用 release 內的 install.sh 完成安裝")
}

func uninstallAction(context.Context, string) error {
	return errors.New("Debian／Ubuntu 請使用 release 內的 uninstall.sh，避免留下 systemd user service")
}

func startDetached(executable string, args ...string) error {
	command := exec.Command(executable, args...)
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return command.Start()
}

func openURL(url string) error { return exec.Command("xdg-open", url).Start() }

func showFatal(err error) { _, _ = fmt.Fprintln(os.Stderr, "TailClip:", err) }
