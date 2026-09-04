# TailClip 技術與產品規格

- **文件版本：** 0.2
- **日期：** 2026-09-04
- **專案狀態：** v0.1-alpha 開發中
- **首版平台：** iOS 26、Windows 11 x64、Ubuntu 26.04 x86_64 GNOME Wayland
- **傳輸：** HTTPS over Tailscale Serve

---

## 0. 實作進度

最後更新：2026-09-04。

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
- [ ] 在 Windows 11 x64 與實際 iPhone 完成首次安裝、QR 配對、雙向文字、Share Sheet、自啟、重開機與解除安裝驗收。
- [ ] 在 Ubuntu 26.04 GNOME Wayland 與實際 iPhone 完成相同 E2E。
- [ ] 實機驗收通過後，才把對應平台從 build candidate 改標為已支援。

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

這些項目只有在目前版本通過實機驗收後才進入後續規格，不先提交尚未使用的抽象層。

## 3. 核心使用流程

### 3.1 Windows 首次設定

1. 使用者下載 Windows x64 ZIP、解壓後雙擊其中唯一需要執行的 `TailClip.exe`；不需安裝 Go 或其他 runtime。
2. 程式將自身安裝到 `%LOCALAPPDATA%\TailClip`，建立使用者層自啟並啟動 Agent；原始前景程序同步完成後續流程，不交給無畫面的安裝子程序。
3. TailClip 讀取本機已登入的 Tailscale tailnet、連線狀態與 `*.ts.net` 名稱，並檢查 `127.0.0.1:17733` 與 `127.0.0.1:17734`。
4. 若既有 `/tailclip` Serve path 已指向 `127.0.0.1:17733`，直接重用，不重新設定或要求 UAC。
5. 只有需要新增 Serve path 時才顯示 Certificate Transparency 說明；先以目前使用者權限設定，權限不足才要求一次 UAC，且不得重設其他 Serve 設定。
6. TailClip 驗證公開 HTTPS health endpoint。
7. 本機設定頁顯示可實際載入與掃描的限時 QR。
8. iPhone 掃 QR，安裝兩支捷徑並點一下複製配對資料，再執行「取回」或從 Share Sheet 執行「傳送」完成配對。

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

1. 「TailClip：傳送」先讀 Share Sheet input；沒有 input 才讀 iOS 剪貼簿。
2. 若 Tailscale 未連線，捷徑自動呼叫 Connect 並等待連線。
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
- clipboard lock 採短暫 exponential backoff，總等待不超過 1 秒。
- EXE 第一次執行自我複製、註冊 `HKCU` 自啟，並由原始前景程序同步完成安裝；Agent 本身才以背景模式啟動。
- Agent 使用 Win32 通知區圖示提供「開啟連線與配對頁面」、「登入 Windows 後自動啟動」核取項目與「結束 TailClip」；不為此導入 GUI framework 或額外背景程序。
- 已存在且目標正確的 `/tailclip` Serve path 直接重用；缺少時先由目前使用者設定，只有失敗且提升權限可能有幫助時才使用同一 EXE 的 elevated helper。
- Serve 完成後必須直接驗證公開 HTTPS health，再開啟設定頁；本機設定頁的 QR 必須在實際瀏覽器中載入，不得出現模板安全替代值。
- 本版不宣稱 binary 已簽章；下載與 SmartScreen 提示在 README 說明。

### 7.3 Ubuntu 26.04 GNOME Wayland

- 只支援 `WAYLAND_DISPLAY` 存在的圖形使用者 session。
- backend 使用 `wl-copy` 與 `wl-paste`，不使用 XWayland clipboard 作為 fallback。
- `systemd --user` service 在 `graphical-session.target` 後啟動。
- 缺少 session 或工具時 `/status` 回 `clipboard_available:false`，讀寫回 `503`。
- v0.1-alpha 只發布 x86_64 tarball。

## 8. iOS Shortcuts

### 8.1 `TailClip：傳送`

