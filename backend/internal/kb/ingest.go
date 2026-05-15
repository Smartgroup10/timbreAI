package kb

import (
	"context"
	"fmt"
	"log/slog"

	"timbre/backend/internal/store"
)

// Service envuelve el pipeline de ingest + retrieval. El handler HTTP
// recibe el upload, persiste el documento en pending, y delega a
// IngestText en background.
type Service struct {
	store  *store.Store
	logger *slog.Logger
}

func NewService(st *store.Store, logger *slog.Logger) *Service {
	return &Service{store: st, logger: logger}
}

// Ingest procesa un documento ya creado en kb_documents (status=pending):
// chunkea el texto, genera embeddings en batches, y persiste los chunks.
// Si algo falla a mitad, marca el documento failed para que el operador
// pueda reintentar manualmente borrándolo y subiendo otra vez.
//
// embedClient debe estar configurado con la API key del tenant. El
// caller (handler HTTP) resuelve la key — esta función no la conoce.
func (s *Service) Ingest(ctx context.Context, tenantID, documentID, text string, embedClient *EmbeddingsClient) {
	if err := s.store.MarkKBDocumentStatus(ctx, documentID, "processing", "", 0); err != nil {
		s.logger.Warn("kb mark processing", "doc", documentID, "error", err)
		return
	}

	chunks := ChunkText(text, DefaultChunkOptions)
	if len(chunks) == 0 {
		_ = s.store.MarkKBDocumentStatus(ctx, documentID, "failed", "empty_or_unparseable", 0)
		return
	}

	// Procesamos en batches de MaxBatchInputs (100) para no exceder el
	// límite de la API y para que un fallo a mitad solo pierda un
	// batch, no todo el documento.
	persisted := 0
	for start := 0; start < len(chunks); start += MaxBatchInputs {
		end := start + MaxBatchInputs
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[start:end]
		inputs := make([]string, len(batch))
		for i, c := range batch {
			inputs[i] = c.Content
		}
		vecs, _, err := embedClient.Embed(ctx, inputs)
		if err != nil {
			s.logger.Warn("kb embed failed", "doc", documentID, "batch_start", start, "error", err)
			_ = s.store.MarkKBDocumentStatus(ctx, documentID, "failed", fmt.Sprintf("embed: %s", err.Error()), persisted)
			return
		}
		rows := make([]store.KBChunk, len(batch))
		for i, c := range batch {
			rows[i] = store.KBChunk{
				TenantID:   tenantID,
				DocumentID: documentID,
				ChunkIndex: c.Index,
				Content:    c.Content,
				Tokens:     c.Tokens,
				Embedding:  vecs[i],
			}
		}
		if err := s.store.InsertKBChunks(ctx, rows); err != nil {
			s.logger.Warn("kb insert chunks", "doc", documentID, "error", err)
			_ = s.store.MarkKBDocumentStatus(ctx, documentID, "failed", fmt.Sprintf("insert: %s", err.Error()), persisted)
			return
		}
		persisted += len(rows)
	}

	_ = s.store.MarkKBDocumentStatus(ctx, documentID, "ready", "", persisted)
	s.logger.Info("kb ingest complete", "doc", documentID, "chunks", persisted)
}

// Retrieve es la cara de RAG hacia las tools: embedding de la query +
// top-K cosine search. Devuelve formato listo para enviar al LLM.
func (s *Service) Retrieve(ctx context.Context, tenantID, query string, k int, embedClient *EmbeddingsClient) ([]store.KBSearchHit, error) {
	vecs, _, err := embedClient.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("kb retrieve embed: %w", err)
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("kb retrieve: empty embedding response")
	}
	return s.store.SearchKBChunks(ctx, tenantID, vecs[0], k, 0.45)
}
