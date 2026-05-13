# CallHub

Plataforma SaaS multi-tenant para que clientes inmobiliarios configuren bots que llaman a leads. CallHub es el nombre interno de trabajo; cada cliente final (p.ej. Atrium) vive como un tenant aislado en la misma instancia.

El producto tiene dos flujos principales:

1. Lead inbound: una persona escribe interesada en rentar una propiedad y la IA la llama para explicar la propiedad, requisitos, proceso de aplicacion y proximos pasos.
2. Lead outbound: la IA llama a propietarios o leads del cliente para explicar el servicio, responder preguntas y calificar interes.

La recomendacion es construir primero un MVP controlado, con leads que ya dieron permiso o que vienen de una relacion previa, antes de escalar a campañas de cold calling.

## Estado actual

El repositorio trae un MVP operativo end-to-end:

- `frontend`: portal cliente y backoffice en Next.js 16 (client components, login JWT, drawer de llamada de prueba, toasts).
- `backend`: API Go con `pgx/v5`, migrations runner, JWT HS256 + bcrypt, middleware de tenant, cliente ARI con Originate y loop de eventos.
- `postgres`: persistencia real con schema multi-tenant, indices por `tenant_id` y tabla `schema_migrations`.
- `redis`: definido en compose para colas/workers (todavia sin worker activo).
- `asterisk`: PBX con ARI, SIP, RTP, dialplan `callhub-test`/`9000` y plantilla de trunk SIP comentada.
- `backend/migrations`: `001_init.sql` (schema completo) y `002_seed.sql` (tenants demo). Los usuarios bootstrap se crean en codigo en el primer arranque.
- `docs/deploy-coolify.md`: guia de despliegue en Coolify.

## Arquitectura propuesta

```mermaid
flowchart LR
    CRM["Atrium / CRM / formulario"]
    API["Backend de campañas y leads"]
    DB["Postgres"]
    Queue["Cola de llamadas"]
    Asterisk["Asterisk + SIP trunk"]
    Voice["Servicio de voz IA"]
    AI["STT / LLM / TTS o Realtime"]
    Agent["Humano / equipo comercial"]

    CRM --> API
    API --> DB
    API --> Queue
    Queue --> API
    API --> Asterisk
    Asterisk <--> Voice
    Voice <--> AI
    API --> Agent
    Voice --> DB
```

## Componentes

- Asterisk: PBX, SIP trunk, entrantes, salientes, grabaciones y control de canales.
- Backend: API para leads, campañas, propiedades, consentimientos, logs, resumen de llamadas y handoff a humano.
- Base de datos: leads, propiedades, propietarios, estados de llamada, transcripciones, resultado, opt-out y consentimientos.
- Worker de llamadas: decide a quien llamar, horarios permitidos, reintentos, limites por campaña y bloqueo de numeros excluidos.
- Servicio de voz IA: conecta audio de Asterisk con el proveedor de IA, maneja turnos, interrupciones, herramientas y transferencia.
- Panel web: ver leads, lanzar campañas, escuchar grabaciones, revisar transcripciones y configurar scripts.

## Integracion con Asterisk

Para el MVP hay dos opciones:

### Opcion A: ARI + External Media

Es la opcion recomendada para conversaciones naturales en tiempo real.

- El backend origina la llamada con ARI o AMI.
- Asterisk mete la llamada en una aplicacion Stasis.
- Se crea un canal External Media para enviar/recibir audio RTP desde un servicio propio.
- El servicio de voz transforma ese audio para STT/LLM/TTS o para una API realtime.
- El bot puede hablar, escuchar, interrumpir, transferir y registrar resultados.

### Opcion B: AGI/EAGI

Mas simple para un piloto muy guiado, pero peor para conversaciones naturales.

- Asterisk ejecuta scripts por llamada.
- Sirve para menus, preguntas cerradas y flujos lineales.
- No es ideal si el usuario interrumpe, cambia de tema o hace preguntas abiertas.

## MVP recomendado

Primera version:

- Solo llamadas a leads que entraron por formulario o CRM.
- Un tipo de llamada: interesado en rentar propiedad.
- Una propiedad de prueba con datos estructurados.
- Script conversacional con objetivos claros:
  - Identificarse como asistente de Atrium.
  - Confirmar que puede hablar.
  - Explicar propiedad, precio, disponibilidad y requisitos.
  - Resolver preguntas usando datos verificados.
  - Calificar: presupuesto, fecha de mudanza, ocupantes, ingresos, mascotas, interes.
  - Enviar resumen y siguiente paso por SMS/email/CRM.
  - Transferir a humano si hay alta intencion o pregunta sensible.
- Panel minimo para cargar leads y ver resultados.

Despues:

- Campañas para propietarios.
- Integracion completa con Atrium.
- Reglas de reintentos.
- Deteccion de voicemail.
- Handoff en vivo.
- Analitica de conversion.

## Guardrails imprescindibles

