package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"timbre/backend/internal/store"
)

// handleInternalRecording recibe el WAV del voice-agent al cierre de la
// sesión. Lo sube a MinIO y persiste metadata en call_recordings con
// retention_due_at calculado desde tenant_settings.recording_retention_days.
//
// IMPORTANTE: ya NO devolvemos una presigned URL como `recording_url`.
// Esa URL caduca a los 7 días y rompe el detalle de llamada. Ahora
// persistimos solo el storage_key; la UI pide la URL fresca via
// /api/calls/:id/recording cada vez que la necesita.
func (s *Server) handleInternalRecording(w http.ResponseWriter, r *http.Request) {
	if s.storage == nil || !s.storage.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "storage_not_configured")
		return
	}
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_required")
		return
	}
	call, err := s.store.FindCallByVoiceSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "session_not_linked")
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 50*1024*1024)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_failed")
		return
	}
	if len(body) == 0 {
		writeError(w, http.StatusBadRequest, "empty_body")
		return
	}
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "audio/wav"
	}
	ext := "wav"
	if strings.Contains(contentType, "mpeg") {
		ext = "mp3"
	} else if strings.Contains(contentType, "ogg") {
		ext = "ogg"
	} else if strings.Contains(contentType, "opus") {
		ext = "opus"
	}

	// La key incluye el timestamp para no colisionar si re-grabamos la
	// misma call (rare pero posible — testing, re-encoding).
	key := "recordings/" + call.TenantID + "/" + call.ID + "-" + strconv.FormatInt(time.Now().Unix(), 10) + "." + ext
	if _, err := s.storage.PutObject(r.Context(), key, body, contentType); err != nil {
		s.logger.Error("recording put", "error", err)
		writeError(w, http.StatusBadGateway, "upload_failed")
		return
	}

	// Retention de tenant. 0 = guardar indefinido (NULL en BD).
	retentionDays := 0
	if ts, err := s.store.GetTenantSettings(r.Context(), call.TenantID); err == nil {
		retentionDays = ts.RecordingRetentionDays
	}

	duration := call.DurationSec
	rec, err := s.store.CreateCallRecording(r.Context(), store.CallRecording{
		CallID:      call.ID,
		TenantID:    call.TenantID,
		StorageKey:  key,
		ContentType: contentType,
		SizeBytes:   int64(len(body)),
		DurationSec: duration,
	}, retentionDays)
	if err != nil {
		s.logger.Error("create call_recording", "error", err)
		// El objeto ya está en MinIO; lo limpiará el operador o el next
		// retention worker pass (huérfanos detectables por SELECT key
		// que no esté en BD).
		writeError(w, http.StatusInternalServerError, "persist_failed")
		return
	}

	// Mantenemos calls.recording_url por compatibilidad con clientes
	// viejos, pero apuntamos a nuestro endpoint de presigned-on-demand
	// para que nunca caduque silenciosamente. Path relativo, el frontend
	// lo prefija con la API base.
	apiURL := "/api/calls/" + call.ID + "/recording"
	if err := s.store.SetCallRecording(r.Context(), call.TenantID, call.ID, apiURL); err != nil {
		s.logger.Warn("recording persist legacy url", "error", err)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"recordingId": rec.ID,
		"bytes":       len(body),
	})
}

// handleGetCallRecording devuelve presigned URL fresca + metadata para
// la grabación de una call. El frontend la llama justo antes de mostrar
// el <audio> — así las URLs nunca caducan en pantalla.
func (s *Server) handleGetCallRecording(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	callID := r.PathValue("id")
	rec, err := s.store.GetActiveRecordingForCall(r.Context(), tenantID, callID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "no_recording")
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	if s.storage == nil || !s.storage.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "storage_not_configured")
		return
	}
	// 1h es suficiente para una escucha. Si el operador deja la pestaña
	// abierta más tiempo y pulsa play, refresca la página → URL nueva.
	url, err := s.storage.PresignGet(rec.StorageKey, time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "presign_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          rec.ID,
		"url":         url,
		"contentType": rec.ContentType,
		"sizeBytes":   rec.SizeBytes,
		"durationSec": rec.DurationSec,
		"createdAt":   rec.CreatedAt,
		"expiresAt":   time.Now().Add(time.Hour).UTC(),
	})
}

// handleListRecordings es la página "Grabaciones" del portal: listado
// paginado con info de la call (nombre lead, teléfono, outcome) y URL
// presigned al vuelo para reproducir inline.
//
// Query params: ?outcome=qualified&from=2026-01-01&to=2026-02-01&page=2&pageSize=50
func (s *Server) handleListRecordings(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	q := r.URL.Query()
	f := store.CallRecordingListFilter{TenantID: tenantID, Outcome: q.Get("outcome")}
	if v := q.Get("from"); v != "" {
		if d, err := time.Parse("2006-01-02", v); err == nil {
			f.FromDate = &d
		}
	}
	if v := q.Get("to"); v != "" {
		if d, err := time.Parse("2006-01-02", v); err == nil {
			// Sumamos 24h para incluir el día completo.
			d = d.Add(24 * time.Hour)
			f.ToDate = &d
		}
	}
	pageSize := 50
	if v := q.Get("pageSize"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			pageSize = n
		}
	}
	page := 1
	if v := q.Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	f.Limit = pageSize
	f.Offset = (page - 1) * pageSize

	items, total, err := s.store.ListCallRecordings(r.Context(), f)
	if err != nil {
		s.logger.Error("list recordings", "error", err)
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	// Para cada item, generar presigned URL fresca. 1h. Lo hacemos en
	// el server (no lazy en el cliente) porque el cliente puede mostrar
	// 50 elementos pero solo reproducir 1 — peor: hacer 50 round-trips
	// por click. Mejor 50 firmas baratas server-side de una vez.
	type listResponse struct {
		store.CallRecordingListItem
		URL string `json:"url"`
	}
	out := make([]listResponse, 0, len(items))
	for _, i := range items {
		url := ""
		if s.storage != nil && s.storage.Enabled() {
			u, err := s.storage.PresignGet(i.StorageKey, time.Hour)
			if err == nil {
				url = u
			}
		}
		out = append(out, listResponse{CallRecordingListItem: i, URL: url})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":    out,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// handleDeleteRecording soft-deletes la fila Y borra el objeto en
// MinIO inmediatamente. Si el delete remoto falla, dejamos la fila
// marcada y el worker reintenta en el próximo pass.
func (s *Server) handleDeleteRecording(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	rec, err := s.store.GetCallRecordingByID(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "recording_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	if err := s.store.SoftDeleteCallRecording(r.Context(), tenantID, id); err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	// Intentar borrar el objeto inmediatamente. Best-effort — si falla
	// el worker lo recoge después (rec.deleted_at != nil + storage_key
	// pendiente; el worker mira todas las deleted que tengan objeto
	// que aún existe).
	if s.storage != nil && s.storage.Enabled() {
		if err := s.storage.DeleteObject(r.Context(), rec.StorageKey); err != nil {
			s.logger.Warn("delete recording object (will retry async)", "key", rec.StorageKey, "error", err)
		}
	}
	s.audit(r, "recording.delete", "call_recording", id, map[string]any{"callId": rec.CallID})
	w.WriteHeader(http.StatusNoContent)
}

// handleRecordingUsage devuelve cuántas grabaciones tiene el tenant y
// cuántos bytes acumulados. El dashboard lo pinta como stat card.
func (s *Server) handleRecordingUsage(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	u, err := s.store.TenantRecordingUsage(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "usage_failed")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// json marshaller utility (avoid pulling encoding/json everywhere here).
var _ = json.Marshal
