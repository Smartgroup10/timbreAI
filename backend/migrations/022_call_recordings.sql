-- 022_call_recordings.sql
-- Tabla dedicada para grabaciones de llamadas. Antes guardábamos solo
-- `calls.recording_url` con una presigned URL de 7 días → a los 7 días
-- los audios morían en la UI. Ahora persistimos:
--   - storage_key: ruta del objeto en MinIO (no expira)
--   - size_bytes / duration_sec: para mostrar y calcular quota
--   - deleted_at: soft delete (cumplimiento GDPR — el operador puede
--     pedir borrar la grabación sin perder el resto del histórico de
--     la call)
--   - retention_due_at: cuándo debe ser borrada automáticamente según
--     la política del tenant. Lo calculamos al crearla y el worker
--     compara con now().
--
-- Permitimos múltiples grabaciones por call (status="archived" cuando
-- llega una nueva) para soportar más adelante grabaciones chunked o
-- re-grabar la misma llamada en otro formato.

CREATE TABLE IF NOT EXISTS call_recordings (
  id              text PRIMARY KEY,
  call_id         text NOT NULL REFERENCES calls(id) ON DELETE CASCADE,
  tenant_id       text NOT NULL,
  storage_key     text NOT NULL,
  content_type    text NOT NULL DEFAULT 'audio/wav',
  size_bytes      bigint NOT NULL DEFAULT 0,
  duration_sec    int NOT NULL DEFAULT 0,
  status          text NOT NULL DEFAULT 'available',
  -- soft delete por GDPR / petición del operador. Cuando deleted_at no es
  -- NULL, el listing en UI lo omite y el objeto en MinIO también ha sido
  -- (o será) borrado por el worker. Mantenemos la fila para auditar.
  deleted_at      timestamptz,
  -- TTL automático según política del tenant. NULL = sin retención
  -- (conservar indefinido). El worker borra storage + marca deleted_at
  -- cuando retention_due_at <= now().
  retention_due_at timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT call_recordings_status_check CHECK (status IN ('available', 'archived'))
);

CREATE INDEX IF NOT EXISTS idx_call_recordings_call
  ON call_recordings(call_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_call_recordings_tenant
  ON call_recordings(tenant_id, created_at DESC) WHERE deleted_at IS NULL;
-- Crítico para el worker de retención: scan secuencial de candidatos a
-- borrar cada hora. Index parcial para que sea barato.
CREATE INDEX IF NOT EXISTS idx_call_recordings_retention
  ON call_recordings(retention_due_at) WHERE deleted_at IS NULL AND retention_due_at IS NOT NULL;

-- Política de retención por tenant. 0 = no borrar nunca.
-- Default 90 días = razonable para cumplimiento + revisión de calidad.
ALTER TABLE tenant_settings
  ADD COLUMN IF NOT EXISTS recording_retention_days int NOT NULL DEFAULT 90;
