package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"warehouse/backend/internal/app"
	"warehouse/backend/internal/platform/config"
)

func main() { os.Exit(run()) }

func run() int {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		return healthcheck()
	}
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration invalid", "reason", err.Error())
		return 1
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, cfg, logger); err != nil {
		logger.Error("application failed", "reason", err.Error())
		return 1
	}
	return 0
}

// Container probes reuse the binary, avoiding curl/wget runtime dependencies.
func healthcheck() int {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 1
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	var body struct {
		Success bool `json:"success"`
	}
	if resp.StatusCode != 200 || json.NewDecoder(resp.Body).Decode(&body) != nil || !body.Success {
		return 1
	}
	return 0
}
