TailClip v0.2.0-alpha.2 雙入口測試版
=================================

請完整解壓，保留 tailclip-cloudflared-* 檔案，再雙擊 TailClip.exe。
已有 Tailscale 的設定、Serve、配對和捷徑保持不變。

簡易連線不需 Tailscale、VPN 或 iPhone App：
1. 桌面選「使用簡易連線」，等隧道就緒後用 iPhone 相機掃 QR。
2. 安裝「TailClip：簡易傳送」與「TailClip：簡易取回」。
3. 按「連接並取回」；若未帶入資料，複製配對資料後執行「簡易取回」。
4. 首次允許網路與 iCloud Drive 權限。以後直接執行兩支簡易捷徑。

QR 五分鐘有效且只能用一次。重啟電腦、TailClip、隧道或切換入口後須掃新 QR。
捷徑只需安裝一次；保持同一隧道運作即可繼續收發。
Cloudflare HTTPS 中繼可處理內容，不是端到端加密；Quick Tunnel 沒有可用性保證。
從 alpha.1 升級請重新安裝兩支「簡易」捷徑並取代舊版，修正備援配對後空取回導致票券重放的問題。
Windows 原生剪貼簿與檔案安裝檢查已通過；iPhone、通知區／UAC／登入重啟的完整實機驗收仍待完成。
若臨時網址持續無法連線，請稍後重試或換網路；程式不會自動修改電腦 DNS。
要退回 alpha.7，先切回「使用現有 Tailscale」再結束新版，保留設定檔。

以下為既有 Tailscale 入口的使用方式：
TailClip v0.1-alpha Windows 11 x64 測試版
========================================

需要先完成：

1. Windows 與 iPhone 都已安裝 Tailscale。
2. 兩台裝置已登入同一個 tailnet 並連線。
3. iPhone 使用 iOS 26。

開始使用：

1. 雙擊 TailClip.exe。
2. TailClip 會沿用本機已登入的 Tailscale tailnet 與既有 /tailclip Serve 設定。
3. 需要新增 Serve 時請閱讀 HTTPS 名稱提示；只有目前使用者權限不足時才會要求一次 Windows UAC。
4. TailClip 會自動安裝至目前使用者、啟動 Agent、常駐在右下角通知區，並開啟配對 QR。
5. 用 iPhone 相機掃描 QR，依畫面安裝兩支捷徑並複製配對資料。
6. 執行「TailClip：取回」，或從分享選單執行「TailClip：傳送」。

不需要安裝 Go，也不需要手動輸入網址或 token。

若已在 alpha.6 完成配對，這次只需執行新版 TailClip.exe；捷徑與配對資料不需要重裝或重設。alpha.7 修正電腦端剪貼簿偶發占用，並讓狀態查詢與文字傳輸依序執行。

若從 alpha.5 或更舊版升級，執行新版 TailClip.exe 後請從配對頁安裝兩支新版、複製配對資料並執行「TailClip：取回」。同名時選擇取代；日後請使用中文名稱的新版，避免誤開 TailClip-Send 2 等舊副本。既有桌面設定會保留。

第一次若先執行「TailClip：傳送」，只會完成配對；再複製要傳送的文字並執行一次。遇到「無效的 URL：/status」可按以上方式重新配對，不需手動改捷徑或刪除設定檔。

重新開啟設定：

按兩下右下角通知區的 TailClip 圖示，或在圖示的右鍵選單選擇「開啟連線與配對頁面」。結束 TailClip 後，可再次雙擊本資料夾的 TailClip.exe 啟動。

通知區選單：

- 「開啟連線與配對頁面」：檢查狀態、顯示 QR 或重新配對。
- 「登入 Windows 後自動啟動」：可直接勾選或取消；只控制 TailClip，不會更改 Windows 帳號登入方式。
- 「結束 TailClip」：結束目前執行中的 Agent；下次可再雙擊 TailClip.exe。

解除安裝：

雙擊 Uninstall-TailClip.cmd，再於確認畫面選擇同意。TailClip 不會移除 Tailscale 或改動其他 Serve path。

注意：

- 此版本尚未購買 Windows code signing，SmartScreen 可能顯示警告。
- TailClip-Send.shortcut 與 TailClip-Pull.shortcut 是已簽署的備援檔；正常情況由 QR 配對頁安裝，不需手動處理。
- SHA256SUMS.txt 可用來核對本資料夾內的執行檔與捷徑。

Start-TailClip.cmd 是一鍵啟動入口，會開啟 TailClip 服務與配對頁；直接雙擊 TailClip.exe 亦可。啟動腳本採 ASCII 編碼，避免中文系統字碼造成解析失敗。

剪貼簿暫時忙碌時會自動重試取得開啟鎖，最多 1 秒。若反覆失敗，請從通知區結束 TailClip 後重新啟動；其他應用若持續占用 Windows 剪貼簿，仍須等它釋放。
