# TailClip 技術與產品規格

- **文件版本：** 0.3
- **日期：** 2026-09-07
- **專案狀態：** v0.1-alpha 開發中；M2 通用捷徑需求與計畫待使用者確認，尚未實作
- **首版平台：** iOS 26、Windows 11 x64、Ubuntu 26.04 x86_64 GNOME Wayland
- **傳輸：** HTTPS over Tailscale Serve

---

## 0. 實作進度

最後更新：2026-09-07。

- [x] 將規格收斂為 UTF-8 純文字 M1，完成共用設定、認證、API、限流、配對與無內容日誌。
- [x] 完成 Windows Win32 clipboard backend、自我安裝、自啟、Serve 設定、HTTPS health 與本機 QR 頁。
- [x] 建立並簽署「TailClip：傳送」與「TailClip：取回」，將成品內嵌至 Windows／Linux binary。
- [x] 完成 Ubuntu Wayland backend、安裝腳本與 `systemd --user` service。
- [x] 通過 format、vet、一般／race 測試、Windows／Linux x86_64 交叉編譯及 release 組裝測試。
- [x] 產生可直接下載測試的 Windows x64 ZIP；使用者不需 Go 或自行編譯。
- [x] `v0.1.0-alpha.2` 改以 Win32 `RtlMoveMemory` 複製 clipboard buffer，排除 Windows vet 的 uintptr 轉指標警告，不增加第三方依賴。
- [x] `v0.1.0-alpha.3` 修正本機設定頁 QR 被 `html/template` 改寫為 `#ZgotmplZ` 的問題，並讓 Windows 首次安裝以前景程序同步完成、重用既有 Serve、目前使用者權限優先及必要時 UAC fallback。
- [x] `v0.1.0-alpha.4` 修正兩支 Shortcut 的空白條件與 RTF 隱式轉 URL 問題，並以原生 Win32 通知區圖示加入開啟設定頁、自啟切換與結束操作；安裝新版時會停止仍在執行的舊版 Agent 並啟動新版。已完成捷徑重開、重新匯出、簽署、Windows x64 交叉編譯與完整 ZIP 驗證；Git 追蹤的同名 ZIP 必須與 GitHub Release 的乾淨 tag 建置完全一致，不得保留由前一 commit 的 dirty 工作樹產生、僅版本字串相同的候選包。
- [x] `v0.1.0-alpha.5` 明確將四個 API 回應從 URL 內容解析為 Dictionary，再讀取欄位，排除 iOS 將 JSON 回應視為文字時的「文字無法轉換到辭典」錯誤；已完成捷徑重開、重新匯出、簽署、測試與 Windows x64 ZIP 解壓驗證。
- [x] `v0.1.0-alpha.5` 後續修正 Windows 升級清理時序：先完成舊版 Agent 停止與新版接手，再重試移除暫存的 `TailClip.exe.old`，避免執行中的舊映像仍被 Windows 鎖定而留下整份舊 EXE。
- [x] `v0.1.0-alpha.6` 重建兩支捷徑：明確 UUID 資料流、設定驗證、固定 config.json 檔名及讀回、重新配對、空值與 API 錯誤處理；macOS 15.7.7 原生 Shortcuts + 真實 Go API 的 17 項隔離整合驗收全通過。
- [x] `v0.1.0-alpha.7` 修正 Windows 剪貼簿操作未固定 OS thread、狀態查詢未與讀寫序列化及 CloseClipboard 結果被忽略的問題；維持同一捷徑／API／配對格式。
- [x] 2026-09-05 使用者回報 alpha.6 iPhone ↔ Windows 核心捷徑傳輸實機測試成功，確認本次修復可用；未將此回報擴張為以下完整平台驗收。
- [ ] 在 Windows 11 x64 與實際 iPhone 完成首次安裝、QR 配對、雙向文字、Share Sheet、自啟、重開機與解除安裝驗收。
- [ ] 在 Ubuntu 26.04 GNOME Wayland 與實際 iPhone 完成相同 E2E。
- [ ] 實機驗收通過後，才把對應平台從 build candidate 改標為已支援。
- [x] 2026-09-07 讀取 GitHub issue #1 與目前來源碼，完成 M2 需求、取捨與實作計畫草案（第 13、14 節）。此勾選僅代表文件完成。
- [ ] 使用者確認 M2 計畫後，依第 14 節執行；未開始功能程式碼修改或新版本封裝。

目前決策：Git 只追蹤版本化的完整 Windows 測試 ZIP 與其 checksum；裸 EXE、臨時 staging 目錄及其他可重建輸出仍忽略。原因是測試者必須能從 GitHub 直接下載使用，但倉庫不應混入每次 CI 都會變動的中繼檔。

## 1. 產品目標

TailClip 讓使用者在 iPhone 與 Windows／Linux 電腦之間手動傳送目前的純文字剪貼簿。它利用既有的 Tailscale 私有網路，不建立公開中繼站、帳號系統或剪貼簿歷史。

首版的日常操作必須維持一步完成：

- 在 iPhone 執行「TailClip：傳送」，桌面剪貼簿立即變成 iPhone 的 Share Sheet 文字或目前剪貼簿文字。
- 在 iPhone 執行「TailClip：取回」，桌面目前的文字剪貼簿立即寫入 iPhone 剪貼簿。
- 日常操作不得再要求選方向、選裝置、輸入網址或貼上 token。

第一次設定可以包含 Apple／Tailscale 無法省略的安裝與授權畫面，但 TailClip 必須自動完成可自動化的部分。

## 2. v0.1-alpha 範圍

### 2.1 必須支援

- UTF-8 純文字、Unicode、Emoji 與多行文字。
- 最大 1,048,576 UTF-8 bytes。
- iOS 剪貼簿傳送至 Windows 11 x64。
- Windows 11 x64 剪貼簿取回至 iOS 26。
- iOS Share Sheet 的純文字與網址傳送；網址在本版視為純文字。
- Ubuntu 26.04 x86_64 GNOME Wayland 的雙向文字傳送。
- 兩支固定捷徑：「TailClip：傳送」與「TailClip：取回」。
- iOS 端在需要時自動連線 Tailscale。
- Windows 單一 EXE 自動安裝、通知區常駐、可切換的使用者登入自啟、Serve 設定、診斷與 QR 配對。
- Ubuntu 一支安裝腳本、`systemd --user` 自啟、Serve 設定、診斷與 QR 配對。
- 未配對請求無法讀取或覆蓋桌面剪貼簿。
- 不保存剪貼簿內容、不上傳遙測、不啟用 Tailscale Funnel。

### 2.2 本版不做

- HTML、Rich Text、圖片、實體檔案與 Taildrop。
- Linux X11、KDE、其他發行版與 headless session。
- 多台桌面選擇或多組獨立 iPhone token。
- Linux 桌面選單。
- 原生 iOS App、Keychain、App Intents。
- 被動背景監聽、完全自動同步、離線排隊。
- 自動更新、Windows code signing、deb／rpm／AppImage。
- 公開 SaaS、跨 tailnet 傳送或剪貼簿歷史。

除使用者於 2026-09-07 指示提前規劃的 M2（第 13、14 節）外，這些項目只有在目前版本通過實機驗收後才進入後續規格，不先提交尚未使用的抽象層。M2 規劃不代表 M1 未完成驗收已通過。

## 3. 核心使用流程

