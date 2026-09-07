package tunnel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCompanionInstallReuseRepair(t *testing.T) {
	original := manifest
	defer func() { manifest = original }()
	data := []byte("合成隧道執行檔")
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	name := "tailclip-cloudflared-" + sha[:12]
	manifest, _ = json.Marshal(map[string]any{"assets": map[string]Binary{runtime.GOOS + "-" + runtime.GOARCH: {Filename: name, SHA256: sha}}})
	src, dst := t.TempDir(), t.TempDir()
	source, target := filepath.Join(src, name), filepath.Join(dst, name)
	os.WriteFile(source, data, 0700)
	if err := InstallCompanion(src, dst); err != nil {
		t.Fatal(err)
	}
	if err := Verify(target); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(target)
	if err := InstallCompanion(t.TempDir(), dst); err != nil {
		t.Fatal("既有正確依賴仍要求來源", err)
	}
	after, _ := os.Stat(target)
	if !os.SameFile(before, after) {
		t.Fatal("重寫既有依賴")
	}
	os.WriteFile(target, []byte("損壞"), 0700)
	if err := InstallCompanion(src, dst); err != nil {
		t.Fatal("無法修復", err)
	}
	if Verify(target) != nil {
		t.Fatal("修復未核對")
	}
}
