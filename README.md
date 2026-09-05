# TailClip

TailClip 是一個以 Tailscale 私有網路連接 iPhone 與 Windows／Linux 的輕量剪貼簿工具。它不需要公開中繼站，不保存剪貼簿歷史，也不會在背景自動傳送每一次複製內容。

> **目前提供未簽章的 alpha 測試版。** Windows 可能顯示 SmartScreen 警告；尚未通過實機驗收的平台只代表可測試，不代表正式支援。

## Windows 11 快速開始

需要：

- Windows 11 x64。
- Windows 與 iPhone 已安裝 Tailscale、登入同一個 tailnet 並連線。
- iPhone 使用 iOS 26。

下載並使用：

1. 下載 [TailClip Windows x64 alpha 壓縮包](dist/TailClip-v0.1.0-alpha.6-windows-x64.zip)。
2. 在檔案總管按「全部解壓縮」，不要直接在 ZIP 預覽中執行。
3. 雙擊解壓後的 `TailClip.exe`。不需要安裝 Go 或其他 runtime；亦可雙擊同資料夾的 `Start-TailClip.cmd` 一鍵啟動服務與配對頁。
4. TailClip 會沿用本機已登入的 Tailscale tailnet 與既有 `/tailclip` Serve 設定，並常駐在 Windows 右下角通知區；只有需要新增 Serve 且目前使用者權限不足時才會要求一次 UAC。
5. 用 iPhone 相機掃描 QR，依畫面安裝「TailClip：傳送」與「TailClip：取回」，再按一下複製配對資料。
6. 執行「TailClip：取回」完成配對並取得電腦文字。首次使用出現檔案或網路權限時允許存取。若先執行「傳送」，這一次只完成配對；請再複製要傳送的文字並執行一次。

不需從原始碼編譯。正式標記版本也會附在 [GitHub Releases](https://github.com/kuanfu0430/tailclip/releases)；下載後可用 [SHA-256 檔](dist/TailClip-v0.1.0-alpha.6-windows-x64.zip.sha256) 核對完整性。

從 alpha.5 或更舊版本升級時，請解壓新版並執行 `TailClip.exe`，從配對頁重新安裝兩支捷徑、複製配對資料，再執行「TailClip：取回」。同名時選擇取代；之後使用中文名稱的新版，避免誤開 `TailClip-Send 2`、`TailClip-Pull 4` 等舊副本。桌面既有設定會保留。

若曾遇到「無效的 URL：/status」，請依上述步驟重新配對，不必手改捷徑變數或刪除設定檔。重新配對時，新複製的配對資料會優先驗證，成功後才取代舊設定。

## 日常使用

- **傳送：** 在 iPhone 複製文字後執行「TailClip：傳送」，或直接從 App 的分享選單執行它；之後可在電腦貼上。
- **取回：** 在 iPhone 執行「TailClip：取回」；之後可貼上電腦目前的文字剪貼簿。
- 捷徑會呼叫 Tailscale 連線並短暫等待，不顯示方向或裝置選單。連線仍失敗時，確認兩端 Tailscale 與電腦 TailClip 已啟動後再執行。
- 電腦沒有文字時，取回只會通知，保留手機原有剪貼簿。
- **Windows 通知區：** 按兩下 TailClip 圖示會開啟連線與配對頁面；按右鍵可開啟頁面、切換「登入 Windows 後自動啟動」，或結束 TailClip。這個選項只控制登入後啟動程式，不會修改 Windows 帳號的登入方式。

結束後要重新啟動時，再次雙擊壓縮包內的 `TailClip.exe`；Agent 已在執行時，雙擊只會開啟狀態與配對頁。需要移除時，雙擊同一資料夾內的 `Uninstall-TailClip.cmd`，再於確認畫面選擇同意；Tailscale 本身不會被移除。

## Alpha 測試範圍

- UTF-8 純文字、Unicode、Emoji、多行文字與 1 MiB 上限。
- iOS 26 Shortcuts 與 Share Sheet 純文字／網址；網址視為純文字。
- Windows 11 x64。
- Ubuntu 26.04 x86_64 GNOME Wayland；Linux 測試包請由 [GitHub Releases](https://github.com/kuanfu0430/tailclip/releases) 下載。

alpha.6 已通過 Mac 原生捷徑執行器連接 Go API 的 17 項整合驗收，包括配對保存、再次執行、分享／剪貼簿、Unicode、1 MiB 與錯誤分支。2026-09-05 使用者亦回報 iPhone ↔ Windows 核心傳輸實機測試成功；自啟、重開機、解除安裝及完整邊界案例仍未逐項驗收。

尚未支援：圖片、HTML、檔案、Taildrop、X11、多桌面選擇、原生 iOS App、自動同步與自動更新。

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
