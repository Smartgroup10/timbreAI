package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ─── CRUD reglas ────────────────────────────────────────────────────────

// ListDIDRoutingRules devuelve todas las reglas del DID con info del bot
// target/fallback para que la UI pueda pintar nombres sin hacer N+1.
func (s *Store) ListDIDRoutingRules(ctx context.Context, tenantID, didID string) ([]DIDRoutingRule, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.tenant_id, r.did_id, r.name, r.priority, r.enabled,
		       r.timezone, r.days_of_week, r.start_minute, r.end_minute,
		       r.caller_prefixes, r.language, r.target_bot_id, r.fallback_bot_id,
		       r.created_at, r.updated_at,
		       COALESCE(bt.name, ''), COALESCE(bf.name, '')
		FROM did_routing_rules r
		LEFT JOIN bots bt ON bt.id = r.target_bot_id
		LEFT JOIN bots bf ON bf.id = r.fallback_bot_id
		WHERE r.tenant_id = $1 AND r.did_id = $2
		ORDER BY r.priority ASC, r.created_at ASC`, tenantID, didID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DIDRoutingRule{}
	for rows.Next() {
		r, err := scanDIDRoutingRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetDIDRoutingRule(ctx context.Context, tenantID, id string) (DIDRoutingRule, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT r.id, r.tenant_id, r.did_id, r.name, r.priority, r.enabled,
		       r.timezone, r.days_of_week, r.start_minute, r.end_minute,
		       r.caller_prefixes, r.language, r.target_bot_id, r.fallback_bot_id,
		       r.created_at, r.updated_at,
		       COALESCE(bt.name, ''), COALESCE(bf.name, '')
		FROM did_routing_rules r
		LEFT JOIN bots bt ON bt.id = r.target_bot_id
		LEFT JOIN bots bf ON bf.id = r.fallback_bot_id
		WHERE r.tenant_id = $1 AND r.id = $2`, tenantID, id)
	r, err := scanDIDRoutingRule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

func (s *Store) CreateDIDRoutingRule(ctx context.Context, r DIDRoutingRule) (DIDRoutingRule, error) {
	if r.ID == "" {
		r.ID = newID("ddrule")
	}
	if r.Timezone == "" {
		r.Timezone = "Europe/Madrid"
	}
	if r.DaysOfWeek == nil {
		r.DaysOfWeek = []int{}
	}
	if r.CallerPrefixes == nil {
		r.CallerPrefixes = []string{}
	}
	// Verificar que DID y bots pertenecen al tenant antes de insertar —
	// payload malicioso podría intentar enlazar bots de otro tenant.
	var didTenant string
	if err := s.pool.QueryRow(ctx, `SELECT tenant_id FROM dids WHERE id = $1`, r.DIDID).Scan(&didTenant); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return r, ErrNotFound
		}
		return r, err
	}
	if didTenant != r.TenantID {
		return r, ErrNotFound
	}
	if !s.botInTenant(ctx, r.TenantID, r.TargetBotID) {
		return r, ErrNotFound
	}
	if r.FallbackBotID != nil && *r.FallbackBotID != "" {
		if !s.botInTenant(ctx, r.TenantID, *r.FallbackBotID) {
			return r, ErrNotFound
		}
	}

	err := s.pool.QueryRow(ctx, `
		INSERT INTO did_routing_rules
		  (id, tenant_id, did_id, name, priority, enabled, timezone,
		   days_of_week, start_minute, end_minute, caller_prefixes, language,
		   target_bot_id, fallback_bot_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING created_at, updated_at`,
		r.ID, r.TenantID, r.DIDID, r.Name, r.Priority, r.Enabled, r.Timezone,
		smallintArray(r.DaysOfWeek), r.StartMinute, r.EndMinute,
		stringArray(r.CallerPrefixes), r.Language,
		r.TargetBotID, r.FallbackBotID).
		Scan(&r.CreatedAt, &r.UpdatedAt)
	return r, err
}

func (s *Store) UpdateDIDRoutingRule(ctx context.Context, r DIDRoutingRule) (DIDRoutingRule, error) {
	if r.DaysOfWeek == nil {
		r.DaysOfWeek = []int{}
	}
	if r.CallerPrefixes == nil {
		r.CallerPrefixes = []string{}
	}
	if r.Timezone == "" {
		r.Timezone = "Europe/Madrid"
	}
	if !s.botInTenant(ctx, r.TenantID, r.TargetBotID) {
		return r, ErrNotFound
	}
	if r.FallbackBotID != nil && *r.FallbackBotID != "" {
		if !s.botInTenant(ctx, r.TenantID, *r.FallbackBotID) {
			return r, ErrNotFound
		}
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE did_routing_rules
		SET name = $3, priority = $4, enabled = $5, timezone = $6,
		    days_of_week = $7, start_minute = $8, end_minute = $9,
		    caller_prefixes = $10, language = $11,
		    target_bot_id = $12, fallback_bot_id = $13, updated_at = now()
		WHERE tenant_id = $1 AND id = $2`,
		r.TenantID, r.ID, r.Name, r.Priority, r.Enabled, r.Timezone,
		smallintArray(r.DaysOfWeek), r.StartMinute, r.EndMinute,
		stringArray(r.CallerPrefixes), r.Language,
		r.TargetBotID, r.FallbackBotID)
	if err != nil {
		return r, err
	}
	if tag.RowsAffected() == 0 {
		return r, ErrNotFound
	}
	return s.GetDIDRoutingRule(ctx, r.TenantID, r.ID)
}

func (s *Store) DeleteDIDRoutingRule(ctx context.Context, tenantID, id string) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM did_routing_rules WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) botInTenant(ctx context.Context, tenantID, botID string) bool {
	if botID == "" {
		return false
	}
	var ok bool
	_ = s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM bots WHERE id = $1 AND tenant_id = $2)`,
		botID, tenantID).Scan(&ok)
	return ok
}

