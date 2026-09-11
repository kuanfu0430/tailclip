package tunnel

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Handle interface {
	URL() string
	Done() <-chan struct{}
	Close()
}
type Starter func(context.Context, string) (Handle, error)

type process struct {
	cmd     *exec.Cmd
	url     string
	done    chan struct{}
	once    sync.Once
	cleanup func()
}

func (p *process) URL() string           { return p.url }
func (p *process) Done() <-chan struct{} { return p.done }
func (p *process) Close()                { p.once.Do(func() { _ = p.cmd.Process.Kill(); <-p.done }) }

func Start(ctx context.Context, address string) (Handle, error) {
	binary, err := BinaryPath()
	if err != nil {
		return nil, err
	}
	return startBinary(ctx, binary, address)
}

func startBinary(ctx context.Context, binary, address string) (Handle, error) {
	if ctx.Err() != nil {
		return nil, errors.New("臨時隧道啟動已取消")
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil || host != "127.0.0.1" {
		return nil, errors.New("隧道只可連接本機專用埠")
	}
	dir, err := os.MkdirTemp("", "tailblink-tunnel-")
	if err != nil {
		return nil, err
	}
	config := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(config, []byte("{}\n"), 0600); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	// 指定空白設定，避免套用使用者自己的 named tunnel 或憑證。
	cmd := exec.Command(binary, "tunnel", "--config", config, "--no-autoupdate", "--url", "http://"+address, "--metrics", "127.0.0.1:0", "--protocol", "http2")
	for _, e := range os.Environ() {
		upper := strings.ToUpper(e)
		if !strings.HasPrefix(upper, "TUNNEL_") && !strings.HasPrefix(upper, "CLOUDFLARED_") {
			cmd.Env = append(cmd.Env, e)
		}
	}
	output := &urlOutput{found: make(chan string, 1)}
	cmd.Stdout, cmd.Stderr = output, output
	cleanup, err := prepareProcess(cmd)
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		cleanup()
		os.RemoveAll(dir)
		return nil, errors.New("無法啟動隧道程式，請檢查完整發行包與系統權限")
	}
	if err = attachProcess(cmd); err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		cleanup()
		os.RemoveAll(dir)
		return nil, errors.New("無法管理隧道子程序")
	}
	p := &process{cmd: cmd, done: make(chan struct{}), cleanup: cleanup}
	go func() { _ = cmd.Wait(); cleanup(); os.RemoveAll(dir); close(p.done) }()
	timer := time.NewTimer(25 * time.Second)
	defer timer.Stop()
	select {
	case p.url = <-output.found:
		return p, nil
	case <-p.done:
		return nil, errors.New("臨時隧道啟動失敗，請確認可連接 Cloudflare 後重試")
	case <-ctx.Done():
		p.Close()
		return nil, errors.New("臨時隧道啟動已取消或逾時，請重試")
	case <-timer.C:
		p.Close()
		return nil, errors.New("取得臨時網址逾時，請檢查網路後重新連接")
	}
}

var urlPattern = regexp.MustCompile(`https://[a-z0-9]+(?:-[a-z0-9]+)*\.trycloudflare\.com\b`)

type urlOutput struct {
	mu     sync.Mutex
	buffer string
	sent   bool
	found  chan string
}

var _ io.Writer = (*urlOutput)(nil)

func (w *urlOutput) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.sent {
		w.buffer += string(b)
		if u := urlPattern.FindString(w.buffer); u != "" {
			w.found <- u
			w.sent = true
			w.buffer = ""
		}
		if len(w.buffer) > 8192 {
			w.buffer = w.buffer[len(w.buffer)-4096:]
		}
	}
	return len(b), nil
}
