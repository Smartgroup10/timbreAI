package api

// Knowledge Base — upload, listado, borrado, search debug.
//
// El upload es multipart/form-data (no JSON) porque viene un fichero.
// Solo aceptamos TXT/MD por ahora (Content-Type text/*). PDF se queda
// para iteración futura — necesita parser dedicado.

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"timbre/backend/internal/kb"
	"timbre/backend/internal/store"
)

const (
	// MaxUploadBytes = 2 MB. Documentos típicos de FAQ/inmuebles entran
	// holgados; PDFs grandes los rechazamos hasta tener parser.
	maxKBUploadBytes = 2 * 1024 * 1024
)

func (s *Server) handleListKBDocuments(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	docs, err := s.store.ListKBDocuments(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}
	writeJSON(w, http.StatusOK, docs)
}

func (s *Server) handleDeleteKBDocument(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := r.PathValue("id")
	if err := s.store.DeleteKBDocument(r.Context(), tenantID, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "doc_not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	s.audit(r, "kb.delete", "kb_document", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// handleUploadKBDocument acepta multipart con un campo "file". Crea el
// documento en pending, contesta 202 con el id, y arranca el ingest en
// background.
//
// La API key OpenAI necesaria para embeddings se resuelve del tenant
// (voice_credentials.openai_api_key). Si no hay key, devolvemos 400
// para que la UI muestre "configura OpenAI en Settings primero".
func (s *Server) handleUploadKBDocument(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// API key check antes de leer el body — más rápido para el usuario.
	creds, _ := s.store.GetVoiceCredentials(r.Context(), tenantID)
	if creds.OpenAIAPIKey == "" {
		// La key del tenant es la que paga los embeddings — sin ella
		// no podemos ingestar ni buscar.
		writeError(w, http.StatusBadRequest, "openai_key_required")
		return
	}
	apiKey := creds.OpenAIAPIKey

	// Limit el body antes de leer — defensa contra uploads bomba.
	r.Body = http.MaxBytesReader(w, r.Body, maxKBUploadBytes)
	if err := r.ParseMultipartForm(maxKBUploadBytes); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "file_too_large")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file_required")
		return
	}
	defer file.Close()

	mime := header.Header.Get("Content-Type")
	if mime == "" {
		mime = "text/plain"
	}
	// Solo aceptamos texto plano por ahora. PDF en v2 — necesita pdftotext.
	if !strings.HasPrefix(mime, "text/") {
		writeError(w, http.StatusUnsupportedMediaType, "only_text_supported")
		return
	}

	raw, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read_failed")
		return
	}
	if len(raw) == 0 {
		writeError(w, http.StatusBadRequest, "empty_file")
		return
	}

	doc, err := s.store.CreateKBDocument(r.Context(), store.KBDocument{
		TenantID:  tenantID,
		Name:      header.Filename,
		MimeType:  mime,
		SizeBytes: int64(len(raw)),
		Status:    "pending",
	})
	if err != nil {
		s.logger.Error("create kb document", "error", err)
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	s.audit(r, "kb.upload", "kb_document", doc.ID, map[string]any{
		"name": header.Filename, "size": len(raw), "mime": mime,
	})

	// Ingest en background. El ctx del request se cancela al volver, así
	// que usamos uno propio. text es la copia del body — el reader ya está
	// agotado en este punto.
	go func(text string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*60*1000*1000*1000) // 5 min
		defer cancel()
		ec := kb.NewEmbeddingsClient(apiKey)
		s.kb.Ingest(ctx, tenantID, doc.ID, text, ec)
	}(string(raw))

	writeJSON(w, http.StatusAccepted, doc)
}

// handleKBSearch es un endpoint de debug — permite al operador probar
// retrieval sin tener que esperar a que un bot lo use. La UI lo usa
// para el "Probar búsqueda" del panel.
func (s *Server) handleKBSearch(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "q_required")
		return
	}

	creds, _ := s.store.GetVoiceCredentials(r.Context(), tenantID)
	if creds.OpenAIAPIKey == "" {
		// La key del tenant es la que paga los embeddings — sin ella
		// no podemos ingestar ni buscar.
		writeError(w, http.StatusBadRequest, "openai_key_required")
		return
	}
	apiKey := creds.OpenAIAPIKey

	ec := kb.NewEmbeddingsClient(apiKey)
	hits, err := s.kb.Retrieve(r.Context(), tenantID, q, 5, ec)
	if err != nil {
		s.logger.Warn("kb search", "tenant", tenantID, "error", err)
		writeError(w, http.StatusInternalServerError, "search_failed")
		return
	}
	writeJSON(w, http.StatusOK, hits)
}
