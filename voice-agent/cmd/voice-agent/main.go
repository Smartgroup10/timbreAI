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

	"timbre/voice-agent/internal/api"
	"timbre/voice-agent/internal/audiosocket"
	"timbre/voice-agent/internal/config"
	"timbre/voice-agent/internal/provider"
	"timbre/voice-agent/internal/rtp"
	"timbre/voice-agent/internal/session"
	"timbre/voice-agent/internal/webhook"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	registry := session.NewRegistry(cfg.SessionTTL)
	providers := provider.NewRegistry(cfg, logger)
	hook := webhook.New(cfg.BackendURL, cfg.BackendAuthKey, logger)
	rtpPool := rtp.NewPool(cfg.RTP.PortStart, cfg.RTP.PortEnd)
	server := api.New(cfg, registry, providers, hook, rtpPool, logger)
	logger.Info("rtp pool ready", "range", cfg.RTP.PortStart, "advertise", cfg.RTP.AdvertiseHost)

	httpSrv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("voice-agent listening", "port", cfg.Port, "providers", providers.Names())
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}()

	// AudioSocket TCP server: Asterisk se conecta aquí por sesión y nos manda
	// audio slin 8 kHz directo, sin RTP ni transcoding.
	if cfg.AudioSocket.Enabled {
		asAddr := "0.0.0.0:" + cfg.AudioSocket.Port
		// Pasamos URL del backend para subir grabaciones cuando la sesión
		// las tenga habilitadas. Sin esto, RecordingEnabled=true se ignora.
		asSrv := audiosocket.New(asAddr, registry, logger).
			WithRecordingUpload(cfg.BackendURL, cfg.BackendAuthKey)
		go func() {
			if err := asSrv.Run(ctx); err != nil {
				logger.Error("audiosocket server stopped", "error", err)
			}
		}()
	} else {
		logger.Info("audiosocket disabled; legacy RTP/External Media path active")
	}

	<-ctx.Done()
	logger.Info("shutdown signal received")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}
