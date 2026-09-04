package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadOrCreateAndRotate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	cfg, created, err := LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("第一次載入應建立設定")
	}
	if !ValidToken(cfg.PairingToken) {
		t.Fatal("應建立 256-bit token")
	}
	old := cfg.PairingToken

	loaded, created, err := LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}
	if created || loaded.PairingToken != old {
		t.Fatal("再次載入不得輪替 token")
	}
	if err := loaded.RotateToken(); err != nil {
		t.Fatal(err)
	}
	if loaded.PairingToken == old || !ValidToken(loaded.PairingToken) {
		t.Fatal("輪替後應為新的有效 token")
	}
	if err := Save(path, loaded); err != nil {
		t.Fatal(err)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("設定權限 = %o，預期 600", info.Mode().Perm())
		}
	}
}

func TestRejectUnknownVersionAndFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"version":2,"device_name":"pc","pairing_token":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("預期版本錯誤，得到 %v", err)
	}

	if err := os.WriteFile(path, []byte(`{"version":1,"device_name":"pc","pairing_token":"x","secret_extra":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("未知欄位應被拒絕")
	}

	if err := os.WriteFile(path, []byte(`{"version":1,"device_name":"pc","pairing_token":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("設定檔尾端的第二個 JSON 值應被拒絕")
	}
}

func TestStoreUpdatePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store, _, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(cfg *Config) error {
		cfg.DeviceName = "工作電腦"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DeviceName != "工作電腦" || store.Snapshot().DeviceName != "工作電腦" {
		t.Fatal("更新應同時保存至記憶體與磁碟")
	}
}
