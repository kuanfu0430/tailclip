package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	CurrentVersion = 1
	tokenBytes     = 32
)

var ErrUnsupportedVersion = errors.New("不支援的設定版本")

// Config 是桌面 Agent 唯一的持久設定，不包含任何剪貼簿內容。
type Config struct {
	Version         int    `json:"version"`
	DeviceName      string `json:"device_name"`
	TailscaleDevice string `json:"tailscale_device,omitempty"`
	PairingToken    string `json:"pairing_token"`
}

// Store 讓 API 與設定頁安全共享最新設定。
type Store struct {
	path string
	mu   sync.RWMutex
	cfg  Config
}

func OpenStore(path string) (*Store, bool, error) {
	cfg, created, err := LoadOrCreate(path)
	if err != nil {
		return nil, false, err
	}
	return &Store{path: path, cfg: cfg}, created, nil
}

func (s *Store) Snapshot() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *Store) Update(update func(*Config) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cfg
	if err := update(&next); err != nil {
		return err
	}
	if err := Save(s.path, next); err != nil {
		return err
	}
	s.cfg = next
	return nil
}

func (s *Store) RotateToken() error {
	return s.Update(func(cfg *Config) error { return cfg.RotateToken() })
}

func DefaultPath() (string, error) {
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			return "", errors.New("找不到 LOCALAPPDATA")
		}
		return filepath.Join(base, "TailClip", "config.json"), nil
	}

	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("找不到使用者目錄: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "tailclip", "config.json"), nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("設定檔格式無效: %w", err)
	}
	if err := ensureJSONEOF(dec); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func LoadOrCreate(path string) (Config, bool, error) {
	cfg, err := Load(path)
	if err == nil {
		return cfg, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Config{}, false, err
	}

	hostname, hostErr := os.Hostname()
	if hostErr != nil || strings.TrimSpace(hostname) == "" {
		hostname = "TailClip 電腦"
	}
	hostname = strings.TrimSuffix(strings.TrimSpace(hostname), ".")
	if dot := strings.IndexByte(hostname, '.'); dot > 0 {
		hostname = hostname[:dot]
	}

	token, err := GenerateToken()
	if err != nil {
		return Config{}, false, err
	}
	cfg = Config{
		Version:         CurrentVersion,
		DeviceName:      hostname,
		TailscaleDevice: hostname,
		PairingToken:    token,
	}
	if err := Save(path, cfg); err != nil {
		return Config{}, false, err
	}
	return cfg, true, nil
}

func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("無法編碼設定: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("無法建立設定目錄: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("無法限制設定目錄權限: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("無法建立暫存設定: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(0o600); err != nil && runtime.GOOS != "windows" {
		tmp.Close()
		return fmt.Errorf("無法限制設定檔權限: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("無法寫入設定: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("無法同步設定: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("無法關閉設定: %w", err)
	}

	if err := replaceFile(tmpName, path); err != nil {
		return fmt.Errorf("無法套用設定: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("無法限制設定檔權限: %w", err)
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	} else if err != nil {
		return fmt.Errorf("設定檔格式無效: %w", err)
	}
	return errors.New("設定檔只能包含一個 JSON 物件")
}

func GenerateToken() (string, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("無法產生配對憑證: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func ValidToken(token string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(raw) == tokenBytes
}

func (c Config) Validate() error {
	if c.Version != CurrentVersion {
		return fmt.Errorf("%w: %d", ErrUnsupportedVersion, c.Version)
	}
	if strings.TrimSpace(c.DeviceName) == "" {
		return errors.New("裝置名稱不可空白")
	}
	if strings.ContainsAny(c.DeviceName, "\r\n") {
		return errors.New("裝置名稱不可包含換行")
	}
	if !ValidToken(c.PairingToken) {
		return errors.New("配對憑證格式無效")
	}
	return nil
}

func (c *Config) RotateToken() error {
	token, err := GenerateToken()
	if err != nil {
		return err
	}
	c.PairingToken = token
	return nil
}
