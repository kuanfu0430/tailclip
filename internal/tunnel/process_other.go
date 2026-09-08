//go:build !windows

package tunnel

import "os/exec"

func prepareProcess(cmd *exec.Cmd) (func(), error) { return func() {}, nil }
func attachProcess(cmd *exec.Cmd) error            { return nil }
