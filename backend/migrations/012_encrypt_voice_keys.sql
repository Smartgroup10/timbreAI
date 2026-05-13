-- Las API keys de proveedores de voz (OpenAI/Deepgram/AssemblyAI) se persisten
-- ahora cifradas con AES-256-GCM (ver backend/internal/secretcrypto). La master
-- key vive en SECRETS_MASTER_KEY (32 bytes base64).
--
-- Sustituimos las columnas TEXT plaintext por BYTEA. Los datos previos en
-- plaintext SE PIERDEN — el operador tiene que volver a meter las keys desde
-- /portal/settings. En este punto del proyecto (trial en prod) es aceptable.
-- Cualquier deploy real haría un migrate manual cifrando primero.

ALTER TABLE tenant_voice_credentials DROP COLUMN IF EXISTS openai_api_key;
ALTER TABLE tenant_voice_credentials DROP COLUMN IF EXISTS deepgram_api_key;
ALTER TABLE tenant_voice_credentials DROP COLUMN IF EXISTS assemblyai_api_key;

ALTER TABLE tenant_voice_credentials ADD COLUMN openai_api_key_enc     BYTEA NOT NULL DEFAULT '';
ALTER TABLE tenant_voice_credentials ADD COLUMN deepgram_api_key_enc   BYTEA NOT NULL DEFAULT '';
ALTER TABLE tenant_voice_credentials ADD COLUMN assemblyai_api_key_enc BYTEA NOT NULL DEFAULT '';
