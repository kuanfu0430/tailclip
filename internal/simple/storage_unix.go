//go:build !windows

package simple

import "os"

// Linux 使用設定檔 0600 與使用者目錄權限保護；不儲存剪貼簿內容。
func protect(data []byte) ([]byte, error)      { return data, nil }
func unprotect(data []byte) ([]byte, error)    { return data, nil }
func replaceState(source, target string) error { return os.Rename(source, target) }
