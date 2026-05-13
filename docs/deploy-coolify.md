# Despliegue en Coolify

timbre.ai se despliega como una sola aplicación Docker Compose con 7 servicios. **Solo el
`frontend` tiene dominio público** — el resto vive en la red Docker interna. El Next.js del
frontend hace de reverse proxy: `/api/*` se reenvía al backend y `/storage/*` a MinIO sin pasar
por Traefik.

## Topología en producción

```
                          ┌────────────────────────┐
                          │ Traefik (Coolify)      │
                          │ portal.tudominio.com   │
                          └────────────┬───────────┘
                                       │ HTTPS
                                       ▼
                                  ┌─────────┐
                                  │ frontend│  Next.js (reverse proxy)
                                  │  :3000  │
                                  └────┬────┘
                                       │  Docker network (interno, sin TLS)
              ┌────────────────────────┼────────────────────────┐
              ▼                        ▼                        ▼
        backend :8080            minio :9000             (resto interno)
        + voice-agent :8090                              postgres, redis,
                                                         asterisk :8088
                                  ┌────────────────────────────────────┐
                                  │ UDP directo al host (no via Traefik)│
                                  ├────────────────────────────────────┤
                                  │ asterisk    5060/udp  (SIP)        │
                                  │ asterisk    10000-10020/udp (RTP)  │
                                  │ voice-agent 12000-12099/udp (RTP)  │
                                  └────────────────────────────────────┘
```

**Ventajas**:
- Una sola URL pública → un solo certificado, sin CORS, sin `NEXT_PUBLIC_API_URL` bakeada.
- Backend, voice-agent y MinIO inalcanzables desde internet (defensa en profundidad).
- Rotar el dominio no requiere rebuild del frontend.

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
6. **Domains**: asigna un único hostname al servicio `frontend` (ej.
   `portal.tudominio.com`). NO asignes dominio a `backend`, `voice-agent` ni `minio` — el
   frontend Next los proxea por la red interna.
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
| `BOOTSTRAP_ADMIN_PASSWORD`, `BOOTSTRAP_TENANT_PASSWORD` | Sin default. El backend se niega a arrancar si no están seteadas (mín. 8 chars) |
| `STORAGE_PUBLIC_URL` | Deja **vacío** salvo que uses S3 externo o un dominio dedicado para grabaciones. Vacío = el backend devuelve `/storage/...` y Next lo proxea. |
| `ALLOWED_ORIGINS` | Puede quedar **vacío** en prod (single-domain = same-origin, sin CORS). Solo si pruebas el backend desde otro host. |

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
# Healthz va a través del frontend (Next reescribe a backend)
curl https://portal.tudominio.com/healthz
# {"ariEnabled":true,"status":"ok","time":"...","version":"0.1.0"}

# Login
curl -X POST https://portal.tudominio.com/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@timbre.ai","password":"<el que pusiste>"}'
```
