# Arquitectura del bot de llamadas

## Objetivo

Construir un sistema de llamadas con IA multi-tenant (CallHub), soportando:

- Llamadas a personas interesadas en rentar una propiedad.
- Llamadas comerciales a propietarios/leads para explicar el servicio.
- Recepcion de llamadas entrantes.
- Transferencia a humano cuando convenga.

## Decision principal

Usar Asterisk como capa telefonica y un backend propio como orquestador.

Asterisk no deberia contener la logica de negocio. Su responsabilidad es telefonia: SIP, llamadas entrantes/salientes, grabacion, bridges y eventos. La logica de leads, campañas, IA, consentimientos y resultados vive en servicios de aplicacion.

## Flujo: interesado en rentar

1. El lead entra desde formulario, CRM o sistema del tenant (p.ej. Atrium).
2. El backend valida telefono, consentimiento, propiedad y horario permitido.
3. Se crea una tarea de llamada.
4. El worker pide a Asterisk originar la llamada.
5. Cuando contesta, Asterisk conecta el canal al servicio de voz.
6. La IA conversa usando datos estructurados de la propiedad.
7. La IA guarda resultado: interesado, no contesta, no interesado, transferido, callback, bloqueado.
8. El backend actualiza CRM y dispara siguiente accion.

## Flujo: propietarios

1. El lead propietario existe en CRM con base legal/consentimiento.
2. El worker llama dentro de horarios y limites configurados.
3. La IA se presenta, explica el servicio del tenant y pregunta si tiene sentido hablar.
4. Si hay interes, califica propiedad y agenda llamada humana.
5. Si pide no recibir llamadas, se bloquea el numero.

## Modelo de datos inicial

- `leads`: nombre, telefono, email, tipo, fuente, estado, idioma, zona horaria.
- `properties`: direccion parcial, precio, habitaciones, baños, requisitos, disponibilidad, politicas.
- `campaigns`: nombre, tipo, horario, max_retries, caller_id, script_id.
- `calls`: lead_id, campaign_id, estado, started_at, ended_at, duration, recording_url.
- `call_events`: call_id, timestamp, tipo, payload.
- `transcripts`: call_id, role, text, timestamp.
- `consents`: lead_id, source, lawful_basis, consent_text, captured_at.
- `do_not_call`: telefono, reason, created_at.

## Herramientas del agente

La IA debe poder llamar funciones del backend:

- `get_property_details(property_id)`
- `check_application_requirements(property_id)`
- `qualify_renter(answers)`
- `qualify_owner(answers)`
- `schedule_callback(lead_id, datetime)`
- `transfer_to_human(reason)`
- `mark_do_not_call(phone, reason)`
- `send_followup(lead_id, template)`

## Tecnologia recomendada

- Asterisk 20/22 LTS o Certified Asterisk 20.x.
- PJSIP para trunk y extensiones.
- ARI para control de llamadas.
- External Media para sacar/inyectar audio RTP hacia el servicio de voz.
- Backend en FastAPI o NestJS.
- Postgres para datos.
- Redis para colas y jobs.
- OpenAI Realtime, o alternativa STT + LLM + TTS si queremos desacoplar proveedores.
- Web admin en React/Next.js cuando el MVP telefonico este validado.

## Prioridades de implementacion

1. Montar Asterisk local con una extension SIP de prueba.
2. Crear backend con leads, propiedades y endpoint para originar llamada.
3. Conectar ARI y recibir eventos de llamada.
4. Implementar External Media hacia un servicio de voz.
5. Crear primer agente para interesados en rentar.
6. Guardar transcripcion y resultado.
7. Añadir panel minimo.
8. Pasar a trunk real y pruebas controladas.

## Riesgos

- Latencia de audio si el pipeline STT/LLM/TTS no es realtime.
- Interrupciones y turn-taking en llamadas naturales.
- Calidad de trunk, codecs y NAT.
- Normativa de llamadas comerciales.
- Datos incorrectos de propiedades.
- Transferencias a humano fuera de horario.

## Regla de producto

El sistema debe comportarse como un asistente de ventas/control de leads, no como un marcador masivo indiscriminado.

