-- 016_webhooks.sql
-- Webhooks salientes para integrar con CRMs externos (HubSpot, Salesforce,
-- Pipedrive, Make, n8n…). El operador define una URL y una lista de
-- eventos a los que se suscribe; el backend dispatcha en background con
-- firma HMAC-SHA256 en cabecera X-Timbre-Signature.
--
-- Eventos soportados (validados en backend):
--   - call.completed       → al cerrar una llamada
--   - call.qualified       → cuando outcome = qualified
--   - lead.status_changed  → cuando un lead cambia de estado
--   - tool.invoked         → cuando el bot llama a una tool

CREATE TABLE IF NOT EXISTS webhook_endpoints (
  id          text PRIMARY KEY,
  tenant_id   text NOT NULL,
  name        text NOT NULL,
  url         text NOT NULL,
  -- secret de firma. Se genera al crear y se muestra UNA SOLA VEZ — quien lo
  -- pierda se va a "regenerar secret". Almacenado en claro: estamos del lado
  -- del emisor de la firma, no de quien la verifica.
  secret      text NOT NULL,
  events      jsonb NOT NULL DEFAULT '[]'::jsonb,
  active      boolean NOT NULL DEFAULT true,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_tenant
  ON webhook_endpoints(tenant_id, active);

-- Log de cada entrega. Útil para auditar y debugging cuando un cliente
-- nos dice "no me llegan los webhooks". Conservamos response_status e
-- intentos pero no el payload completo (el evento se reconstruye desde
-- la tabla origen — call_id, lead_id…).
CREATE TABLE IF NOT EXISTS webhook_deliveries (
  id              text PRIMARY KEY,
  tenant_id       text NOT NULL,
  endpoint_id     text REFERENCES webhook_endpoints(id) ON DELETE SET NULL,
  event_type      text NOT NULL,
  payload         jsonb NOT NULL,
  status_code     int NOT NULL DEFAULT 0,
  error           text NOT NULL DEFAULT '',
  attempt         int NOT NULL DEFAULT 1,
  delivered_at    timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_tenant_time
  ON webhook_deliveries(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_endpoint
  ON webhook_deliveries(endpoint_id);