### 3.1 Windows 首次設定

1. 使用者下載 Windows x64 ZIP、解壓後雙擊其中唯一需要執行的 `TailClip.exe`；不需安裝 Go 或其他 runtime。
2. 程式將自身安裝到 `%LOCALAPPDATA%\TailClip`，建立使用者層自啟並啟動 Agent；原始前景程序同步完成後續流程，不交給無畫面的安裝子程序。
3. TailClip 讀取本機已登入的 Tailscale tailnet、連線狀態與 `*.ts.net` 名稱，並檢查 `127.0.0.1:17733` 與 `127.0.0.1:17734`。
4. 若既有 `/tailclip` Serve path 已指向 `127.0.0.1:17733`，直接重用，不重新設定或要求 UAC。
5. 只有需要新增 Serve path 時才顯示 Certificate Transparency 說明；先以目前使用者權限設定，權限不足才要求一次 UAC，且不得重設其他 Serve 設定。
6. TailClip 驗證公開 HTTPS health endpoint。
7. 本機設定頁顯示可實際載入與掃描的限時 QR。
8. iPhone 掃 QR，安裝兩支捷徑並點一下複製配對資料，再執行「取回」完成配對並取得電腦文字。先執行「傳送」時只完成配對，需再複製內容後執行一次。

TailClip 不建立新的 tailnet、也不替兩台裝置執行 Tailscale 帳號配對；Windows 與 iPhone 必須已登入同一個 tailnet。安裝時不要求指定 peer 當下在線，避免 iPhone 暫時離線時阻擋桌面端設定。

安裝完成後 Agent 必須在 Windows 通知區顯示 TailClip 圖示。按兩下圖示或選擇「開啟連線與配對頁面」會打開本機設定頁；「登入 Windows 後自動啟動」可直接勾選或取消，且不得改動 Windows 帳號登入方式；「結束 TailClip」只結束目前程序。再次雙擊已安裝的 EXE 只開啟本機設定頁。解除安裝必須經過明確確認，只移除 TailClip 自啟、程序、檔案與自己的 Serve path。

### 3.2 Ubuntu 首次設定

1. 使用者執行隨 release 提供的 `install.sh`。
2. 腳本檢查 Ubuntu 26.04 x86_64、Tailscale 與 `wl-clipboard`。
3. 缺少 `wl-clipboard` 時，腳本以單次 `sudo apt-get install` 補齊。
4. 腳本安裝 binary 與 `systemd --user` unit，啟動 Agent。
5. 腳本以 `sudo tailscale serve` 新增 `/tailclip` path，完成 HTTPS health check。
6. 開啟與 Windows 相同的本機設定／QR 頁。

### 3.3 每日傳送

1. 「TailClip：傳送」先驗證共用設定；一般傳送以 Share Sheet input 為內容來源，沒有 input 才使用 iOS 剪貼簿。若剪貼簿是新配對資料則優先進行配對，本次不傳送。
2. 捷徑呼叫 Tailscale Connect 並等待兩秒；已連線時保持連線。
3. 捷徑送出 `POST /v1/clipboard/text`。
4. Agent 驗證 token、內容類型與大小後寫入桌面剪貼簿。
5. 捷徑顯示不需確認的短通知：「已傳到〈裝置名〉」。

### 3.4 每日取回

1. 「TailClip：取回」確認 Tailscale 已連線。
2. 捷徑呼叫 `GET /v1/clipboard/text`。
3. Agent 即時讀取桌面剪貼簿，不讀取歷史或快取內容。
4. 捷徑以 Local Only 寫入 iOS 剪貼簿。
5. 捷徑顯示不需確認的短通知：「已從〈裝置名〉取回」。

## 4. 系統架構

```text
iOS 26 Shortcuts
    │ HTTPS request + Bearer token
    ▼
Tailscale Serve: https://<device>.<tailnet>.ts.net/tailclip
    │ proxy only
    ▼
127.0.0.1:17733 TailClip Agent
    ├─ HTTP API
    ├─ token、大小、逾時與限流
    ├─ 限時配對頁
    └─ Clipboard Backend
         ├─ Windows CF_UNICODETEXT
         └─ Linux wl-copy / wl-paste

127.0.0.1:17734 Local Setup UI
    └─ 診斷、Serve 設定、QR 與 token 輪替
```

設計限制：

- Agent 固定監聽 `127.0.0.1:17733`，不得提供切換成 `0.0.0.0` 的設定。
- 設定頁固定使用 `127.0.0.1:17734`，只在開啟設定時啟動。
- 綁定前必須檢查 port；被其他程式占用時顯示程序與處理方式，不自動改成隨機 port。
- Serve 只代理 Agent port，不代理本機設定 port。
- 不使用資料庫、訊息佇列、容器、web framework、RPC generator 或 plugin system。

## 5. HTTP API

### 5.1 公開位址

外部 base URL：

```text
https://<device>.<tailnet>.ts.net/tailclip/v1
```

Agent 本機 base URL：

```text
http://127.0.0.1:17733/v1
```

Tailscale Serve 會在代理前移除 `/tailclip` mount prefix，因此 Agent 只承載 `/v1` 路由；公開 HTTPS health 仍必須在實機安裝流程中驗證。

### 5.2 共通規則

- JSON 欄位使用 `snake_case`。
- 回應 `Content-Type` 為 `application/json; charset=utf-8`。
- 除 `/health` 與限時 setup route 外都要求 `Authorization: Bearer <token>`。
- token 比對使用 constant-time comparison。
- request body 依 JSON 最壞跳脫量設定硬上限；解碼後文字上限固定為 1,048,576 bytes。
- HTTP server 必須設定 header、read、write 與 idle timeout。
- 已知錯誤回傳 JSON；不得回傳 stack trace、token、內容或本機機密路徑。

### 5.3 `GET /v1/health`

不要求 token，也不得讀取剪貼簿。

```json
{
  "status": "ok",
  "service": "tailclip-agent",
  "api_version": 1,
  "agent_version": "0.1.0-alpha.1"
}
```

### 5.4 `GET /v1/status`

要求 token，用於配對驗證與診斷。

```json
{
  "ok": true,
  "device_name": "工作電腦",
  "tailscale_device": "work-pc",
  "platform": "windows",
  "clipboard_available": true,
  "max_text_bytes": 1048576
}
```

此端點只檢查 backend，不得讀取實際剪貼簿內容。

### 5.5 `POST /v1/clipboard/text`

要求 token。

```json
{
  "text": "從 iPhone 傳來的文字"
}
```

成功：

```json
{
  "ok": true,
  "device_name": "工作電腦",
  "bytes": 31
}
```

規則：

- `text` 必須存在且為 JSON string。
- 空字串、NUL、無效 UTF-8 與超過大小限制均拒絕。
- 寫入同步完成後才回成功。
- 同一時間的剪貼簿讀寫以程序內 mutex 序列化。

### 5.6 `GET /v1/clipboard/text`

要求 token。每次請求都即時讀取桌面剪貼簿。

有文字：

```json
{
  "ok": true,
  "device_name": "工作電腦",
  "text": "desktop clipboard content"
}
```

沒有可用文字：

```json
{
  "ok": true,
  "device_name": "工作電腦",
  "empty": true
}
```

### 5.7 錯誤

```json
{
  "ok": false,
  "error": {
    "code": "clipboard_busy",
    "message": "剪貼簿目前正被其他程式使用，請再試一次。"
  }
}
```

