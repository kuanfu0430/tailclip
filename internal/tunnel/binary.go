// Package tunnel 管理 TailBlink 專用的 Cloudflare 臨時隧道程序。
package tunnel

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

//go:embed cloudflared.json
var manifest []byte

type Binary struct {
	Filename string `json:"filename"`
	SHA256   string `json:"sha256"`
}

func CurrentBinary() (Binary, error) {
	var data struct {
		Assets map[string]Binary `json:"assets"`
	}
	if json.Unmarshal(manifest, &data) != nil {
		return Binary{}, errors.New("隧道版本資訊無效")
	}
	b, ok := data.Assets[runtime.GOOS+"-"+runtime.GOARCH]
	if !ok {
		return Binary{}, errors.New("此平台沒有隨附的 Cloudflare 隧道")
	}
	return b, nil
}

func Verify(path string) error {
	b, err := CurrentBinary()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return errors.New("缺少隧道程式，請重新下載並完整解壓 TailBlink 發行包")
	}
	h := sha256.Sum256(data)
	if hex.EncodeToString(h[:]) != b.SHA256 {
		return errors.New("隧道程式校驗失敗，請重新下載並完整解壓 TailBlink 發行包")
	}
	return nil
}

func BinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	b, err := CurrentBinary()
	if err != nil {
		return "", err
	}
	p := filepath.Join(filepath.Dir(exe), b.Filename)
	return p, Verify(p)
}

// InstallCompanion 只複製發行包內已釘選的依賴；相同版本不改寫執行中的檔案。
func InstallCompanion(sourceDir, targetDir string) error {
	b, err := CurrentBinary()
	if err != nil {
		return err
	}
	source, target := filepath.Join(sourceDir, b.Filename), filepath.Join(targetDir, b.Filename)
	if Verify(target) == nil {
		return nil
	}
	if err := Verify(source); err != nil {
		return err
	}
	if err := os.MkdirAll(targetDir, 0700); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.CreateTemp(targetDir, ".cloudflared-*")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name())
	defer out.Close()
	if err = out.Chmod(0700); err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	if err = out.Sync(); err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	if err = Verify(out.Name()); err != nil {
		return err
	}
	return os.Rename(out.Name(), target)
}
