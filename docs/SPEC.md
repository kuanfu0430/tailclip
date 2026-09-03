# TailClip 技術與產品規格

- **文件版本：** 0.1-draft
- **日期：** 2026-09-03
- **專案狀態：** Specification-first / Pre-alpha
- **目標平台：** iOS、Windows、Linux
- **主要網路基礎：** Tailscale Serve
- **輔助檔案通道：** Tailscale Taildrop

---

## 1. 文件目的

本文件定義 TailClip 的產品範圍、使用者體驗、系統架構、傳輸協定、平台整合、安全模型、里程碑與驗收條件。

README 僅說明專案用途與預計使用方式；涉及工程決策的內容以本文件為準。

本文中的規範詞使用以下含義：

- **MUST／必須：** 不符合即視為未達規格。
- **MUST NOT／不得：** 明確禁止。
- **SHOULD／應：** 原則上應遵循，偏離時必須有具體理由。
- **MAY／可以：** 可選能力，不影響最低相容性。

---

## 2. 摘要

TailClip 的目標是在 iOS 與 Windows／Linux 之間建立近似 Apple Universal Clipboard 的跨平台剪貼簿傳送體驗，但不依賴公開雲端中繼服務。

核心架構為：

```text
┌──────────────────────────── iOS ────────────────────────────┐
│                                                             │
│  Shortcuts / Share Sheet / Future App Intent                │
│      ├─ Send: 讀取或接收內容，POST 到桌面                   │
│      └─ Pull: GET 桌面當前剪貼簿，寫入 iOS 剪貼簿          │
│                                                             │
└──────────────────────────────┬──────────────────────────────┘
                               │ HTTPS over tailnet
                               │ Tailscale Serve
                               ▼
┌──────────────────── Windows / Linux Desktop ────────────────┐
│                                                             │
│  Tailscale Serve                                             │
│      └─ proxy → 127.0.0.1:17733                              │
│                       │                                     │
│                       ▼                                     │
│                TailClip Agent                               │
│      ├─ HTTP API                                            │
│      ├─ 配對與授權                                          │
│      ├─ 格式驗證、TTL、去重與限流                           │
│      ├─ Windows / Wayland / X11 Clipboard Adapter           │
│      └─ Optional Taildrop Worker                            │
│                                                             │
└─────────────────────────────────────────────────────────────┘

大型內容或實體檔案：iOS／Desktop ── Taildrop ── Desktop／iOS
```

最重要的架構原則如下：

1. **Tailscale Serve 是主通道。** 文字、網址與可控大小的圖片透過版本化 HTTPS API 傳輸。
2. **Taildrop 是輔助通道。** 只用於大型二進位內容與實體檔案，不作為剪貼簿事件佇列。
3. **iOS 主動發起兩個方向的要求。** iPhone 不需要常駐 HTTP Server，也不依賴背景輪詢。
4. **手動觸發優先。** 預設不監聽並同步每一次剪貼簿變更。
5. **不架設公開中繼站。** 桌面 Agent 只監聽 loopback，由 Tailscale Serve 提供 tailnet 內入口。
6. **內容最小留存。** MVP 不保存剪貼簿歷史，日誌不得記錄內容本身。

---

## 3. 問題定義

Apple 裝置之間可透過 Universal Clipboard 在裝置間接續複製與貼上，但 iOS 與 Windows／Linux 之間缺乏同等、私有且容易部署的能力。

使用者已經擁有 Tailscale，因此 TailClip 不應再建立另一套帳號、公開伺服器、NAT 穿透或 VPN。專案需要利用現有 tailnet 完成：

- iPhone 將剪貼簿內容送至指定 Windows／Linux 裝置。
- iPhone 取回指定 Windows／Linux 裝置目前的剪貼簿。
- 在不犧牲安全性的前提下，將操作壓縮為一次捷徑、Share Sheet、控制中心或動作按鈕操作。
- 在 Windows 與常見 Linux 圖形工作階段中可靠讀寫剪貼簿。

---

## 4. 產品目標

### 4.1 MVP 目標

MVP 必須做到：

1. iOS → Windows 的 UTF-8 文字傳送。
2. iOS → Linux 的 UTF-8 文字傳送。
3. Windows → iOS 的 UTF-8 文字取回。
4. Linux → iOS 的 UTF-8 文字取回。
5. 每次操作只需要一次明確的 iOS 使用者動作；首次設定與裝置選擇除外。
6. 桌面 API 不公開到網際網路。
7. 未配對的 tailnet 成員無法讀取或寫入剪貼簿。
8. 不建立剪貼簿歷史資料庫。
9. 支援 Unicode、Emoji、換行與至少 1 MiB 的純文字內容。
10. 裝置離線、認證失敗、格式不支援與內容過大時，向使用者回報可理解的錯誤。

### 4.2 後續目標

- 網址與 `text/uri-list`。
- HTML／Rich Text，且必須附純文字 fallback。
- PNG 圖片。
- iOS Share Sheet。
- 多台目標電腦與預設裝置。
- Taildrop 大型內容／檔案輔助傳輸。
- Windows Tray 與 Linux 桌面選單。
- 原生 iOS App、App Intents、控制中心與動作按鈕。
- 可選的桌面主動推送或半自動同步模式。

---

## 5. 非目標

第一階段明確不處理：

- 完全複製 Apple Universal Clipboard 的零操作、被動背景體驗。
- 在 iOS 背景永久監聽系統剪貼簿。
- 未經使用者操作，從桌面任意喚醒 iOS 並覆蓋 iPhone 剪貼簿。
- 跨不同 tailnet 的內容交換。
- 建立公開 SaaS、中繼站或使用者帳號系統。
- 多使用者共享剪貼簿、團隊聊天室或檔案雲端硬碟。
- 伺服器端保存完整剪貼簿歷史。
- 在無圖形登入工作階段的 headless Linux 上提供「桌面剪貼簿」。
- 預設同步密碼、OTP、API Key 等所有敏感內容。
- 第一版完整保留所有平台專有剪貼簿格式。

---

## 6. 平台限制與設計前提

### 6.1 iOS 限制

第三方 App 讀取其他 App 寫入的 pasteboard 會觸及 iOS 的使用者隱私與意圖判定；一般 App 也不能把自己當成無限期運作的背景 daemon。背景工作由系統排程，App 可能很快被暫停。

因此 TailClip MUST 採以下前提：

