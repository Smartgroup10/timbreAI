-- Campañas con scheduling explícito y límite de concurrencia.
--
-- start_at NULL = "lanzar inmediatamente cuando status='active'".
-- end_at   NULL = "sin fecha de fin (corre hasta agotar leads)".
-- max_concurrent: cuántas llamadas en vuelo a la vez por esta campaña.
--                 Limitado además por los puertos RTP de Asterisk y por
--                 el plan del proveedor SIP.

ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS start_at        TIMESTAMPTZ;
ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS end_at          TIMESTAMPTZ;
ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS max_concurrent  INTEGER NOT NULL DEFAULT 3;

-- Renombramos cualquier 'scheduled' previo a 'active'. El expander filtraba
-- por 'scheduled' hasta ahora; mantenemos esos datos vivos en el nuevo flujo.
UPDATE campaigns SET status = 'active' WHERE status = 'scheduled';