| HTTP | code | 用途 |
|---:|---|---|
| `400` | `invalid_request`／`invalid_text` | JSON、欄位、UTF-8 或 NUL 無效 |
| `401` | `not_paired` | token 缺少或無效 |
| `405` | `method_not_allowed` | 方法錯誤 |
| `413` | `text_too_large` | 解碼後文字超過 1 MiB |
| `429` | `rate_limited` | 超過每分鐘 60 次全域限制 |
| `503` | `clipboard_busy`／`clipboard_unavailable` | backend 暫時或持續不可用 |
| `500` | `internal_error` | 未預期錯誤 |

## 6. 配對

### 6.1 Token

- 初次啟動產生 32-byte 密碼學安全亂數，以無 padding base64url 儲存。
- Windows 設定放在 `%LOCALAPPDATA%\TailClip\config.json`，繼承使用者私有 ACL。
- Linux 設定放在 `${XDG_CONFIG_HOME:-~/.config}/tailclip/config.json`，目錄 `0700`、檔案 `0600`。
- token 不得出現在命令列參數、log、Shortcut artifact 或公開錯誤中。
- 本機設定頁可建立新配對頁；只有明確執行「撤銷並重新配對」才輪替 token。

### 6.2 限時設定頁

- 只能由本機設定頁建立。
- nonce 至少 192-bit，保存於記憶體，15 分鐘失效。
- 公開 route 為 `/setup/<nonce>`，只在 tailnet 內透過 Serve 可達。
- 所有回應設定 `Cache-Control: no-store`、`Referrer-Policy: no-referrer` 與不依賴外站的 CSP。
- access log 只記錄 `setup` route 類型，不記錄 nonce 或完整 URL。
- 頁面內含兩個無秘密、已簽署的 `.shortcut` 下載與一個「複製配對資料」按鈕。

配對資料格式：

```json
{
  "version": 1,
  "base_url": "https://work-pc.example.ts.net/tailclip/v1",
  "token": "<base64url>",
  "device_name": "工作電腦",
  "tailscale_device": "work-pc"
}
```

捷徑在儲存前必須驗證 version、HTTPS、`.ts.net` hostname、token 格式與 `/status`。成功後保存同一份 JSON 到 `iCloud Drive/Shortcuts/TailClip/config.json`，並清除剪貼簿中的配對資料。

## 7. 平台實作

### 7.1 共用 Go 核心

- Go module：`github.com/kuanfu0430/tailclip`。
- Go toolchain：1.27.1。
- binary 名稱：Windows `TailClip.exe`；Linux `tailclip`。
- 核心 clipboard interface 只有 `Available`、`ReadText`、`WriteText`。
- 除標準函式庫外，只使用 `golang.org/x/sys` 與一個離線 QR encoder。
- config 使用 JSON 與明確版本，不加入 TOML parser。
- 使用 `log/slog`；日誌只含時間、方向、bytes、status、error code 與 latency。

### 7.2 Windows 11 x64

- 發行物為一個可直接解壓的 ZIP，內含 `TailClip.exe`、兩支已簽署 Shortcut、Windows 使用說明、版本、內部 checksum 與解除安裝入口。
- 安裝只要求雙擊 `TailClip.exe`；其他檔案是說明、備援或移除入口，不得要求使用者執行額外安裝腳本。
- Agent 必須在互動式登入使用者 session 運行，不建立 Session 0 service。
- 使用 Win32 `OpenClipboard`、`EmptyClipboard`、`SetClipboardData`、`GetClipboardData` 與 `CF_UNICODETEXT`。
- 剪貼簿的狀態查詢、讀取與寫入共用同一個 mutex；完整 native 交易透過 runtime.LockOSThread 固定 OS thread，按 Create owner → Open → 操作 → Close → Destroy owner 順序執行，清理結果不可忽略。
- 每次交易使用有效的隱藏 message-only owner 視窗；資料使用立即呈現的 CF_UNICODETEXT，不保留視窗或延遲呈現狀態。
- 只有 ERROR_ACCESS_DENIED 對應的占用錯誤做短暫 exponential backoff，重試取得開啟鎖的總等待上限為 1 秒，每次等待裁切至剩餘預算；context 取消後不開始新操作。其它 Win32 錯誤直接回報，不能全數標示為「其他程式使用中」。
- EXE 第一次執行自我複製、註冊 `HKCU` 自啟，並由原始前景程序同步完成安裝；Agent 本身才以背景模式啟動。
- EXE 原地升級先以 `.old` 保留正在使用的舊映像，待舊版 Agent 停止、新版 Agent 與設定流程完成後再重試清除；不得在舊 Agent 仍執行時只做一次忽略錯誤的刪除。
- Agent 使用 Win32 通知區圖示提供「開啟連線與配對頁面」、「登入 Windows 後自動啟動」核取項目與「結束 TailClip」；不為此導入 GUI framework 或額外背景程序。
- 已存在且目標正確的 `/tailclip` Serve path 直接重用；缺少時先由目前使用者設定，只有失敗且提升權限可能有幫助時才使用同一 EXE 的 elevated helper。
- Serve 完成後必須直接驗證公開 HTTPS health，再開啟設定頁；本機設定頁的 QR 必須在實際瀏覽器中載入，不得出現模板安全替代值。
- 本版不宣稱 binary 已簽章；下載與 SmartScreen 提示在 README 說明。

#### alpha.7 決策與依據

使用者在 alpha.6 核心傳輸成功後遇到 `clipboard_busy`。此訊息來自桌面 API；程式檢查找到未固定 OS thread 及 Available 未序列化兩項可導致間歇性占用的缺陷。舊邏輯在隔離回歸副本中重現實際 OS thread 遷移與狀態／讀寫重疊；這是已確認的程式缺陷，尚無該次 Windows 現場的持鎖程序證據，不能斷言所有占用皆由 TailClip 引起。