- iOS 剪貼簿的讀取與寫入由可見的使用者操作觸發。
- MVP 不依賴 iOS 常駐背景監聽。
- Windows／Linux → iOS 不由桌面直接推入 iPhone，而是由 iPhone 主動 GET。
- 原生 App 階段仍以 App Intent、Control、Widget、Action Button 或 Share Extension 表達明確意圖。

### 6.2 Tailscale 前提

- iPhone 與桌面裝置 MUST 已加入可互通的 tailnet。
- 桌面端 MUST 能啟用 Tailscale Serve。
- TailClip 不使用 Funnel；Funnel 會把服務公開到網際網路，與本專案邊界不符。
- Tailscale 網路身分不是 TailClip 唯一認證層，應再使用應用程式配對憑證。

### 6.3 桌面剪貼簿前提

- Windows Agent MUST 在互動式登入使用者的 session 中運作。
- Linux Agent MUST 在擁有 Wayland／X11 session 的登入使用者環境中運作。
- 系統級 root daemon 不應直接承擔使用者剪貼簿讀寫。

---

## 7. 使用者故事

### US-01：iPhone 傳文字到工作電腦

使用者在 iPhone 複製一段文字，執行「TailClip：傳送到工作電腦」，之後可在 Windows／Linux 直接貼上同一段內容。

### US-02：iPhone 取回工作電腦文字

使用者先在 Windows／Linux 複製內容，再於 iPhone 執行「TailClip：從工作電腦取回」，之後可在 iOS 其他 App 貼上。

### US-03：從 Share Sheet 傳送

使用者在 Safari、照片或其他 App 選取文字、網址、圖片或檔案，透過 Share Sheet 執行 TailClip，而不必先改寫全域剪貼簿。

### US-04：指定不同桌面

使用者可選擇家用 Windows、工作 Windows 或 Linux 桌面，並設定一台預設裝置。

### US-05：拒絕不可信請求

即使某個其他裝置已在同一 tailnet，若沒有有效配對憑證，也不能讀取或覆蓋桌面剪貼簿。

### US-06：安全地處理離線與過期內容

目標裝置離線或內容已超過 TTL 時，TailClip 應回報失敗，不應稍後靜默覆蓋使用者的新剪貼簿。

---

## 8. 核心架構決策

### ADR-001：Serve API 作為主通道

**決策：** 使用 Tailscale Serve 將桌面 Agent 的 loopback HTTP API 以 tailnet 內 HTTPS 暴露。

**理由：**

- 雙向文字操作都能由 iPhone 主動發起。
- API 可明確定義版本、MIME、大小、錯誤與認證。
- 可套用 Tailscale Grants／ACL。
- 不依賴 Taildrop inbox 的檔案輪詢語義。
- 不需要 iOS 端常駐服務。

### ADR-002：Taildrop 只作輔助通道

**決策：** Taildrop 只處理大型二進位內容與實體檔案。

**理由：**

- Taildrop 目前仍標示為 alpha。
- 它主要是檔案傳送功能，不是 request/response API。
- 官方限制為個人擁有裝置間傳送。
- Taildrop 傳輸可越過一般裝置間 access-control 限制，安全模型與 Serve 不同。
- tagged node 不能使用 Taildrop。
- iOS Shortcuts 官方整合提供 Send File，但沒有「收到 Taildrop 後自動執行寫入剪貼簿」的對稱觸發流程。

### ADR-003：兩個方向都由 iOS 發起

**決策：**

- iOS → Desktop：iOS `POST /v1/clipboard`。
- Desktop → iOS：iOS `GET /v1/clipboard/current`。

**理由：** 避免在 iOS 架設服務、長輪詢或依賴不可靠的背景喚醒。

### ADR-004：手動優先

**決策：** MVP 預設只在使用者明確操作時傳送或取回。

**理由：** 剪貼簿常含密碼、OTP、token、私人訊息與公司資料。全自動同步會提高誤傳、循環與資料外洩風險。

### ADR-005：MVP 不保存歷史

**決策：** `GET /v1/clipboard/current` 在請求發生時讀取桌面當前剪貼簿；Agent 不建立內容資料庫。

**理由：** 最小化敏感資料留存，並確保取回內容與桌面當前狀態一致。

### ADR-006：桌面 Agent 優先使用 Go

**決策：** 第一個桌面 Agent 建議以 Go 實作；Windows 與 Linux 使用平台 adapter。

**理由：**

- 可產生單一執行檔。
- 標準函式庫足以處理 HTTP、JSON、雜湊與並行。
- 跨 Windows／Linux 發布與交叉編譯成本低。
- 後續可視需求直接整合 Tailscale 的 Go 生態，但 MVP 不要求嵌入 tsnet。

原生 iOS App 若進入實作，使用 Swift、App Intents 與平台原生安全儲存。

---

## 9. 系統元件

### 9.1 iOS Shortcut Client

MVP 包含兩個捷徑：

#### TailClip：傳送到電腦

職責：

1. 取得剪貼簿或 Share Sheet 輸入。
2. 判斷內容類型。
3. 建立 TailClip envelope。
4. 對目標 Agent 發送 HTTPS POST。
5. 解析成功或錯誤回應。
6. 顯示可理解的結果通知。

#### TailClip：從電腦取回

職責：

1. 對目標 Agent 發送 HTTPS GET。
2. 驗證 response schema。
3. 選擇 iOS 可接受的 representation。
4. 將內容寫入 iOS 剪貼簿。
5. 顯示來源裝置、格式與結果。

MVP 捷徑可透過匯入問題保存 URL 與 token，但 MUST 在文件中明示：Shortcuts 並非等同 Keychain 的高強度秘密儲存。原生 App 階段應將配對 token 放入 Keychain。

### 9.2 TailClip Desktop Agent

職責：

- 提供版本化 HTTP API。
- 驗證配對 token 與可用的 Tailscale 身分資訊。
- 驗證 schema、MIME、編碼、大小、雜湊與 TTL。
- 維護短期 replay／dedup cache。
- 呼叫平台 Clipboard Adapter。
- 回報 metadata-only 日誌與 health status。
- 選配：協調 Taildrop inbox／outbox。

### 9.3 Windows Clipboard Adapter

職責：

- 在互動式桌面 session 讀寫剪貼簿。
- MVP 支援 Unicode 純文字。
- 後續支援 HTML、PNG 與檔案清單。
- 自動同步模式可使用 clipboard format listener 與 sequence number 偵測變更。

### 9.4 Linux Clipboard Adapter

