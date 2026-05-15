-- 018_kb_tool_action.sql
-- Permite action_type = 'search_knowledge_base' en bot_tools.
--
-- Postgres no soporta ADD VALUE en CHECK constraints sin recrear, así
-- que dropeamos la antigua y volvemos a crear con la nueva whitelist.

ALTER TABLE bot_tools DROP CONSTRAINT IF EXISTS bot_tools_action_type_check;

ALTER TABLE bot_tools
  ADD CONSTRAINT bot_tools_action_type_check CHECK (action_type IN
    ('set_lead_outcome', 'set_lead_status', 'schedule_callback',
     'webhook', 'end_call', 'transfer_human', 'search_knowledge_base'));
