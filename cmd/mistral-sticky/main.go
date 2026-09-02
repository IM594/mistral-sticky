package main

import (
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/IM594/mistral-sticky/internal/pool"
	"github.com/IM594/mistral-sticky/internal/proxy"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	listen := getenv("LISTEN", ":8080")
	upstreamRaw := getenv("UPSTREAM", "https://api.mistral.ai")
	keysFile := getenv("KEYS_FILE", "/data/keys.txt")
	cooldownFile := getenv("COOLDOWN_FILE", "/data/cooldown.json")
	token := os.Getenv("PROXY_TOKEN")
	if token == "" {
		log.Error("PROXY_TOKEN is required")
		os.Exit(1)
	}

	upstream, err := url.Parse(upstreamRaw)
	if err != nil {
		log.Error("invalid UPSTREAM", "err", err)
		os.Exit(1)
	}
	p, err := pool.LoadKeys(keysFile)
	if err != nil {
		log.Error("load keys", "err", err)
		os.Exit(1)
	}
	if err := p.SetCooldownFile(cooldownFile); err != nil {
		log.Error("load cooldown", "err", err)
		os.Exit(1)
	}
	log.Info("started", "listen", listen, "keys", p.Size(), "disabled", p.DisabledCount())

	srv := &http.Server{
		Addr:              listen,
		Handler:           proxy.New(proxy.Config{ProxyToken: token, Upstream: upstream, Pool: p, Logger: log}),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