職責：

- 偵測 Wayland 或 X11 session。
- Wayland 優先透過 `wl-copy`／`wl-paste`。
- X11 可使用 `xclip`、`xsel` 或後續原生 backend。
- Agent 以 `systemd --user` 或桌面自啟方式運行。

### 9.5 Taildrop Worker（選配）

職責：

- 接收大型內容或檔案。
- 將傳入檔案放入受控臨時目錄。
- 依 transfer ID 與 manifest 配對。
- 驗證檔名、大小與 SHA-256。
- 將接收到的本機檔案路徑寫入桌面剪貼簿，或通知使用者另存。
- 定期清除過期暫存檔。

### 9.6 Future Native iOS App

職責：

- Keychain 儲存配對資料。
- 裝置管理與 QR 配對。
- App Intents、Control Center、Widget 與 Action Button 整合。
- Share Extension。
- 更完整的 MIME 與錯誤處理。
- 在平台允許範圍內提供較順暢的前景／短背景流程。

---

## 10. 網路拓撲

### 10.1 Agent 監聽

Agent MUST 預設只監聽：

```text
127.0.0.1:17733
```

不得預設監聽 `0.0.0.0`、實體 LAN IP 或 Tailscale IP，避免攻擊者繞過 Serve 層並偽造 Tailscale identity headers。

### 10.2 Tailscale Serve

預計啟用命令：

```bash
tailscale serve --bg --https=443 http://127.0.0.1:17733
```

預計存取形式：

```text
https://<device>.<tailnet>.ts.net/v1/...
```

`--bg` 用於讓設定持續存在，並在裝置或 Tailscale 重新啟動後恢復服務。

### 10.3 禁止 Funnel

TailClip MUST NOT 在預設安裝流程中啟用 Funnel。

若使用者自行透過其他 reverse proxy 暴露服務，該部署不屬於 MVP 支援範圍，且不得假定 tailnet 身分標頭仍然可信。

### 10.4 Tailscale Grants 範例

概念範例：只允許指定 iPhone 連到指定桌面的 TCP 443。

```json
{
  "hosts": {
    "near-iphone": "100.64.1.10",
    "work-pc": "100.64.1.20"
  },
  "grants": [
    {
      "src": ["near-iphone"],
      "dst": ["work-pc"],
      "ip": ["tcp:443"]
    }
  ]
}
```

實際 IP 與 selector MUST 依使用者 tailnet 配置調整。Grants 是網路層限制，不取代 TailClip token。

---

## 11. 配對、認證與授權

### 11.1 防護層

TailClip 使用三層控制：

1. **Tailnet membership／網路可達性。** 非 tailnet 裝置無法直接連線。
2. **Tailscale Grants／ACL。** 限制來源裝置或使用者對桌面 443 的存取。
3. **TailClip pairing token。** Agent 在應用層驗證每個敏感端點。

### 11.2 配對 token

- MUST 使用密碼學安全亂數產生，最低 256 bit entropy。
- SHOULD 使用無 padding 的 base64url 表示。
- MUST 支援輪替與撤銷。
- MUST 儲存在只允許當前使用者讀取的桌面檔案中。
- MUST NOT 寫入日誌、crash report 或 HTTP error body。
- MVP 可由 QR code 或手動匯入方式交給 iOS Shortcut。
- 原生 iOS App MUST 使用 Keychain。

### 11.3 HTTP 認證

敏感端點要求：

```http
Authorization: Bearer <pairing-token>
```

比較 token 時 SHOULD 使用 constant-time comparison。

### 11.4 Tailscale Serve 身分標頭

Serve 可向 loopback backend 提供已驗證的 Tailscale 使用者身分標頭，例如使用者登入與顯示名稱。Agent MAY 將其作為 allowlist、稽核 metadata 或第二條件。

但這些欄位主要表達 Tailscale 使用者身分，不應單獨作為指定 iPhone 的唯一配對證明。Agent 仍 MUST 驗證 pairing token。

Agent MUST 僅在請求確實來自 loopback Serve proxy 時信任這些標頭。

### 11.5 授權規則

- `GET /v1/health` MAY 不要求 token，但不得回傳任何剪貼簿、token、使用者或內部路徑資訊。
- `GET /v1/capabilities` MUST 要求 token。
- `GET /v1/clipboard/current` MUST 要求 token。
- `POST /v1/clipboard` MUST 要求 token。
- 所有未來的檔案 manifest 與管理端點 MUST 要求 token。

---

## 12. HTTP API

### 12.1 通則

- Base path：`/v1`
- 傳輸：HTTPS，由 Tailscale Serve 終止 TLS。
- Payload：除實際 Taildrop 檔案外均使用 JSON。
- 時間：RFC 3339 UTC。
- ID：UUIDv7 優先，UUIDv4 可接受。
- 雜湊：SHA-256，以小寫 hex 表示。
- JSON 欄位使用 `snake_case`。
- 未識別但可安全忽略的欄位 SHOULD 被忽略，以容許向前相容。
- 未識別且會改變安全語義的值 MUST 被拒絕。

建議請求標頭：

```http
Authorization: Bearer <token>
Content-Type: application/json
Accept: application/json
X-TailClip-Request-ID: <uuid>
```

### 12.2 `GET /v1/health`

用途：確認 Agent 是否在線。

成功回應：

```json
{
  "status": "ok",
  "service": "tailclip-agent",
  "api_version": 1
}
```

此端點不得讀取剪貼簿。

### 12.3 `GET /v1/capabilities`

用途：讓 iOS client 在傳送前判斷內容格式與大小限制。

成功回應：

```json
{
  "api_version": 1,
  "agent_version": "0.1.0",
  "platform": "windows",
  "device_id": "01J...",
  "read_mime_types": [
    "text/plain;charset=utf-8",
    "text/uri-list"
  ],
  "write_mime_types": [
    "text/plain;charset=utf-8",
    "text/uri-list"
  ],
  "max_text_bytes": 1048576,
  "max_inline_binary_bytes": 10485760,
  "default_ttl_seconds": 120,
  "taildrop_available": false
}
```

### 12.4 `POST /v1/clipboard`

用途：iOS 將內容寫入桌面剪貼簿。

請求：

