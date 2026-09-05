package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/kuanfu0430/tailclip/internal/buildinfo"
	"github.com/kuanfu0430/tailclip/internal/clipboard"
	"github.com/kuanfu0430/tailclip/internal/config"
)

const (
	MaxTextBytes = 1_048_576
	// encoding/json 最壞可將單一 byte 跳脫成六個 ASCII 字元；仍以解碼後文字判定 1 MiB 上限。
	maxBodyBytes = MaxTextBytes*6 + 4096
)

type ConfigSource interface {
	Snapshot() config.Config
}

type Server struct {
	config    ConfigSource
	clipboard clipboard.Backend
	logger    *slog.Logger
	limiter   *rateLimiter
}

type Options struct {
	Config     ConfigSource
	Clipboard  clipboard.Backend
	Logger     *slog.Logger
	RateLimit  int
	RateWindow time.Duration
	LimiterNow func() time.Time
}

func New(options Options) *Server {
	logger := options.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	limit := options.RateLimit
	if limit <= 0 {
		limit = 60
	}
	window := options.RateWindow
	if window <= 0 {
		window = time.Minute
	}
	limiter := newRateLimiter(limit, window)
	if options.LimiterNow != nil {
		limiter.now = options.LimiterNow
	}
	return &Server{
		config:    options.Config,
		clipboard: options.Clipboard,
		logger:    logger,
		limiter:   limiter,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", s.health)
	mux.HandleFunc("/v1/status", s.protected("status", s.status))
	mux.HandleFunc("/v1/clipboard/text", s.protected("clipboard", s.clipboardText))
	return securityHeaders(mux)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "這個端點只接受 GET。")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"service":       "tailclip-agent",
		"api_version":   1,
		"agent_version": buildinfo.Version,
	})
}

func (s *Server) protected(direction string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		statusCode := http.StatusOK
		errorCode := ""
		bytesCount := 0

		if !s.limiter.Allow() {
			statusCode = http.StatusTooManyRequests
			errorCode = "rate_limited"
			writeError(w, statusCode, errorCode, "操作太頻繁，請稍候再試。")
			s.log(direction, bytesCount, statusCode, errorCode, time.Since(started))
			return
		}
		if !s.authorized(r) {
			statusCode = http.StatusUnauthorized
			errorCode = "not_paired"
			writeError(w, statusCode, errorCode, "配對已失效，請在電腦上重新顯示配對 QR。")
			s.log(direction, bytesCount, statusCode, errorCode, time.Since(started))
			return
		}

		recorder := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		ctx := context.WithValue(r.Context(), requestMetricsKey{}, &requestMetrics{})
		next(recorder, r.WithContext(ctx))
		statusCode = recorder.status
		metrics := ctx.Value(requestMetricsKey{}).(*requestMetrics)
		bytesCount = metrics.bytes
		errorCode = metrics.errorCode
		s.log(direction, bytesCount, statusCode, errorCode, time.Since(started))
	}
}

func (s *Server) authorized(r *http.Request) bool {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	provided := strings.TrimPrefix(header, prefix)
	expected := s.config.Snapshot().PairingToken
	return len(provided) == len(expected) && subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		setMetrics(r, 0, "method_not_allowed")
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "這個端點只接受 GET。")
		return
	}
	cfg := s.config.Snapshot()
	available := s.clipboard.Available(r.Context()) == nil
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                  true,
		"device_name":         cfg.DeviceName,
		"tailscale_device":    cfg.TailscaleDevice,
		"platform":            runtime.GOOS,
		"clipboard_available": available,
		"max_text_bytes":      MaxTextBytes,
	})
}

func (s *Server) clipboardText(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.readClipboard(w, r)
	case http.MethodPost:
		s.writeClipboard(w, r)
	default:
		setMetrics(r, 0, "method_not_allowed")
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "這個端點只接受 GET 或 POST。")
	}
}

