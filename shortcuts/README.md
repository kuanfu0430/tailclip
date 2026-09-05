# TailClip 原生捷徑

使用者安裝方式見專案 [README](../README.md)。`build.py` 是兩支原生捷徑的可重建來源，`send/spec.json` 與 `pull/spec.json` 說明各邏輯群組；手機只需要 Shortcuts 與 Tailscale。

## 重建與簽署

在專案根目錄，以 macOS 執行：

```sh
python3 shortcuts/build.py
python3 shortcuts/sign.py
python3 shortcuts/sign.py --verify
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
open -a Shortcuts build/shortcuts/native/TailClip-QA-Send.shortcut
open -a Shortcuts build/shortcuts/native/TailClip-QA-Pull.shortcut
python3 shortcuts/native_test.py run
```

首次執行需處理 Shortcuts 的檔案／網路隱私權提示；它可能在獨立的 ShortcutsViewService 視窗。CLI 有 60 秒逾時。測試會暫時使用 Mac 剪貼簿，完成或失敗後以所有 pasteboard 資料型別還原。請勿與日常複製貼上同時執行。

QA 只改 `TailClip-QA-20260905/config.json` 路徑、localhost URL 驗證、略過 iOS Tailscale ConnectIntent，並把通知換為原生 Stop and Output。其他動作與 production 同源。測試後關閉服務、刪除自己匯入的 QA 捷徑與 iCloud Shortcuts 中的 QA 資料夾；不要刪除正式 `TailClip` 設定。

17 項原生案例涵蓋配對、再次執行、Unicode、Share Sheet 文字輸入、1 MiB 與錯誤分支。結果寫入 `build/shortcuts/native/report.json`。Mac 原生執行與 Go API 通過不代表 iPhone、Windows 或 Tailscale 實機通過，裝置驗收範圍見 [SPEC](../docs/SPEC.md)。
