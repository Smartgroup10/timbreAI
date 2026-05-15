package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ListTools devuelve la biblioteca de tools del tenant. onlyEnabled
// filtra las archivadas (enabled=false en la biblioteca).
func (s *Store) ListTools(ctx context.Context, tenantID string, onlyEnabled bool) ([]Tool, error) {
	query := `
		SELECT id, tenant_id, name, description, parameters_schema, action_type, action_config,
		       enabled, created_at, updated_at
		FROM tools
		WHERE tenant_id = $1`
	if onlyEnabled {
		query += ` AND enabled = true`
	}
	query += ` ORDER BY name`
	rows, err := s.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Tool{}
	for rows.Next() {
		t, err := scanTool(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) GetTool(ctx context.Context, tenantID, id string) (Tool, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, description, parameters_schema, action_type, action_config,
		       enabled, created_at, updated_at
		FROM tools WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	t, err := scanTool(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

func (s *Store) CreateTool(ctx context.Context, t Tool) (Tool, error) {
	if t.ID == "" {
		t.ID = newID("tool")
	}
	if t.ParametersSchema == nil {
		t.ParametersSchema = map[string]any{"type": "object", "properties": map[string]any{}}
	}
	if t.ActionConfig == nil {
		t.ActionConfig = map[string]any{}
	}
	params, _ := json.Marshal(t.ParametersSchema)
	action, _ := json.Marshal(t.ActionConfig)

	err := s.pool.QueryRow(ctx, `
		INSERT INTO tools (id, tenant_id, name, description, parameters_schema, action_type, action_config, enabled)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7::jsonb, $8)
		RETURNING created_at, updated_at`,
		t.ID, t.TenantID, t.Name, t.Description, params, t.ActionType, action, t.Enabled).
		Scan(&t.CreatedAt, &t.UpdatedAt)
	return t, err
}

// UpdateTool — action_type es inmutable (cambiarlo invalida el
// action_config silenciosamente). Para cambiar el tipo: borra y crea.
func (s *Store) UpdateTool(ctx context.Context, t Tool) (Tool, error) {
	params, _ := json.Marshal(t.ParametersSchema)
	action, _ := json.Marshal(t.ActionConfig)
	tag, err := s.pool.Exec(ctx, `
		UPDATE tools
		SET name = $3, description = $4, parameters_schema = $5::jsonb,
		    action_config = $6::jsonb, enabled = $7, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		t.TenantID, t.ID, t.Name, t.Description, params, action, t.Enabled)
	if err != nil {
		return Tool{}, err
	}
	if tag.RowsAffected() == 0 {
		return Tool{}, ErrNotFound
	}
	return s.GetTool(ctx, t.TenantID, t.ID)
}

func (s *Store) DeleteTool(ctx context.Context, tenantID, id string) error {
	// CASCADE en FK borra asignaciones automáticamente.
	tag, err := s.pool.Exec(ctx, `DELETE FROM tools WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AssignToolToBot inserta o actualiza la asignación (bot, tool). Si ya
// existe, actualiza solo el flag enabled. Verificamos primero que la
// tool y el bot pertenecen al mismo tenant — defensa contra payload
// malicioso que mande IDs cruzados de tenants.
func (s *Store) AssignToolToBot(ctx context.Context, tenantID, botID, toolID string, enabled bool) error {
	var ok bool
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM bots b JOIN tools t ON t.tenant_id = b.tenant_id
		  WHERE b.id = $1 AND t.id = $2 AND b.tenant_id = $3
		)`, botID, toolID, tenantID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("bot or tool not in tenant %s: %w", tenantID, ErrNotFound)
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO bot_tool_assignments (bot_id, tool_id, enabled)
		VALUES ($1, $2, $3)
		ON CONFLICT (bot_id, tool_id) DO UPDATE SET enabled = EXCLUDED.enabled`,
		botID, toolID, enabled)
	return err
}

func (s *Store) UnassignToolFromBot(ctx context.Context, tenantID, botID, toolID string) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM bot_tool_assignments a
		USING bots b
		WHERE a.bot_id = b.id AND b.tenant_id = $1
		  AND a.bot_id = $2 AND a.tool_id = $3`,
		tenantID, botID, toolID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListBotToolViews devuelve TODAS las tools de la biblioteca del tenant
// con su estado de asignación al bot — la UI las pinta como una lista
// con un checkbox + switch enabled.
func (s *Store) ListBotToolViews(ctx context.Context, tenantID, botID string) ([]BotToolView, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.tenant_id, t.name, t.description, t.parameters_schema, t.action_type, t.action_config,
		       t.enabled, t.created_at, t.updated_at,
		       COALESCE(a.bot_id IS NOT NULL, false) AS assigned,
		       COALESCE(a.enabled, false) AS assigned_enabled
		FROM tools t
		LEFT JOIN bot_tool_assignments a ON a.tool_id = t.id AND a.bot_id = $2
		WHERE t.tenant_id = $1
		ORDER BY t.name`, tenantID, botID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BotToolView{}
	for rows.Next() {
		var v BotToolView
		var params, action []byte
		if err := rows.Scan(&v.ID, &v.TenantID, &v.Name, &v.Description,
			&params, &v.ActionType, &action, &v.Enabled, &v.CreatedAt, &v.UpdatedAt,
			&v.Assigned, &v.AssignedEnabled); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(params, &v.ParametersSchema)
		_ = json.Unmarshal(action, &v.ActionConfig)
		out = append(out, v)
	}
	return out, rows.Err()
}

// ListBotTools devuelve las tools efectivamente activas en un bot —
// es lo que pasa el dispatcher al voice-agent al iniciar sesión.
func (s *Store) ListBotTools(ctx context.Context, tenantID, botID string, onlyEnabled bool) ([]Tool, error) {
	query := `
		SELECT t.id, t.tenant_id, t.name, t.description, t.parameters_schema, t.action_type, t.action_config,
		       t.enabled, t.created_at, t.updated_at
		FROM tools t
		JOIN bot_tool_assignments a ON a.tool_id = t.id
		WHERE t.tenant_id = $1 AND a.bot_id = $2`
	if onlyEnabled {
		query += ` AND a.enabled = true AND t.enabled = true`
	}
	query += ` ORDER BY t.name`
	rows, err := s.pool.Query(ctx, query, tenantID, botID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Tool{}
	for rows.Next() {
		t, err := scanTool(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// scanTool firma uniforme para Row y Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanTool(r rowScanner) (Tool, error) {
	var t Tool
	var params, action []byte
	if err := r.Scan(&t.ID, &t.TenantID, &t.Name, &t.Description,
		&params, &t.ActionType, &action, &t.Enabled, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return t, err
	}
	_ = json.Unmarshal(params, &t.ParametersSchema)
	_ = json.Unmarshal(action, &t.ActionConfig)
	return t, nil
}

// ─── Invocations + helpers usados por executeToolAction ────────────────

func (s *Store) LogBotToolInvocation(ctx context.Context, inv BotToolInvocation) error {
	if inv.ID == "" {
		inv.ID = newID("inv")
	}
	if inv.Arguments == nil {
		inv.Arguments = map[string]any{}
	}
	if inv.Result == nil {
		inv.Result = map[string]any{}
	}
	args, _ := json.Marshal(inv.Arguments)
	result, _ := json.Marshal(inv.Result)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO bot_tool_invocations (id, tenant_id, call_id, tool_id, tool_name, arguments, result, success, error)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8, $9)`,
		inv.ID, inv.TenantID, inv.CallID, inv.ToolID, inv.ToolName, args, result, inv.Success, inv.Error)
	return err
}

func (s *Store) ListBotToolInvocations(ctx context.Context, tenantID, callID string) ([]BotToolInvocation, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, call_id, tool_id, tool_name, arguments, result, success, error, created_at
		FROM bot_tool_invocations
		WHERE tenant_id = $1 AND call_id = $2
		ORDER BY created_at`, tenantID, callID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BotToolInvocation{}
	for rows.Next() {
		var i BotToolInvocation
		var args, result []byte
		if err := rows.Scan(&i.ID, &i.TenantID, &i.CallID, &i.ToolID, &i.ToolName,
			&args, &result, &i.Success, &i.Error, &i.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(args, &i.Arguments)
		_ = json.Unmarshal(result, &i.Result)
		out = append(out, i)
	}
	return out, rows.Err()
}

// ─── Helpers compartidos por handlers de tools ─────────────────────────

func (s *Store) UpdateCallOutcome(ctx context.Context, tenantID, id, outcome string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE calls SET outcome = $3 WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, outcome)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateLeadStatus(ctx context.Context, tenantID, leadID, status string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE leads SET status = $3, last_activity = now()
		WHERE tenant_id = $1 AND id = $2`, tenantID, leadID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetCampaign(ctx context.Context, tenantID, id string) (Campaign, error) {
	var c Campaign
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, status, COALESCE(bot_id, ''),
		       COALESCE(schedule, ''), max_concurrent, max_attempts,
		       start_at, end_at, retry_cooldown_minutes
		FROM campaigns WHERE tenant_id = $1 AND id = $2`, tenantID, id).
		Scan(&c.ID, &c.TenantID, &c.Name, &c.Status, &c.BotID,
			&c.Schedule, &c.MaxConcurrent, &c.MaxAttempts,
			&c.StartAt, &c.EndAt, &c.RetryCooldownMinutes)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}