func (s *Server) readClipboard(w http.ResponseWriter, r *http.Request) {
	text, err := s.clipboard.ReadText(r.Context())
	if errors.Is(err, clipboard.ErrNoText) || (err == nil && text == "") {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":          true,
			"device_name": s.config.Snapshot().DeviceName,
			"empty":       true,
		})
		return
	}
	if err != nil {
		s.writeClipboardError(w, r, err)
		return
	}
	if !utf8.ValidString(text) || strings.IndexByte(text, 0) >= 0 {
		setMetrics(r, 0, "invalid_text")
		writeError(w, http.StatusInternalServerError, "invalid_text", "電腦剪貼簿不是有效的純文字。")
		return
	}
	if len(text) > MaxTextBytes {
		setMetrics(r, len(text), "text_too_large")
		writeError(w, http.StatusRequestEntityTooLarge, "text_too_large", "電腦剪貼簿文字超過 1 MiB。")
		return
	}
	setMetrics(r, len(text), "")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"device_name": s.config.Snapshot().DeviceName,
		"text":        text,
	})
}

func (s *Server) writeClipboard(w http.ResponseWriter, r *http.Request) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		setMetrics(r, 0, "invalid_request")
		writeError(w, http.StatusUnsupportedMediaType, "invalid_request", "請以 JSON 傳送純文字。")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		setMetrics(r, 0, "text_too_large")
		writeError(w, http.StatusRequestEntityTooLarge, "text_too_large", "文字超過 1 MiB。")
		return
	}
	if !utf8.Valid(body) {
		setMetrics(r, 0, "invalid_text")
		writeError(w, http.StatusBadRequest, "invalid_text", "文字不是有效的 UTF-8。")
		return
	}
	var request struct {
		Text *string `json:"text"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || decoder.Decode(&struct{}{}) != io.EOF || request.Text == nil {
		setMetrics(r, 0, "invalid_request")
		writeError(w, http.StatusBadRequest, "invalid_request", "JSON 必須只包含 text 字串。")
		return
	}
	text := *request.Text
	if text == "" || strings.IndexByte(text, 0) >= 0 {
		setMetrics(r, len(text), "invalid_text")
		writeError(w, http.StatusBadRequest, "invalid_text", "文字不可為空或包含 NUL。")
		return
	}
	if len(text) > MaxTextBytes {
		setMetrics(r, len(text), "text_too_large")
		writeError(w, http.StatusRequestEntityTooLarge, "text_too_large", "文字超過 1 MiB。")
		return
	}
	if err := s.clipboard.WriteText(r.Context(), text); err != nil {
		s.writeClipboardError(w, r, err)
		return
	}
	setMetrics(r, len(text), "")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"device_name": s.config.Snapshot().DeviceName,
		"bytes":       len(text),
	})
}

func (s *Server) writeClipboardError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, clipboard.ErrBusy) {
		setMetrics(r, 0, "clipboard_busy")
		writeError(w, http.StatusServiceUnavailable, "clipboard_busy", "電腦剪貼簿暫時忙碌，請稍候再試。若持續發生，請重新啟動 TailClip。")
		return
	}
	if errors.Is(err, clipboard.ErrUnavailable) {
		setMetrics(r, 0, "clipboard_unavailable")
		writeError(w, http.StatusServiceUnavailable, "clipboard_unavailable", "找不到可用的桌面剪貼簿工作階段。")
		return
	}
	setMetrics(r, 0, "internal_error")
	writeError(w, http.StatusInternalServerError, "internal_error", "TailClip 暫時無法完成操作。")
}

func (s *Server) log(direction string, bytesCount, status int, errorCode string, duration time.Duration) {
	s.logger.Info("request",
		"direction", direction,
		"bytes", bytesCount,
		"status", status,
		"error_code", errorCode,
		"latency_ms", duration.Milliseconds(),
	)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"ok": false,
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

type requestMetricsKey struct{}
type requestMetrics struct {
	bytes     int
	errorCode string
}

func setMetrics(r *http.Request, bytesCount int, errorCode string) {
	if metrics, ok := r.Context().Value(requestMetricsKey{}).(*requestMetrics); ok {
		metrics.bytes = bytesCount
		metrics.errorCode = errorCode
	}
}

type rateLimiter struct {
	mu          sync.Mutex
	limit       int
	window      time.Duration
	windowStart time.Time
	count       int
	now         func() time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{limit: limit, window: window, now: time.Now}
}

func (l *rateLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if l.windowStart.IsZero() || !now.Before(l.windowStart.Add(l.window)) {
		l.windowStart = now
		l.count = 0
	}
	if l.count >= l.limit {
		return false
	}
	l.count++
	return true
}
