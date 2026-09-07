//go:build !windows && !linux

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func defaultAction(context.Context, string) error {
	return errors.New("這個 build 只供開發驗證；v0.1-alpha 桌面端支援 Windows 與 Linux")
}

func finishInstall(context.Context, string) error { return errors.New("此平台沒有安裝流程") }

func prepareTailnet(context.Context) error { return errors.New("此平台沒有連線設定流程") }

func uninstallAction(context.Context, string) error {
	return errors.New("此平台沒有解除安裝流程")
}

func startDetached(executable string, args ...string) error {
	command := exec.Command(executable, args...)
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return command.Start()
}

func openURL(url string) error { return exec.Command("open", url).Start() }

func showFatal(err error) { _, _ = fmt.Fprintln(os.Stderr, "TailClip:", err) }
