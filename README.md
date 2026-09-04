# TailClip

TailClip 是一個以 Tailscale 私有網路連接 iPhone 與 Windows／Linux 的輕量剪貼簿工具。它不需要公開中繼站，不保存剪貼簿歷史，也不會在背景自動傳送每一次複製內容。

> **目前狀態：v0.1-alpha 開發中。** 倉庫正在建立第一個可日用的純文字版本；尚未發布已完成實機驗收的安裝檔。

## 目標體驗

第一次設定：

1. 在 Windows 雙擊 `TailClip.exe`，或在 Ubuntu 執行一支安裝腳本。
2. TailClip 自動啟動 Agent、設定 Tailscale Serve 並顯示 QR。
3. iPhone 掃一次 QR，安裝「TailClip：傳送」與「TailClip：取回」。
4. 不需要手動輸入 URL 或 token。

日常使用：

- **傳送：** 在 iPhone 複製文字或從 Share Sheet 執行「TailClip：傳送」，之後可直接在電腦貼上。
- **取回：** 在 iPhone 執行「TailClip：取回」，之後可直接貼上電腦目前的文字剪貼簿。
- 捷徑會在需要時自動連線 Tailscale；每個方向都只需一次明確操作。

## v0.1-alpha 支援範圍

- iOS 26 Shortcuts。
- Windows 11 x64。
- Ubuntu 26.04 x86_64 GNOME Wayland。
- UTF-8 純文字、Unicode、Emoji、多行文字與 1 MiB 上限。
- Share Sheet 純文字／網址；網址在本版視為純文字。

尚未支援：圖片、HTML、檔案、Taildrop、X11、多桌面選擇、原生 iOS App、自動同步、Tray 與自動更新。

## 架構

```text
iOS Shortcuts
    ↓ HTTPS + pairing token
Tailscale Serve /tailclip
    ↓
127.0.0.1:17733 TailClip Agent
    ↓
Windows CF_UNICODETEXT / Linux wl-clipboard
```

Agent 永遠只監聽 `127.0.0.1`。Tailscale Serve 提供 tailnet 內 HTTPS；TailClip 不會啟用 Funnel。

## 開發

本專案使用 Go 1.27.1。標準檢查為：

```bash
go test ./...
go test -race ./...
go vet ./...
```

平台 release 由 GitHub Actions 產出；Windows／Linux 的實際剪貼簿測試仍必須在互動式桌面 session 完成，不能以交叉編譯代替。

## 目前開發中斷點（2026-09-04）

- 共用 Agent、HTTP API、設定、認證、配對頁、Tailscale Serve 控制與 fake clipboard 測試已建立。
- 「TailClip：傳送」的空白條件參數已修正、重開驗證並重新簽署；兩支捷徑成品均已內嵌至設定頁下載流程。
- 本機格式、vet、一般／race 測試、Windows／Linux x86_64 交叉編譯與完整 release 組裝檢查已通過。
- 下一個最短路徑是以目前 build candidate 直接進行 iPhone ↔ Windows 實機驗收，再做 Ubuntu Wayland 實機驗收；只修正驗收發現的問題。
- 尚未完成 Windows 安裝實測、Ubuntu 安裝實測與任何平台的完整 E2E，因此目前不宣稱平台支援已通過。

## 文件

- [技術與產品規格](docs/SPEC.md)

`docs/SPEC.md` 是目前唯一工程契約；README 只保留使用方式、支援範圍與開發入口。

## 隱私與安全預設

- 手動觸發，不監聽所有剪貼簿變更。
- 不建立剪貼簿歷史或離線佇列。
- 日誌不記錄內容、token 或配對網址。
- 以 256-bit pairing token 保護讀寫 endpoint。
- 設定頁與 API 都只在 loopback 監聽，再由 Serve 暴露必要路徑。
- HTTPS 憑證會讓完整 `*.ts.net` 裝置名稱出現在 Certificate Transparency 紀錄；設定畫面會先顯示實際名稱並說明。

## 專案階段

- [x] 產品方向與安全邊界。
- [x] v0.2 精簡規格。
- [x] 共用 Agent 與 API（fake clipboard 自測）。
- [ ] Windows 11 x64 垂直切片。
- [ ] iOS 26 雙捷徑與一次 QR 配對（捷徑已簽署並內嵌，尚待實機驗收）。
- [ ] Ubuntu 26.04 GNOME Wayland。
- [ ] `v0.1.0-alpha.1` release。

## 授權與商標

本專案尚未選定開源授權；在加入 LICENSE 前，程式碼與文件不應被視為已授予通用再利用權利。

TailClip 是獨立專案，與 Tailscale Inc. 無隸屬或官方合作關係。Tailscale、Taildrop 與 Apple Shortcuts 等名稱屬其各自權利人所有。
