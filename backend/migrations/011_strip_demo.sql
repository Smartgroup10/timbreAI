-- Limpieza de datos de demo que vinieron de 002_seed.sql (Maria Lopez,
-- Sunset Villas, Leasing Assistant, etc.). En producción no queremos que
-- el tenant arranque con datos ficticios visibles.
--
-- El tenant "atrium" SE QUEDA porque el bootstrap user (owner@atrium.local)
-- apunta a él vía BOOTSTRAP_TENANT_ID. Solo borramos su contenido.
--
-- Idempotente: si ya estaba limpio, no rompe.

-- Tenant secundario de demo y todos sus datos vía CASCADE.
DELETE FROM tenants WHERE id = 'demo-homes';

-- Datos demo del tenant principal (no borramos el tenant, solo el contenido).
DELETE FROM calls       WHERE id IN ('call_001','call_002','call_003');
DELETE FROM campaigns   WHERE id IN ('camp_001','camp_002');
DELETE FROM bots        WHERE id IN ('bot_001','bot_002');
DELETE FROM properties  WHERE id IN ('prop_001','prop_002');
DELETE FROM leads       WHERE id IN ('lead_001','lead_002','lead_003');

-- El bootstrap se encarga ahora de UPSERT del tenant principal con el nombre
-- que defina BOOTSTRAP_TENANT_NAME, así que si el operador renombra su tenant
-- en el .env eso sobrescribe el "Atrium Leasing" del seed antiguo.
