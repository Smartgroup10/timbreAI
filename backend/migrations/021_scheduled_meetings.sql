-- 021_scheduled_meetings.sql
-- Registro local de reuniones programadas vía calendar_schedule_meeting.
-- Sin esto no podemos validar ownership cuando el lead pide cancelar o
-- mover su cita: necesitamos saber qué event_id de Google pertenece
-- a qué lead/teléfono.
--
-- La identidad la atamos a DOS columnas — lead_id (preferido) y
-- lead_phone (fallback). El segundo cubre llamadas inbound de leads
-- que aún no están en la tabla `leads`, o el caso "perdí lead_id por
-- una migración chunga".

CREATE TABLE IF NOT EXISTS scheduled_meetings (
  id              text PRIMARY KEY,
  tenant_id       text NOT NULL,
  bot_id          text NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
  -- Lead que originó la cita. Puede ser NULL si la cita salió de un
  -- test call manual sin lead asociado.
  lead_id         text REFERENCES leads(id) ON DELETE SET NULL,
  -- E.164 del lead — clave de fallback para identificar al dueño cuando
  -- vuelve a llamar. Lo guardamos siempre, incluso con lead_id presente.
  lead_phone      text NOT NULL DEFAULT '',
  provider        text NOT NULL DEFAULT 'google',
  -- event_id en el sistema del provider (id de Google calendar event).
  -- Combinado con (provider, calendar_id) identifica únicamente el evento.
  provider_event_id text NOT NULL,
  calendar_id     text NOT NULL DEFAULT 'primary',
  html_link       text NOT NULL DEFAULT '',
  title           text NOT NULL DEFAULT '',
  start_at        timestamptz NOT NULL,
  end_at          timestamptz NOT NULL,
  attendee_email  text NOT NULL DEFAULT '',
  -- 'scheduled' | 'cancelled' — ESTADO LOCAL, no fuente de verdad. Si
  -- el comercial borra el evento desde su Google, nuestra fila queda
  -- en 'scheduled' pero el evento ya no existe. La tool de reschedule/
  -- cancel maneja 404 de Google → marca local cancelled.
  status          text NOT NULL DEFAULT 'scheduled',
  -- Para auditoría: qué llamada lo creó. Útil para "dame el resumen
  -- de la llamada que generó esta cita".
  created_call_id text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT scheduled_meetings_status_check CHECK (status IN ('scheduled', 'cancelled'))
);

-- Lookup principal de ownership: dame las citas activas de este lead
-- (por id o por phone) en este tenant. Index compuesto soporta ambos
-- paths.
CREATE INDEX IF NOT EXISTS idx_scheduled_meetings_owner
  ON scheduled_meetings(tenant_id, lead_id, status) WHERE status = 'scheduled';
CREATE INDEX IF NOT EXISTS idx_scheduled_meetings_phone
  ON scheduled_meetings(tenant_id, lead_phone, status) WHERE status = 'scheduled';

-- Migration de constraint: añade las nuevas action_types calendar_*.
ALTER TABLE bot_tools DROP CONSTRAINT IF EXISTS bot_tools_action_type_check;
ALTER TABLE bot_tools
  ADD CONSTRAINT bot_tools_action_type_check CHECK (action_type IN
    ('set_lead_outcome', 'set_lead_status', 'schedule_callback',
     'webhook', 'end_call', 'transfer_human', 'search_knowledge_base',
     'calendar_check_availability', 'calendar_schedule_meeting',
     'calendar_list_my_meetings', 'calendar_cancel_meeting',
     'calendar_reschedule_meeting'));