```json
{
  "protocol_version": 1,
  "item_id": "0199c8c8-8fd4-7ad6-b854-6fd26861e732",
  "source": {
    "device_id": "near-iphone",
    "platform": "ios"
  },
  "created_at": "2026-09-03T07:30:00Z",
  "expires_at": "2026-09-03T07:32:00Z",
  "sequence": 42,
  "representations": [
    {
      "mime_type": "text/plain;charset=utf-8",
      "encoding": "utf8",
      "bytes": 39,
      "sha256": "<lowercase-hex>",
      "data": "這是從 iPhone 複製的內容"
    }
  ]
}
```

成功回應：

```json
{
  "ok": true,
  "item_id": "0199c8c8-8fd4-7ad6-b854-6fd26861e732",
  "applied_mime_type": "text/plain;charset=utf-8",
  "bytes": 39
}
```

Agent MUST：

1. 驗證 token。
2. 驗證 schema 與 protocol version。
3. 檢查 `expires_at`。
4. 檢查 `item_id` 是否重播。
5. 檢查 MIME allowlist。
6. 解碼內容並驗證 bytes 與 SHA-256。
7. 選擇本平台可寫入的 representation。
8. 寫入剪貼簿。
9. 將 `item_id` 加入短期 replay cache。
10. 只記錄 metadata。

### 12.5 `GET /v1/clipboard/current`

用途：iOS 取回桌面目前的剪貼簿。

可選查詢：

```text
?accept=text/plain;charset=utf-8,text/uri-list
```

成功回應：

```json
{
  "protocol_version": 1,
  "item_id": "0199c8cf-6c17-7e94-9061-93eb2f91339c",
  "source": {
    "device_id": "work-pc",
    "platform": "windows"
  },
  "created_at": "2026-09-03T07:31:00Z",
  "expires_at": "2026-09-03T07:33:00Z",
  "sequence": 1053,
  "representations": [
    {
      "mime_type": "text/plain;charset=utf-8",
      "encoding": "utf8",
      "bytes": 31,
      "sha256": "<lowercase-hex>",
      "data": "desktop clipboard content"
    }
  ]
}
```

Agent MUST 在請求當下讀取剪貼簿，不得以舊歷史記錄冒充當前內容。

若剪貼簿為空，可回應：

```json
{
  "protocol_version": 1,
  "empty": true
}
```

### 12.6 錯誤格式

```json
{
  "ok": false,
  "error": {
    "code": "payload_too_large",
    "message": "Clipboard payload exceeds the configured limit.",
    "request_id": "0199..."
  }
}
```

`message` 不得包含 token、內容原文、作業系統機密路徑或 stack trace。

### 12.7 HTTP 狀態碼

| 狀態碼 | 用途 |
|---|---|
| `200` | 成功 |
| `400` | JSON／schema／欄位無效 |
| `401` | 缺少或無效 token |
| `403` | 身分或裝置不在 allowlist |
| `406` | 無可接受的 representation |
| `409` | 重複 item、sequence 衝突或狀態衝突 |
| `410` | 內容已過 TTL |
| `413` | 內容超過限制 |
| `415` | MIME／encoding 不支援 |
| `429` | 超過速率限制 |
| `500` | 未預期內部錯誤 |
| `503` | 無可用圖形剪貼簿 session／暫時不可用 |

---

## 13. 內容資料模型

### 13.1 Representation

一個 clipboard item MAY 同時包含多種 representation，例如 HTML 與純文字 fallback。

欄位：

| 欄位 | 必要性 | 說明 |
|---|---:|---|
| `mime_type` | MUST | 正規化 MIME，例如 `text/plain;charset=utf-8` |
| `encoding` | MUST | `utf8` 或 `base64` |
| `data` | MUST | 內嵌內容 |
| `bytes` | MUST | 解碼後 byte 數 |
| `sha256` | MUST | 解碼後內容的 SHA-256 |
| `file_name` | MAY | 檔案／圖片的建議名稱，不得含路徑 |

### 13.2 MVP MIME

MUST：

- `text/plain;charset=utf-8`

SHOULD：

- `text/uri-list`

### 13.3 第二階段 MIME

- `text/html`，且同一 item MUST 有 `text/plain;charset=utf-8` fallback。
- `image/png`，以 base64 內嵌，受 binary size limit 約束。

### 13.4 編碼規則

- `utf8` 僅用於 UTF-8 text MIME。
- `base64` 用於 binary。
- MIME 與 encoding 組合不合理時 MUST 回 `415`。
- 解碼後 bytes 或 SHA-256 不符時 MUST 拒絕。

### 13.5 大小限制

預設：

```text
純文字：             1 MiB
API 內嵌二進位：    10 MiB
更大內容：           拒絕或轉用 Taildrop
```

大小限制 MUST 可設定，但不得無限制。

### 13.6 TTL

- 預設 TTL：120 秒。
- Client MUST 設定 `created_at` 與 `expires_at`。
- Agent MUST 使用本機目前時間檢查過期。
- 過期內容 MUST NOT 寫入剪貼簿。
- 時鐘偏差容忍 MAY 設為 30 秒，但不得以此延長已明顯過期的 payload。

### 13.7 Representation 選擇

接收端依平台能力與 client accept 順序選擇：

1. 共同支援的最高保真 representation。
2. 純文字 fallback。
3. 若無共同格式，回 `406`。

接收端不得因不認識其中一個 representation 而忽略整份 item，只要仍有安全且支援的 fallback。

---

## 14. 詳細資料流程

### 14.1 iOS → Desktop

```text
使用者在 iOS 複製內容
  → 執行 Send Shortcut
  → Shortcut 讀取 Clipboard 或 Share Sheet input
  → 正規化為 representations
  → 查詢或快取 capabilities
  → 建立 UUID、TTL、hash
  → POST /v1/clipboard
  → Serve 轉發到 loopback Agent
  → Agent 驗證 token / identity / schema / TTL / hash
  → Clipboard Adapter 寫入桌面剪貼簿
  → 回傳 applied MIME
  → Shortcut 顯示完成
```

失敗時不得自動重試到超過 TTL。MVP 最多 MAY 立即重試一次可判定為暫時性的網路錯誤。

### 14.2 Desktop → iOS

```text
使用者在桌面複製內容
  → 使用者在 iOS 執行 Pull Shortcut
  → GET /v1/clipboard/current
  → Agent 即時讀取桌面剪貼簿
  → 正規化 / 限制大小 / 建立 hash 與 TTL
  → 回傳 representations
  → Shortcut 選擇可支援格式
  → Copy to Clipboard
  → 顯示完成
```

MVP 不要求桌面持續監聽剪貼簿，也不保存變更歷史。

