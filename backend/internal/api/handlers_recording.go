package api

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"timbre/backend/internal/store"
)

// handleInternalRecording is called by the voice-agent at session close. The body is the raw WAV
// audio of the conversation. The backend stores it in MinIO and writes the public URL back to
// calls.recording_url.
//
// URL: POST /api/internal/voice/recordings?sessionId=<uuid>
// Body: audio/wav (binary)
// Auth: X-Internal-Secret header (same as transcripts webhook).
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
	}

	key := "recordings/" + call.TenantID + "/" + call.ID + "." + ext
	if _, err := s.storage.PutObject(r.Context(), key, body, contentType); err != nil {
		s.logger.Error("recording put", "error", err)
		writeError(w, http.StatusBadGateway, "upload_failed")
		return
	}
	// Presigned URL valid for 7 days. The UI uses this directly in <audio>.
	url, err := s.storage.PresignGet(key, 7*24*time.Hour)
	if err != nil {
		s.logger.Error("recording presign", "error", err)
		writeError(w, http.StatusInternalServerError, "presign_failed")
		return
	}
	if err := s.store.SetCallRecording(r.Context(), call.TenantID, call.ID, url); err != nil {
		s.logger.Warn("recording persist", "error", err)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"recordingUrl": url, "bytes": len(body)})
}
