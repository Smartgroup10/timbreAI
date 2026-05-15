-- 020_calendar_tool_actions.sql
-- Permite los nuevos action_types calendar_* en bot_tools.

ALTER TABLE bot_tools DROP CONSTRAINT IF EXISTS bot_tools_action_type_check;

ALTER TABLE bot_tools
  ADD CONSTRAINT bot_tools_action_type_check CHECK (action_type IN
    ('set_lead_outcome', 'set_lead_status', 'schedule_callback',
     'webhook', 'end_call', 'transfer_human', 'search_knowledge_base',
     'calendar_check_availability', 'calendar_schedule_meeting'));
