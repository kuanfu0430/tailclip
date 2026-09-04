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
