//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func defaultAction(ctx context.Context, executable string) error {
	return openSettings(ctx, executable)
}

func finishInstall(context.Context, string) error {
	return errors.New("Ubuntu 請使用 release 內的 install.sh 完成安裝")
}

func uninstallAction(context.Context, string) error {
	return errors.New("Ubuntu 請使用 release 內的 uninstall.sh，避免留下 systemd user service")
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
