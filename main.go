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

	"go-unit-mangement/frontend"
	"go-unit-mangement/internal/auth"
	"go-unit-mangement/internal/config"
	"go-unit-mangement/internal/database"
	"go-unit-mangement/internal/server"
	"go-unit-mangement/internal/units"
	"go-unit-mangement/internal/version"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		return err
	}

	authService := auth.NewService(db, cfg.SessionTTL)
	created, err := authService.EnsureAdmin(ctx, cfg.AdminUsername, cfg.AdminPassword)
	if err != nil {
		return err
	}
	if created {
		slog.Info("created initial admin user", "username", cfg.AdminUsername)
	}
	promoted, err := authService.PromoteAdminIfNone(ctx, cfg.AdminUsername)
	if err != nil {
		return err
	}
	if promoted {
		slog.Warn("no administrator existed; granted the role to ADMIN_USERNAME", "username", cfg.AdminUsername)
	}

	var oidcProvider *auth.OIDCProvider
	if cfg.OIDCEnabled() {
		discoverCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		oidcProvider, err = auth.NewOIDCProvider(discoverCtx, cfg.OIDC)
		cancel()
		if err != nil {
			return err
		}
		slog.Info("sso enabled", "issuer", cfg.OIDC.IssuerURL)
	}
	go authService.CleanupExpiredSessions(ctx, time.Hour)

	app := server.New(cfg, authService, oidcProvider, units.NewService(db))
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           app.Handler(frontend.Dist()),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.Addr, "version", version.Version, "commit", version.Commit)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return errors.Join(srv.Shutdown(shutdownCtx), app.CloseStreams(shutdownCtx))
}
