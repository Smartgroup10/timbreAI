# Backend

## Recomendacion

Usar Go para el backend principal y los workers de telefonia.

Motivos:

- Buen manejo de concurrencia para llamadas simultaneas.
- Servicios estables de larga vida.
- WebSockets y streaming bien soportados.
- Buen encaje con Asterisk ARI/AMI.
- Binarios simples de desplegar.
- Menos magia que frameworks muy pesados.

## Servicios

### API

Responsable de:

- autenticacion
- tenants/clientes
- usuarios y roles
- leads
- propiedades
- bots
- campañas
- llamadas
- resultados
- integraciones

### Call worker

Responsable de:

- tomar llamadas pendientes de la cola
- validar horarios, opt-outs y limites
- pedir a Asterisk originar llamadas
- recibir estados
- manejar reintentos

### Asterisk bridge service

Responsable de:

- conectar con ARI
- escuchar eventos de canales
- crear bridges
- crear External Media
- colgar, transferir o grabar llamadas

### Voice agent service

Puede estar en Go, pero tambien es razonable separarlo si el SDK/proveedor de IA encaja mejor en Node.js o Python.

Responsable de:

- recibir audio desde Asterisk
- enviar audio al modelo realtime o pipeline STT/LLM/TTS
- ejecutar tools del backend
- devolver audio a Asterisk
- guardar transcripcion parcial/final

## Stack actual

- HTTP: `net/http` con `ServeMux` (sin frameworks, suficiente para el tamaño actual)
- DB: Postgres 16 via `github.com/jackc/pgx/v5/pgxpool`
- Migraciones: runner propio en `internal/db`, archivos SQL en `backend/migrations/*.sql`, tracking en tabla `schema_migrations`
- Jobs/cola: Redis definido en compose, worker todavia no implementado
- Config: env vars cargadas en `internal/config`
- Logs: `slog` JSON
- Auth: JWT HS256 (HMAC-SHA256, implementacion propia ~50 lineas) + `bcrypt` (`golang.org/x/crypto/bcrypt`). Claims llevan `sub`, `tenant`, `role`, `email`, `exp`
- ARI: cliente propio sobre `net/http` y `github.com/coder/websocket` (Originate + loop de eventos con backoff exponencial)

## Layout del codigo

```
backend/
├── cmd/api/main.go              Entrypoint, bootstrap users, lifecycle
├── internal/
│   ├── config/                  Env vars
│   ├── db/                      pgx pool + migrations runner
│   ├── store/                   Modelos + queries SQL (filtradas por tenant_id)
│   ├── auth/                    JWT HS256, bcrypt, middleware context
│   ├── ari/                     Cliente ARI (Originate + WS events)
│   └── api/                     Router, middleware, handlers
└── migrations/
    ├── 001_init.sql             Schema (users con password_hash, etc.)
    └── 002_seed.sql             Tenants demo (usuarios bootstrap se crean en codigo)
```

## Base de datos

Postgres como fuente de verdad.

Tablas clave:

- tenants
- users
- roles
- leads
- properties
- bots
- bot_versions
- campaigns
- campaign_leads
- calls
- call_events
- transcripts
- consents
- do_not_call
- integrations
- audit_logs

## API implementada

Publicos:
- `GET /healthz`
- `POST /api/auth/login` — devuelve `{token, expiresAt, user}`

Autenticado (Bearer token):
- `GET /api/auth/me`
- `GET /api/overview`
- `GET /api/leads`, `POST /api/leads`
- `GET /api/properties`
- `GET /api/bots`
- `GET /api/campaigns`, `POST /api/campaigns`
- `GET /api/calls`
- `POST /api/calls/test`

Solo `platform_admin`:
- `GET /api/admin/tenants`
- `GET /api/admin/operations`

Multi-tenancy: usuarios con tenant fijo lo llevan en el JWT. Los `platform_admin` pueden añadir `?tenant=xxx` a cualquier endpoint scopeado para operar como si fueran ese cliente. Sin override, el admin opera sobre su propio tenant si lo tiene.

## Pendiente

- `POST /api/campaigns/{id}/schedule`, `pause`
- `POST /api/do-not-call`
- `GET /api/calls/{id}` con transcripts
- Worker que consume llamadas en `queued` y respeta horario/intentos
- Voice agent (External Media + LLM)

## Separacion importante

El backend decide quien puede llamar, cuando y con que datos.

Asterisk solo ejecuta telefonia.

El agente de voz solo conversa y llama herramientas autorizadas.

