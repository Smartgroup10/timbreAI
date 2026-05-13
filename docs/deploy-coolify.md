# Despliegue en Coolify

## Donde montarlo

El VPS con Coolify es una buena primera opcion para este producto si se arranca con bajo volumen y campañas controladas.

Recomendacion inicial:

- 2 vCPU minimo.
- 4 GB RAM minimo.
- 40 GB disco.
- Ubuntu/Debian.
- Puertos web gestionados por Coolify/Traefik.
- Puertos SIP/RTP abiertos solo cuando conectemos un trunk o extensiones externas.

Cuando haya llamadas reales con volumen, conviene separar telefonia:

- App SaaS en Coolify.
- Asterisk en VPS dedicado o nodo separado con IP publica estable.
- Postgres gestionado o backups automatizados.

## Servicios del compose

- `frontend`: portal cliente y backoffice.
- `backend`: API Go.
- `postgres`: base de datos.
- `redis`: colas y jobs.
- `asterisk`: PBX con ARI preparado.

## Pasos

1. Subir este repo a GitHub.
2. En Coolify, crear nuevo recurso desde GitHub.
3. Elegir Docker Compose.
4. Configurar variables de entorno desde `.env.example`.
5. Asignar dominio al servicio `frontend`.
6. Opcional: asignar subdominio privado al `backend`.
7. Desplegar.

## Variables importantes

- `POSTGRES_PASSWORD`
- `DATABASE_URL`
- `JWT_SECRET`
- `NEXT_PUBLIC_API_URL`
- `ASTERISK_ARI_USER`
- `ASTERISK_ARI_PASSWORD`
- `OPENAI_API_KEY`

## Notas de Asterisk

El archivo `asterisk/etc/asterisk/ari.conf` trae credenciales demo. Antes de exponer ARI en produccion, cambiar usuario/password y restringir acceso por red.

Puertos:

- `8088`: ARI/HTTP de Asterisk.
- `5060/udp`: SIP.
- `10000-10020/udp`: RTP de prueba.

En produccion real, RTP necesitara un rango mas amplio y reglas NAT/Firewall cuidadas.

## Primer despliegue seguro

Para el primer deploy, se puede dejar Asterisk sin exponer SIP publicamente y usar solo:

- frontend por HTTPS
- backend interno
- Postgres/Redis internos
- ARI accesible solo dentro de la red Docker

Luego se conecta el trunk SIP cuando ya tengamos:

- autenticacion real
- tenant isolation
- opt-out
- consentimientos
- logs
- limites de campaña

