# TailClip 技術與產品規格

- **文件版本：** 0.4
- **日期：** 2026-09-07
- **專案狀態：** v0.1-alpha 開發中；M2 已收斂為單主機雙入口，待實作
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
- [x] 2026-09-07 依使用者指示整合 issue #1／#4，將 M2 收斂為單台 Windows 與 iPhone 的雙入口；第 13、14 節取代先前較廣的規劃。此勾選僅代表文件完成。
- [ ] 依第 14 節實作與驗收 M2；目前未修改功能程式碼或產生新版本。

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

M2 僅執行第 13、14 節的單主機雙入口；未採用的 issue #1／#4 擴充範圍移至 [#5 待做功能](https://github.com/kuanfu0430/tailclip/issues/5)。其餘項目仍另行評估：

1. 多主機管理、多目標切換與進階授權（待做，不屬 M2）。
2. `text/uri-list`、HTML + plain fallback、PNG。
3. Taildrop 檔案傳送與受控暫存。
4. X11／其他 Linux 桌面與正式套件。
5. A 的原生 iOS App 與擴充 App Intents（B 必要的 iPhone App／Keychain 已納入 M2）。
6. 明確 opt-in 的半自動同步。

上述項目目前不是相容性承諾，也不應提前建立程式碼骨架。

## 13. M2 需求 SPEC：單主機雙入口

### 13.1 本次範圍

