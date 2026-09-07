package api

import (
	"context"
	"mime"
	"net"
	"net/http"
	"strings"
)

const ShortcutClientHeader = "X-TailClip-Client"
const ShortcutClientVersion = "shortcuts-v2"

// OwnerIdentity 每次取得目前本機節點擁有者與 HTTPS 名稱，不採用遠端輸入。
type OwnerIdentity func(context.Context) (login, dnsName string, err error)

type authorizationError struct {
	status        int
	code, message string
}

func (s *Server) authorize(r *http.Request) *authorizationError {
	markers := r.Header.Values(ShortcutClientHeader)
	if len(markers) == 0 {
		if s.authorized(r) {
			return nil
		}
		return &authorizationError{http.StatusUnauthorized, "not_paired", "配對已失效，請在電腦上重新顯示配對 QR。"}
	}
	// 新版明確走身分驗證，不能因仍帶著舊 token 而繞過拒絕。
	if len(markers) != 1 || markers[0] != ShortcutClientVersion ||
		len(r.Header.Values("Origin")) != 0 || len(r.Header.Values("Sec-Fetch-Site")) != 0 {
		return &authorizationError{http.StatusForbidden, "request_not_allowed", "請使用 TailClip 捷徑操作。"}
	}
	remote, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || !net.ParseIP(remote).IsLoopback() {
		return &authorizationError{http.StatusForbidden, "request_not_allowed", "請透過 Tailscale 連接這台電腦。"}
	}
	values := r.Header.Values("Tailscale-User-Login")
	if len(values) != 1 || values[0] == "" {
		return &authorizationError{http.StatusUnauthorized, "identity_required", "無法辨識 Tailscale 帳號，請確認已登入自己的帳號。"}
	}
	login, err := new(mime.WordDecoder).DecodeHeader(values[0])
	if err != nil || login == "" || strings.TrimSpace(login) != login || strings.ContainsAny(login, "\r\n\x00") {
		return &authorizationError{http.StatusUnauthorized, "identity_required", "Tailscale 身分資料無效。"}
	}
	if s.owner == nil {
		return &authorizationError{http.StatusForbidden, "identity_unavailable", "這台電腦尚無法使用免 QR 連接，原配對仍可使用。"}
	}
	owner, dnsName, err := s.owner(r.Context())
	if err != nil || owner == "" || !strings.HasSuffix(dnsName, ".ts.net") {
		return &authorizationError{http.StatusForbidden, "identity_unavailable", "無法確認電腦的 Tailscale 帳號，原配對仍可使用。"}
	}
	if r.Host != dnsName && r.Host != dnsName+":443" {
		return &authorizationError{http.StatusForbidden, "request_not_allowed", "請使用這台電腦的 Tailscale HTTPS 位址。"}
	}
	if login != owner {
		return &authorizationError{http.StatusForbidden, "access_denied", "請在手機與電腦使用相同的 Tailscale 帳號。"}
	}
	return nil
}
