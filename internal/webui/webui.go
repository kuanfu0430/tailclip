package webui

import (
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/kuanfu0430/tailclip/internal/buildinfo"
	"github.com/kuanfu0430/tailclip/internal/pairing"
	qrcode "github.com/skip2/go-qrcode"
)

type ShortcutAssets struct {
	Send []byte
	Pull []byte
}

func (a ShortcutAssets) Ready() bool { return len(a.Send) > 0 && len(a.Pull) > 0 }

type PublicHandler struct {
	manager *pairing.Manager
	assets  ShortcutAssets
}

func NewPublicHandler(manager *pairing.Manager, assets ShortcutAssets) http.Handler {
	return &PublicHandler{manager: manager, assets: assets}
}

func (h *PublicHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setPrivateHeaders(w)
	if r.Method != http.MethodGet {
		http.Error(w, "這個頁面只接受 GET。", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 || len(parts) > 3 || parts[0] != "setup" {
		h.expired(w)
		return
	}
	session, ok := h.manager.Get(parts[1])
	if !ok {
		h.expired(w)
		return
	}
	if len(parts) == 3 {
		h.shortcut(w, parts[2])
		return
	}

	pairingJSON, err := session.Data.JSON()
	if err != nil {
		http.Error(w, "無法準備配對資料。", http.StatusInternalServerError)
		return
	}
	data := pairingPageData{
		DeviceName:  session.Data.DeviceName,
		Nonce:       session.Nonce,
		PairingData: base64.StdEncoding.EncodeToString(pairingJSON),
		ExpiresAt:   session.ExpiresAt.Local().Format("15:04"),
		AssetsReady: h.assets.Ready(),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pairingPage.Execute(w, data); err != nil {
		return
	}
}

func (h *PublicHandler) shortcut(w http.ResponseWriter, name string) {
	var content []byte
	var filename string
	switch name {
	case "TailClip-Send.shortcut":
		content = h.assets.Send
		filename = "TailClip：傳送.shortcut"
	case "TailClip-Pull.shortcut":
		content = h.assets.Pull
		filename = "TailClip：取回.shortcut"
	default:
		h.expired(w)
		return
	}
	if len(content) == 0 {
		http.Error(w, "這個測試版本尚未包含捷徑檔。", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (h *PublicHandler) expired(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_ = expiredPage.Execute(w, nil)
}

type DashboardData struct {
	Mode               string
	ActionPath         string
	ConnectionMessage  string
	SimplePaired       bool
	Configure          func(context.Context, string) error
	DeviceName         string
	DNSName            string
	PairingURL         string
	RotatePath         string
	ExpiresAt          time.Time
	ClipboardAvailable bool
	ShortcutsReady     bool
}

func RenderDashboard(w http.ResponseWriter, data DashboardData) error {
	setPrivateHeaders(w)
	var png []byte
	if data.PairingURL != "" {
		var err error
		png, err = qrcode.Encode(data.PairingURL, qrcode.Medium, 360)
		if err != nil {
			return fmt.Errorf("無法建立配對 QR: %w", err)
		}
	}
	view := dashboardPageData{
		DashboardData: data,
		// QR 內容完全由程式產生；標成 template.URL，避免 html/template 將 data URL 改成 #ZgotmplZ。
		QRCode:        template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(png)),
		ExpiresAtText: data.ExpiresAt.Local().Format("15:04"),
		AgentVersion:  buildinfo.Version,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return dashboardPage.Execute(w, view)
}

func setPrivateHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src data:; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
}

type pairingPageData struct {
	DeviceName  string
	Nonce       string
	PairingData string
	ExpiresAt   string
	AssetsReady bool
}

type dashboardPageData struct {
	DashboardData
	QRCode        template.URL
	ExpiresAtText string
	AgentVersion  string
}

var pairingPage = template.Must(template.New("pairing").Parse(pageShellStart + `
<main class="shell">
  <section class="glass hero">
    <div class="brand"><span class="mark">T</span><span>TailClip</span></div>
    <p class="eyebrow">與 {{.DeviceName}} 配對</p>
    <h1>跟著做，就設定完成。</h1>
    <p class="lead">依序安裝兩支捷徑並複製配對資料，再執行「TailClip：取回」完成配對並取得電腦文字。若先執行「傳送」，本次只完成配對；請再複製要傳送的文字並執行一次。</p>
    {{if .AssetsReady}}
    <div class="stack">
      <a class="button secondary" href="{{.Nonce}}/TailClip-Send.shortcut">① 安裝「TailClip：傳送」</a>
      <a class="button secondary" href="{{.Nonce}}/TailClip-Pull.shortcut">② 安裝「TailClip：取回」</a>
      <button class="button primary" id="copy" type="button">③ 複製配對資料</button>
    </div>
    <p id="result" class="result" role="status" aria-live="polite"></p>
    {{else}}
    <div class="notice">這個開發版本尚未內含可安裝的捷徑；Agent 配對頁本身已可測試。</div>
    {{end}}
    <p class="foot">舊版使用者請安裝兩支新版捷徑；同名時選擇取代。之後請執行中文名稱的新版，避免誤開 TailClip-Send 2 等舊副本。此頁於 {{.ExpiresAt}} 失效。</p>
  </section>
</main>
<span id="payload" hidden>{{.PairingData}}</span>
<script>
const button = document.getElementById('copy');
if (button) button.addEventListener('click', async () => {
  const result = document.getElementById('result');
  button.disabled = true;
  button.textContent = '正在複製…';
  try {
    const encoded = document.getElementById('payload').textContent;
    const bytes = Uint8Array.from(atob(encoded), c => c.charCodeAt(0));
    await navigator.clipboard.writeText(new TextDecoder().decode(bytes));
    result.textContent = '已複製。現在執行「TailClip：取回」，或從分享選單執行「TailClip：傳送」。';
    button.textContent = '已複製配對資料';
  } catch (_) {
    result.textContent = 'Safari 未允許自動複製。請長按此按鈕後再試一次。';
    button.disabled = false;
    button.textContent = '重新複製配對資料';
  }
});
</script>` + pageShellEnd))

var expiredPage = template.Must(template.New("expired").Parse(pageShellStart + `
<main class="shell"><section class="glass hero compact">
  <div class="brand"><span class="mark">T</span><span>TailClip</span></div>
  <p class="eyebrow">安全連結已關閉</p>
  <h1>配對頁已失效。</h1>
  <p class="lead">請回到電腦，再按一次「顯示配對 QR」取得新連結。</p>
</section></main>` + pageShellEnd))

var dashboardPage = template.Must(template.New("dashboard").Parse(pageShellStart + `
<main class="shell wide">
  <section class="glass dashboard">
    <header><div class="brand"><span class="mark">T</span><span>TailClip</span></div><span class="badge">{{.AgentVersion}}</span></header>
    {{if .Configure}}
    <p class="lead">{{if eq .Mode "choose"}}你想如何連接裝置？{{else if eq .Mode "simple"}}目前使用簡易連線服務。{{else}}目前使用自己的 Tailscale。{{end}}</p>
    {{if .ConnectionMessage}}<p class="notice" role="status">{{.ConnectionMessage}}</p>{{end}}
    <form method="post" action="{{.ActionPath}}" class="stack">
      <button class="button secondary" name="action" value="tailscale">使用現有 Tailscale</button>
      <button class="button secondary" name="action" value="simple">使用簡易連線／重新連接</button>
    </form>
    {{end}}
    {{if eq .Mode "choose"}}
      <p class="foot">已有連線的人沿用 Tailscale；沒有 Tailscale 的人選簡易連線。只會啟用你選擇的入口。</p>
    {{else if eq .Mode "simple"}}
      <p class="lead">{{if .SimplePaired}}本次通道已配對。{{else}}尚未連接手機。{{end}}用 iPhone 相機掃描下方 QR，安裝簡易捷徑並按「連接並取回」。</p>
      <p class="notice">臨時隧道使用 Cloudflare HTTPS 代理，不需手機 VPN。電腦或隧道重新啟動後須重掃；Cloudflare 不保證臨時通道可用率。</p>
      <form method="post" action="{{.ActionPath}}" class="stack">
        <button class="button primary" name="action" value="pair" {{if .SimplePaired}}onclick="return confirm('新手機完成配對後會取代舊手機，繼續嗎？')"{{end}}>連接手機／產生新 QR</button>
        <button class="button danger" name="action" value="revoke" onclick="return confirm('確定解除手機連接？')">解除手機連接</button>
      </form>
      {{if .PairingURL}}<div class="qr-wrap"><img class="qr" src="{{.QRCode}}" alt="TailClip 簡易連線配對 QR"><span>此 QR 於 {{.ExpiresAtText}} 失效</span></div>{{end}}
    {{else if .PairingURL}}
    <div class="grid">
      <div class="copy">
        <p class="eyebrow">{{.DeviceName}}</p>
        <h1>掃一次，之後每次只要一下。</h1>
        <p class="lead">用 iPhone 相機掃描 QR，依序安裝兩支捷徑並複製配對資料。</p>
        <div class="status-list">
          <div><span>Tailnet 位址</span><strong>{{.DNSName}}</strong></div>
          <div><span>桌面剪貼簿</span><strong class="{{if .ClipboardAvailable}}ok{{else}}warn{{end}}">{{if .ClipboardAvailable}}可使用{{else}}目前不可使用{{end}}</strong></div>
          <div><span>捷徑檔案</span><strong class="{{if .ShortcutsReady}}ok{{else}}warn{{end}}">{{if .ShortcutsReady}}已就緒{{else}}尚未內含{{end}}</strong></div>
        </div>
        <form method="post" action="{{.RotatePath}}" onsubmit="return rotatePairing(this)">
          <button class="button danger" type="submit">撤銷舊配對並建立新 QR</button>
        </form>
        <p class="foot">QR 將於 {{.ExpiresAtText}} 失效。HTTPS 憑證會讓上方完整 ts.net 名稱出現在公開的 Certificate Transparency 紀錄。</p>
      </div>
      <div class="qr-wrap"><img class="qr" src="{{.QRCode}}" alt="TailClip 限時配對 QR"><span>使用 iPhone 相機掃描</span></div>
    </div>
    {{else}}
      <p class="notice">請確認 Tailscale 已登入並連線，再重新選擇入口。</p>
    {{end}}
  </section>
</main><script>
function rotatePairing(form) {
  if (!confirm('舊捷徑會立即失效，確定要重新配對嗎？')) return false;
  const button = form.querySelector('button');
  button.disabled = true;
  button.textContent = '正在建立新配對…';
  return true;
}
</script>` + pageShellEnd))

const pageShellStart = `<!doctype html><html lang="zh-Hant"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover"><title>TailClip</title><style>
:root{color-scheme:light;--ink:#17202b;--muted:#637083;--line:rgba(255,255,255,.72);--glass:rgba(255,255,255,.69);--blue:#087cff;--ok:#087a55;--warn:#a45b00}*{box-sizing:border-box}body{margin:0;min-height:100vh;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:var(--ink);background:radial-gradient(circle at 12% 0%,#dff4ff 0,transparent 38%),radial-gradient(circle at 92% 90%,#e9e2ff 0,transparent 42%),#f4f7fb}.shell{min-height:100vh;display:grid;place-items:center;padding:24px}.shell.wide{padding:36px}.glass{width:min(100%,540px);background:var(--glass);border:1px solid var(--line);box-shadow:0 24px 70px rgba(53,72,98,.17),inset 0 1px 0 rgba(255,255,255,.9);backdrop-filter:blur(28px) saturate(145%);-webkit-backdrop-filter:blur(28px) saturate(145%);border-radius:30px}.hero{padding:34px}.hero.compact{padding-bottom:40px}.brand{display:flex;align-items:center;gap:10px;font-weight:680;letter-spacing:-.02em}.mark{display:grid;place-items:center;width:32px;height:32px;border-radius:10px;color:white;background:linear-gradient(145deg,#2797ff,#6558ff);box-shadow:inset 0 1px 0 rgba(255,255,255,.45),0 6px 16px rgba(42,111,238,.25)}.eyebrow{margin:36px 0 9px;color:var(--blue);font-size:14px;font-weight:700}h1{font-size:clamp(32px,8vw,48px);line-height:1.02;letter-spacing:-.05em;margin:0;max-width:720px}.lead{color:var(--muted);font-size:17px;line-height:1.55;margin:20px 0 26px}.stack{display:grid;gap:11px}.button{appearance:none;width:100%;border:0;border-radius:16px;padding:15px 18px;font:inherit;font-weight:650;text-align:center;text-decoration:none;cursor:pointer;transition:transform .15s,opacity .15s}.button:active{transform:scale(.985)}.button:disabled{opacity:.66;cursor:wait}.primary{background:#111b29;color:#fff}.secondary{background:rgba(255,255,255,.76);color:#17202b;border:1px solid rgba(108,128,155,.16)}.danger{margin-top:14px;background:transparent;color:#b13b45;border:1px solid rgba(177,59,69,.18)}.result{min-height:24px;margin:14px 2px 0;color:var(--ok);font-size:14px;line-height:1.45}.foot{margin:18px 2px 0;color:#778395;font-size:12px;line-height:1.45}.notice{padding:16px;border-radius:16px;background:#fff3dc;color:#794c0c;line-height:1.5}.dashboard{width:min(100%,940px);padding:30px}.dashboard header{display:flex;justify-content:space-between;align-items:center}.badge{padding:7px 10px;border-radius:999px;background:rgba(255,255,255,.62);font-size:12px;color:var(--muted)}.grid{display:grid;grid-template-columns:minmax(0,1fr) 330px;gap:54px;align-items:center;padding:28px 6px 8px}.status-list{display:grid;gap:1px;overflow:hidden;border-radius:18px;background:rgba(121,139,164,.14)}.status-list div{display:flex;justify-content:space-between;gap:20px;padding:13px 15px;background:rgba(255,255,255,.7);font-size:14px}.status-list span{color:var(--muted)}.status-list strong{max-width:65%;overflow-wrap:anywhere;text-align:right}.ok{color:var(--ok)}.warn{color:var(--warn)}.qr-wrap{display:grid;justify-items:center;gap:13px;color:var(--muted);font-size:13px}.qr{display:block;width:100%;max-width:330px;border-radius:24px;background:#fff;padding:14px;box-shadow:0 18px 40px rgba(44,63,88,.14)}@media(max-width:760px){.shell.wide{padding:18px}.dashboard{padding:24px}.grid{grid-template-columns:1fr;gap:30px}.qr-wrap{order:-1}.qr{max-width:280px}.dashboard h1{font-size:38px}.dashboard .eyebrow{margin-top:14px}}@media(prefers-reduced-motion:reduce){*{scroll-behavior:auto!important;transition:none!important}}
</style></head><body>`

const pageShellEnd = `</body></html>`