2026-09-07 依使用者最新指示整合 [issue #1](https://github.com/kuanfu0430/tailclip/issues/1) 與 [issue #4](https://github.com/kuanfu0430/tailclip/issues/4)：只改善接入方式，維持單台 Windows 11 x64 與 iPhone 的手動純文字傳送／取回。既有 Ubuntu 功能保留並做必要回歸，不新增平台支援。第 1–11 節仍描述已存在的 M1；本節是待實作規格。

| 入口 | 使用者 | 本次要做到的操作 |
| --- | --- | --- |
| A：使用現有 Tailscale | 已安裝並使用自己 tailnet 的人 | 原連線與舊捷徑繼續使用；另可安裝新版「傳送／取回」，首次選一次電腦，之後一步操作，不掃 QR、不貼 token。 |
| B：簡易連線 | 沒有 Tailscale 基礎的新手 | 開啟桌面 TailClip、選簡易連線，安裝 iPhone TailClip App 並掃碼一次；之後開 App 即可重連，按傳送／取回。 |

**免 QR 是 A 新捷徑的需求；B 沿用 issue #4 的一次掃碼。** B 不要求 Tailscale App／帳號／tailnet、Tailcat CLI、捷徑或手動網路設定。原生 iPhone App 是 B 的必要客戶端，不是另做一套多平台產品。

本次不做多主機清單與切換捷徑、Mac／iPad 支援、群組或多人權限管理、共用剪貼簿暫存、自動同步、歷史、離線佇列或兩入口同時對外服務。未採用的功能統一移至 [#5 待做功能](https://github.com/kuanfu0430/tailclip/issues/5)；不沿用前版 SPEC 的三支通用捷徑、跨 Apple 平台與強制切換認證模式方案。

### 13.2 入口 A：沿用連線，新增免 QR 捷徑

1. 已有設定的使用者升級後直接維持 A，不重跑連線精靈，不重新登入，不改現有 Serve、網路設定或配對 token。原 QR／舊捷徑保持有效；要使用免 QR 時只需加入新版兩支捷徑。
2. 新捷徑沿用 Tailscale 的裝置查找，首次讓使用者選一台電腦；只驗證選定電腦的 TailClip 與授權，成功後保存單一目標。列的是候選裝置，不能假稱全部已安裝 TailClip；不逐台 HTTP 掃描。
3. 首次選定後同次完成傳送或取回；傳送先保留 Share Sheet 輸入，沒有輸入才用目前剪貼簿，不為配對而清除內容。
4. 日常直接使用該目標，已連線不額外固定等待；目標離線或授權失敗便停止，不自動改送其他電腦、不重送結果不明的 POST。
5. 只保存一個目標與必要的帳號／節點辨識，與舊設定檔分開。帳號或目標身分不符時停止並提供「重設連接電腦」；這是重做首次設定，不新增日常多主機切換功能。同一份 iCloud 設定仍只代表一台目標，不引入跨裝置偏好管理。
6. 新捷徑有不依賴限時 QR 的固定下載入口；仍只有「TailClip：傳送」「TailClip：取回」兩支。舊使用者不必安裝 B 的 App。

**最小授權方案：** 新捷徑以 Serve 提供的身分核對桌面節點擁有者，只開放同一 Tailscale 使用者；不做可編輯允許清單或群組。身分無法可靠辨識、tagged 或不同使用者來源，免 QR 請求拒絕並提示，既有合法 token 流程仍可用。不得以第一個遠端請求決定擁有者。

新舊認證按明確請求類型分流：舊捷徑維持原 Bearer 驗證；新版標記請求只走身分驗證，失敗不得 fallback 至 token。加入必要的客戶端標記與跨站請求防護，不開放 CORS；「同 tailnet 可連線」本身不構成讀寫權限。本機管理仍走原有獨立檢查，不能接受新版外部身分作為管理授權。

### 13.3 入口 B：新手掃碼連接

1. 全新啟動可選「使用現有 Tailscale」或「簡易連線」；選 B 後直接進入 B 的啟動與配對，不能先卡在 Tailscale CLI、登入或 Serve 檢查。
2. 桌面與 iPhone App 內嵌 Tailcat library，建立應用程式加密通道，使用者不輸入地址或中繼設定。沿用上游直連／DERP 能力，不自建網路控制平面，不另做 VPN 或公開網址入口。
3. 桌面按「連接手機」產生短效一次性 QR；iPhone App 掃碼、驗證預期桌面身分及票券，保存本次配對。首版只維護一個手機配對；本機可解除或確認取代，取代即撤銷舊授權，不做逐裝置管理清單。
4. 配對完成後，App 只提供連線狀態、傳送、取回與重新連接。讀取手機剪貼簿由使用者操作觸發，依 iOS 正常權限處理；不承諾背景監聽或任意 App 複製後自動同步。
5. 保存桌面／手機身分、必要會合資訊及配對憑證。正常關閉重開、Windows 重啟、App 回到前景與 Wi-Fi／行動網路切換後可重連，不再掃碼；桌面睡眠或離線時顯示未連線，不假報完成。
6. 首次測試包需附可實際安裝的 iPhone App 與安裝方式（例如 TestFlight），不能交付只含原始碼的「新手版」。Apple 簽署／分發是交付條件，正式上架另行處理。

配對票券須限時、一次使用且防重放；配對資料包含格式／API 版本，不相容時停止並提示更新。通道地址外洩不等於取得剪貼簿權限，每次操作另驗配對憑證。手機以 Keychain 保存秘密，桌面採適當憑證保護；QR、地址秘密、token 與文字不進日誌。解除／取代配對後，舊連線的後續請求也須拒絕。

### 13.4 共用邏輯與切換邊界

- A、B 共用目前文字驗證、1 MiB 限制、空值／錯誤模型及 Windows clipboard backend，不做 Shared Store 或第二套同步邏輯。
- B 僅開放配對及必要的文字端點，不能把含 `/local/shutdown`、設定頁或 token 管理的整個 Agent handler 轉發出去。B 憑證與 A 憑證分開，B 不偽裝成 Serve 身分。
- 首版一次只啟用選定入口；保存選擇，下次直接恢復。使用者主動切換時才停止原入口並啟動另一入口，不暗中 fallback。
- 停用 B 停止 listener／重連工作，保留配對供下次恢復；只有明確解除／取代才清除 B 授權。A 的 Serve、token、舊捷徑與設定不因切換 B 被移除或重建。
- 選 B 時既有 Serve 設定可留存，但 A 的遠端文字端點不應繼續提供服務；切回 A 重用原設定。本機通知區與管理能力在兩模式均可用，無 Tailscale 時也能管理 B。
- 有需要的設定變更採最小相容遷移，缺少入口欄位的舊設定視為 A；不因新增 B 重寫或輪替 A 憑證。兩種入口的設定格式需能分辨，失敗不得覆蓋已有效的設定。
- 本機 HTTP listener 維持 `127.0.0.1`，綁定前檢查 port；B 使用程序內通道，不增加 LAN／公網 HTTP listener。保留既有安裝、自啟與升級交接行為。

### 13.5 實作前需驗證的兩件事

**A：iPhone 原生捷徑。** 驗證 Tailscale Find Devices 能取得可信的目標 DNS／辨識資料，兩支正式成品能完成首次選擇、授權及再次執行。服務不符或原生網路錯誤可停止；不新增全網探測／錯誤恢復框架。文字只能送往選定 HTTPS 主機，測試重新導向不洩漏內容。

**B：Tailcat iPhone 真機。** 先證明 library 可嵌入 App，在未安裝 Tailscale 的兩端完成加密測試請求、持久身分與重連，再接真實剪貼簿。做不到時回報具體限制，不默默改成要求裝 Tailscale 或新增其他隧道產品。

官方參考：[Tailscale Shortcuts](https://tailscale.com/docs/features/mac-ios-shortcuts)、[Serve 身分](https://tailscale.com/docs/features/tailscale-serve)、[Tailcat README](https://github.com/tailscale/tailcat/blob/main/README.md)、[Tailcat SECURITY](https://github.com/tailscale/tailcat/blob/main/SECURITY.md)。2026-09-07 已閱讀；它們不構成 TailClip iPhone 整合已驗收的證據。Tailcat 提供 library 與無帳號通道，公共中繼仍是限速、best-effort；先以有限測試版驗證，標明中繼來源與故障提示。自行營運 DERP、SLA 與自動中繼遷移移至待做，不把有限測試結果寫成大眾穩定服務承諾。

### 13.6 完成條件

| 編號 | 驗收 |
| --- | --- |
| AC-M2-01 | 舊版升級後，A 的原 Serve、QR、token 與捷徑照常使用，不要求重新配置；新入口不影響此流程。 |
| AC-M2-02 | A 新捷徑首次選一次 Windows，免 QR／貼 token，同次完成傳送或取回；下次無裝置選單，僅連所選主機。 |
| AC-M2-03 | A 身分不符、目標離線或非 TailClip 時停止，不 fallback、不誤送；新版身分請求不能取得本機管理權。 |
| AC-M2-04 | B 在兩端皆未安裝 Tailscale、沒有帳號／CLI／網路手動設定時，以可安裝的 App 掃碼完成雙向文字。 |
| AC-M2-05 | B 在不同網路及直連受阻的中繼情境可用；正常重啟、前景恢復與網路切換可重連；離線正確提示，不要求每次掃碼。 |
| AC-M2-06 | 過期／重放票券、未配對、被解除／取代的手機均不能讀寫；B 無法存取本機管理，日誌不含文字或秘密。 |
| AC-M2-07 | A/B 明確切換、保存選擇及停止原入口有效；B 可在沒有 Tailscale 時啟動與管理，切回 A 不需重配。 |
| AC-M2-08 | 兩入口都通過 Unicode、多行、Emoji、1 MiB／超量、空取回及 Windows 占用回歸；失敗不清空手機剪貼簿、不背景上傳。 |

## 14. M2 簡化實作計畫

| 階段 | 工作 | 交付 |
| --- | --- | --- |
| 1：先完成 A | iPhone 最小捷徑驗證、首次單目標保存、同帳號身分授權及舊捷徑相容；調整 `shortcuts`、`internal/tailscale`、`internal/api` 與下載入口。 | 兩支免 QR 捷徑、AC-M2-01–03；此階段可先驗收，不等待 B 完成。 |
| 2：打通 B | 在獨立 App 資料夾做 iPhone Tailcat 原型；桌面內嵌通道、最小配對與持久重連。先用隔離測試文字驗證，再接共用 backend。 | 實機可用的加密連線與配對證據；不能只用桌面測試代替 iPhone。 |
| 3：整合雙入口 | 啟動前分流、設定保存、切換與端點隔離，完成 B App 的收發畫面、包裝及兩條使用說明。 | AC-M2-04–08、完整回歸與可安裝測試包，清楚標示 B 的測試版性質。 |

沿用 Go／捷徑既有測試，新增必要的分流、拒絕、配對失敗與重啟案例。核心實機範圍只有 Windows 11 與 iPhone；Mac 可作 iOS 建置工具，但不因此增加 Mac 客戶端／Agent 支援工作。實作時同步 README；本次僅更改規劃文件，README 繼續描述目前可用功能。

### 14.1 Issue 整理與交付狀態

- 已關閉 [issue #1](https://github.com/kuanfu0430/tailclip/issues/1)／[issue #4](https://github.com/kuanfu0430/tailclip/issues/4)，原文保留並附整合說明；本次採用內容以本 SPEC 第 13、14 節為準，未採用內容記於 [#5 待做功能](https://github.com/kuanfu0430/tailclip/issues/5)。
- 關閉原 issue 的理由是「規劃已整合與收斂」，不是宣稱功能完成；不更動 #2／#3 的狀態，也不自動把其工作併入本次。
- 本次只修改文件及依使用者指示整理 GitHub issues，尚未編程、部署或發布套件。文件修改在既有 `codex/new_fork` 做本機 commit，不 push 程式碼。
- 文件自檢包含章節／連結／需求範圍一致性與 `git diff --check`；功能測試與 iPhone 實機驗收待開發時執行。