- 接受 Share Sheet 的 Text 與 URL。
- 有 Shortcut Input 時優先使用；沒有才 `Get Clipboard`。
- config 不存在時進入一次性配對流程，不把配對 JSON 當作剪貼簿內容傳送。
- `/status` 與 `/clipboard/text` 的文字位址必須先經過原生 `URL` action，再交給 `Get Contents of URL`，不得依賴 iOS 將 RTF 隱式轉為 URL。
- 每次 `Get Contents of URL` 後必須明確使用 `Get Dictionary from Input` 解析 JSON；後續 `Get Dictionary Value` 只可讀取該 Dictionary 輸出，不得依賴 iOS 將文字隱式轉為辭典。
- Tailscale 未連線時呼叫 Connect；連線後只送一次 API request。
- POST JSON 至 `/clipboard/text`，解析回應後顯示短通知。

### 8.2 `TailClip：取回`

- 與傳送捷徑使用相同 config 與連線流程。
- 所有 `If` action 只保留必要且已填值的條件列；不得存在會觸發「請選擇此動作中每個參數的值」的空白條件。
- `/status` 與 `/clipboard/text` 的位址同樣先經過原生 `URL` action。
- `/status` 與 `/clipboard/text` 的回應同樣先經過 `Get Dictionary from Input`，再讀取 `ok`、`empty`、`text` 或 `error`。
- GET `/clipboard/text`。
- `empty:true` 時顯示「電腦剪貼簿沒有文字」。
- 有文字時使用 Copy to Clipboard 並開啟 Local Only，再顯示短通知。

### 8.3 建立與交付

- 每支捷徑先有 JSON build spec，並通過規格 validator。
- 只使用 iOS／Tailscale 原生 actions，不使用 Run Shell Script 或 Run AppleScript。
- 透過 macOS Shortcuts 編輯器建立，完成重開、CLI 與 iPhone 實測。
- export 後以 `shortcuts sign --mode anyone` 簽署。
- 簽署前 artifact 不得含真實 endpoint、token 或個人資料。

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
- CI：format、vet、race tests、Windows x64 build、Linux x86_64 build。

### 10.2 Windows 實機

- 中文、英文、Emoji、多行、LF／CRLF 與 1 MiB 文字雙向傳送。
- clipboard 被暫時鎖定時可重試並於 1 秒內結束。
- 第一次安裝、再次雙擊、使用者登入自啟、重開機、token 輪替與解除安裝。
- 通知區圖示持續存在；可由圖示開啟配對頁、切換登入後自啟及結束目前程序。
- Tailscale 未安裝、未連線、HTTPS 未啟用、Serve path 衝突與 port 占用均有可理解指引。

### 10.3 iOS 26 實機

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
- 第一個可下載測試包為 `v0.1.0-alpha.1`；目前 Windows build candidate 為 `v0.1.0-alpha.5`。
- Git 追蹤的 Windows 測試產物：`dist/TailClip-v0.1.0-alpha.5-windows-x64.zip` 及其 `.sha256`；舊版測試包保留供回歸比對。
- GitHub Release artifacts：版本化 Windows x64 ZIP、Linux x86_64 tarball、兩支 signed Shortcuts 與 `SHA256SUMS`。
- Windows ZIP 必須包含 `TailClip.exe`、`README-Windows.txt`、`Uninstall-TailClip.cmd`、`VERSION.txt`、兩支 signed Shortcuts 與包內 `SHA256SUMS.txt`。
- release ZIP 解壓後只需雙擊 `TailClip.exe`；不得要求終端機、Go toolchain 或手動複製檔案。
- CI 不保存或產生真實配對 token。
- 本版沒有自動更新；升級前保留相容的 `config.json`，未知 config version 安全停止並提示重新設定。

## 12. 後續 Roadmap

只有 M1 實機完成後再依序評估：

1. 多裝置與每裝置 token。
2. `text/uri-list`、HTML + plain fallback、PNG。
3. Taildrop 檔案傳送與受控暫存。
4. X11／其他 Linux 桌面與正式套件。
5. 原生 iOS App、Keychain 與 App Intents。
6. 明確 opt-in 的半自動同步。

上述項目目前不是相容性承諾，也不應提前建立程式碼骨架。
