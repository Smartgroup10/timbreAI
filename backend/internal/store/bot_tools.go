package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ListBotTools devuelve las tools de un bot. Solo enabled=true cuando
// onlyEnabled=true (lo usa el dispatcher al iniciar sesión, no la UI).
func (s *Store) ListBotTools(ctx context.Context, tenantID, botID string, onlyEnabled bool) ([]BotTool, error) {
	query := `
		SELECT id, tenant_id, bot_id, name, description, parameters_schema, action_type, action_config,
		       enabled, created_at, updated_at
		FROM bot_tools
		WHERE tenant_id = $1 AND bot_id = $2`
	if onlyEnabled {
		query += ` AND enabled = true`
	}
	query += ` ORDER BY created_at`

	rows, err := s.pool.Query(ctx, query, tenantID, botID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BotTool{}
	for rows.Next() {
		var t BotTool
		var params, action []byte
		if err := rows.Scan(&t.ID, &t.TenantID, &t.BotID, &t.Name, &t.Description,
			&params, &t.ActionType, &action, &t.Enabled, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(params, &t.ParametersSchema)
		_ = json.Unmarshal(action, &t.ActionConfig)
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetBotTool devuelve una tool concreta scoped por tenant.
func (s *Store) GetBotTool(ctx context.Context, tenantID, id string) (BotTool, error) {
	var t BotTool
	var params, action []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, bot_id, name, description, parameters_schema, action_type, action_config,
		       enabled, created_at, updated_at
		FROM bot_tools WHERE tenant_id = $1 AND id = $2`, tenantID, id).
		Scan(&t.ID, &t.TenantID, &t.BotID, &t.Name, &t.Description,
			&params, &t.ActionType, &action, &t.Enabled, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	if err != nil {
		return t, err
	}
	_ = json.Unmarshal(params, &t.ParametersSchema)
	_ = json.Unmarshal(action, &t.ActionConfig)
	return t, nil
}

// CreateBotTool inserta una tool. Valida que el bot pertenezca al tenant
// (defensa en profundidad ante un cliente malicioso que mande bot_id ajeno).
func (s *Store) CreateBotTool(ctx context.Context, t BotTool) (BotTool, error) {
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
		INSERT INTO bot_tools (id, tenant_id, bot_id, name, description, parameters_schema, action_type, action_config, enabled)
		SELECT $1, $2, $3, $4, $5, $6::jsonb, $7, $8::jsonb, $9
		WHERE EXISTS (SELECT 1 FROM bots WHERE id = $3 AND tenant_id = $2)
		RETURNING created_at, updated_at`,
		t.ID, t.TenantID, t.BotID, t.Name, t.Description, params, t.ActionType, action, t.Enabled).
		Scan(&t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return BotTool{}, fmt.Errorf("bot %s not in tenant %s: %w", t.BotID, t.TenantID, ErrNotFound)
	}
	return t, err
}

// UpdateBotTool actualiza los campos editables. action_type es inmutable
// para evitar invalidación silenciosa del action_config (cambiar de tipo
// implica re-escribir el config; mejor borrar y crear).
func (s *Store) UpdateBotTool(ctx context.Context, t BotTool) (BotTool, error) {
	params, _ := json.Marshal(t.ParametersSchema)
	action, _ := json.Marshal(t.ActionConfig)
	tag, err := s.pool.Exec(ctx, `
		UPDATE bot_tools
		SET name = $3, description = $4, parameters_schema = $5::jsonb,
		    action_config = $6::jsonb, enabled = $7, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		t.TenantID, t.ID, t.Name, t.Description, params, action, t.Enabled)
	if err != nil {
		return BotTool{}, err
	}
	if tag.RowsAffected() == 0 {
		return BotTool{}, ErrNotFound
	}
	return s.GetBotTool(ctx, t.TenantID, t.ID)
}

func (s *Store) DeleteBotTool(ctx context.Context, tenantID, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM bot_tools WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// LogBotToolInvocation persiste el resultado de invocar una tool. Best-effort:
// si falla el insert, el llamador NO debe romper la respuesta al provider —
// la tool ya se ejecutó, queremos registrarla pero la conversación sigue.
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
		INSERT INTO bot_tool_invocations (id, tenant_id, call_id, bot_tool_id, tool_name, arguments, result, success, error)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8, $9)`,
		inv.ID, inv.TenantID, inv.CallID, inv.BotToolID, inv.ToolName, args, result, inv.Success, inv.Error)
	return err
}

// --- Helpers usados por la ejecución de tools ------------------------------
// Viven en este archivo porque solo aplican a tools por ahora; si se usan
// fuera mueven a mutations.go.

// UpdateCallOutcome cambia el outcome de una llamada (qualified, callback,
// completed, etc.). Sirve para que set_lead_outcome / end_call tools afecten
// directamente el histórico de la call sin tener que duplicar campos.
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

// UpdateLeadStatus cambia el status del lead (new → qualified, callback...).
// Atajo del UpdateLead más general — los tools solo cambian status, evitan
// el patch object completo.
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

// GetCampaign devuelve una campaign por id dentro del tenant. Usado por
// resolveBotIDForCall — necesitamos el bot_id que la campaign apunta.
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

// ListBotToolInvocations devuelve las últimas N invocaciones para una call.
// Lo usa el detalle de llamada para mostrar "el bot ejecutó X tools".
func (s *Store) ListBotToolInvocations(ctx context.Context, tenantID, callID string) ([]BotToolInvocation, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, call_id, bot_tool_id, tool_name, arguments, result, success, error, created_at
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
		if err := rows.Scan(&i.ID, &i.TenantID, &i.CallID, &i.BotToolID, &i.ToolName,
			&args, &result, &i.Success, &i.Error, &i.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(args, &i.Arguments)
		_ = json.Unmarshal(result, &i.Result)
		out = append(out, i)
	}
	return out, rows.Err()
}
