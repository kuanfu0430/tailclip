//go:build !windows

package main

import "context"

func startDesktopUI(context.Context, string, context.CancelFunc) (desktopUI, error) {
	return desktopUI{}, nil
}
