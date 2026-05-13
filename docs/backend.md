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

## Stack Go sugerido

- HTTP: `net/http` o `chi`
- DB: Postgres
- SQL: `sqlc` + `pgx`
- Migraciones: `goose` o `atlas`
- Jobs/cola: Redis + workers propios, o NATS si queremos eventos mas serios
- Config: env vars
- Logs: `slog`
- Auth: JWT/sesiones con tenant_id y roles
- OpenAPI: generado desde handlers o especificacion

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

## API inicial

- `POST /auth/login`
- `GET /me`
- `GET /tenants`
- `POST /leads`
- `GET /leads`
- `POST /properties`
- `GET /properties`
- `POST /bots`
- `GET /bots`
- `POST /campaigns`
- `POST /campaigns/{id}/schedule`
- `POST /campaigns/{id}/pause`
- `GET /calls`
- `GET /calls/{id}`
- `POST /calls/test`
- `POST /do-not-call`

## Separacion importante

El backend decide quien puede llamar, cuando y con que datos.

Asterisk solo ejecuta telefonia.

El agente de voz solo conversa y llama herramientas autorizadas.

