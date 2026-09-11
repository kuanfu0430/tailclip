# TailBlink 原生捷徑

使用者安裝方式見專案 [README](../README.md)。`build.py` 是兩支原生捷徑的可重建來源，`send/spec.json` 與 `pull/spec.json` 說明各邏輯群組；這兩支既有捷徑使用 Shortcuts 與 Tailscale。

`build_simple.py` 另建「TailBlink：簡易傳送／簡易取回」，只需 Shortcuts，使用獨立 `TailBlink-Simple/config.json`。兩種入口的捷徑不能混用。掃臨時隧道 QR 後按「連接並取回」，重啟隧道後重掃，不必重裝捷徑。

alpha.2 修正備援剪貼簿票券在成功保存後未清除的問題，避免同次取回為空時下一次重放票券；從 alpha.1 升級須取代兩支簡易捷徑。連結／分享配對不清除原有文字，配對失敗及日常取回失敗亦保留剪貼簿。

## 重建與簽署

在專案根目錄，以 macOS 執行：

```sh
python3 shortcuts/build.py
python3 shortcuts/sign.py
python3 shortcuts/sign.py --verify
python3 shortcuts/sign_simple.py
python3 shortcuts/sign_simple.py --verify
python3 -m unittest discover -s shortcuts/tests -v
```

簽署需要 Apple 簽署服務可用。sign.py 先解封候選檔核對所有 actions／參數，通過才更新 `dist/*.shortcut` 及 `manifest.json`。CI 在其他平台只做來源與成品 hash 及結構檢查，不會假裝重新簽署或執行捷徑。

## Mac 原生整合測試

先啟動隔離服務；自動分配空閒 loopback port，不接觸桌面真實剪貼簿：

```sh
mkdir -p build/shortcuts/native
go run ./shortcuts/testhost --state build/shortcuts/native/state.json
```

在另一個終端產生 QA 捷徑並匯入兩個簽署檔：

```sh
python3 shortcuts/native_test.py prepare
open -a Shortcuts build/shortcuts/native/TailBlink-QA-Send.shortcut
open -a Shortcuts build/shortcuts/native/TailBlink-QA-Pull.shortcut
python3 shortcuts/native_test.py run
```

首次執行需處理 Shortcuts 的檔案／網路隱私權提示；它可能在獨立的 ShortcutsViewService 視窗。CLI 有 60 秒逾時。測試會暫時使用 Mac 剪貼簿，完成或失敗後以所有 pasteboard 資料型別還原。請勿與日常複製貼上同時執行。

QA 只改 `TailBlink-QA-20260905/config.json` 路徑、localhost URL 驗證、略過 iOS Tailscale ConnectIntent，並把通知換為原生 Stop and Output。其他動作與 production 同源。測試後關閉服務、刪除自己匯入的 QA 捷徑與 iCloud Shortcuts 中的 QA 資料夾；不要刪除正式 `TailBlink` 設定。

17 項原生案例涵蓋配對、再次執行、Unicode、Share Sheet 文字輸入、1 MiB 與錯誤分支。結果寫入 `build/shortcuts/native/report.json`。Mac 原生執行與 Go API 通過不代表 iPhone、Windows 或 Tailscale 實機通過，裝置驗收範圍見 [SPEC](../docs/SPEC.md)。

## 簡易連線原生 QA

先以 `go build -o build/deps/simple-testhost ./shortcuts/simple-testhost` 建置記憶體測試服務，並用 `packaging/package.py` 的 `dependency('darwin-arm64')` 下載核對過的 Mac cloudflared 到同一目錄；執行 `build/deps/simple-testhost --state build/simple-state.json`。測試會建立真正的公開臨時隧道，只連到合成記憶體剪貼簿；管理 fixture 只在隨機 loopback 埠。

執行 `python3 shortcuts/simple_native_test.py prepare`，將 `build/shortcuts/simple-native/` 的兩支 QA 捷徑匯入，再執行 `python3 shortcuts/simple_native_test.py run`。正式 builder 的 URL、HTTP 與資料流保持相同；QA 只用獨立設定路徑、名稱及可斷言的通知輸出。14 組案例及平台限制寫入該目錄 `report.json`；alpha.2 新增「備援配對空取回後再次執行」及「連結配對空取回保留文字」，兩項尚待原生執行，不沿用 alpha.1 的 12 組通過紀錄。完成後中止 testhost，移除兩支 QA 與 `TailBlink-QA-Simple-20260907` 設定；程式會在成功或失敗時還原 Mac 剪貼簿。

本次 Mac 系統 DNS 曾快取新臨時網址的查無結果，驗收時僅對 testhost 使用 `GODEBUG=netdns=go`，未修改系統 DNS；Shortcuts 仍直接使用正式 HTTPS 網址。Windows／Linux 發行包沒有套用此 Mac 測試環境設定。
