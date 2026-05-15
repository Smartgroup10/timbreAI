-- 023_tools_library.sql
-- Refactor de bot_tools (1 tool por bot) → biblioteca por tenant +
-- asignaciones. Permite que varios bots compartan la misma tool sin
-- duplicarla, y simplifica al operador: crea "marcar_cualificado" una
-- vez y la activa en N bots.
--
-- Cambios:
--   - tools (renombrada bot_tools): pierde bot_id; gana UNIQUE(tenant_id, name).
--   - bot_tool_assignments: tabla join (bot_id, tool_id, enabled).
--   - bot_tool_invocations.bot_tool_id → tool_id (rename de columna).
--
-- Migración de datos: cada bot_tools existente se convierte en una tool
-- + una asignación al bot que la creó. Si dos bots del mismo tenant
-- tenían tools con el mismo nombre (legítimo en el esquema viejo),
-- desambiguamos añadiendo sufijo numérico al nombre — el operador puede
-- consolidarlas a mano después desde la UI de la biblioteca.

-- 1. Crear bot_tool_assignments primero (FK la añadimos después del rename
--    de bot_tools→tools).
CREATE TABLE IF NOT EXISTS bot_tool_assignments (
  bot_id     text NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
  tool_id    text NOT NULL,
  enabled    boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (bot_id, tool_id)
);

-- 2. Sembrar asignaciones desde bot_tools existente. 1:1 — cada row vieja
--    se conserva como (su bot original) → (esa tool).
INSERT INTO bot_tool_assignments (bot_id, tool_id, enabled, created_at)
SELECT bot_id, id, enabled, created_at
FROM bot_tools
ON CONFLICT (bot_id, tool_id) DO NOTHING;

-- 3. Deduplicar nombres dentro del mismo tenant. Mantenemos la fila
--    más antigua con su nombre original; añadimos sufijo a las demás.
WITH dups AS (
  SELECT id,
         ROW_NUMBER() OVER (PARTITION BY tenant_id, name ORDER BY created_at) AS rn
  FROM bot_tools
)
UPDATE bot_tools t
SET name = t.name || '_' || dups.rn
FROM dups
WHERE t.id = dups.id AND dups.rn > 1;

-- 4. Reset de UNIQUE constraints: era (bot_id, name); ahora (tenant_id, name).
ALTER TABLE bot_tools DROP CONSTRAINT IF EXISTS bot_tools_name_unique;
ALTER TABLE bot_tools ADD CONSTRAINT tools_tenant_name_unique UNIQUE (tenant_id, name);

-- 5. Drop bot_id — la relación ahora vive en bot_tool_assignments.
ALTER TABLE bot_tools DROP COLUMN IF EXISTS bot_id;

-- 6. Renombrar la tabla por semántica.
ALTER TABLE bot_tools RENAME TO tools;

-- 7. El CHECK constraint llevaba el nombre antiguo — lo renombramos para
--    coherencia. (Postgres permite renombrar constraints.)
ALTER TABLE tools RENAME CONSTRAINT bot_tools_action_type_check TO tools_action_type_check;

-- 8. Añadir FK a bot_tool_assignments ahora que tools existe.
ALTER TABLE bot_tool_assignments
  ADD CONSTRAINT bot_tool_assignments_tool_fk
  FOREIGN KEY (tool_id) REFERENCES tools(id) ON DELETE CASCADE;

-- 9. Rename de la columna en invocations + el FK target.
ALTER TABLE bot_tool_invocations RENAME COLUMN bot_tool_id TO tool_id;
-- El FK original apuntaba a bot_tools — Postgres lo sigue tras el rename
-- de la tabla, así que no necesitamos recrearlo. Pero el nombre del
-- constraint es viejo; lo renombramos por estética.
ALTER TABLE bot_tool_invocations
  RENAME CONSTRAINT bot_tool_invocations_bot_tool_id_fkey TO bot_tool_invocations_tool_id_fkey;

-- 10. Index por tenant — la biblioteca se consulta scoped por tenant.
CREATE INDEX IF NOT EXISTS idx_tools_tenant ON tools(tenant_id, created_at DESC);
-- Index para resolver "tools activas de este bot" en el dispatcher.
CREATE INDEX IF NOT EXISTS idx_bot_tool_assignments_bot
  ON bot_tool_assignments(bot_id) WHERE enabled = true;