- La IA debe decir que es un asistente automatizado de Atrium al inicio.
- No debe inventar disponibilidad, precios, fees, requisitos legales ni condiciones.
- Si no sabe algo, debe ofrecer pasar la consulta a un humano.
- Debe respetar opt-out: si dicen "no me llames", guardar el numero como bloqueado.
- Debe registrar consentimiento/base legal para cada llamada.
- Para cold calling real, revisar normativa por pais/estado antes de activar campañas.

## Estructura sugerida del repo

```text
.
├── asterisk/
│   ├── extensions.conf
│   ├── pjsip.conf
│   └── ari.conf
├── backend/
│   ├── app/
│   └── tests/
├── voice-agent/
│   ├── app/
│   └── tests/
├── web/
├── docker-compose.yml
└── docs/
```

## Siguiente paso tecnico

Hecho en esta iteracion:

- Migraciones aplicadas en Postgres con runner propio (`schema_migrations`).
- Store SQL con `pgx/v5`, filtrado por `tenant_id` en todas las queries.
- Auth JWT HS256 + bcrypt, middleware `requireAuth`/`requireRole`, login UI y persistencia de sesion en `localStorage`.
- Multi-tenancy: cada usuario lleva `tenantId` en sus claims; los `platform_admin` pueden suplantar tenant via selector en la sidebar y query param `?tenant=`.
- Cliente ARI con `Originate` y loop de eventos en WebSocket (`StasisStart`, `ChannelDestroyed`, etc.) que persiste duracion y outcome.
- `POST /api/calls/test` ahora origina canal real cuando `ASTERISK_ARI_ENABLED=true`; si no, registra la llamada en Postgres como queued.
- Trunk SIP configurable via `SIP_TRUNK_ENDPOINT=PJSIP/{phone}@trunk`. Plantilla comentada en `asterisk/etc/asterisk/pjsip.conf`.

Pendiente (fase 4):

- Worker que consume llamadas en cola de Redis y respeta horario/intentos.
- `voice-agent` para External Media (audio realtime) con OpenAI Realtime o pipeline ASR/LLM/TTS.
- Persistencia de transcripts (`transcripts` ya existe en schema).
- Tests de integracion (DB con `testcontainers`, mock ARI).

## Desarrollo local

```bash
cp .env.example .env
# Edita .env: pon un JWT_SECRET largo y aleatorio (`openssl rand -base64 48`)
docker compose up --build
```

Servicios:

- Frontend: <http://localhost:3000> (redirige a `/login`)
- Backend health: <http://localhost:8080/healthz>
- Asterisk ARI: <http://localhost:8088/ari>

Credenciales de bootstrap (creadas la primera vez que el backend arranca contra una DB vacia):

- `admin@callhub.local` / `atrium123` (rol `platform_admin`, ve `/admin`)
- `owner@atrium.local` / `atrium123` (rol `tenant_admin`, scopeado al tenant `atrium`)

Cambialos via `BOOTSTRAP_*_PASSWORD` en `.env` antes del primer arranque o creando otros usuarios manualmente.

### Solo backend

```bash
cd backend
DATABASE_URL=postgres://atrium:change-me@localhost:5432/atrium_calls?sslmode=disable \
JWT_SECRET=$(openssl rand -base64 48) \
go run ./cmd/api
```

### Solo frontend

```bash
cd frontend
npm install
NEXT_PUBLIC_API_URL=http://localhost:8080 npm run dev
```

## Trunks SIP y DIDs

La configuracion telefonica vive en dos sitios complementarios:

1. **Credenciales SIP** en `asterisk/etc/asterisk/pjsip.conf` (host del proveedor, usuario, password). Asterisk las carga al arrancar.
2. **Metadata y enrutamiento** en Postgres, gestionada desde `/admin/trunks` (rol `platform_admin`):
   - **Trunks**: cada fila apunta a un endpoint PJSIP por nombre (`asteriskEndpoint`, p.ej. `twilio-eu`). Es la metadata visible para el admin (proveedor, host de display, notas, contador de DIDs).
   - **DIDs**: numeros E.164 asociados a un trunk. El admin los asigna a un tenant o los deja en el pool global.
3. **Asignacion en el tenant**: el cliente entra en `/portal/bots` y elige que DID usa cada bot. Sin DID asignado el bot solo puede marcar a la extension sandbox interna.

Cuando un tenant lanza una llamada con un bot, el backend resuelve `bot.didId → did.trunkId → trunk.asteriskEndpoint` y origina via ARI como `PJSIP/{phone}@{asteriskEndpoint}` usando el E.164 del DID como caller-id.

Para añadir un proveedor real:

1. Pega los bloques `[endpoint]`, `[auth]`, `[aor]`, `[identify]` en `pjsip.conf` (hay plantilla comentada con tres ejemplos en el archivo) y reinicia el contenedor Asterisk.
2. Como `platform_admin`, en `/admin/trunks` → **Nuevo trunk**, pon el mismo `name` del bloque como `Endpoint Asterisk`.
3. En la pestaña DIDs añade los numeros (E.164) que te dio el proveedor y asignalos al tenant correspondiente.
4. El cliente entra como `tenant_admin`, abre `/portal/bots` y selecciona el DID en cada bot.