依 [Go runtime.LockOSThread](https://pkg.go.dev/runtime#LockOSThread) 保留 OS thread 狀態；依 [Microsoft OpenClipboard](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-openclipboard) 使用有效 owner 並關閉每次成功的 Open。立即呈現的剪貼簿資料由 Windows 接管，無需長期保留 owner 視窗，參見 [Microsoft Clipboard Operations](https://learn.microsoft.com/en-us/windows/win32/dataxchg/clipboard-operations)。升級停止舊版 Agent 後可清除舊程序遺留的占用；不修改手機捷徑、不輪替 token，也不強制關閉其他應用。

### 7.3 Ubuntu 26.04 GNOME Wayland

- 只支援 `WAYLAND_DISPLAY` 存在的圖形使用者 session。
- backend 使用 `wl-copy` 與 `wl-paste`，不使用 XWayland clipboard 作為 fallback。
- `systemd --user` service 在 `graphical-session.target` 後啟動。
- 缺少 session 或工具時 `/status` 回 `clipboard_available:false`，讀寫回 `503`。
- v0.1-alpha 只發布 x86_64 tarball。

## 8. iOS Shortcuts

### 8.1 `TailClip：傳送`

- 接受 Share Sheet 的 Text 與 URL。
- 傳送內容以 Shortcut Input 優先，沒有才使用本次開始時取得的剪貼簿文字；兩種來源都拒絕空字串與配對 JSON。
- config 不存在時提示複製配對資料；每次執行有新配對 JSON 時優先配對，即使舊設定損壞也可修復。不把配對 JSON 當作剪貼簿內容傳送。
- `/status` 與 `/clipboard/text` 的文字位址必須先經過原生 `URL` action，再交給 `Get Contents of URL`，不得依賴 iOS 將 RTF 隱式轉為 URL。
- 每次 `Get Contents of URL` 後必須明確使用 `Get Dictionary from Input` 解析 JSON；後續 `Get Dictionary Value` 只可讀取該 Dictionary 輸出，不得依賴 iOS 將文字隱式轉為辭典。
- 呼叫 Tailscale Connect 並等待兩秒；日常傳輸只送一次 API request，新配對另需一次 /status 驗證。
- POST JSON 至 `/clipboard/text`，解析回應後顯示短通知。

### 8.2 `TailClip：取回`

- 與傳送捷徑使用相同 config 與連線流程。
- 所有 `If` action 只保留必要且已填值的條件列；不得存在會觸發「請選擇此動作中每個參數的值」的空白條件。
- `/status` 與 `/clipboard/text` 的位址同樣先經過原生 `URL` action。
- `/status` 與 `/clipboard/text` 的回應同樣先經過 `Get Dictionary from Input`，再讀取 `ok`、`text` 或 `error`。
- GET `/clipboard/text`。
- API 的 `empty:true` 對應空 `text`；捷徑以 text 是否有內容決定，空值顯示「電腦剪貼簿沒有文字」，不清空手機剪貼簿。
- 有文字時使用 Copy to Clipboard 並開啟 Local Only，再顯示短通知。

### 8.3 建立與交付

- 每支捷徑先有 JSON build spec，並通過規格 validator。
- 只使用 iOS／Tailscale 原生 actions，不使用 Run Shell Script 或 Run AppleScript。
- 以版本控制的 Python builder 產生原生 action plist，再匯入 macOS Shortcuts 重開及 CLI 實測；iPhone／Windows 實機驗收另行記錄，不以簽署或檔案大小代替執行驗證。
- builder 輸出經 `shortcuts sign --mode anyone` 簽署；解封後比對所有 actions 與參數，僅忽略簽署器移除的 WFWorkflowName 與重寫的 WFWorkflowClientVersion。manifest 記錄來源與成品 SHA-256，CI 拒絕 builder 與成品不一致。
- 簽署前 artifact 不得含真實 endpoint、token 或個人資料。

### 8.4 alpha.6 捷徑重建決策

- 舊成品與 build spec 不一致：token 取值未綁定辭典、Save File 缺少目的檔名、未驗證設定欄位，取回也未處理錯誤與空值。`/status` 錯誤表示前綴已為空，須在建 URL 之前攔截，而非繼續增加型別轉換。
- 合併配對與讀檔後的解析流程；所有取值明確綁定同一辭典，網址與認證直接引用動作輸出，避免跨分支命名變數漂移。
- 新複製的配對 JSON 優先於既有檔案，通過格式與 `/status` 驗證後才覆寫 `Shortcuts/TailClip/config.json`，因此可修復壞設定或更新已輪替的 token。
- 傳送捷徑完成配對後停止，提示使用者複製要傳送的文字再執行；配對資料不得作為 payload。取回可在配對後立即取得電腦文字。
- 原生驗收確認 Save File 會依內容型別改副檔名，須先 Set Name(config.json, WFDontIncludeFileExtension=false)，儲存後讀回存在才清除配對剪貼簿。Text(空字串) 仍可包含一個項目，傳送改用非空字串正則符合結果判斷，避免空內容走到 HTTP。
- 配對頁以 UTF-8 Content-Disposition 檔名交付中文名稱，避免使用者繼續誤開 TailClip-Send 2 等舊副本。
- 成功前均檢查 API `ok`；空剪貼簿或錯誤不改寫手機文字。原生網路動作的連線／TLS 錯誤由 Shortcuts 呈現，不宣稱有該動作未提供的自訂 timeout 或 try/catch。

## 9. 安全與隱私

必須：

- 只綁定 loopback；不得提供危險覆寫旗標。
- 只使用 Tailscale Serve，安裝流程不得執行 Funnel。
- 所有剪貼簿 endpoint 驗證 token。
- 全域限制每分鐘 60 次 request。
- 限制 body、headers 與各種 server timeout。
- setup nonce 與 token 使用 `crypto/rand`。
- HTML 不載入第三方 script、font、image 或 analytics。
- 不記錄 request body、response text、token、clipboard hash 或完整 setup URL。
- 不建立內容 cache、歷史、離線佇列或 crash upload。

邊界：TailClip 無法防止已取得本機登入 session 的惡意程式讀取系統剪貼簿，也無法保護已被攻陷的 iPhone 或桌面。

## 10. 測試與驗收

### 10.1 自動測試

- config：初建、權限、無效版本、token 產生與輪替。
- API：health、status、auth、JSON、NUL、空字串、1 MiB 邊界、過大內容、rate limit。
- clipboard：fake backend、並行序列化、平台錯誤映射。
- pairing：nonce entropy、過期、no-store headers、未知 nonce、輸出無秘密日誌。
- Serve：既有 root mapping 保留、同 path 衝突、idempotent setup、絕不 reset。
- CI：format、vet、race tests、Windows x64 build、Linux x86_64 build、捷徑資料流與成品來源雜湊。
- alpha.7：本機測試涵蓋完整交易保持 OS thread、所有成功／失敗／panic 路徑清理、狀態／讀寫互斥、取消、短暫占用恢復、永久占用截止及非占用錯誤不重試。舊版語意的隔離副本在「執行緒遷移」與「並行操作」兩項測試失敗，修正後通過。
- Windows 原生測試 `TestWindowsClipboardIntegration` 含 100 輪 Unicode 寫入／狀態／讀回、90 個並行查詢／傳輸、另一 OS thread 持鎖後釋放、等待取消及持續占用後恢復；CI 的可丟棄 Windows runner 以 `TAILCLIP_WINDOWS_CLIPBOARD_TEST=1` 啟用。一般 go test 會略過此實體剪貼簿測試，避免改寫開發者內容。本次只完成 Windows 測試 EXE 交叉編譯，未取得 Windows 執行結果。
- 2026-09-05 macOS 原生整合：17/17 通過。正式簽署成品已在 Mac 以中文名稱匯入並重開；QA 副本與設定已清理，原始剪貼簿已還原。涵蓋首次配對保存／讀回、分享 Unicode/CRLF/跳脫、剪貼簿 fallback、網址、取回、雙向空值、配對憑證拒送、1 MiB／超量、四種無效設定、失效 token、失敗配對保留設定及損壞設定修復。
- 原生測試從同一 builder 產生 QA 捷徑，只替換設定路徑、loopback URL 規則、移除 iOS ConnectIntent、將通知換為原生 Stop and Output；HTTP、檔案、JSON 與剪貼簿動作保持原生。testhost 使用真實 API 與記憶體剪貼簿，port 由 OS 在 127.0.0.1 分配。這些結果不代表 iPhone、Windows 剪貼簿或 Tailscale 跨裝置連線已驗收。

### 10.2 Windows 實機

2026-09-05，使用者在收到 alpha.6 配對與雙向傳輸操作步驟後回覆「我測試成功了」。據此記錄 iPhone ↔ Windows 核心傳輸成功；下列完整矩陣仍待逐項確認，包括自啟、重開機、解除安裝及邊界條件。這是使用者實機回報，與 10.1 的 agent 原生測試分開記錄。

- 中文、英文、Emoji、多行、LF／CRLF 與 1 MiB 文字雙向傳送。
- clipboard 被暫時鎖定時可重試並於 1 秒內結束。
- 第一次安裝、再次雙擊、使用者登入自啟、重開機、token 輪替與解除安裝。
- 通知區圖示持續存在；可由圖示開啟配對頁、切換登入後自啟及結束目前程序。
- Tailscale 未安裝、未連線、HTTPS 未啟用、Serve path 衝突與 port 占用均有可理解指引。

### 10.3 iOS 26 實機

核心捷徑傳輸的使用者成功回報同 10.2；未據此宣稱所有分享類型、離線恢復或 iOS 背景行為均已測試。

- 兩支 shortcut 可安裝、重開與執行。
- 所有 API request 使用明確 URL 型別，且所有 `If` 條件均已完整設定。
- 一次 QR 配對，不輸入 endpoint 或 token。
- 自動連線 Tailscale。
- 剪貼簿與 Share Sheet 文字／網址傳送。
- Local Only 取回、空剪貼簿、離線與 token 無效。
- 成功通知不需要額外確認。

### 10.4 Ubuntu 26.04 實機

- GNOME Wayland 雙向文字。
- `systemd --user` 在登出／登入後恢復。
- 缺少 `wl-clipboard`、沒有 Wayland session 與 Serve 失敗均能診斷。

### 10.5 完成定義

Windows alpha 只有在「下載後雙擊一次、至多一次必要 UAC、iPhone 不手輸設定、日後每個方向一次動作」全流程通過後才算完成。

完整 M1 只有在實際 iPhone、Windows 11 x64 與 Ubuntu 26.04 x86_64 GNOME Wayland 全部通過後才可標示支援。未完成實機測試的平台只能標示為 build candidate。

## 11. 發布

- 第一個 tag：`v0.1.0-alpha.1`。
- 第一個可下載測試包為 `v0.1.0-alpha.1`；目前 Windows build candidate 為 `v0.1.0-alpha.7`（本機建置，未發布 GitHub）。
- alpha.6 本機 ZIP 由乾淨來源 commit `e644810` 建置，EXE 的 `vcs.modified=false`；Windows x64 GUI PE、兩支內嵌捷徑、ZIP 每檔 SHA-256 與 ASCII／CRLF 啟動腳本已核對。未執行 GitHub 發布或任何雲端倉庫操作。
- alpha.7 本機 ZIP 由乾淨來源 commit `b4ecbd0` 建置，EXE 的 `vcs.modified=false`；已核對 Windows x64 GUI PE、8 個封裝檔案、內嵌捷徑與全檔 SHA-256。既有 alpha.6 捷徑直接相容，不需重新配對。
- Git 追蹤的 Windows 測試產物：`dist/TailClip-v0.1.0-alpha.7-windows-x64.zip` 及其 `.sha256`；舊版測試包保留供回歸比對。
- GitHub Release artifacts：版本化 Windows x64 ZIP、Linux x86_64 tarball、兩支 signed Shortcuts 與 `SHA256SUMS`。
- Windows ZIP 必須包含 `TailClip.exe`、`README-Windows.txt`、`Start-TailClip.cmd`、`Uninstall-TailClip.cmd`、`VERSION.txt`、兩支 signed Shortcuts 與包內 `SHA256SUMS.txt`。
- `Start-TailClip.cmd` 使用 ASCII／CRLF 與完整引號路徑啟動 TailClip，供一鍵啟動服務與配對頁；原有 EXE 直接啟動入口保留。
- release ZIP 解壓後只需雙擊 `TailClip.exe`；不得要求終端機、Go toolchain 或手動複製檔案。
- CI 不保存或產生真實配對 token。
- 本版沒有自動更新；升級前保留相容的 `config.json`，未知 config version 安全停止並提示重新設定。

## 12. 後續 Roadmap

M2 通用捷徑改依第 13、14 節規劃；其餘項目仍於 M1 實機完成後再評估：

1. 通用捷徑、明確使用者授權與多目標選擇（M2）；不再預設採用每裝置 token。
2. `text/uri-list`、HTML + plain fallback、PNG。
3. Taildrop 檔案傳送與受控暫存。
4. X11／其他 Linux 桌面與正式套件。
5. 原生 iOS App、Keychain 與 App Intents。
6. 明確 opt-in 的半自動同步。

上述項目目前不是相容性承諾，也不應提前建立程式碼骨架。

## 13. M2 需求 SPEC：Tailnet 通用捷徑

### 13.1 來源、狀態與適用範圍

- 來源：[GitHub issue #1：Tailnet 通用捷徑免 QR 配對 + 預計實作計劃](https://github.com/kuanfu0430/tailclip/issues/1)。2026-09-07 讀取時為 open，最後更新 2026-09-05，沒有留言。
- 本次核對的本機來源為 `04d55b9`，分支 `codex/new_fork`，修改前工作樹乾淨；未 fetch、pull、push 或修改 issue。
- **本節是待確認需求，不是已交付功能。** 第 1–11 節繼續描述 M1；M2 完成時才將相應章節更新為新行為並保留必要的舊版相容說明。
- 保留 issue 的目標：Apple 裝置共用通用捷徑、免逐台 QR 配對、可切換主機、明確授權。調整實作順序及服務發現方式，避免將未驗證的捷徑能力當成既定事實。
- M2 第一個交付單位為 Windows／既有 Linux Agent 的身分授權與 iPhone、iPad、Mac 客戶端捷徑。Mac 作為被遠端讀寫的主機是獨立 M3，不能以 Mac 捷徑通過代替 Mac Agent 驗收。

### 13.2 已確認依據與待實測事項

| 依據 | 結論及影響 |
| --- | --- |
| `shortcuts/build.py`、`internal/api/server.go` | 現行捷徑保存單一 `base_url + token`，API 本身沒有綁定 iPhone；可沿用文字 API 與 backend。 |
| `shortcuts/build.py`、`shortcuts/README.md` | 目前正式捷徑寫死 iOS ConnectIntent；既有 Mac QA 略過此動作，因此尚未證明同一份正式成品跨 Apple 平台可用。 |
| `internal/agent/agent.go`、`internal/config/config.go` | 本機管理與外部 API 共用 PairingToken；改外部認證前必須拆出本機控制憑證。設定檔目前 version 1，嚴格拒絕未知欄位。 |
| `internal/webui/webui.go` | 捷徑下載目前綁限時 setup nonce；免 QR 也必須解開「取得捷徑本身」對配對頁的依賴。 |
| `internal/clipboard/backend_unsupported.go`、`cmd/tailclip/platform_other.go` | Darwin 尚無正式剪貼簿與安裝流程，不能只新增捷徑就宣稱 Mac 可當主機。 |

外部官方依據（2026-09-07 查閱）：

- [Tailscale Apple Shortcuts](https://tailscale.com/docs/features/mac-ios-shortcuts)：提供 Find Devices、Connect、Get Status；可取得候選裝置及連線／帳號資訊。裝置在線不代表 TailClip 存在或可存取。本文沒有證明端點探測失敗後可繼續迴圈，亦未完整列出裝置輸出欄位或跨平台匯出的 action identifier。
- [Tailscale Serve](https://tailscale.com/docs/features/tailscale-serve)：Serve 會注入使用者身分 header 並移除客戶端偽造值；tagged 來源不提供一般使用者 header，共享裝置的外部使用者則可能有 header。只綁 localhost 可限制直接偽造的範圍，但不能把 loopback 誤當成只有 Serve 能使用。
- [Tailscale macOS variants](https://tailscale.com/docs/concepts/macos-variants)：Mac 有不同安裝形式，驗收須記錄實際種類與版本，不能只寫「Mac 通過」。

尚待原生驗證：Find Devices 的完整 HTTPS DNS 名稱／穩定 ID、三平台共用匯出成品、HTTP 非 2xx／DNS／TLS 失敗行為、HTTP 自動重新導向、設定檔讀寫與首次權限畫面。P0 關卡負責取得這些證據；本次只完成文件與程式閱讀。

### 13.3 產品決策與避免過度工程化

| 編號 | 定案提案 | 原因 |
| --- | --- | --- |
| DEC-M2-01 | 先驗證平台能力，再做授權，最後接通用捷徑；不採 issue 原本先免 QR、再補授權的順序。 | 任一可執行階段都不能出現無保護的剪貼簿 API。 |
| DEC-M2-02 | 首版使用 Find Devices 列出候選，使用者首次選一次，僅驗證選定主機；不逐台 HTTP 掃描整個 tailnet。 | 原生網路錯誤可能中斷捷徑，逐台探測也可能反覆詢問不同網域權限。省去掃描器、背景目錄與額外服務。 |
| DEC-M2-03 | 保留兩支日常捷徑，另提供低頻使用的「TailClip：切換電腦」。 | 使用者不需編輯捷徑或輸入特殊文字來切換；日常不增加選單。 |
| DEC-M2-04 | 預設主機不可用時停止，不默默改送其他主機；切換必須由使用者選擇。 | 避免私人內容送到錯誤電腦，尤其是同名裝置與工作／個人環境。 |
| DEC-M2-05 | 首版採明確 Tailscale 使用者清單，所有獲准者具有文字讀寫權；不建立角色、群組解析或每裝置憑證系統。 | 滿足本人多裝置與少量指定使用者的需求；網路 ACL／grants 仍是額外限制。 |
| DEC-M2-06 | 新舊認證模式互斥，升級不自動放寬；Mac Agent 獨立 M3。 | 限制相容性分支，避免 Bearer fallback 繞過身分拒絕，也避免擴充平台拖住免 QR。 |

**相較 issue 的明確縮減：** M2 保證「自動取得候選、首次選一次、驗證後記住」，不承諾「自動排除所有未安裝 TailClip 的主機、首次完全零選擇」。候選列表須標示為 Tailscale 裝置，不能假稱都是可用 TailClip 主機。這是本次請使用者確認的產品取捨。

本版不新增資料庫、公開中繼、中央裝置 registry、Tailscale 管理 API key、OAuth／OIDC、tsnet、新原生 App、第三方捷徑外掛、全網 IP 掃描、定期探測、背景同步、歷史或離線佇列。也不為裝置發現替使用者電腦加 tag 或改名。

### 13.4 使用者流程

**桌面首次設定／升級：**

1. 沿用原本安裝、通知區與 Tailscale Serve；維持兩個 loopback port，變更前檢查占用與既有 Serve 路徑。
2. 本機「連線與存取」頁顯示目前電腦名稱、Tailscale 狀態及允許帳號。能可靠辨識本機節點擁有者時預填該 login，使用者確認一次「允許這個帳號的裝置讀寫這台電腦的文字剪貼簿」。不得取第一個登入請求當擁有者。
3. 無法確認擁有者或主機為 tagged 時，保留未啟用狀態，讓使用者在本機指定允許的完整 login。不得把空清單解讀成允許全部。
4. 新安裝不提供正常流程的 token／配對 QR；顯示通用捷徑安裝入口。舊版升級則保留原模式，使用者按「啟用免 QR」並確認允許帳號後才切換。
5. 捷徑從不含憑證的固定下載入口取得，亦隨版本包附帶；iPhone／iPad／Mac 各自加入同一套捷徑，iCloud 同步由 Apple 處理。不能要求掃某台電腦的限時 QR 才能下載。

**首次傳送／取回：**

1. 傳送先保存本次 Share Sheet 文字；沒有分享輸入才取本機剪貼簿文字，避免後續選單改變來源。保留拒送配對資料與空字串的規則。
2. 取得 Tailscale 狀態；已連線不固定再等兩秒。未連線時呼叫 Connect 並有限次重查（上限 3 次、每次間隔 1 秒）；這不是 HTTP timeout 的保證。不得自動切換 Tailscale 帳號、DNS 或 exit node。
3. 沒有有效預設目標時列出候選；顯示名稱與可辨識的完整 DNS，優先排序在線裝置，同名不得合併。只看到一台也須首次確認；取消不讀寫遠端剪貼簿。
4. 選擇後只對該主機檢查 `/health` 與已授權的 `/status`；服務版本、身分授權及 backend 可用均通過後才存成預設目標。
5. 同次執行接續完成原方向操作，第一次「傳送」不用犧牲內容來換配對資料。成功後顯示「已傳到〈電腦〉」或「已從〈電腦〉取回」。Apple 首次檔案／網路／剪貼簿授權畫面如實保留。

**日常與切換：**

- 有有效預設時直接操作該主機，不每次重掃所有 HTTP 服務或顯示選單。每次請求仍驗證身分，快取不能代表永久授權。
- 「切換電腦」顯示目前預設、候選與重新整理；驗證新目標成功才取代舊預設。此捷徑完全不讀寫剪貼簿，切換失敗保留舊設定。
- 離線、無權限、非 TailClip、版本不相容或原生網路錯誤均停止本次傳輸；引導重試或執行「切換電腦」。能取得結構化錯誤時顯示中文原因，原生動作直接中止時不假裝可攔截並自動跳至自訂選單。
- POST 已送出但沒有收到回應時，結果可能已生效；不自動重送、不切換、不排隊，下次由使用者重新執行。取回錯誤或無文字一律保留 Apple 裝置原剪貼簿。

### 13.5 目標設定與發現限制

- 新捷徑使用獨立 `iCloud Drive/Shortcuts/TailClip/client-v2.json`，不覆寫舊 `config.json`。只保存 schema version、目前帳號範圍、選定節點身分／DNS、顯示名稱及最近成功驗證時間；不存 token、文字或整個 tailnet 裝置清單。
- 為沿用現有無額外 App 的儲存方式，首版的預設目標由同 Apple ID 的捷徑共用；切換畫面明示「會同步變更其他 Apple 裝置的預設電腦」。不承諾每台 Apple 裝置各自保存偏好。跨裝置同時切換以最後同步成功設定為準，不新增同步協調服務。
- 每次執行開始讀取一次目標快照，執行中不接受另一裝置同步造成的目標切換；只有首次選定／明確切換才寫預設，不在每次成功傳輸時寫檔。
- 以當前 Tailscale 帳號及可取得的 tailnet 身分限制快取範圍；若沒有可靠 tailnet ID，至少重新確認選定 DNS／節點仍在當前候選列表。不同帳號、找不到目標、同名節點重建或無法核對時要求重新選定，不能只靠顯示名稱或登入 email 就沿用另一網路的目標。
- P0 先驗證節點穩定 ID 與完整 DNS 輸出；拿不到穩定 ID 時採完整 DNS 並在發現身分不一致時重選，不自行發明辨識欄位。
- URL 僅由當前候選產生，使用 HTTPS、完整 `.ts.net` hostname 與固定 `/tailclip/v1` 路徑；拒絕 userinfo、自訂 port、query、fragment、外部網域或直接 IP。必須驗證憑證，不信任 health 回應提供的替代網址。
- 不把 Ping 成功當服務存在，不把 health 成功當授權成功。已儲存檔仍視為不可信輸入，讀回並驗證後才使用；損壞／未知版本可由切換流程重建新設定，失敗不得先刪舊檔。
- 原生 HTTP 的重新導向行為需在 P0 查清。跨主機跳轉不得攜帶文字；如果無法約束，停止該傳送方案並回報影響，不以先 GET health 就宣稱已阻止 POST 重新導向。

### 13.6 授權、安全與相容性契約

**外部授權：**

- 桌面設定升為 version 2，包含 `auth_mode`（`legacy_token` 或 `tailscale_user`）、`allowed_users`、獨立 `control_token`，以及仍供舊模式使用的 `pairing_token`。欄位、版本與清單需完整驗證；新增可變清單後 Store Snapshot／Update 必須保有隔離與同步安全。
- `tailscale_user` 模式僅接受 Serve 注入的單一、有效 `Tailscale-User-Login`，以完整 login 精確核對允許清單；正確處理官方指定的 header 編碼，拒絕空白、重複、格式不合法或解碼失敗值。不用 display name、email 網域或 HTTP Host 當使用者授權。
- 新捷徑送出 `X-TailClip-Client: shortcuts-v2`，用於辨識預期請求及阻擋瀏覽器簡單跨站請求；這個公開字串不是密碼。身分模式的 status 與文字 API 都要求此 header，且拒絕不允許的 Origin／瀏覽器跨站來源，不開放跨來源 CORS。P0 必須驗證原生捷徑送出的實際 headers。
- 缺身分回 `401 identity_required`；有身分但不在清單回 `403 access_denied`；政策尚未完成設定回 `403 access_not_configured`；瀏覽器來源／客戶端標記不符回 `403 request_not_allowed`。一律在呼叫剪貼簿 backend 前拒絕。
- tagged 來源、未允許的 shared 使用者、缺少 header／Funnel 流量都不會因可連線或持有舊 pairing token 而放行。此版不實作 app capabilities、群組展開或 tagged 來源授權；tagged 主機仍可在本機明確允許一般使用者來源。
- 網路 ACL／grants 與 TailClip 清單同時限制存取。TailClip 不替使用者修改 tailnet policy，也不要求管理 API 金鑰。取消允許後下一次請求即拒絕，不要求捷徑先刪快取。
- 沿用 Serve → loopback 的信任邊界。能在該電腦執行程式的本機攻擊者可能直接偽造 header，仍屬第 9 節既定「本機 session 被攻陷」範圍；不得宣稱僅靠 RemoteAddr 為 loopback 即能證明經過 Serve。

**本機控制與安裝：**

- `/local/open-setup`、`/local/shutdown` 繼續要求 loopback 來源與合法本機 Host，加上只存在桌面端的 `control_token`；Tailscale 身分與 pairing token 都不能替代它。管理頁政策變更沿用限時本機 session，補 POST、來源檢查與 CSRF 防護。
- 控制憑證不出現在通用捷徑、health、status、下載頁、QR 或日誌；撤銷／切換外部模式不影響通知區的開啟與結束能力。
- 升級停止舊 Agent 時，安裝器只對既有固定 loopback 端點使用舊版控制格式完成交接，再建立新版控制憑證；新版 handler 不留下永久相容的 pairing-token 管理後門。保留既有 EXE 交接與 `.old` 清理回歸。

**API 與下載：**

- 保留 `/v1/clipboard/text` 的請求／成功回應、1 MiB、Unicode、多行、空取回、mutex、OS thread、timeout 與無內容日誌規則。認證模式由明確 capability 指示，不因路徑仍為 v1 而猜測。
- `/v1/health` 保留現有欄位並新增 `auth_mode` 與 `capabilities`，使用者身分模式宣告 `tailscale_user_v1`；不得含允許清單、憑證或剪貼簿。`/v1/status` 才回已授權裝置資訊與 backend 可用狀態。
- 舊 health 沒 capability、新捷徑遇 legacy 模式時，停止並提示更新／啟用免 QR；不把 Bearer 憑證發送給發現到的其他主機。
- 新增不含機密的固定 `/shortcuts/` 安裝頁與三支已簽署檔案下載（經 Serve 為 `/tailclip/shortcuts/`）；只回傳靜態說明及捷徑，不提供設定寫入或 clipboard 權限。原限時 `/setup/` 僅 legacy 模式可使用，身分模式關閉其 token 配對能力。

**遷移與回復：**

- 升級 version 1 時保持 legacy 外部行為與舊 token，安全保存版本化原設定備份；新安裝使用身分模式，但允許帳號確認前拒絕存取。
- 本機啟用免 QR 前說明「原 QR 捷徑將停止使用，請安裝通用版」；成功儲存政策後才切換，不自動刪除使用者既有捷徑或舊設定。
- 身分模式拒絕任何以 Bearer 取代身分的 fallback。若使用者在本機明確切回 legacy，輪替 pairing token 並重新配對，避免恢復早已撤銷的憑證。
- 舊 binary 無法讀 version 2，不能只換回 EXE 就宣稱可回退；文件需說明停止新版後還原對應版本設定及成品。備份不得記錄剪貼簿，不清理無關版本包／使用者檔案。

### 13.7 驗收條件

| 編號 | 情境 | 必須結果 |
| --- | --- | --- |
| AC-M2-01 | 已設定桌面、全新 Apple 客戶端 | 安裝通用捷徑、首次選電腦後同次完成傳送／取回；不掃 QR、不貼 token、不編輯 action。 |
| AC-M2-02 | 同一份正式簽署成品在 iPhone／iPad／Mac | 三平台的 Tailscale 動作、分享／剪貼簿來源與儲存皆可用；QA 略過動作不算此項通過。 |
| AC-M2-03 | 第二次操作／原本已連線 | 一次執行，無裝置選單、無全網 HTTP 掃描、無固定兩秒等待。 |
| AC-M2-04 | 切換、取消、同名主機、非 TailClip 候選 | 名稱與 DNS 可區分；取消／失敗不改預設、不碰剪貼簿，驗證成功才保存。 |
| AC-M2-05 | 預設離線、DNS／TLS 失敗、POST 回應遺失 | 不改送其他電腦、不自動重送、不排隊；空值／失敗取回不清空本機剪貼簿。 |
| AC-M2-06 | 帳號／tailnet 切換、節點重建、損壞設定、iCloud 同步 | 不跨範圍沿用目標；可重選修復；執行中不換目標；明示共用預設。 |
| AC-M2-07 | 同帳號獲准、多個指定使用者、移除權限 | 僅清單內使用者可讀寫，移除後下一請求拒絕，無需清快取。 |
| AC-M2-08 | 無身分、tagged 來源、未獲准 shared 使用者、Funnel／假 header | 拒絕且 backend 零讀寫；真 Serve 入口覆寫假 header 的行為有實測證據。 |
| AC-M2-09 | 惡意網頁、表單、跨站 fetch、GET 導覽與重新導向 | 無未授權讀寫或文字跨主機洩漏；不以「無 CORS 所以不能送 POST」作為證據。 |
| AC-M2-10 | 外部呼叫 local API、假 Host、有舊 pairing token | 無法開管理頁、改允許清單或停止 Agent；合法本機控制仍可用。 |
| AC-M2-11 | 升級／啟用新模式／回復 | 既有使用者升級不自動變模式；免 QR 後無 Bearer fallback；交接與舊 EXE 清理正常，回復含對應設定。 |
| AC-M2-12 | Unicode、Emoji、多行、1 MiB／超量、剪貼簿占用 | 沿用 M1 正確性與 Windows alpha.7 回歸；日誌不含文字或憑證。 |
| AC-M2-13 | 原版 GitHub／版本包捷徑下載 | 無限時配對依賴，三支檔案可原生匯入、重開、執行且 hash 對應來源；只在另獲授權後發布 GitHub。 |

效能驗收記錄成功與失敗各分支的實測耗時、候選數與 HTTP 次數；暖啟動只連目標主機，與 alpha.7 同網路基線比較。不得以猜測的原生 timeout 宣稱整體搜尋可在固定秒數完成。

## 14. M2 實作計畫

### 14.1 工作階段與完成閘門

以下皆為待執行，使用者確認計畫後才開始功能實作。P0 是階段名稱，不是缺陷嚴重度。

| 階段 | 工作與主要位置 | 交付／離開條件 |
| --- | --- | --- |
| P0：原生能力驗證 | 在實際 iPhone、iPad、Mac 製作最小 QA 捷徑，匯出 Find Devices／Get Status／Connect 與選單動作；核對 DNS、ID、帳號、HTTP headers、錯誤、redirect 與 iCloud 存取。 | 記錄 OS／Tailscale 版本及 Mac 變體、可重建 action 參數、跨平台匯入／執行證據；確定首版選定目標流程可行。 |
| P1：授權與遷移 | `internal/config`、`internal/tailscale`、`internal/api`、`internal/agent`、`cmd/tailclip`。讀取可靠擁有者資訊、version 2 遷移、模式互斥、獨立控制 token、身分／瀏覽器來源檢查、health capability。 | AC-M2-07–11 的 handler／遷移測試通過，至少一組真 Serve 身分測試通過；不以 fake header 單元測試代替 Serve 驗收。 |
| P2：桌面設定與下載 | `internal/webui`、通知區文字與安裝入口。允許帳號、啟用／回復模式、狀態與靜態捷徑入口。 | 本機可操作且有 CSRF／來源保護，外部不能更動；既有 Serve 重用、連接埠、安裝／升級行為無回歸。 |
| P3：通用捷徑 | `shortcuts/build.py`、send／pull spec、新增 switch spec、測試與 assets。共用 builder 產生三支成品，候選選定、預設儲存、同次首次操作、明確切換與錯誤訊息。 | AC-M2-01–06 通過；兩支日常捷徑不依賴第三支才能傳輸，切換捷徑不碰剪貼簿。 |
| P4：整合與交付 | `shortcuts/sign.py`、manifest、原生 QA、Windows／Linux 包裝與 CI 清單、README／shortcuts README／本 SPEC。 | 完成下列平台矩陣與安全回歸，核對三支 signed 成品及封裝；標明每平台已實測或 candidate；取得另行發布授權才 push／tag／Release。 |

P0 的最小 QA 製作仍屬功能實作，**本次文件工作沒有執行**。若同一份成品無法跨平台匯入、拿不到可信 DNS／目標範圍、POST 重新導向會洩漏內容等核心條件不成立，將證據與最小替代方案寫回 SPEC 並回報；不得私下擴張為原生 App、公開中繼或管理 API，也不得把不同平台專用捷徑當成通用成品交付。

### 14.2 測試與平台交付矩陣

- 自動檢查：沿用 `go test ./...`、`go vet ./...`、Linux `go test -race ./...`、`python -m unittest discover -s shortcuts/tests -v`，依修改補授權拒絕／模式切換／儲存失敗／本機控制測試；涵蓋 backend 零呼叫與敏感資料不進日誌。測試有意義的行為，不用測試數量代替驗收。
- Windows 剪貼簿原生回歸於可丟棄 Windows runner 或隔離測試環境執行，避免改寫開發者日常內容；保留 alpha.7 的 thread、並行、占用與取消案例。
- 原生捷徑 QA 必須從同一 builder 產生，測試儲存路徑隔離並保留／還原測試裝置剪貼簿。signed 正式成品另行驗證重開與跨平台執行，不能只驗 plist 結構。
- E2E 必測 iPhone ↔ Windows、iPad ↔ Windows、Mac 客戶端 ↔ Windows、iPhone ↔ Ubuntu Wayland；其餘 Apple ↔ Linux 配對逐項記錄，不自動外推支援。
- 至少兩台主機測同名／切換／其中一台離線；使用允許使用者、拒絕使用者、shared／tagged 來源及受限 ACL 的隔離案例驗授權。涉及 Tailscale policy 的 fixture 只用獨立測試環境，不修改現用 tailnet policy。
- Windows／Linux 封裝清單由兩支捷徑改為三支，內嵌 assets、manifest、hash、安裝頁與 ZIP／tarball 要一致。正式下載方式由原生裝置驗證，沒有 Apple 裝置或簽署能力時明確記錄該閘門尚未完成。

### 14.3 M3：Mac 作為 TailClip 主機（獨立後續工作）

保留 issue Phase 3 的方向，M2 驗收後再細化並確認需求，不預先新增空 backend 或套件骨架：

1. 實測純文字 clipboard backend 的最小可行方式與 Unicode／空值／1 MiB 行為，再選定 macOS 實作；不能把已有測試用 Swift 程式直接視為可交付 Agent。
2. 使用登入使用者 session 的常駐方式，規劃啟動、退出、設定與移除；不以 root daemon 讀寫使用者剪貼簿。
3. 核對 macOS Tailscale CLI 路徑、Serve、權限提示、睡眠恢復及重開機。安裝形式、最低 macOS 版本、處理器與簽署方式需另行記錄依據。
4. 驗收 iPhone ↔ Mac 主機、Mac ↔ Mac、Mac ↔ Windows／Linux；未驗收的組合維持 candidate。

### 14.4 Git、文件與本次交付紀錄

- 本次僅修改本 SPEC，留在既有 `codex/new_fork` 做一個本機文件 commit；README 尚在說明已交付功能，因此本次不把待實作行為寫成使用說明。
- 使用者確認後，正式 M2 功能建議自核對過的基準建立 `codex/tailnet-shortcuts` 分支，按 P0–P4 產生可檢查的 commits。既有不相關修改與版本包保留，合併前查衝突；功能性矛盾停止回報。
- 各階段完成即更新本節狀態及第 13 節決策；功能可用時同步更新 README 與對應舊章節，不留下互相矛盾的 QR／token 說明。
- 本次自檢範圍為 Markdown 結構、內部路徑、來源與計畫一致性、`git diff --check` 及文件差異；功能測試與 Apple／Serve 實機關卡均未執行，不宣稱 M2 已實作或驗收。
