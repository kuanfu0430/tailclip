package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kuanfu0430/tailclip/internal/agent"
	"github.com/kuanfu0430/tailclip/internal/buildinfo"
	"github.com/kuanfu0430/tailclip/internal/clipboard"
	"github.com/kuanfu0430/tailclip/internal/config"
	"github.com/kuanfu0430/tailclip/internal/logging"
	"github.com/kuanfu0430/tailclip/internal/tailscale"
	"github.com/kuanfu0430/tailclip/internal/webui"
	shortcutassets "github.com/kuanfu0430/tailclip/shortcuts"
)

func main() {
	if err := run(); err != nil {
		showFatal(err)
		os.Exit(1)
	}
}

func run() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("找不到目前程式位置: %w", err)
	}
	ctx := context.Background()
	if len(os.Args) == 1 {
		return defaultAction(ctx, executable)
	}
	switch os.Args[1] {
	case "agent":
		return runAgent(executable)
	case "open":
		return openSettings(ctx, executable)
	case "finish-install":
		return finishInstall(ctx, executable)
	case "serve-install":
		return configureServe(ctx)
	case "serve-remove":
		return removeServe(ctx)
	case "uninstall":
		return uninstallAction(ctx, executable)
	case "version", "--version", "-version":
		fmt.Println(buildinfo.Version)
		return nil
	default:
		return fmt.Errorf("不支援的指令：%s", os.Args[1])
	}
}

func runAgent(executable string) error {
	configPath, err := config.DefaultPath()
	if err != nil {
		return err
	}
	store, _, err := config.OpenStore(configPath)
	if err != nil {
		return err
	}
	logPath, err := logging.DefaultPath()
	if err != nil {
		return err
	}
	logger, closer, err := logging.Open(logPath)
	if err != nil {
		return err
	}
	defer closer.Close()
	service, err := agent.New(agent.Options{
		PrepareTailnet: prepareTailnet,
		Store:          store, Clipboard: clipboard.NewSystemBackend(), Tailnet: lazyTailnet{},
		Shortcuts: webui.ShortcutAssets{Send: shortcutassets.Send, Pull: shortcutassets.Pull}, Logger: logger,
	})
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	agentDone := make(chan error, 1)
	go func() { agentDone <- service.Run(ctx, agent.ListenAddress) }()

	desktop, err := startDesktopUI(ctx, executable, stop)
	if err != nil {
		stop()
		<-agentDone
		return err
	}
	defer desktop.Close()

	select {
	case err := <-agentDone:
		return err
	case err := <-desktop.Done():
		stop()
		agentErr := <-agentDone
		if err != nil {
			return err
		}
		return agentErr
	}
}

// B 與首次選擇頁不需要 Tailscale；只有 A 查詢狀態時才尋找 CLI。
type lazyTailnet struct{}

func (lazyTailnet) Status(ctx context.Context) (tailscale.Status, error) {
	client, err := tailscale.New()
	if err != nil {
		return tailscale.Status{}, err
	}
	return client.Status(ctx)
}

func configureServe(ctx context.Context) error {
	client, err := tailscale.New()
	if err != nil {
		return err
	}
	status, err := client.Status(ctx)
	if err != nil {
		return err
	}
	if _, err := client.EnsureServe(ctx); err != nil {
		return err
	}
	if err := tailscale.VerifyHealth(ctx, status); err != nil {
		return err
	}
	return nil
}

func removeServe(ctx context.Context) error {
	client, err := tailscale.New()
	if err != nil {
		return err
	}
	_, err = client.RemoveServe(ctx)
	return err
}

func openSettings(ctx context.Context, executable string) error {
	if err := ensureAgent(ctx, executable); err != nil {
		return err
	}
	configPath, err := config.DefaultPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	requestContext, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodPost, "http://"+agent.ListenAddress+"/local/open-setup", nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+cfg.PairingToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("無法請 Agent 開啟設定頁: %w", err)
	}
	defer response.Body.Close()
	var result struct {
		OK    bool   `json:"ok"`
		URL   string `json:"url"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 32<<10))
	if err := decoder.Decode(&result); err != nil {
		return errors.New("Agent 回傳了無效的設定頁資訊")
	}
	if response.StatusCode != http.StatusOK || !result.OK {
		if strings.TrimSpace(result.Error.Message) != "" {
			return errors.New(result.Error.Message)
		}
		return fmt.Errorf("Agent 無法開啟設定頁（HTTP %d）", response.StatusCode)
	}
	if !strings.HasPrefix(result.URL, "http://127.0.0.1:") {
		return errors.New("Agent 拒絕了不安全的設定頁位址")
	}
	return openURL(result.URL)
}

func ensureAgent(ctx context.Context, executable string) error {
	if runningVersion, healthy := localAgentVersion(ctx); healthy {
		if runningVersion == buildinfo.Version {
			return nil
		}
		if err := stopAgent(ctx); err != nil {
			return fmt.Errorf("無法結束舊版 TailClip Agent: %w", err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) && localHealth(ctx) {
			timer := time.NewTimer(100 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		if localHealth(ctx) {
			return errors.New("舊版 TailClip Agent 未能結束；請從工作管理員結束 TailClip 後再試")
		}
	}
	if err := startDetached(executable, "agent"); err != nil {
		return fmt.Errorf("無法啟動 TailClip Agent: %w", err)
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		timer := time.NewTimer(125 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		if localHealth(ctx) {
			return nil
		}
	}
	return errors.New("Agent 未能啟動；請確認 127.0.0.1:17733 未被占用，並查看 TailClip 日誌")
}

func localHealth(ctx context.Context) bool {
	_, healthy := localAgentVersion(ctx)
	return healthy
}

func localAgentVersion(ctx context.Context) (string, bool) {
	requestContext, cancel := context.WithTimeout(ctx, 600*time.Millisecond)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, "http://"+agent.ListenAddress+"/v1/health", nil)
	if err != nil {
		return "", false
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", false
	}
	var result struct {
		Status       string `json:"status"`
		Service      string `json:"service"`
		AgentVersion string `json:"agent_version"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 32<<10)).Decode(&result) != nil ||
		result.Status != "ok" || result.Service != "tailclip-agent" {
		return "", false
	}
	return result.AgentVersion, true
}

func stopAgent(ctx context.Context) error {
	configPath, err := config.DefaultPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	requestContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodPost, "http://"+agent.ListenAddress+"/local/shutdown", nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+cfg.PairingToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Agent 停止請求回傳 HTTP %d", response.StatusCode)
	}
	return nil
}
