package worker

import (
	"context"
	"log/slog"
	"time"

	"timbre/backend/internal/storage"
	"timbre/backend/internal/store"
)

// RetentionWorker borra grabaciones que superaron su política de
// retención. Corre cada hora; en cada pass coge hasta N candidatos
// (cap para no monopolizar la BD ni MinIO), borra el objeto, marca
// la fila como deleted_at.
//
// Es safe ante reinicios: idempotente — el next pass vuelve a coger
// los que quedaron. El borrado en MinIO trata 404 como éxito.
type RetentionWorker struct {
	Store    *store.Store
	Storage  *storage.Client
	Logger   *slog.Logger
	Interval time.Duration // default 1h si <=0
	BatchSize int          // default 100
}

func (w *RetentionWorker) Run(ctx context.Context) {
	interval := w.Interval
	if interval <= 0 {
		interval = time.Hour
	}
	batchSize := w.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	// Primer pass al arrancar, luego cada `interval`. Esto cubre el
	// caso "estuvimos N días apagados y hay backlog grande".
	w.tick(ctx, batchSize)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.tick(ctx, batchSize)
		}
	}
}

func (w *RetentionWorker) tick(ctx context.Context, batchSize int) {
	expired, err := w.Store.ListExpiredRecordings(ctx, batchSize)
	if err != nil {
		w.Logger.Warn("retention list expired", "error", err)
		return
	}
	if len(expired) == 0 {
		return
	}
	deleted := 0
	for _, r := range expired {
		if w.Storage != nil && w.Storage.Enabled() {
			if err := w.Storage.DeleteObject(ctx, r.StorageKey); err != nil {
				w.Logger.Warn("retention delete object", "key", r.StorageKey, "error", err)
				// No marcamos como deleted en BD si el storage falló —
				// dejamos que el próximo pass reintente. Evita inconsistencia.
				continue
			}
		}
		if err := w.Store.SoftDeleteCallRecording(ctx, r.TenantID, r.ID); err != nil {
			w.Logger.Warn("retention mark deleted in DB", "id", r.ID, "error", err)
			continue
		}
		deleted++
	}
	w.Logger.Info("retention pass complete",
		"candidates", len(expired), "deleted", deleted)
}
