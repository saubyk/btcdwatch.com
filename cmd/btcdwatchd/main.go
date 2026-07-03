// btcdwatchd is the btcdwatch.com server: a REST (and, in later
// milestones, WebSocket) API over a local btcd node.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"btcdwatch.com/internal/api"
	"btcdwatch.com/internal/chain"
	"btcdwatch.com/internal/config"
	"btcdwatch.com/internal/explorer"
	"btcdwatch.com/internal/node"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "",
		"path to YAML config (default: config.yaml if present)")
	flag.Parse()

	path := *configPath
	if path == "" {
		if _, err := os.Stat("config.yaml"); err == nil {
			path = "config.yaml"
		}
	}

	cfg, err := config.Load(path)
	if err != nil {
		return err
	}

	params, err := chain.ParamsForNetwork(cfg.Node.Network)
	if err != nil {
		return err
	}

	backend, err := node.New(node.Config{
		Host:     cfg.Node.RPCHost,
		User:     cfg.Node.RPCUser,
		Pass:     cfg.Node.RPCPass,
		CertPath: cfg.Node.RPCCert,
	})
	if err != nil {
		return err
	}
	defer backend.Shutdown()

	// Static price until the live price service milestone.
	price := func() (float64, bool) {
		if cfg.Price.StaticUSD > 0 {
			return cfg.Price.StaticUSD, true
		}
		return 0, false
	}

	mempool := explorer.NewMempool(backend)
	svc := explorer.NewService(backend, params, mempool, price)

	server := &http.Server{
		Addr:              cfg.Server.Listen,
		Handler:           api.New(svc, backend, params, cfg.Node.Network),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("btcdwatchd listening",
			"addr", cfg.Server.Listen, "network", cfg.Node.Network)
		if err := server.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {

			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-stop:
		slog.Info("shutting down", "signal", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}
