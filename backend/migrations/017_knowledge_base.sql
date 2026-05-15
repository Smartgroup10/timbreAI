-- 017_knowledge_base.sql
-- RAG (Retrieval-Augmented Generation) por tenant. El bot puede invocar
-- una tool search_knowledge_base(query) y el backend devuelve los chunks
-- más similares semánticamente.
--
-- pgvector es el vector store: misma base de datos, sin servicio extra.
-- Requiere imagen Docker pgvector/pgvector:pg16 (cambio en docker-compose).
--
-- Dimensiones 1536 corresponden a text-embedding-3-small de OpenAI, que
-- es el embedding más barato ($0.02 / 1M tokens) y suficiente para RAG
-- típico. Si en el futuro cambias a otro modelo de dimensiones distintas,
-- hay que migrar la tabla (ALTER COLUMN vector(N)) o crear una nueva
-- con un campo "model" y soportar varios en paralelo.

CREATE EXTENSION IF NOT EXISTS vector;

-- Un documento subido por el operador (PDF, TXT, MD). Lo identificamos
-- para poder borrar todos sus chunks atomicamente.
CREATE TABLE IF NOT EXISTS kb_documents (
  id          text PRIMARY KEY,
  tenant_id   text NOT NULL,
  name        text NOT NULL,
  -- "text/markdown" | "text/plain" | "application/pdf"
  mime_type   text NOT NULL,
  size_bytes  bigint NOT NULL DEFAULT 0,
  -- "pending" | "processing" | "ready" | "failed"
  -- pending: subido pero aún no procesado; processing: chunking + embedding
  -- en curso; ready: listo para retrieval; failed: error en pipeline.
  status      text NOT NULL DEFAULT 'pending',
  error       text NOT NULL DEFAULT '',
  chunk_count int  NOT NULL DEFAULT 0,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_kb_documents_tenant
  ON kb_documents(tenant_id, created_at DESC);

-- Chunk = fragmento de texto + su embedding. Cada chunk pertenece a un
-- documento (cascade delete) y al tenant para que el search pueda
-- filtrar sin join. content_tokens es aproximado (count de palabras)
-- — sirve para tener una idea de tamaño en la UI.
CREATE TABLE IF NOT EXISTS kb_chunks (
  id           text PRIMARY KEY,
  tenant_id    text NOT NULL,
  document_id  text NOT NULL REFERENCES kb_documents(id) ON DELETE CASCADE,
  chunk_index  int  NOT NULL,
  content      text NOT NULL,
  content_tokens int NOT NULL DEFAULT 0,
  embedding    vector(1536) NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now()
);

-- ANN index sobre el embedding con cosine distance. ivfflat es buena
-- elección para colecciones de <1M chunks; si crece mucho hay que pasar
-- a hnsw (postgres 16 + pgvector 0.5+ lo soportan).
--
-- lists = sqrt(rows) es la heurística estándar; para tenants chicos
-- (cientos de chunks) ponemos lists=100 como compromise inicial.
CREATE INDEX IF NOT EXISTS idx_kb_chunks_embedding
  ON kb_chunks USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- Filtro por tenant — el search siempre va scoped por tenant.
CREATE INDEX IF NOT EXISTS idx_kb_chunks_tenant
  ON kb_chunks(tenant_id);
