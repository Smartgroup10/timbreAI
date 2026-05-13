# Despliegue en Coolify

timbre.ai se despliega como una sola aplicación Docker Compose con 7 servicios. Coolify se encarga
de Traefik (TLS + routing por hostname) para los servicios HTTP; los puertos SIP/RTP (UDP) se
exponen directamente al host.

## Topología en producción

```
                          ┌────────────────────────┐
                          │ Traefik (Coolify)      │
                          │ portal.tudominio.com   │
                          │ api.tudominio.com      │
                          │ voice.tudominio.com    │
                          │ storage.tudominio.com  │
                          └────────────┬───────────┘
                                       │ HTTPS
       ┌───────────────┬───────────────┼───────────────┬────────────────┐
       ▼               ▼               ▼               ▼                ▼
   frontend        backend        voice-agent       minio           (interno)
   :3000           :8080          :8090             :9000           postgres, redis
                                                                    asterisk
                                  ┌────────────────────────────────────┐
                                  │ UDP directo al host (no via Traefik)│
                                  ├────────────────────────────────────┤
                                  │ asterisk    5060/udp  (SIP)        │
                                  │ asterisk    10000-10020/udp (RTP)  │
                                  │ voice-agent 12000-12099/udp (RTP)  │
                                  └────────────────────────────────────┘
```

## Recursos mínimos

- 2 vCPU, 4 GB RAM, 40 GB disco para sandbox/pre-prod.
- 4 vCPU, 8 GB RAM si hay > 50 llamadas concurrentes (cada conversación cuesta CPU por la
  conversión PCM/RTP en el voice-agent y por el LLM remoto).

Cuando crezca, separar Asterisk en un VPS dedicado con IP pública estable (los proveedores SIP
prefieren IP fija para identify).

## Pasos en Coolify

1. **Push** del repo a GitHub: `https://github.com/Smartgroup10/timbreAI`
2. **New resource** → **Application** → **Public Repository** o conectar GitHub directamente
3. **Build pack**: Docker Compose
4. **Compose file**: `docker-compose.yml`
5. **Environment variables**: pega el bloque de la sección "Variables clave" del [README](../README.md#variables-clave-para-coolify)
6. **Domains**: añade los 4 hostnames y mapéalos a sus servicios
   - `FRONTEND_HOST` → `frontend`
   - `BACKEND_HOST` → `backend`
   - `VOICE_AGENT_HOST` → `voice-agent`
   - `MINIO_HOST` → `minio`
7. **Open ports** en la sección de network del nodo Coolify:
   - 5060/udp (SIP, solo si vas a recibir llamadas inbound)
   - 10000-10020/udp (RTP del proveedor SIP)
   - 12000-12099/udp (RTP del External Media)
8. **Deploy**

## Variables que SÍ o SÍ debes cambiar

| Variable | Por qué |
|---|---|
| `JWT_SECRET` | El default rechaza el arranque. `openssl rand -base64 48` |
| `VOICE_AGENT_SHARED_SECRET` | Sin esto, los endpoints internos están abiertos a cualquiera que llegue a la red docker |
| `STORAGE_SECRET_KEY` | Clave MinIO. Cambia también `STORAGE_ACCESS_KEY` |
| `POSTGRES_PASSWORD` y `DATABASE_URL` | Default = `change-me` |
| `BOOTSTRAP_ADMIN_PASSWORD`, `BOOTSTRAP_TENANT_PASSWORD` | Default = `atrium123`. Cámbialos antes del primer arranque |
| `STORAGE_PUBLIC_URL` | Debe ser HTTPS pública (`https://storage.tudominio.com`) para que el navegador pueda reproducir grabaciones |
| `ALLOWED_ORIGINS` | Lista exacta de orígenes que pueden hacer CORS al backend. En prod: `https://portal.tudominio.com` |

## Voice agent y proveedores

El voice-agent registra solo los proveedores que tienen API key configurada al boot:

- `OPENAI_API_KEY` → habilita `openai_realtime` y la rama LLM por defecto de Deepgram/AssemblyAI
- `DEEPGRAM_API_KEY` → habilita `deepgram` (Nova-3 ASR + LLM via OpenAI + Aura TTS)
- `ASSEMBLYAI_API_KEY` → habilita `assemblyai` (Universal Streaming + LLM via OpenAI + OpenAI TTS)
- Sin keys → solo `echo` (sandbox)

El bot elige su provider desde `/portal/bots` → editar → "Provider de voz".

## Asterisk en producción

El contenedor `asterisk` corre Asterisk 22 con módulos PJSIP y ARI. Lo que **debes** hacer
antes de exponerlo:

1. Cambiar credenciales ARI (`asterisk/etc/asterisk/ari.conf` + `ASTERISK_ARI_USER/PASSWORD`)
2. Configurar al menos un trunk SIP en `asterisk/etc/asterisk/pjsip.conf` (hay 3 plantillas
   comentadas para Twilio, Vonage y Telnyx)
3. Restringir el puerto 8088 al host de backend (`network_mode: bridge` ya lo aísla; verifica
   el firewall del nodo)
4. Habilitar ARI desde el backend: `ASTERISK_ARI_ENABLED=true`
5. Reload de Asterisk tras cambios en pjsip.conf: `docker compose exec asterisk asterisk -rx "pjsip reload"`

## Storage (MinIO) en producción

MinIO funciona perfecto para sandbox y producción ligera. Para volúmenes serios, sustituye por
S3 real (Backblaze B2, R2, Wasabi) cambiando solo:

```bash
STORAGE_ENDPOINT=https://s3.us-east-1.amazonaws.com
STORAGE_ACCESS_KEY=AKIA...
STORAGE_SECRET_KEY=...
STORAGE_BUCKET=callhub-prod-recordings
STORAGE_PUBLIC_URL=https://callhub-prod-recordings.s3.us-east-1.amazonaws.com
```

El cliente S3 del backend es signature-v4 nativo, no SDK pesado: cualquier endpoint S3-compatible
funciona.

## Rollbacks

Coolify guarda imágenes anteriores. Para revertir, vuelve al deploy anterior en la UI. Las
migraciones de DB no se revierten automáticamente — si una migración rompió algo, hay que
hacer rollback manual con un script SQL antes de redeployar la versión anterior.

## Healthchecks y zero-downtime

Todos los servicios HTTP tienen healthcheck. Coolify hace rolling restart respetando los checks,
así que las migraciones corren antes de que la versión nueva pase a "healthy".

Para que la conversación no se interrumpa en deploys, el voice-agent **no** persiste sesiones
entre restarts (las llamadas activas se cortan). Si necesitas zero-downtime para llamadas en
curso, hay que duplicar la instancia y orquestar el drenaje — fuera del MVP.

## Primer test post-deploy

```bash
curl https://api.tudominio.com/healthz
# {"ariEnabled":true,"status":"ok","time":"...","version":"0.1.0"}

# Login
curl -X POST https://api.tudominio.com/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@timbre.ai","password":"<el que pusiste>"}'
```