## Probar una llamada de prueba

1. Levanta el stack con `docker compose up --build`.
2. Login en `http://localhost:3000` con `owner@atrium.local` / `atrium123`.
3. Dashboard → **Llamada de prueba** → escribe un numero, opcionalmente elige un bot, lanza.
4. Si `ASTERISK_ARI_ENABLED=false` (default), la llamada queda en Postgres como `queued` (valida persistencia).
5. Si `ASTERISK_ARI_ENABLED=true`:
   - Sin bot: hace `Originate` a `PJSIP/6001` (registra un softphone tipo Zoiper en `localhost:5060` user `6001` pass `change-me` para contestar).
   - Con bot que tiene DID asignado: hace `Originate` a `PJSIP/{phone}@{trunkEndpoint}` con el DID como caller-id.

## Despliegue (Coolify + Traefik)

El repo está preparado para deployarse en Coolify detrás de Traefik. La configuración usa labels condicionales (`TRAEFIK_ENABLE=true`) para no estorbar al desarrollo local.

### Servicios públicos (vía Traefik)

| Servicio | Host por defecto | Puerto interno | Propósito |
|---|---|---|---|
| `frontend` | `${FRONTEND_HOST}` | 3000 | Portal Next.js |
| `backend` | `${BACKEND_HOST}` | 8080 | API REST/JSON |
| `voice-agent` | `${VOICE_AGENT_HOST}` | 8090 | HTTP + WebSocket de sesiones |
| `minio` | `${MINIO_HOST}` | 9000 | Recordings (presigned URLs públicas) |

Traefik termina TLS, hace HTTP↔HTTPS y emite certs Let's Encrypt automáticamente cuando Coolify lo gestiona.

### Servicios internos (no expuestos)

- `postgres` — datos
- `redis` — colas/futuro
- `asterisk` — PBX (puertos UDP directos al host, **no** detrás de Traefik porque SIP/RTP son UDP)

### Puertos UDP directos

Traefik no proxea UDP. Asterisk y el voice-agent abren puertos UDP directos al host:

| Servicio | Puertos UDP | Para qué |
|---|---|---|
| asterisk | 5060 | SIP signaling |
| asterisk | 10000-10020 | RTP audio del proveedor SIP |
| voice-agent | 12000-12099 | RTP audio del External Media de Asterisk |

En Coolify abre estos rangos en el firewall del nodo.

### Variables clave para Coolify

```bash
# Habilita los Traefik labels
TRAEFIK_ENABLE=true

# Hosts públicos
FRONTEND_HOST=portal.tudominio.com
BACKEND_HOST=api.tudominio.com
VOICE_AGENT_HOST=voice.tudominio.com
MINIO_HOST=storage.tudominio.com

# Builds: el frontend bakea NEXT_PUBLIC_API_URL en build time
NEXT_PUBLIC_API_URL=https://api.tudominio.com

# CORS del backend debe coincidir con el host del frontend
ALLOWED_ORIGINS=https://portal.tudominio.com

# Storage público accesible (URL que verá el navegador)
STORAGE_PUBLIC_URL=https://storage.tudominio.com

# Hostname/IP que Asterisk usa para llegar al voice-agent (RTP)
# Si Asterisk corre en el mismo node, "voice-agent" funciona (nombre del servicio).
# Si Asterisk corre fuera, usa la IP pública del nodo.
RTP_ADVERTISE_HOST=voice-agent

# Producción: generar secretos largos
JWT_SECRET=$(openssl rand -base64 48)
VOICE_AGENT_SHARED_SECRET=$(openssl rand -base64 32)
STORAGE_SECRET_KEY=$(openssl rand -base64 32)
BOOTSTRAP_ADMIN_PASSWORD=$(openssl rand -base64 24)
BOOTSTRAP_TENANT_PASSWORD=$(openssl rand -base64 24)
```

### Pasos en Coolify

1. **Nueva aplicación** → Source: Git repository → URL: `https://github.com/Smartgroup10/timbreAI`
2. **Build pack**: Docker Compose
3. **Compose file**: `docker-compose.yml`
4. **Environment**: pega las variables de la sección anterior
5. **Domains**: añade los 4 hostnames a sus servicios respectivos
6. **Deploy**

El primer arranque corre las migraciones (007 archivos `.sql`) y crea los usuarios bootstrap. Login con `admin@callhub.local` y la password que pusiste.

### Healthchecks

Todos los servicios tienen healthcheck. Coolify los respeta para el rolling deploy:

- backend: `GET /healthz`
- voice-agent: `GET /healthz`
- frontend: `GET /` (spider)
- postgres: `pg_isready`
- redis: `redis-cli ping`
- minio: `mc ready`

Ver más detalle en [docs/deploy-coolify.md](docs/deploy-coolify.md).
