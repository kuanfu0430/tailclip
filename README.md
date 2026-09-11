# TailBlink

TailBlink 是透過 Tailscale 私有網路或臨時 HTTPS 隧道，連接 iPhone 與 Windows／Linux 的輕量剪貼簿工具。手動傳送與取回目前的純文字；不保存剪貼簿歷史，也不會在背景自動傳送每次複製內容。

> 目前為未簽章的 alpha 測試版。Windows 可能顯示 SmartScreen 警告。自動化測試與交叉編譯不等於 iPhone／桌面實機驗收。

## 下載與安裝

更名版：**v0.2.0-alpha.3**。請完整解壓套件，不要只取出主程式，附帶的 `tailblink-cloudflared-*` 是簡易連線所需的隧道程式。

- [Windows x64 完整安裝包](dist/TailBlink-v0.2.0-alpha.3-windows-x64.zip)（[SHA-256](dist/TailBlink-v0.2.0-alpha.3-windows-x64.zip.sha256)）：解壓後雙擊 `TailBlink.exe`。不需安裝 Go 或其他 runtime。
- [Linux x64 完整安裝包](dist/TailBlink-v0.2.0-alpha.3-linux-x64.tar.gz)（[SHA-256](dist/TailBlink-v0.2.0-alpha.3-linux-x64.tar.gz.sha256)）：在 Debian 13／Ubuntu 26.04 的 GNOME Wayland 桌面解壓後執行 `bash install.sh`；安裝器按需安裝 `wl-clipboard` 並建立使用者服務。

套件內 `SHA256SUMS.txt` 涵蓋全部附帶檔案；`SOURCE.txt` 記錄建置所用的原始碼 commit。

## 從先前命名版本升級

本次是完整更名，執行檔、設定目錄、服務、自啟項目、Serve 路徑、配對格式及四支捷徑都改用 TailBlink。不提供舊名稱相容別名，也不自動搬移舊 token。

**先使用原版附帶的解除安裝程式移除桌面端，再安裝新版。** 這可避免原版程序占用相同連接埠或留下原版自啟與 Serve 路徑。手機端移除原版捷徑，從新版 QR 頁安裝 TailBlink 捷徑並重新配對；原版的設定檔不會由新版偷偷刪除。

## 兩種連線入口

**已有 Tailscale：** 電腦與 iPhone 安裝 Tailscale、登入同一個 tailnet 並連線。桌面選「使用現有 Tailscale」，依畫面建立 `/tailblink` 的 Serve 路徑，再用 iPhone 相機掃 QR。安裝「TailBlink：傳送」與「TailBlink：取回」，複製配對資料後執行「取回」。若先執行「傳送」，首次只完成配對，需再複製要傳送的文字並執行一次。

**簡易連線：** 不需要 Tailscale、VPN 或自製 iPhone App。桌面選「使用簡易連線」，臨時隧道就緒後掃 QR，安裝「TailBlink：簡易傳送」與「TailBlink：簡易取回」，按「連接並取回」。若按鈕無法帶入資料，先複製配對資料，再執行「簡易取回」。首次使用須允許網路與 iCloud Drive 權限。

QR 五分鐘有效且僅能配對一次。簡易連線在電腦重開機、TailBlink／隧道重啟或切換入口後，必須重新掃 QR；同一隧道程序持續運作時可繼續使用。兩種入口同時只啟用一個；新的簡易配對成功後，舊簡易憑證立即失效。

## 日常使用

**傳送：** iPhone 複製文字後執行對應的「傳送」捷徑，或從 App 分享選單執行；之後可在電腦貼上。

**取回：** iPhone 執行對應的「取回」捷徑；之後可貼上電腦目前的文字剪貼簿。電腦沒有文字時只通知，不清空手機原有剪貼簿。

Tailscale 入口的捷徑會呼叫 Tailscale 連線並短暫等待，不另顯示方向或裝置選單。連線失敗時確認兩端 Tailscale 與桌面 TailBlink 已啟動。桌面剪貼簿短暫忙碌會自動重試；若持續失敗，可從通知區結束 TailBlink 後重開。

Windows 通知區圖示可開啟設定頁、切換登入後自啟或結束程式。再次雙擊 EXE 會開啟已執行的 Agent 設定頁。移除時執行套件內 `Uninstall-TailBlink.cmd`，確認後只移除本工具，不移除 Tailscale。Linux 使用隨附的 `uninstall.sh`。

## 範圍與隱私

支援上限 1 MiB 的 UTF-8 純文字、多行文字、Unicode、Emoji 與文字網址；iPhone 以 iOS 26 Shortcuts／Share Sheet 為測試範圍。尚未支援圖片、HTML、檔案、Taildrop、X11、多桌面選擇、原生 iOS App、自動同步及自動更新。

不建立剪貼簿歷史或離線佇列，日誌不記錄內容、token 或配對網址。Tailscale 入口僅監聽 localhost，再由 Serve 提供 tailnet 內 HTTPS；不啟用 Funnel。HTTPS 憑證的完整裝置名稱會出現在 Certificate Transparency 紀錄，設定頁會事先說明。

簡易連線使用 Cloudflare 公開 HTTPS 中繼，請求必須帶配對憑證；它**不是裝置間端到端加密**，Cloudflare 可處理傳輸內容，Quick Tunnel 也沒有可用性保證。桌面簡易憑證只存在記憶體；iPhone 設定存在 iCloud Drive 的 `Shortcuts/TailBlink-Simple/config.json`。若臨時網址暫時無法連線，可稍後重試或換網路；程式不會自行修改 DNS。

## 開發文件

架構、API、安全邊界、實作決策與驗收範圍見 [技術與產品規格](docs/SPEC.md)。可用 `python3 tools/check_branding.py` 檢查目前工作樹及套件中的名稱殘留；macOS 另執行 `python3 shortcuts/sign.py --verify` 與 `python3 shortcuts/sign_simple.py --verify` 驗證已簽署捷徑與來源一致。

## 授權與商標

本專案尚未選定開源授權；在加入 LICENSE 前，程式碼與文件不應被視為已授予通用再利用權利。

TailBlink 是獨立專案，與 Tailscale Inc. 無隸屬或官方合作關係。Tailscale、Taildrop 與 Apple Shortcuts 等名稱屬其各自權利人所有。