### 14.3 Share Sheet

```text
使用者在其他 App 選取內容
  → Share
  → TailClip Shortcut / Share Extension
  → 直接使用傳入資料
  → POST /v1/clipboard
```

Share Sheet 路徑 SHOULD 優先使用傳入資料，而非再次讀取全域剪貼簿。

### 14.4 大型內容／檔案

#### iOS → Desktop

```text
Shortcut 取得檔案
  → 建立 transfer_id
  → 將檔名改為或包裝成 tailclip-<transfer_id>-<safe-name>
  → 透過 Tailscale Send File / Taildrop 傳送
  → Desktop Taildrop Worker 接收
  → 驗證大小與 hash（若有 manifest）
  → 移至受控暫存目錄
  → 將本機檔案 reference 寫入剪貼簿或顯示完成通知
```

#### Desktop → iOS

Taildrop 可傳送檔案到 iPhone，但 iOS 端無法假定收到後會自動執行 TailClip 並靜默寫入剪貼簿。因此第一版將其視為檔案交付流程，而不是完全對稱的自動剪貼簿流程。

較小圖片與二進位內容應優先經 Serve API，以保留 request/response 體驗。

---

## 15. Windows 實作規格

### 15.1 程序模型

- Agent MUST 在當前登入使用者 session 中運行。
- 可採 Tray App、使用者登入啟動項或使用者層背景程序。
- 不得只部署為 Session 0 Windows Service 後直接假定可操作互動式剪貼簿。
- 若需要系統服務管理更新，系統服務與 user-session helper 必須分離，並使用受控 IPC。

### 15.2 剪貼簿 API

MVP：

- `OpenClipboard`
- `EmptyClipboard`
- `SetClipboardData`
- `GetClipboardData`
- `CloseClipboard`
- Unicode text format

自動監聽階段：

- `AddClipboardFormatListener`
- `WM_CLIPBOARDUPDATE`
- `GetClipboardSequenceNumber`

新程式不應使用舊式 Clipboard Viewer Chain 作為主要方案。

### 15.3 鎖定與重試

Windows clipboard 可能短暫被其他程序鎖定。Adapter SHOULD：

- 使用短暫、有上限的 exponential backoff。
- 總重試時間不超過可設定的小範圍，例如 500–1000 ms。
- 失敗後回 `503`，不得無限阻塞 HTTP worker。

### 15.4 格式生命週期

若使用 Win32 delayed rendering 或傳入記憶體 handle，實作 MUST 遵守 Windows clipboard ownership 規則，避免傳入已釋放記憶體。MVP 應優先使用直接複製、範圍小且可測試的格式。

---

## 16. Linux 實作規格

### 16.1 程序模型

- Agent SHOULD 以 `systemd --user` 運行。
- MUST 繼承或取得正確的 `WAYLAND_DISPLAY`／`DISPLAY` 與 session bus 環境。
- 沒有圖形 session 時，clipboard endpoint 應回 `503`。

### 16.2 Backend 選擇

建議順序：

```text
若 WAYLAND_DISPLAY 有效且 wl-copy/wl-paste 可用：Wayland backend
否則若 DISPLAY 有效且 X11 backend 可用：X11 backend
否則：unavailable
```

### 16.3 Wayland

第一版可使用：

- `wl-copy`
- `wl-paste`
- `wl-paste --watch`（僅未來自動監聽模式）

Adapter MUST 正確傳遞 MIME type，不能一律把 binary 當文字 pipe。

### 16.4 X11

第一版可支援 `xclip` 或 `xsel`；正式發布時需明確列出依賴與支援矩陣。

Linux 通常有 PRIMARY 與 CLIPBOARD selection。TailClip MUST 預設操作一般桌面「複製／貼上」使用的 CLIPBOARD selection；PRIMARY MAY 作為選配功能，不得預設覆蓋。

### 16.5 Wayland 安全模型

不同 compositor 對 clipboard、背景存取與 portal 的行為可能不同。每個支援的桌面環境必須做實機 E2E 測試，不得只以命令在單一發行版成功就宣稱所有 Linux 相容。

---

## 17. iOS Shortcut 規格

### 17.1 Send Shortcut

預計動作：

1. 接受 Share Sheet input；若沒有則 `Get Clipboard`。
2. 判斷輸入類型。
3. 將純文字與 URL 正規化。
4. 產生 item ID、created／expires 時間。
5. 計算必要 metadata；若 Shortcuts 無法可靠計算 hash，MVP MAY 由 Agent 接受缺少 hash 的受限文字 payload，但正式 v1 protocol SHOULD 要求完整 hash。
6. `Get Contents of URL`，方法 POST，JSON body。
7. 顯示成功或錯誤。

> 實作備註：Shortcuts 對複雜 JSON、binary hash 與 base64 的操作能力會影響最終 envelope。若捷徑無法無痛產生完整 schema，可在 MVP 定義簡化的 Shortcut request，再由 Agent 正規化；不可在文件中假裝捷徑具備尚未驗證的動作。

### 17.2 Pull Shortcut

預計動作：

1. `Get Contents of URL`，方法 GET。
2. 解析 JSON。
3. 檢查 `empty` 或錯誤。
4. 取得支援的 representation。
5. `Copy to Clipboard`。
6. SHOULD 啟用 `Local Only`，避免內容又經 Apple Handoff 傳到其他 Apple 裝置。
7. 顯示來源裝置與完成狀態。

### 17.3 Token 保存

Shortcut 版本的 token 可能出現在捷徑動作或匯入設定中。MVP MUST 在文件中提示：

- 不應把捷徑公開分享時連同真實 token 一起匯出。
- token 外洩後應立即在桌面 Agent 輪替。
- 原生 App 版本改用 Keychain。

### 17.4 使用者入口

捷徑 SHOULD 可加入：

- 主畫面。
- Shortcuts Widget。
- Action Button。
- Control Center（視 iOS 與捷徑／App Intent 能力）。
- Share Sheet。

---

## 18. 同步與衝突語義

### 18.1 去重

每個 item MUST 有唯一 `item_id`。Agent 維護最近已處理 ID 的記憶體快取：

- 預設保留至少 TTL + clock skew。
- 相同 ID 與相同 hash 再次到達可回冪等成功。
- 相同 ID 但不同 hash MUST 回 `409` 並記錄安全事件 metadata。

### 18.2 防止循環

未來若桌面啟用 clipboard listener：

