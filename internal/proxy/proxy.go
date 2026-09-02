package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/IM594/mistral-sticky/internal/compat"
	"github.com/IM594/mistral-sticky/internal/pool"
	"github.com/IM594/mistral-sticky/internal/session"
	"github.com/IM594/mistral-sticky/internal/toolid"
)

type ctxKey int

const (
	ctxIndex ctxKey = iota
	ctxSessionFP
)

type Config struct {
	ProxyToken string
	Upstream   *url.URL
	Pool       *pool.Pool
	Logger     *slog.Logger
}

func New(cfg Config) http.Handler {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":       true,
			"keys":     cfg.Pool.Size(),
			"disabled": cfg.Pool.DisabledCount(),
		})
	})
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(cfg.Upstream)
			pr.Out.Host = cfg.Upstream.Host
		},
		ModifyResponse: func(resp *http.Response) error {
			idx, _ := resp.Request.Context().Value(ctxIndex).(int)
			switch {
			case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
				cfg.Pool.Disable(idx, pool.CooldownUnauthorized, strconv.Itoa(resp.StatusCode))
				resp.StatusCode = http.StatusBadGateway
				resp.Status = "502 Bad Gateway"
			case resp.StatusCode >= 500:
				cfg.Pool.Disable(idx, pool.CooldownUpstream, strconv.Itoa(resp.StatusCode))
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if idx, ok := r.Context().Value(ctxIndex).(int); ok {
				cfg.Pool.Disable(idx, pool.CooldownUpstream, "transport")
			}
			cfg.Logger.Warn("upstream error", "err", err.Error())
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
		},
		FlushInterval: -1,
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, cfg.ProxyToken) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}
		_ = r.Body.Close()
		rewritten, err := toolid.RewriteBody(body)
		if err != nil {
			rewritten = body
		}
		sess := session.Key(r.Header, rewritten)
		rewritten = compat.Sanitize(rewritten)
		rewritten = session.InjectPromptCacheKey(rewritten, sess)
		idx, key, ok := cfg.Pool.Pick(sess)
		if !ok {
			http.Error(w, "no available key", http.StatusServiceUnavailable)
			return
		}
		fp := session.Fingerprint(sess)
		cfg.Logger.Info("proxy",
			"path", r.URL.Path,
			"session_fp", fp,
			"key_index", idx,
			"stream", strings.Contains(string(rewritten), `"stream":true`),
		)
		r = r.WithContext(context.WithValue(r.Context(), ctxIndex, idx))
		r.Body = io.NopCloser(bytes.NewReader(rewritten))
		r.ContentLength = int64(len(rewritten))
		r.Header.Set("Content-Length", strconv.Itoa(len(rewritten)))
		r.Header.Set("Authorization", "Bearer "+key)
		start := time.Now()
		rw := &statusWriter{ResponseWriter: w, status: 200}
		rp.ServeHTTP(rw, r)
		cfg.Logger.Info("done",
			"session_fp", fp,
			"key_index", idx,
			"status", rw.status,
			"ms", time.Since(start).Milliseconds(),
		)
	})
	return mux
}

func authorized(r *http.Request, token string) bool {
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return token != "" && got == token
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
