//go:build windows

package tunnel

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestJobChild(t *testing.T) {
	if os.Getenv("TAILCLIP_JOB_CHILD") != "1" {
		return
	}
	for {
		time.Sleep(time.Second)
	}
}
func TestJobCloseTerminatesChild(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestJobChild$")
	cmd.Env = append(os.Environ(), "TAILCLIP_JOB_CHILD=1")
	cleanup, err := prepareProcess(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	if err = attachProcess(cmd); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	// 不呼叫 Process.Kill；關閉 Job handle 模擬擁有者終止時 OS 的清理。
	cleanup()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("子程序未被強制結束")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Job 關閉後子程序殘留")
	}
}