- Agent 寫入遠端內容時 MUST 設置 suppress marker。
- MUST 記錄最近寫入的 sequence／hash／item ID。
- 監聽器不得把同一內容立刻視為新的本機事件再送回。

### 18.3 衝突優先權

- 手動操作永遠優先於背景同步。
- 不得只用 wall-clock timestamp 判定跨裝置先後，因裝置時鐘可能偏移。
- 每個來源 MAY 使用單調遞增 `sequence`。
- MVP 沒有自動雙向推送，因此以收到並成功套用的使用者請求為準。

### 18.4 離線行為

MVP：

- 連線失敗即回報。
- 不做永久排隊。
- 不在裝置重新上線後靜默套用舊內容。

後續 MAY 提供一筆短效 pending item，但必須：

- 遵守 TTL。
- 顯示待套用內容來源與時間。
- 由使用者確認，或明確啟用的自動模式處理。

---

## 19. 安全與隱私模型

### 19.1 需保護的資產

- 當前剪貼簿內容。
- pairing token。
- tailnet 身分與裝置資訊。
- 收到的暫存檔。
- 日誌與 crash report。

### 19.2 主要威脅

| 威脅 | 範例 |
|---|---|
| 未授權 tailnet 成員 | 同 tailnet 其他人讀取工作電腦剪貼簿 |
| token 外洩 | 捷徑被分享時包含真實 token |
| header 偽造 | 攻擊者直接連 Agent 並偽造 Serve identity header |
| replay | 重送先前擷取的 POST 覆蓋剪貼簿 |
| 惡意 payload | 巨大 base64、畸形 MIME、路徑穿越檔名 |
| 敏感資訊誤同步 | 密碼、OTP、API key 被自動傳送 |
| 日誌洩漏 | debug log 記錄完整 clipboard body |
| 本機惡意程式 | 已控制端點的程式直接讀取系統剪貼簿 |
| 暫存檔殘留 | Taildrop 檔案長期留在共用目錄 |

### 19.3 控制措施

MUST：

- Agent 綁定 loopback。
- 使用 Serve，不使用 Funnel。
- 敏感端點驗證 pairing token。
- MIME allowlist。
- request body 上限。
- TTL 與 replay cache。
- rate limit。
- 不記錄 clipboard body。
- 不預設保存歷史。
- 暫存檔使用不可預測名稱與使用者私有權限。
- 淨化檔名，禁止 `..`、絕對路徑、NUL 與平台分隔符注入。
- 錯誤訊息不回傳 secret 或 stack trace。

SHOULD：

- 使用 Grants 縮小來源範圍。
- token 輪替。
- 將 token 放入 OS 安全儲存或權限受限檔案。
- 可設定內容過期後清空 iOS／桌面剪貼簿，但不得預設造成意外資料遺失。
- 對高風險模式顯示明確警告。

### 19.4 信任邊界

TailClip 不能防止：

- 已被攻陷的 iPhone 或桌面讀取本機剪貼簿。
- 擁有使用者桌面 session 權限的惡意程式存取剪貼簿。
- 使用者主動把內容傳到錯誤但已配對的裝置。
- tailnet 管理與端點安全配置不當造成的其他風險。

### 19.5 遙測

- 預設不得上傳遙測。
- 日後若加入 opt-in telemetry，只能收集版本、平台、錯誤代碼與非內容效能資訊。
- 不得收集 clipboard 原文、hash、檔名或完整目標 URL。

---

## 20. Taildrop 輔助設計

### 20.1 適用情境

- 超過 inline API 上限的圖片。
- 實體檔案。
- 多檔傳送。
- 需要保留原始 bytes 而不適合 JSON base64 的內容。

### 20.2 不適用情境

- 純文字 request/response。
- 即時讀取桌面當前剪貼簿。
- 需要細緻 API error 的互動。
- tagged node。
- 不同擁有者裝置之間的分享。
- 依賴普通 Grants 完整約束檔案傳輸的部署。

### 20.3 Desktop 接收

概念命令：

```bash
tailscale file get --loop --conflict=rename <tailclip-inbox>
```

Worker MUST 監控專用目錄，不得直接使用使用者一般 Downloads 作為可信工作佇列。

### 20.4 Manifest

未來若需要 API 與 Taildrop 配對，可新增：

```json
{
  "protocol_version": 1,
  "transfer_id": "0199...",
  "kind": "taildrop",
  "file_name": "report.pdf",
  "bytes": 1234567,
  "sha256": "...",
  "expires_at": "2026-09-03T07:32:00Z"
}
```

Worker 只在實際檔案與有效 manifest 同時存在時自動套用；否則隔離並等待使用者處理。

---

## 21. 設定檔

概念 TOML：

```toml
listen = "127.0.0.1:17733"
device_id = "work-pc"
token_file = "~/.config/tailclip/token"

max_text_bytes = 1048576
max_inline_binary_bytes = 10485760
default_ttl_seconds = 120
clock_skew_seconds = 30
request_rate_per_minute = 60

log_level = "info"
log_content = false
store_history = false

taildrop_enabled = false
taildrop_inbox = "~/.local/share/tailclip/inbox"
temporary_file_ttl_minutes = 30
```

規則：

- `listen` 若不是 loopback，Agent SHOULD 拒絕啟動，除非使用者以明確危險旗標覆寫。
- `log_content=true` 不應在正式版提供，或至少必須標記為危險 debug 模式並自動過期。
- 設定檔不得內嵌 token；使用獨立權限受限檔案或 OS credential store。

---

## 22. 可觀測性

### 22.1 允許記錄

- timestamp。
- request ID。
- 方向：send／pull。
- 來源 Tailscale user（若有且已信任）。
- MIME type。
- bytes。
- status code／error code。
- latency。
- Agent version 與平台。

### 22.2 禁止記錄

- clipboard 原文。
- base64 body。
- pairing token。
- Authorization header。
- clipboard hash。
- 完整檔名或檔案內容。
- 含 query／fragment 的敏感 URL。

### 22.3 Health

Agent SHOULD 提供本機診斷資訊：

- API 是否監聽。
- Clipboard backend 是否可用。
- 當前 desktop session 類型。
- Taildrop worker 是否啟用。
- 最近一次錯誤代碼，不含內容。

---

## 23. 封裝與生命週期

### 23.1 Windows

預計：

- 單一 Agent 執行檔。
- Tray UI 或精簡背景模式。
- 登入使用者自動啟動。
- 安裝器設定防火牆時不需要開 LAN port，因 Agent 只綁 loopback。
- 未簽名 pre-alpha 需明確說明；正式版應評估 code signing。

