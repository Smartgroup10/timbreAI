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

	"timbre/backend/internal/api"
	"timbre/backend/internal/app"
	"timbre/backend/internal/ari"
	"timbre/backend/internal/auth"
	"timbre/backend/internal/config"
	"timbre/backend/internal/db"
	"timbre/backend/internal/storage"
	"timbre/backend/internal/store"
	"timbre/backend/internal/voiceagent"
	"timbre/backend/internal/worker"
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

	st := store.New(pool, cfg.SecretsMasterKey)

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

	// Worker dispatcher: marca campaign_leads como llamables, valida elegibility,
	// origina llamadas y aplica un semáforo per-campaña (max_concurrent).
	dialDeps := app.DialDeps{
		Store:      st,
		ARI:        ariClient,
		VoiceAgent: voiceClient,
		Cfg:        cfg,
		Logger:     logger,
	}
	w := worker.New(dialDeps, 30*time.Second)
	go w.Run(rootCtx)

	// ARI event loop. El handler libera slots del worker cuando un canal se
	// destruye (caller cuelga, fail, timeout, etc.).
	if ariClient != nil {
		handler := app.MakeARIHandler(st, ariClient, voiceClient, cfg.ExternalMedia.Format, w.ReleaseSlot, logger)
		go func() {
			if err := ariClient.RunEventLoop(rootCtx, handler); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("ari loop stopped", "error", err)
			}
		}()
	}

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
	// First boot — refuse to create users with empty or insecure passwords. Operator must set
	// BOOTSTRAP_*_PASSWORD before the first start. This is the only place we'd ever provision
	// real credentials, so we fail fast instead of seeding well-known defaults.
	if len(cfg.BootstrapAdmin.Password) < 8 || len(cfg.BootstrapTenant.Password) < 8 {
		return errors.New("BOOTSTRAP_ADMIN_PASSWORD and BOOTSTRAP_TENANT_PASSWORD must be set (min 8 chars) before first boot")
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
	// Upsert del tenant para que la FK del usuario owner no falle en fresh installs
	// (antes esto se hacía en 002_seed.sql con un INSERT estático). Ahora el nombre
	// sale de BOOTSTRAP_TENANT_NAME, configurable por el operador.
	tenantName := cfg.BootstrapTenant.Name
	if tenantName == "" || tenantName == "Tenant Owner" {
		tenantName = tenantID
	}
	if err := st.EnsureTenant(ctx, tenantID, tenantName); err != nil {
		return err
	}
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
