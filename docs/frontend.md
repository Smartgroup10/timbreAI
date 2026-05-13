# Frontend de la plataforma

## Objetivo

Construir una aplicacion SaaS multi-cliente para gestionar bots de llamadas, campañas, leads, propiedades, configuracion de voz y resultados.

Debe haber dos superficies principales:

1. Backoffice interno de administrador.
2. Portal de cliente.

## Roles

- Super admin: equipo interno de la plataforma. Ve todos los clientes.
- Admin cliente: gestiona la cuenta de su empresa.
- Operador cliente: revisa leads, llamadas y resultados.
- Solo lectura: auditoria, managers o clientes con acceso limitado.

## Backoffice interno

Uso: operar, auditar y dar soporte a todos los clientes.

Vistas principales:

- Dashboard global:
  - llamadas hoy
  - llamadas exitosas
  - coste estimado
  - errores de trunk/IA
  - clientes activos
- Clientes:
  - crear cliente
  - activar/desactivar
  - limites de llamadas
  - configuracion de trunk/caller ID
  - usuarios del cliente
- Bots:
  - ver bots por cliente
  - estado de configuracion
  - idioma, voz, prompt, herramientas activas
- Campañas:
  - campañas activas/pausadas
  - volumen
  - reintentos
  - tasa de contacto
- Llamadas:
  - filtros por cliente, campaña, estado, fecha
  - grabacion
  - transcripcion
  - resumen
  - motivo de finalizacion
- Compliance:
  - consentimientos
  - opt-outs
  - numeros bloqueados
  - auditoria de cambios
- Observabilidad:
  - eventos Asterisk
  - fallos de IA
  - latencia
  - colas
  - alertas

## Portal cliente

Uso: que cada cliente configure y lance sus propios bots sin tocar infraestructura.

Vistas principales:

- Dashboard:
  - llamadas programadas
  - llamadas completadas
  - leads calificados
  - callbacks pendientes
  - conversion por campaña
- Leads:
  - importar CSV
  - crear lead manual
  - estados
  - historial de llamadas
  - opt-out
- Propiedades/servicios:
  - datos de propiedades
  - precio
  - disponibilidad
  - requisitos
  - FAQs
  - documentos o enlaces
- Bots:
  - nombre del bot
  - objetivo
  - idioma
  - voz
  - tono
  - instrucciones
  - reglas de transferencia a humano
  - preguntas obligatorias
  - respuestas prohibidas o sensibles
- Campañas:
  - crear campaña
  - seleccionar bot
  - seleccionar lista de leads
  - horario permitido
  - maximo de intentos
  - cadencia de reintentos
  - fecha de inicio/fin
  - lanzar ahora o programar
- Resultados:
  - tabla de llamadas
  - filtros
  - grabacion/transcripcion
  - resumen generado
  - acciones: llamar de nuevo, marcar como resuelto, transferir a comercial
- Configuracion:
  - usuarios y permisos
  - integraciones CRM
  - numeros/caller IDs permitidos
  - plantillas de follow-up
  - zona horaria

## Flujo de creacion de bot

1. Elegir tipo de bot:
   - interesados en rentar
   - propietarios
   - llamadas entrantes
2. Configurar identidad:
   - nombre visible
   - empresa
   - idioma
   - voz
3. Definir objetivo:
   - calificar lead
   - explicar propiedad
   - agendar llamada
   - transferir a humano
4. Cargar conocimiento:
   - propiedades
   - FAQs
   - requisitos
   - politicas
5. Configurar limites:
   - que no puede decir
   - cuando debe transferir
   - cuando debe parar
6. Probar bot:
   - llamada de prueba
   - simulador de conversacion
7. Activar.

## Flujo de campaña

1. Crear campaña.
2. Elegir bot.
3. Elegir segmento/lista de leads.
4. Validar consentimientos y opt-outs.
5. Configurar horario y reintentos.
6. Revisar estimacion de llamadas/coste.
7. Programar o lanzar.
8. Ver progreso en tiempo real.

## Modelo multi-tenant

Todas las entidades deben estar asociadas a `tenant_id`.

Entidades con tenant:

- users
- leads
- properties
- bots
- campaigns
- calls
- transcripts
- consents
- do_not_call
- integrations

El backoffice interno puede cruzar tenants. El portal cliente nunca debe acceder a datos de otro tenant.

## Stack recomendado

- Next.js para frontend.
- Tailwind o CSS modules, segun el gusto del equipo.
- Shadcn/ui o componentes propios si queremos velocidad.
- TanStack Query para datos de API.
- TanStack Table para tablas grandes.
- Zod + React Hook Form para formularios complejos.
- Auth con roles y tenant activo.

## Pantallas prioritarias del MVP

1. Login.
2. Selector de cliente/tenant para admins internos.
3. Dashboard cliente.
4. CRUD de leads.
5. CRUD de propiedades.
6. Configuracion basica de bot.
7. Crear campaña.
8. Tabla de llamadas con transcripcion/resumen.
9. Lista de opt-out.

## Diseño

Debe sentirse como una herramienta operacional, no como landing page.

- Navegacion lateral.
- Tablas densas y filtrables.
- Estados claros.
- Formularios por pasos para bots/campañas.
- Alertas visibles para compliance y errores.
- Auditoria accesible, pero no invasiva.

