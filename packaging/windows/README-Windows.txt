TailBlink v0.2.0-alpha.3 Windows x64 完整更名測試版
=================================================

請完整解壓套件，保留 tailblink-cloudflared-*，再雙擊 TailBlink.exe。
不需安裝 Go。未購買 Windows code signing，SmartScreen 可能顯示警告。

從先前命名版本升級：
先使用原版附帶的解除安裝程式移除桌面端，再安裝 TailBlink。
本次更名包含設定目錄、自啟項目、Serve 路徑、配對標記與捷徑。
原版捷徑及配對不相容；請移除原版捷徑，從新版 QR 頁安裝並重新配對。
不自動搬移原版 token，也不偷偷刪除原版設定。勿讓兩個版本同時執行。

已有 Tailscale：
1. Windows 與 iPhone 安裝 Tailscale、登入同一 tailnet 並連線。
2. 執行 TailBlink.exe，選「使用現有 Tailscale」。
3. 程式安裝至目前使用者，開啟設定頁並常駐通知區。
4. 依畫面建立 /tailblink 的 Serve 路徑；權限不足時才要求 UAC。
5. iPhone 掃 QR，安裝「TailBlink：傳送」與「TailBlink：取回」。
6. 複製配對資料，執行「取回」完成配對；先執行「傳送」則首次只配對。

簡易連線（不需 Tailscale、VPN 或自製 iPhone App）：
1. 桌面選「使用簡易連線」，隧道就緒後用 iPhone 相機掃 QR。
2. 安裝「TailBlink：簡易傳送」與「TailBlink：簡易取回」。
3. 按「連接並取回」；若未帶入資料，先複製配對資料再執行「簡易取回」。
4. 首次允許網路與 iCloud Drive 權限。之後直接執行兩支簡易捷徑。

QR 五分鐘有效且只能配對一次。簡易入口在重啟電腦、TailBlink、隧道或切換
入口後須掃新 QR；捷徑不需每次重裝。新的簡易配對會使舊簡易憑證失效。
Cloudflare HTTPS 中繼可處理傳輸內容，並非裝置間端到端加密。
Quick Tunnel 沒有可用性保證。臨時網址無法連線時可稍後重試或換網路；
程式不會自動修改電腦 DNS。

日常傳輸：
iPhone 複製文字後執行「傳送」，或從分享選單執行；之後可在電腦貼上。
執行「取回」後可在 iPhone 貼上電腦目前的文字剪貼簿。
電腦剪貼簿沒有文字時只通知，不清空 iPhone 原有文字。
支援最多 1 MiB 的 UTF-8 文字，不支援圖片、HTML、檔案與背景自動同步。
不建立剪貼簿歷史，日誌不記錄內容或 token。

通知區與解除安裝：
按兩下 TailBlink 圖示開啟設定頁；右鍵可切換登入後自啟或結束程式。
再次雙擊 TailBlink.exe 或 Start-TailBlink.cmd 可重新啟動並開啟設定頁。
雙擊 Uninstall-TailBlink.cmd 並確認後解除安裝；不移除 Tailscale，
不更動其他 Serve 路徑。剪貼簿短暫忙碌會自動重試，持續失敗可重開程式。

套件驗證：
SHA256SUMS.txt 涵蓋全部附帶檔案；SOURCE.txt 是建置所用的來源 commit。
四支 .shortcut 是已簽署的備援檔，正常情況從 QR 配對頁安裝。
自動化與交叉編譯驗證不等於 iPhone、通知區、UAC 及登入重啟的完整實機驗收。
