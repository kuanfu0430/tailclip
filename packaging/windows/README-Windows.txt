TailClip v0.1-alpha Windows 11 x64 測試版
========================================

需要先完成：

1. Windows 與 iPhone 都已安裝 Tailscale。
2. 兩台裝置已登入同一個 tailnet 並連線。
3. iPhone 使用 iOS 26。

開始使用：

1. 雙擊 TailClip.exe。
2. 閱讀 HTTPS 名稱提示後繼續，並接受一次 Windows UAC。
3. TailClip 會自動安裝至目前使用者、自動啟動 Agent，並開啟配對 QR。
4. 用 iPhone 相機掃描 QR，依畫面安裝兩支捷徑並複製配對資料。
5. 執行「TailClip：取回」，或從分享選單執行「TailClip：傳送」。

不需要安裝 Go，也不需要手動輸入網址或 token。

重新開啟設定：

再次雙擊 TailClip.exe。

解除安裝：

雙擊 Uninstall-TailClip.cmd，再於確認畫面選擇同意。TailClip 不會移除 Tailscale 或改動其他 Serve path。

注意：

- 此版本尚未購買 Windows code signing，SmartScreen 可能顯示警告。
- TailClip-Send.shortcut 與 TailClip-Pull.shortcut 是已簽署的備援檔；正常情況由 QR 配對頁安裝，不需手動處理。
- SHA256SUMS.txt 可用來核對本資料夾內的執行檔與捷徑。
