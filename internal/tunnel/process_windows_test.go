//go:build windows

package tunnel

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestJobChild(t *testing.T) {
	if os.Getenv("TAILCLIP_JOB_CHILD") != "1" {
		return
	}
	if err := os.WriteFile(os.Getenv("TAILCLIP_JOB_READY"), []byte("ready"), 0600); err != nil {
		t.Fatal(err)
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
	ready := filepath.Join(t.TempDir(), "ready")
	cmd.Env = append(os.Environ(), "TAILCLIP_JOB_CHILD=1", "TAILCLIP_JOB_READY="+ready)
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
	deadline := time.After(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("子程序在 Job 關閉前已退出：%v", err)
		case <-deadline:
			t.Fatal("子程序未進入持續執行狀態")
		case <-time.After(10 * time.Millisecond):
		}
	}
	select {
	case err := <-done:
		t.Fatalf("子程序在 Job 關閉前已退出：%v", err)
	default:
	}
	// 不呼叫 Process.Kill；關閉 Job handle 模擬擁有者終止時 OS 的清理。
	cleanup()
	select {
	case <-done:
		// Windows Job 關閉可回傳退出碼 0；以已就緒且不會自行退出的子程序確實結束為準。
	case <-time.After(5 * time.Second):
		t.Fatal("Job 關閉後子程序殘留")
	}
}
