package simple

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/kuanfu0430/tailclip/internal/config"
	"github.com/tailscale/tailcat"
)

type savedState struct {
	Version    int                 `json:"version"`
	Identity   *tailcat.PrivateKey `json:"identity"`
	Credential string              `json:"credential"`
	Paired     bool                `json:"paired"`
}

func readState(path string) (savedState, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		token, err := config.GenerateToken()
		return savedState{Version: 1, Identity: tailcat.NewPrivateKey(), Credential: token}, err
	}
	if err != nil {
		return savedState{}, err
	}
	data, err = unprotect(data)
	if err != nil {
		return savedState{}, errors.New("無法解鎖簡易連線設定，請使用原 Windows 帳號")
	}
	var state savedState
	if json.Unmarshal(data, &state) != nil || state.Version != 1 || state.Identity == nil ||
		state.Identity.Private.IsZero() || state.Identity.Public.PresharedKey.IsZero() ||
		!config.ValidToken(state.Credential) ||
		state.Identity.Private.Public() != state.Identity.Public.ServerPublic.NodePublic {
		return savedState{}, errors.New("簡易連線設定無效，未覆蓋原設定")
	}
	return state, nil
}

func saveState(path string, state savedState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	data, err = protect(data)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".simple-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err = file.Chmod(0600); err != nil {
		return err
	}
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return replaceState(file.Name(), path)
}