// ─── Resolución de routing (path caliente al recibir inbound) ──────────

// ResolveDIDRouting decide qué bot atiende una inbound al DID. Si alguna
// regla matchea las condiciones (caller, hora, día, idioma) usa su
// target_bot_id. Si no, fallback al bot default del DID via bots.did_id.
//
// at puede venir IsZero — usamos now() en ese caso. Útil para tests.
func (s *Store) ResolveDIDRouting(ctx context.Context, tenantID, didID, callerNumber, language string, at time.Time) (RoutingDecision, error) {
	if at.IsZero() {
		at = time.Now()
	}
	rules, err := s.ListDIDRoutingRules(ctx, tenantID, didID)
	if err != nil {
		return RoutingDecision{}, err
	}
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		if !ruleMatches(r, callerNumber, language, at) {
			continue
		}
		ruleID := r.ID
		return RoutingDecision{
			MatchedRuleID: &ruleID,
			MatchedRule:   r.Name,
			BotID:         r.TargetBotID,
			Reason:        "matched_rule",
		}, nil
	}
	// Fallback al bot default del DID.
	var defaultBot string
	if err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(b.id, '') FROM bots b WHERE b.did_id = $1 AND b.tenant_id = $2 LIMIT 1`,
		didID, tenantID).Scan(&defaultBot); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return RoutingDecision{}, err
	}
	if defaultBot != "" {
		return RoutingDecision{BotID: defaultBot, Reason: "default_did_bot"}, nil
	}
	return RoutingDecision{Reason: "no_route"}, nil
}

// ruleMatches evalúa las condiciones de una regla contra los parámetros
// de la llamada entrante. Patrón inspirado en SmartSIP.
func ruleMatches(r DIDRoutingRule, callerNumber, language string, at time.Time) bool {
	loc, err := time.LoadLocation(r.Timezone)
	if err != nil {
		loc = time.UTC
	}
	local := at.In(loc)

	// Días de la semana — vacío significa "todos".
	if len(r.DaysOfWeek) > 0 {
		w := int(local.Weekday())
		match := false
		for _, d := range r.DaysOfWeek {
			if d == w {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	// Rango horario — ambos NULL = sin restricción.
	if r.StartMinute != nil && r.EndMinute != nil {
		minute := local.Hour()*60 + local.Minute()
		start := *r.StartMinute
		end := *r.EndMinute
		if start <= end {
			// Ventana normal: [start, end). End exclusivo.
			if minute < start || minute >= end {
				return false
			}
		} else {
			// Ventana overnight (e.g. 22:00 → 06:00 del día siguiente).
			// Matchea si minute >= start O minute < end.
			if minute < start && minute >= end {
				return false
			}
		}
	}

	// Prefijos del caller. Vacío = cualquier número.
	if len(r.CallerPrefixes) > 0 {
		caller := strings.TrimSpace(callerNumber)
		match := false
		for _, p := range r.CallerPrefixes {
			p = strings.TrimSpace(p)
			if p != "" && strings.HasPrefix(caller, p) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	// Idioma — vacío = cualquiera.
	if strings.TrimSpace(r.Language) != "" {
		if !strings.EqualFold(strings.TrimSpace(r.Language), strings.TrimSpace(language)) {
			return false
		}
	}

	return true
}

// scanDIDRoutingRule reusa la firma rowScanner definida en tools.go.
func scanDIDRoutingRule(r rowScanner) (DIDRoutingRule, error) {
	var rule DIDRoutingRule
	var days []int16
	var prefixes []string
	if err := r.Scan(&rule.ID, &rule.TenantID, &rule.DIDID, &rule.Name, &rule.Priority, &rule.Enabled,
		&rule.Timezone, &days, &rule.StartMinute, &rule.EndMinute,
		&prefixes, &rule.Language, &rule.TargetBotID, &rule.FallbackBotID,
		&rule.CreatedAt, &rule.UpdatedAt,
		&rule.TargetBotName, &rule.FallbackBotName); err != nil {
		return rule, err
	}
	rule.DaysOfWeek = int16sToInts(days)
	rule.CallerPrefixes = prefixes
	return rule, nil
}

// Helpers para pasar []int → smallint[] y []string → text[] al driver.
// pgx5 acepta slices directamente pero necesitamos el tipo concreto
// para que Postgres no rechace el binding.

func smallintArray(xs []int) []int16 {
	out := make([]int16, len(xs))
	for i, x := range xs {
		out[i] = int16(x)
	}
	return out
}

func stringArray(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}

func int16sToInts(xs []int16) []int {
	out := make([]int, len(xs))
	for i, x := range xs {
		out[i] = int(x)
	}
	return out
}
