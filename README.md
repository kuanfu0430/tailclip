# TailClip

TailClip 是一個以 Tailscale 私有網路連接 iPhone 與 Windows／Linux 的輕量剪貼簿工具。它不需要公開中繼站，不保存剪貼簿歷史，也不會在背景自動傳送每一次複製內容。

> **目前提供未簽章的 alpha 測試版。** Windows 可能顯示 SmartScreen 警告；尚未通過實機驗收的平台只代表可測試，不代表正式支援。

## Windows 11 快速開始

需要：

- Windows 11 x64。
- Windows 與 iPhone 已安裝 Tailscale、登入同一個 tailnet 並連線。
- iPhone 使用 iOS 26。

下載並使用：

1. 下載 [TailClip Windows x64 alpha 壓縮包](dist/TailClip-v0.1.0-alpha.1-windows-x64.zip)。
2. 在檔案總管按「全部解壓縮」，不要直接在 ZIP 預覽中執行。
3. 雙擊解壓後的 `TailClip.exe`。不需要安裝 Go 或其他 runtime。
4. 閱讀 HTTPS 名稱提示並接受一次 UAC；TailClip 會自動安裝、自啟、設定 Tailscale Serve 並開啟配對 QR。
5. 用 iPhone 相機掃描 QR，依畫面安裝「TailClip：傳送」與「TailClip：取回」，再按一下複製配對資料。
6. 執行「TailClip：取回」，或從分享選單執行「TailClip：傳送」，即可完成配對與首次測試。

不需從原始碼編譯。相同壓縮包也會附在 [GitHub Releases](https://github.com/kuanfu0430/tailclip/releases)；下載後可用 [SHA-256 檔](dist/TailClip-v0.1.0-alpha.1-windows-x64.zip.sha256) 核對完整性。

## 日常使用

- **傳送：** 在 iPhone 複製文字後執行「TailClip：傳送」，或直接從 App 的分享選單執行它；之後可在電腦貼上。
- **取回：** 在 iPhone 執行「TailClip：取回」；之後可貼上電腦目前的文字剪貼簿。
- 捷徑會在需要時自動連線 Tailscale，不顯示方向或裝置選單。

再次雙擊壓縮包內的 `TailClip.exe` 會開啟狀態與配對頁。需要移除時，雙擊同一資料夾內的 `Uninstall-TailClip.cmd`，再於確認畫面選擇同意；Tailscale 本身不會被移除。

## Alpha 測試範圍

- UTF-8 純文字、Unicode、Emoji、多行文字與 1 MiB 上限。
- iOS 26 Shortcuts 與 Share Sheet 純文字／網址；網址視為純文字。
- Windows 11 x64。
- Ubuntu 26.04 x86_64 GNOME Wayland；Linux 測試包請由 [GitHub Releases](https://github.com/kuanfu0430/tailclip/releases) 下載。

尚未支援：圖片、HTML、檔案、Taildrop、X11、多桌面選擇、原生 iOS App、自動同步、Tray 與自動更新。

## 隱私

- 手動觸發，不監聽所有剪貼簿變更。
- 不建立剪貼簿歷史或離線佇列。
- 日誌不記錄內容、token 或配對網址。
- 桌面服務只監聽本機，透過 Tailscale Serve 提供 tailnet 內 HTTPS；不啟用 Funnel。
- HTTPS 憑證會讓完整 `*.ts.net` 裝置名稱出現在 Certificate Transparency 紀錄；設定畫面會先顯示實際名稱並說明。

## 開發文件

需求、架構、API、安全邊界、實作決策、測試條件與目前進度統一記錄於 [TailClip 技術與產品規格](docs/SPEC.md)。README 只說明使用者能做什麼以及如何安裝、操作與移除。

## 授權與商標

本專案尚未選定開源授權；在加入 LICENSE 前，程式碼與文件不應被視為已授予通用再利用權利。

TailClip 是獨立專案，與 Tailscale Inc. 無隸屬或官方合作關係。Tailscale、Taildrop 與 Apple Shortcuts 等名稱屬其各自權利人所有。
