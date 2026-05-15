package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// pgvector recibe el vector como string "[1.0,2.0,...]" (sintaxis del
// tipo). No hay driver-level binding nativo en pgx sin pgvector-go, así
// que formateamos manualmente con %g (precisión razonable, evita
// notación científica fea en logs).
func vectorLiteral(v []float32) string {
	var b strings.Builder
	b.Grow(len(v) * 8)
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(f), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// CreateKBDocument inserta un documento en estado pending. El pipeline
// de ingest (background) lo actualizará a processing → ready/failed.
func (s *Store) CreateKBDocument(ctx context.Context, d KBDocument) (KBDocument, error) {
	if d.ID == "" {
		d.ID = newID("kbdoc")
	}
	if d.Status == "" {
		d.Status = "pending"
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO kb_documents (id, tenant_id, name, mime_type, size_bytes, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at`,
		d.ID, d.TenantID, d.Name, d.MimeType, d.SizeBytes, d.Status).
		Scan(&d.CreatedAt, &d.UpdatedAt)
	return d, err
}

// MarkKBDocumentStatus mueve el estado del documento (processing/ready/failed).
// chunkCount solo aplica cuando newStatus = "ready".
func (s *Store) MarkKBDocumentStatus(ctx context.Context, id, newStatus, errMsg string, chunkCount int) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE kb_documents
		SET status = $2, error = $3, chunk_count = $4, updated_at = now()
		WHERE id = $1`, id, newStatus, errMsg, chunkCount)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListKBDocuments(ctx context.Context, tenantID string) ([]KBDocument, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, mime_type, size_bytes, status, error, chunk_count, created_at, updated_at
		FROM kb_documents WHERE tenant_id = $1
		ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []KBDocument{}
	for rows.Next() {
		var d KBDocument
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.MimeType, &d.SizeBytes,
			&d.Status, &d.Error, &d.ChunkCount, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetKBDocument(ctx context.Context, tenantID, id string) (KBDocument, error) {
	var d KBDocument
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, mime_type, size_bytes, status, error, chunk_count, created_at, updated_at
		FROM kb_documents WHERE tenant_id = $1 AND id = $2`, tenantID, id).
		Scan(&d.ID, &d.TenantID, &d.Name, &d.MimeType, &d.SizeBytes,
			&d.Status, &d.Error, &d.ChunkCount, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	return d, err
}

// DeleteKBDocument cascade-borra los chunks via ON DELETE CASCADE.
func (s *Store) DeleteKBDocument(ctx context.Context, tenantID, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM kb_documents WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// InsertKBChunks inserta una tanda de chunks en una transacción. Si
// algo falla, ningún chunk queda persistido — el documento queda en
// processing/failed y se puede reintentar el ingest.
func (s *Store) InsertKBChunks(ctx context.Context, chunks []KBChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, c := range chunks {
		if c.ID == "" {
			c.ID = newID("kbch")
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO kb_chunks (id, tenant_id, document_id, chunk_index, content, content_tokens, embedding)
			VALUES ($1, $2, $3, $4, $5, $6, $7::vector)`,
			c.ID, c.TenantID, c.DocumentID, c.ChunkIndex, c.Content, c.Tokens, vectorLiteral(c.Embedding)); err != nil {
			return fmt.Errorf("insert kb_chunk #%d: %w", c.ChunkIndex, err)
		}
	}
	return tx.Commit(ctx)
}

// SearchKBChunks devuelve los top-K chunks más similares semánticamente
// a la queryEmbedding, scoped por tenant. Usa cosine distance (<=> en
// pgvector). minScore filtra hits débiles (<0.5 suele ser ruido).
func (s *Store) SearchKBChunks(ctx context.Context, tenantID string, queryEmbedding []float32, k int, minScore float64) ([]KBSearchHit, error) {
	if k <= 0 || k > 20 {
		k = 5
	}
	rows, err := s.pool.Query(ctx, `
		SELECT c.content, COALESCE(d.name, ''), 1 - (c.embedding <=> $2::vector) AS score
		FROM kb_chunks c
		LEFT JOIN kb_documents d ON d.id = c.document_id
		WHERE c.tenant_id = $1
		ORDER BY c.embedding <=> $2::vector
		LIMIT $3`, tenantID, vectorLiteral(queryEmbedding), k)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []KBSearchHit{}
	for rows.Next() {
		var h KBSearchHit
		if err := rows.Scan(&h.Chunk, &h.Document, &h.Score); err != nil {
			return nil, err
		}
		if h.Score < minScore {
			continue
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
