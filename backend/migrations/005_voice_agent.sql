-- Voice-agent integration. Each bot picks one realtime voice provider; each call records the
-- voice-agent session id so we can correlate ARI events with conversation transcripts.
ALTER TABLE bots ADD COLUMN IF NOT EXISTS voice_provider TEXT NOT NULL DEFAULT 'echo';
ALTER TABLE calls ADD COLUMN IF NOT EXISTS voice_session_id TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_calls_voice_session ON calls(voice_session_id) WHERE voice_session_id <> '';
