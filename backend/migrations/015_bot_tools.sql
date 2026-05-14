-- 015_bot_tools.sql
-- Tools (function calling) que el bot puede invocar durante una llamada.
--
-- El provider de voz (OpenAI Realtime, Deepgram Voice Agent) puede emitir
-- "function call requests" cuando decide que una acción debe ejecutarse.
-- Este esquema almacena la definición de cada tool por bot:
--   - name/description: lo que el LLM lee para decidir cuándo llamar.
--   - parameters_schema: JSON Schema de los argumentos (passthrough al provider).
--   - action_type/action_config: qué hace el backend cuando se invoca.
--
-- Acciones soportadas (action_type):
--   - 'set_lead_outcome'  → marca el lead/call con un outcome dado
--   - 'set_lead_status'   → cambia el status del lead
--   - 'schedule_callback' → marca la llamada como callback con timestamp
--   - 'webhook'           → POST async a una URL externa (CRM)
--   - 'end_call'          → hangup desde el lado del bot
--   - 'transfer_human'    → warm transfer (no implementado todavía; deja registrado el intento)
--
-- action_config es JSON libre con la config específica del tipo (URL,
-- método, valor a setear...). Lo validamos en backend cuando se crea o edita.

CREATE TABLE IF NOT EXISTS bot_tools (
  id              text PRIMARY KEY,
  tenant_id       text NOT NULL,
  bot_id          text NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
  name            text NOT NULL,
  description     text NOT NULL,
  parameters_schema jsonb NOT NULL DEFAULT '{"type":"object","properties":{}}'::jsonb,
  action_type     text NOT NULL,
  action_config   jsonb NOT NULL DEFAULT '{}'::jsonb,
  enabled         boolean NOT NULL DEFAULT true,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT bot_tools_name_unique UNIQUE (bot_id, name),
  CONSTRAINT bot_tools_action_type_check CHECK (action_type IN
    ('set_lead_outcome', 'set_lead_status', 'schedule_callback', 'webhook', 'end_call', 'transfer_human'))
);

CREATE INDEX IF NOT EXISTS idx_bot_tools_bot ON bot_tools(bot_id);
CREATE INDEX IF NOT EXISTS idx_bot_tools_tenant ON bot_tools(tenant_id);

-- Log de invocaciones para auditar y debugging. No es la fuente de
-- verdad de los efectos (esos se ven en leads/calls), pero permite
-- reconstruir "qué le dijo el LLM" si algo va raro.
CREATE TABLE IF NOT EXISTS bot_tool_invocations (
  id          text PRIMARY KEY,
  tenant_id   text NOT NULL,
  call_id     text REFERENCES calls(id) ON DELETE SET NULL,
  bot_tool_id text REFERENCES bot_tools(id) ON DELETE SET NULL,
  tool_name   text NOT NULL,
  arguments   jsonb NOT NULL DEFAULT '{}'::jsonb,
  result      jsonb NOT NULL DEFAULT '{}'::jsonb,
  success     boolean NOT NULL,
  error       text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_bot_tool_invocations_tenant_time
  ON bot_tool_invocations(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_bot_tool_invocations_call
  ON bot_tool_invocations(call_id);
