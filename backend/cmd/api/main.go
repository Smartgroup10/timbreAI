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

	"atrium-calls/backend/internal/api"
	"atrium-calls/backend/internal/app"
	"atrium-calls/backend/internal/ari"
	"atrium-calls/backend/internal/auth"
	"atrium-calls/backend/internal/config"
	"atrium-calls/backend/internal/db"
	"atrium-calls/backend/internal/storage"
	"atrium-calls/backend/internal/store"
	"atrium-calls/backend/internal/voiceagent"
	"atrium-calls/backend/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config", "error", err)
		os.Exit(1)
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(rootCtx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("db connect", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(rootCtx, pool, cfg.MigrationsDir, logger); err != nil {
		logger.Error("db migrate", "error", err)
		os.Exit(1)
	}

	st := store.New(pool)

	if err := bootstrapUsers(rootCtx, st, cfg, logger); err != nil {
		logger.Error("bootstrap users", "error", err)
		os.Exit(1)
	}

	voiceClient := voiceagent.New(cfg.VoiceAgent.URL, cfg.VoiceAgent.Secret)
	if voiceClient.Enabled() {
		logger.Info("voice-agent client wired", "url", cfg.VoiceAgent.URL)
	} else {
		logger.Info("voice-agent disabled; set VOICE_AGENT_URL to enable")
	}

	var ariClient *ari.Client
	if cfg.ARI.Enabled {
		ariClient = ari.New(cfg.ARI.URL, cfg.ARI.User, cfg.ARI.Password, cfg.ARI.App, logger)
		handler := app.MakeARIHandler(st, ariClient, voiceClient, logger)
		go func() {
			if err := ariClient.RunEventLoop(rootCtx, handler); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("ari loop stopped", "error", err)
			}
		}()
	} else {
		logger.Info("ari disabled; set ASTERISK_ARI_ENABLED=true to enable originate")
	}

	storageClient := storage.New(storage.Config{
		Endpoint:  cfg.Storage.Endpoint,
		AccessKey: cfg.Storage.AccessKey,
		SecretKey: cfg.Storage.SecretKey,
		Bucket:    cfg.Storage.Bucket,
		Region:    cfg.Storage.Region,
		PublicURL: cfg.Storage.PublicURL,
	})
	if storageClient.Enabled() {
		if err := storageClient.EnsureBucket(rootCtx); err != nil {
			logger.Warn("storage bucket setup", "error", err)
		} else {
			logger.Info("storage ready", "bucket", cfg.Storage.Bucket, "endpoint", cfg.Storage.Endpoint)
		}
	} else {
		logger.Info("storage disabled; recordings will not persist")
	}

	// Background worker: scans queued calls and enforces eligibility (DNC, hours, cap).
	w := worker.New(st, logger, 30*time.Second)
	go w.Run(rootCtx)

	server := api.New(cfg, st, ariClient, voiceClient, storageClient, logger)
	httpSrv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("api listening", "port", cfg.Port, "ari", cfg.ARI.Enabled)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case <-rootCtx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErr:
		logger.Error("server crashed", "error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown", "error", err)
	}
}

func bootstrapUsers(ctx context.Context, st *store.Store, cfg config.Config, logger *slog.Logger) error {
	n, err := st.CountUsers(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	hash, err := auth.HashPassword(cfg.BootstrapAdmin.Password)
	if err != nil {
		return err
	}
	if err := st.CreateUser(ctx, store.User{
		ID: "usr_admin", Email: cfg.BootstrapAdmin.Email, Name: cfg.BootstrapAdmin.Name,
		Role: cfg.BootstrapAdmin.Role, PasswordHash: hash,
	}); err != nil {
		return err
	}
	tenantHash, err := auth.HashPassword(cfg.BootstrapTenant.Password)
	if err != nil {
		return err
	}
	tenantID := cfg.BootstrapTenant.TenantID
	if err := st.CreateUser(ctx, store.User{
		ID: "usr_owner", TenantID: &tenantID, Email: cfg.BootstrapTenant.Email, Name: cfg.BootstrapTenant.Name,
		Role: cfg.BootstrapTenant.Role, PasswordHash: tenantHash,
	}); err != nil {
		return err
	}
	logger.Info("bootstrap users created", "admin", cfg.BootstrapAdmin.Email, "tenant_owner", cfg.BootstrapTenant.Email)
	return nil
}

// (ARI event handling moved to internal/app/ari_handler.go)
