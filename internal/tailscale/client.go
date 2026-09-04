package tailscale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	ServePath   = "/tailclip"
	AgentTarget = "http://127.0.0.1:17733"
)

var (
	ErrNotInstalled  = errors.New("找不到 Tailscale CLI")
	ErrNotRunning    = errors.New("Tailscale 尚未連線")
	ErrServeConflict = errors.New("Tailscale Serve 的 /tailclip 已由其他服務使用")
)

type Runner interface {
	Run(context.Context, ...string) ([]byte, error)
}

type commandRunner struct{ executable string }

func (r commandRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, r.executable, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tailscale %s 失敗: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

type Client struct{ runner Runner }

func New() (*Client, error) {
	executable, err := FindCLI()
	if err != nil {
		return nil, err
	}
	return &Client{runner: commandRunner{executable: executable}}, nil
}

func NewWithRunner(runner Runner) *Client { return &Client{runner: runner} }

func FindCLI() (string, error) {
	if path, err := exec.LookPath("tailscale"); err == nil {
		return path, nil
	}
	if runtime.GOOS == "windows" {
		for _, base := range []string{os.Getenv("ProgramW6432"), os.Getenv("ProgramFiles")} {
			if base == "" {
				continue
			}
			candidate := filepath.Join(base, "Tailscale", "tailscale.exe")
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, nil
			}
		}
	}
	return "", ErrNotInstalled
}

type Status struct {
	BackendState string `json:"BackendState"`
	Self         struct {
		DNSName  string `json:"DNSName"`
		HostName string `json:"HostName"`
	} `json:"Self"`
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	output, err := c.runner.Run(ctx, "status", "--json")
	if err != nil {
		return Status{}, err
	}
	var status Status
	if err := json.Unmarshal(output, &status); err != nil {
		return Status{}, fmt.Errorf("無法解析 Tailscale 狀態: %w", err)
	}
	status.Self.DNSName = strings.TrimSuffix(strings.TrimSpace(status.Self.DNSName), ".")
	if status.BackendState != "Running" || !strings.HasSuffix(strings.ToLower(status.Self.DNSName), ".ts.net") {
		return status, ErrNotRunning
	}
	return status, nil
}

type serveConfig struct {
	Web map[string]struct {
		Handlers map[string]struct {
			Proxy string `json:"Proxy"`
		} `json:"Handlers"`
	} `json:"Web"`
}

func (c *Client) serveTarget(ctx context.Context) (string, error) {
	output, err := c.runner.Run(ctx, "serve", "status", "--json")
	if err != nil {
		return "", err
	}
	var current serveConfig
	if err := json.Unmarshal(output, &current); err != nil {
		return "", fmt.Errorf("無法解析 Tailscale Serve 狀態: %w", err)
	}
	for _, web := range current.Web {
		if handler, ok := web.Handlers[ServePath]; ok {
			return strings.TrimSuffix(handler.Proxy, "/"), nil
		}
	}
	return "", nil
}

func (c *Client) ServeConfigured(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	target, err := c.serveTarget(ctx)
	if err != nil {
		return false, err
	}
	if target == "" {
		return false, nil
	}
	if target != AgentTarget {
		return false, fmt.Errorf("%w: 目前指向 %s", ErrServeConflict, target)
	}
	return true, nil
}

// EnsureServe 只新增 TailClip 自己的 path；若路徑已有其他目標便安全停止。
func (c *Client) EnsureServe(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	target, err := c.serveTarget(ctx)
	if err != nil {
		return false, err
	}
	if target == AgentTarget {
		return false, nil
	}
	if target != "" {
		return false, fmt.Errorf("%w: 目前指向 %s", ErrServeConflict, target)
	}
	if _, err := c.runner.Run(ctx, "serve", "--bg", "--yes", "--https=443", "--set-path="+ServePath, AgentTarget); err != nil {
		return false, err
	}
	return true, nil
}

// RemoveServe 只移除仍指向 TailClip Agent 的 path，絕不呼叫 serve reset。
func (c *Client) RemoveServe(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	target, err := c.serveTarget(ctx)
	if err != nil {
		return false, err
	}
	if target == "" {
		return false, nil
	}
	if target != AgentTarget {
		return false, fmt.Errorf("%w: 目前指向 %s", ErrServeConflict, target)
	}
	if _, err := c.runner.Run(ctx, "serve", "--yes", "--https=443", "--set-path="+ServePath, "off"); err != nil {
		return false, err
	}
	return true, nil
}

func BaseURL(status Status) string {
	return "https://" + status.Self.DNSName + ServePath + "/v1"
}

func VerifyHealth(ctx context.Context, status Status) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, BaseURL(status)+"/health", nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("無法連上 TailClip HTTPS health: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 32<<10))
		return fmt.Errorf("TailClip HTTPS health 回傳 HTTP %d", response.StatusCode)
	}
	var health struct {
		Status  string `json:"status"`
		Service string `json:"service"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 32<<10))
	if err := decoder.Decode(&health); err != nil {
		return fmt.Errorf("TailClip HTTPS health 回應無效: %w", err)
	}
	if health.Status != "ok" || health.Service != "tailclip-agent" {
		return errors.New("TailClip HTTPS health 回應不是預期的服務")
	}
	return nil
}
