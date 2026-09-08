package simple

import (
	"encoding/json"
	"html/template"
	"mime"
	"net/http"
	"net/url"
	"strings"

	shortcutassets "github.com/kuanfu0430/tailclip/shortcuts"
)

func (s *Service) pageLocked(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/shortcuts/") {
		var data []byte
		var name string
		switch r.URL.Path {
		case "/shortcuts/TailClip-Simple-Send.shortcut":
			data = shortcutassets.SimpleSend
			name = "TailClip：簡易傳送.shortcut"
		case "/shortcuts/TailClip-Simple-Pull.shortcut":
			data = shortcutassets.SimplePull
			name = "TailClip：簡易取回.shortcut"
		default:
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
		w.Write(data)
		return
	}
	if s.ticket == "" || r.URL.Path != "/setup/"+s.ticket || !s.now().Before(s.expires) {
		http.Error(w, "配對頁已失效，請在電腦產生新 QR。", 404)
		return
	}
	p := Pairing{1, "cloudflare", "https://" + s.host + "/v1", s.ticket, s.expires}
	raw, _ := json.Marshal(p)
	q := url.Values{"name": {"TailClip：簡易取回"}, "input": {"text"}, "text": {string(raw)}}
	// URL 僅由固定 scheme、固定捷徑名稱及 JSON 編碼的本機配對票券生成。
	link := template.URL("shortcuts://run-shortcut?" + q.Encode())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
	_ = setupPage.Execute(w, struct {
		Device, Payload string
		Link            template.URL
	}{s.deviceName, string(raw), link})
}

var setupPage = template.Must(template.New("simple").Parse(`<!doctype html><html lang="zh-Hant"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>TailClip 簡易連線</title><style>body{font:17px -apple-system,sans-serif;background:#f4f7fb;color:#17202b;margin:0;padding:24px}main{max-width:480px;margin:36px auto;padding:28px;background:white;border-radius:24px}a,button{display:block;box-sizing:border-box;width:100%;padding:16px;margin:14px 0;border:0;border-radius:14px;background:#087cff;color:white;text-decoration:none;text-align:center;font:inherit}p{line-height:1.6}.secondary{background:#e9eff7;color:#17202b}small{color:#637083}</style><main><h1>連接 {{.Device}}</h1><p>首次先安裝兩支簡易捷徑，之後每次產生新網址，只需掃碼並按「連接並取回」。</p><a class="secondary" href="/shortcuts/TailClip-Simple-Send.shortcut">① 安裝「TailClip：簡易傳送」</a><a class="secondary" href="/shortcuts/TailClip-Simple-Pull.shortcut">② 安裝「TailClip：簡易取回」</a><a href="{{.Link}}">③ 連接並取回</a><button class="secondary" id="copy">備援：複製配對資料</button><p id="message"></p><small>首次依 iOS 提示允許捷徑存取網路與檔案。連線使用 Cloudflare；電腦或隧道重新啟動後須重掃 QR。</small><span id="payload" hidden>{{.Payload}}</span></main><script>document.getElementById('copy').onclick=async()=>{try{await navigator.clipboard.writeText(document.getElementById('payload').textContent);document.getElementById('message').textContent='已複製，請執行「TailClip：簡易取回」。'}catch(e){document.getElementById('message').textContent='無法複製，請使用上方「連接並取回」。'}};</script></html>`))