### 23.2 Linux

預計：

- 單一 Agent 執行檔。
- `systemd --user` unit。
- Wayland 依賴 `wl-clipboard`。
- X11 依賴 `xclip` 或 `xsel`，或後續以 native library 取代。
- 套件格式可先提供 tarball，再評估 deb／rpm／AUR。

### 23.3 iOS

MVP：

- 可匯入的 `.shortcut` 或可重建步驟。
- 匯入時詢問 Serve URL、token 與預設裝置。

後續：

- 原生 App。
- App Store／TestFlight 發布策略。
- QR 配對。

### 23.4 更新

MVP 不自動更新。正式版若加入自動更新：

- MUST 驗證簽章。
- MUST 不經由剪貼簿 API 自行傳送更新檔。
- MUST 提供關閉與 rollback 路徑。

---

## 24. 建議倉庫結構

```text
.
├── README.md
├── docs/
│   └── SPEC.md
├── cmd/
│   └── tailclip-agent/
├── internal/
│   ├── api/
│   ├── clipboard/
│   │   ├── windows/
│   │   └── linux/
│   ├── protocol/
│   ├── security/
│   └── transfer/
├── shortcuts/
│   ├── send-to-desktop/
│   └── pull-from-desktop/
├── packaging/
│   ├── windows/
│   └── linux/
├── tests/
│   ├── integration/
│   └── e2e/
└── go.mod
```

目錄應隨實際程式碼建立，不需為了結構圖提交大量空資料夾。

---

## 25. 實作里程碑與驗收條件

### M0：規格與概念驗證

內容：

- README 與本 SPEC。
- 驗證 iOS Tailscale Shortcuts 可選擇目標並 Taildrop 傳送檔案。
- 驗證 Windows／Linux 可持續接收 Taildrop 檔案。
- 驗證 Tailscale Serve 可將 HTTPS 代理至 loopback Agent。
- 建立最小文字 read/write prototype。

驗收：

- 在實際 iPhone、Windows 與 Linux 各完成一次概念驗證。
- 記錄 Taildrop、Serve、Shortcuts 的實際限制。
- 不把 PoC 誤標為可用版本。

### M1：雙向純文字 MVP

內容：

- Go Agent。
- `/v1/health`、`/v1/capabilities`、`POST /v1/clipboard`、`GET /v1/clipboard/current`。
- Windows Unicode text adapter。
- Linux Wayland text adapter；X11 至少一個 backend。
- pairing token。
- Send／Pull Shortcuts。
- metadata-only log、TTL、size limit、rate limit 與 dedup。

驗收：

- iOS ↔ Windows 雙向成功。
- iOS ↔ Linux Wayland 雙向成功。
- X11 支援範圍明確並通過至少一個桌面測試。
- 中文、英文、Emoji、CRLF／LF 與 1 MiB 文字 round-trip。
- 無效 token、過期內容、重播、過大內容與離線均有正確錯誤。
- Agent 未監聽非 loopback。
- 日誌無剪貼簿內容。

### M2：URL、HTML、PNG 與 Share Sheet

內容：

- `text/uri-list`。
- `text/html` + plain fallback。
- `image/png`。
- capability negotiation。
- iOS Share Sheet 路徑。

驗收：

- URL 可在雙向傳送後正常貼入支援 App。
- HTML 在支援平台保留基本格式，不支援時回退純文字。
- PNG 在大小限制內雙向成功。
- 惡意 HTML 不在 Agent 中渲染或執行。
- Share Sheet 不需要先覆蓋 iOS 全域剪貼簿。

### M3：大型內容與檔案

內容：

- Taildrop Worker。
- transfer manifest。
- 暫存、hash、過期清理與檔名淨化。
- Windows／Linux 檔案 clipboard reference。

驗收：

- iOS → Windows／Linux 的單檔傳送成功。
- 大型檔案不經 JSON base64。
- 路徑穿越、重名、截斷與 hash 不符安全失敗。
- tagged node、不同擁有者裝置等限制在 UI／文件中明確顯示。
- iOS 接收端的非自動化限制不被隱藏。

### M4：原生 iOS App

內容：

- Swift App。
- Keychain。
- QR 配對。
- App Intents。
- Share Extension。
- 控制中心／Action Button 入口。

驗收：

- token 不出現在可公開分享的捷徑內容中。
- 使用者仍有明確操作意圖。
- App 被暫停或終止後，產品不宣稱可被動常駐同步。
- 配對撤銷與輪替可操作。

### M5：可選半自動／自動同步

內容：

- Windows／Linux clipboard listener。
- loop suppression。
- 裝置線上狀態。
- 明確 opt-in 安全設定。

驗收：

- 預設仍為手動模式。
- 不會形成回傳循環。
- 敏感內容策略與 allow／deny 規則可設定。
- 離線舊內容不會無提示覆蓋新內容。
- 對 iOS 背景限制的行為描述準確。

---

## 26. 測試計畫

### 26.1 單元測試

- JSON schema。
- MIME negotiation。
- UTF-8／base64 decode。
- byte length 與 SHA-256。
- TTL／clock skew。
- replay cache。
- token constant-time compare。
- filename sanitization。
- error mapping。

### 26.2 API 整合測試

- 有效／無效 token。
- body 上限。
- malformed JSON。
- 不支援 MIME。
- 多 representation fallback。
- 冪等重送。
- 同 ID 不同 hash。
- rate limiting。
- concurrent read／write。

### 26.3 平台測試

Windows：

- Windows 11 當前支援版本。
- 一般桌面 session。
- RDP／快速使用者切換行為。
- clipboard 被短暫鎖定。
- Tray 自啟。

Linux：

- 至少 GNOME Wayland。
- 至少 KDE Plasma Wayland。
- 至少一個 X11 session。
- session 登出／登入。
- `wl-clipboard` 缺少時錯誤。

iOS：

- Get Clipboard／Copy to Clipboard。
- Local Only。
- Shortcuts 從前景、Widget、Share Sheet 啟動。
- token 錯誤。
- 目標離線。
- 大型 payload。

### 26.4 安全測試

- 直接連非 loopback 失敗。
- 偽造 Serve headers 不足以繞過 token。
- replay 失敗。
- 過期 payload 失敗。
- zip bomb／超大 base64／deep JSON 被限制。
- path traversal 檔名被拒絕。
- log snapshot 不含內容與 secret。
- Taildrop 暫存權限與清理。

