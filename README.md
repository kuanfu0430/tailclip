# TailClip

TailClip 是一個規劃中的跨平台剪貼簿傳送工具，目標是在 **iOS、Windows 與 Linux** 之間，透過既有的 **Tailscale 私有網路**傳送文字、網址、圖片與檔案。

它想解決的核心操作很直接：

- 在 iPhone 複製內容後，執行一次捷徑，讓內容成為指定 Windows／Linux 電腦的剪貼簿。
- 在 Windows／Linux 複製內容後，於 iPhone 執行一次捷徑，把電腦目前的內容取回 iPhone 剪貼簿。
- 不架設公開中繼站，也不把剪貼簿歷史預設保存到雲端。

> **目前狀態：Pre-alpha / specification-first。** 這個倉庫目前先保存產品方向與工程規格，尚未提供可直接安裝的正式版本。

## 預計使用體驗

TailClip 預計提供兩個最基本的 iOS 動作：

### 傳送到電腦

1. 在 iPhone 複製文字、網址或圖片。
2. 執行「TailClip：傳送到電腦」捷徑。
3. 選定的 Windows／Linux 電腦立即寫入該內容。
4. 在電腦按貼上即可使用。

### 從電腦取回

1. 在 Windows／Linux 複製內容。
2. 在 iPhone 執行「TailClip：從電腦取回」捷徑。
3. TailClip 讀取該電腦目前的剪貼簿。
4. 內容寫入 iPhone 剪貼簿，可直接貼到其他 App。

桌面端會以登入使用者的常駐程式運行；iOS 端則由 Shortcuts 起步，後續再評估原生 App、App Intents、控制中心與動作按鈕整合。

## 為什麼使用 Tailscale

TailClip 不自行重做 VPN、NAT 穿透或公開轉發服務，而是把這些工作交給 Tailscale：

- 裝置之間可經由 tailnet 互相連線。
- 桌面服務只需要監聽本機位址，再由 Tailscale Serve 暴露給 tailnet。
- 不需要公網 IP、路由器轉發或公開 API。
- 可搭配 Tailscale Grants／ACL 限制哪些裝置可以存取 TailClip。

TailClip 的主通道預計使用 **HTTPS over Tailscale Serve**；Tailscale Taildrop 只作為大型內容與實體檔案的輔助傳輸方式。

## 預計安裝與設定方式

正式 MVP 完成後，預計會採以下流程：

### 1. 前置條件

- iPhone 與目標 Windows／Linux 電腦已加入同一個 tailnet。
- 各裝置上的 Tailscale 均已連線。
- 桌面端已安裝 TailClip Agent。
- iPhone 已匯入 TailClip 的兩個 Shortcuts。

### 2. 啟動桌面 Agent

TailClip Agent 預計只監聽本機：

```text
127.0.0.1:17733
```

桌面 Agent 第一次啟動時會產生配對憑證，供 iPhone 捷徑設定目標電腦。

### 3. 透過 Tailscale Serve 提供 tailnet 內 HTTPS 入口

預計使用方式：

```bash
tailscale serve --bg --https=443 http://127.0.0.1:17733
```

TailClip 不需要、也不建議使用 Tailscale Funnel，因為剪貼簿服務不應暴露到公網。

### 4. 設定 iOS 捷徑

捷徑需保存：

- 目標電腦的 Tailscale Serve 位址。
- TailClip 配對憑證。
- 預設目標裝置與可接受的內容格式。

完成後即可使用「傳送到電腦」與「從電腦取回」兩個動作。

## 預計支援的內容

第一個可用版本優先支援：

- UTF-8 純文字。
- 網址。
- Unicode、Emoji 與多行文字。

後續版本再加入：

- HTML／Rich Text，並附純文字 fallback。
- PNG 圖片。
- iOS Share Sheet。
- 大型內容與實體檔案的 Taildrop 輔助傳輸。
- 多台桌面裝置選擇。

## 安全預設

TailClip 將採保守預設：

- 預設由使用者手動觸發，不自動同步所有剪貼簿變更。
- 預設不保存剪貼簿歷史。
- 日誌只記錄請求結果、格式、大小與錯誤，不記錄實際內容。
- 桌面服務只監聽 `127.0.0.1`。
- 使用 Tailscale 網路控制、裝置配對憑證、內容大小限制與短效 TTL。
- 大型或不支援的內容不直接塞入 API。

完整的安全模型、協定與平台限制請見技術規格。

## 倉庫結構

```text
README.md          倉庫用途、預計使用方式與目前狀態
docs/SPEC.md       完整產品與工程規格
```

實作開始後，預計再加入桌面 Agent、平台剪貼簿介面、iOS Shortcuts、封裝與測試目錄。

## 技術規格

完整內容放在獨立文件：

- [TailClip 技術與產品規格](docs/SPEC.md)

README 僅作為使用者入口，不承載 API schema、威脅模型、同步演算法或平台實作細節。

## 專案階段

- [x] 確立產品目標與主要架構。
- [x] 拆分 README 與詳細規格。
- [ ] 建立 Taildrop 概念驗證。
- [ ] 完成雙向文字剪貼簿 MVP。
- [ ] 加入網址、HTML、圖片與 Share Sheet。
- [ ] 加入大型內容與檔案傳送。
- [ ] 評估原生 iOS App 與可選自動同步。

## 貢獻

在開始實作或提出架構變更前，請先閱讀 [`docs/SPEC.md`](docs/SPEC.md)。會改變安全邊界、傳輸協定、iOS 互動模型或桌面剪貼簿行為的修改，應先更新規格再提交程式碼。

## 授權與商標

本專案尚未選定開源授權；在加入 LICENSE 前，程式碼與文件不應被視為已授予通用再利用權利。

TailClip 是獨立專案，與 Tailscale Inc. 無隸屬或官方合作關係。Tailscale 與 Taildrop 等名稱屬其各自權利人所有。
