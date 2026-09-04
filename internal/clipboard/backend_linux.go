//go:build linux

package clipboard

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type linuxBackend struct{}

func NewSystemBackend() Backend { return linuxBackend{} }

func (linuxBackend) Available(context.Context) error {
	if strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) == "" {
		return fmt.Errorf("%w: 找不到 WAYLAND_DISPLAY", ErrUnavailable)
	}
	for _, command := range []string{"wl-copy", "wl-paste"} {
		if _, err := exec.LookPath(command); err != nil {
			return fmt.Errorf("%w: 找不到 %s", ErrUnavailable, command)
		}
	}
	return nil
}

func (backend linuxBackend) ReadText(ctx context.Context) (string, error) {
	if err := backend.Available(ctx); err != nil {
		return "", err
	}
	command := exec.CommandContext(ctx, "wl-paste", "--no-newline")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		message := strings.ToLower(stderr.String())
		if strings.Contains(message, "no selection") || strings.Contains(message, "does not offer") {
			return "", ErrNoText
		}
		return "", fmt.Errorf("%w: wl-paste: %v", ErrUnavailable, err)
	}
	if len(output) == 0 {
		return "", ErrNoText
	}
	return string(output), nil
}

func (backend linuxBackend) WriteText(ctx context.Context, text string) error {
	if err := backend.Available(ctx); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "wl-copy", "--type", "text/plain;charset=utf-8")
	command.Stdin = strings.NewReader(text)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: wl-copy: %v", ErrUnavailable, err)
	}
	return nil
}