### 26.5 E2E 驗收矩陣

| 來源 | 目標 | 文字 | URL | PNG | 檔案 |
|---|---|---:|---:|---:|---:|
| iOS | Windows | M1 | M2 | M2 | M3 |
| Windows | iOS | M1 | M2 | M2 | M3（受 iOS UX 限制） |
| iOS | Linux Wayland | M1 | M2 | M2 | M3 |
| Linux Wayland | iOS | M1 | M2 | M2 | M3（受 iOS UX 限制） |
| iOS | Linux X11 | M1 | M2 | M2 | M3 |
| Linux X11 | iOS | M1 | M2 | M2 | M3（受 iOS UX 限制） |

---

## 27. 效能與可靠性目標

MVP 在正常 tailnet 連線下 SHOULD 達成：

- 文字 API 本機處理 P95 < 100 ms，不含網路與 iOS Shortcut 啟動時間。
- 一般短文字端到端在使用者感知上接近即時。
- Agent idle memory 保持在桌面常駐工具合理範圍；具體門檻於 prototype 後固定。
- 單一請求不因 clipboard lock 無限等待。
- Agent crash 不得破壞原有剪貼簿內容或設定檔。
- 同一時間的寫入採序列化，讀取具有明確一致性。

不應為了追求微小 latency 而移除驗證、TTL、hash 或安全限制。

---

## 28. 風險與未決問題

### 28.1 產品風險

- iOS 操作無法完全做到 Universal Clipboard 的零操作體驗。
- Shortcuts 對複雜 binary／hash／JSON 的可用性需實機驗證。
- 使用者可能期待桌面主動推入 iPhone，與 iOS 平台限制衝突。
- 多台裝置選擇若做得太重，會破壞「一鍵」目標。

### 28.2 技術風險

- Windows clipboard ownership 與 session 切換。
- Wayland compositor 差異。
- Tailscale Serve identity header 與 app capability 的版本差異。
- Taildrop alpha 狀態與限制未來可能改變。
- iOS Shortcuts 匯入與秘密管理不夠理想。

### 28.3 未決決策

- 開源授權。
- Go module path 與最終 binary 名稱。
- Windows UI framework 與簽章策略。
- Linux 首批正式支援的 distributions／desktops。
- Shortcut 是否採單一選單捷徑或 Send／Pull 兩支。
- pairing QR schema。
- 是否在 v1 強制 client 計算 SHA-256，或允許 Shortcut 簡化 request。
- App capability 是否可取代部分 bearer token 操作；在完成版本與相容性測試前不得假設。
- 檔案由 Taildrop 交付後，在 iOS 上應進入 Files、Share Sheet 或 clipboard 的最終 UX。

### 28.4 Apple 平台互通能力的未來觀察

截至 2026-09-03，Apple 已公開 Accessory Transport Extension，可在與 AccessorySetupKit 配對後，透過 Bluetooth、local network 或 internet 等 transport 與配件安全交換資料。其公開文件目前列出的系統整合重點是 Wi-Fi network sharing 與 notification forwarding；尚未提供可供 TailClip 直接採用的通用 pasteboard／clipboard interoperability API。客戶端安裝目前也限於符合條件的歐盟裝置，雖然開發與測試可在其他地區進行。

因此：

- MVP MUST NOT 依賴 Accessory Transport Extension。
- 專案 SHOULD 監看 Apple 後續是否公開 pasteboard interoperability、相關 entitlement 與區域限制。
- 若未來能取得系統級 clipboard change notification 與安全 import/export 能力，原生 iOS App MAY 新增一個平台原生通道。
- 新通道必須作為可替換 transport／event source，不得迫使 Serve API 與桌面 clipboard adapter 重寫。
- 在官方 API、entitlement、發布區域與審核條件完整確定前，不得在 README 宣稱支援被動自動同步。

---

## 29. 外部技術依據

以下連結作為本規格的初始官方依據；實作時應再次核對當前版本：

### Tailscale

- [Tailscale Serve](https://tailscale.com/docs/features/tailscale-serve)
- [Tailscale Serve CLI](https://tailscale.com/docs/reference/tailscale-cli/serve)
- [Taildrop](https://tailscale.com/docs/features/taildrop)
- [Tailscale Shortcuts for macOS and iOS](https://tailscale.com/docs/features/mac-ios-shortcuts)
- [Tailscale Grants syntax](https://tailscale.com/docs/reference/syntax/grants)

### Apple

- [Use the Clipboard in Shortcuts on iPhone or iPad](https://support.apple.com/guide/shortcuts/apd081d9d61f/ios)
- [Request your first API in Shortcuts on iPhone or iPad](https://support.apple.com/guide/shortcuts/apd58d46713f/ios)
- [Launch a shortcut from another app on iPhone or iPad](https://support.apple.com/guide/shortcuts/apd163eb9f95/ios)
- [UIPasteboard](https://developer.apple.com/documentation/uikit/uipasteboard)
- [Choosing Background Strategies for Your App](https://developer.apple.com/documentation/backgroundtasks/choosing-background-strategies-for-your-app)
- [App Intents](https://developer.apple.com/documentation/appintents/app-intents)
- [Accessory Transport Extension](https://developer.apple.com/documentation/accessorytransportextension)

### Windows

- [Using the Clipboard — Win32](https://learn.microsoft.com/windows/win32/dataxchg/using-the-clipboard)
- [Clipboard — Win32](https://learn.microsoft.com/windows/win32/dataxchg/clipboard)

### Linux

- [wl-clipboard](https://github.com/bugaevc/wl-clipboard)

---

## 30. 決策摘要

TailClip 的第一條可用路線不是把 Taildrop 當成剪貼簿資料庫，而是：

```text
iOS Shortcuts
    ↓ HTTPS request
Tailscale Serve
    ↓ loopback proxy
TailClip Desktop Agent
    ↓ platform adapter
Windows / Linux Clipboard
```

反方向仍由 iOS 主動 GET：

```text
iOS Pull Shortcut
    ↓ GET current clipboard
Tailscale Serve
    ↓
Desktop Agent 即時讀取
    ↓ response
Copy to Clipboard on iOS
```

Taildrop 保留給大型內容與檔案；預設手動觸發、不保存歷史、不公開服務。這條路線在 iOS 平台限制、Tailscale 能力、安全性與開發成本之間取得最可實作的平衡。
