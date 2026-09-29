package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const shutdownTimeout = 10 * time.Second

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := loadConfig()
	if err != nil {
		logger.Error("configuration", "err", err)
		os.Exit(1)
	}

	logger.Info("starting",
		"name", cfg.AppName,
		"version", cfg.AppVersion,
		"port", cfg.Port,
		"region", cfg.AWSRegion,
		"bucket", cfg.S3Bucket,
		"storage", "s3",
	)

	store, err := newS3Store(context.Background(), cfg.AWSRegion, cfg.S3Bucket)
	if err != nil {
		logger.Error("s3 client", "err", err)
		os.Exit(1)
	}

	tmpl, err := loadTemplates()
	if err != nil {
		logger.Error("templates", "err", err)
		os.Exit(1)
	}

	hostname, err := os.Hostname()
	if err != nil {
		logger.Error("hostname", "err", err)
		hostname = "unknown"
	}

	app := newApp(cfg, store, tmpl, logger, hostname)
	if err := app.listenAndServe(); err != nil {
		logger.Error("server", "err", err)
		os.Exit(1)
	}
}

func (a *App) listenAndServe() error {
	srv := &http.Server{
		Addr:              ":" + a.cfg.Port,
		Handler:           a.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		a.log.Info("listening", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		a.log.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	a.log.Info("shutting down")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	a.log.Info("server shutdown complete")
	return nil
}

func osExit(code int) {
	os.Exit(code)
}
