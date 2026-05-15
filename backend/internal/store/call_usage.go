package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// CallUsage representa el desglose de consumo y coste de una llamada.
// Granularidad en micro-céntimos (1e-6 USD) para que las sumas grandes
// no pierdan precisión.
type CallUsage struct {
	CallID           string    `json:"callId"`
	TenantID         string    `json:"tenantId"`
	Provider         string    `json:"provider"`
	DurationSec      int       `json:"durationSec"`
	STTSeconds       int       `json:"sttSeconds"`
	LLMInputTokens   int       `json:"llmInputTokens"`
	LLMOutputTokens  int       `json:"llmOutputTokens"`
	TTSChars         int       `json:"ttsChars"`
	TTSSeconds       int       `json:"ttsSeconds"`
	STTMicroCents    int64     `json:"sttMicroCents"`
	LLMMicroCents    int64     `json:"llmMicroCents"`
	TTSMicroCents    int64     `json:"ttsMicroCents"`
	TrunkMicroCents  int64     `json:"trunkMicroCents"`
	OtherMicroCents  int64     `json:"otherMicroCents"`
	TotalMicroCents  int64     `json:"totalMicroCents"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// ProviderRate es una fila de la tabla global de tarifas por componente.
type ProviderRate struct {
	Provider          string `json:"provider"`
	Component         string `json:"component"`
	Unit              string `json:"unit"`
	MicroCentsPerUnit int64  `json:"microCentsPerUnit"`
}

// UpsertCallUsage inserta o actualiza el row 1:1 con la call. Lo llama
// el handler internal cuando el voice-agent reporta usage al cerrar.
// total_micro_cents se calcula como suma del breakdown (el caller ya
// debe haber hecho las multiplicaciones por las tarifas).
func (s *Store) UpsertCallUsage(ctx context.Context, u CallUsage) error {
	u.TotalMicroCents = u.STTMicroCents + u.LLMMicroCents + u.TTSMicroCents + u.TrunkMicroCents + u.OtherMicroCents
	_, err := s.pool.Exec(ctx, `
		INSERT INTO call_usage (
			call_id, tenant_id, provider, duration_sec,
			stt_seconds, llm_input_tokens, llm_output_tokens, tts_chars, tts_seconds,
			stt_micro_cents, llm_micro_cents, tts_micro_cents, trunk_micro_cents,
			other_micro_cents, total_micro_cents, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, now())
		ON CONFLICT (call_id) DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id,
			provider = EXCLUDED.provider,
			duration_sec = EXCLUDED.duration_sec,
			stt_seconds = EXCLUDED.stt_seconds,
			llm_input_tokens = EXCLUDED.llm_input_tokens,
			llm_output_tokens = EXCLUDED.llm_output_tokens,
			tts_chars = EXCLUDED.tts_chars,
			tts_seconds = EXCLUDED.tts_seconds,
			stt_micro_cents = EXCLUDED.stt_micro_cents,
			llm_micro_cents = EXCLUDED.llm_micro_cents,
			tts_micro_cents = EXCLUDED.tts_micro_cents,
			trunk_micro_cents = EXCLUDED.trunk_micro_cents,
			other_micro_cents = EXCLUDED.other_micro_cents,
			total_micro_cents = EXCLUDED.total_micro_cents,
			updated_at = now()`,
		u.CallID, u.TenantID, u.Provider, u.DurationSec,
		u.STTSeconds, u.LLMInputTokens, u.LLMOutputTokens, u.TTSChars, u.TTSSeconds,
		u.STTMicroCents, u.LLMMicroCents, u.TTSMicroCents, u.TrunkMicroCents,
		u.OtherMicroCents, u.TotalMicroCents)
	return err
}

// GetCallUsage devuelve el row de una llamada concreta (UI de detalle).
func (s *Store) GetCallUsage(ctx context.Context, tenantID, callID string) (CallUsage, error) {
	var u CallUsage
	err := s.pool.QueryRow(ctx, `
		SELECT call_id, tenant_id, provider, duration_sec,
		       stt_seconds, llm_input_tokens, llm_output_tokens, tts_chars, tts_seconds,
		       stt_micro_cents, llm_micro_cents, tts_micro_cents, trunk_micro_cents,
		       other_micro_cents, total_micro_cents, created_at, updated_at
		FROM call_usage WHERE tenant_id = $1 AND call_id = $2`, tenantID, callID).
		Scan(&u.CallID, &u.TenantID, &u.Provider, &u.DurationSec,
			&u.STTSeconds, &u.LLMInputTokens, &u.LLMOutputTokens, &u.TTSChars, &u.TTSSeconds,
			&u.STTMicroCents, &u.LLMMicroCents, &u.TTSMicroCents, &u.TrunkMicroCents,
			&u.OtherMicroCents, &u.TotalMicroCents, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

// ListProviderRates devuelve todas las filas de provider_rates. Vacío =
// el caller cae a los defaults del paquete pricing (flat cents/min).
func (s *Store) ListProviderRates(ctx context.Context) ([]ProviderRate, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT provider, component, unit, micro_cents_per_unit
		FROM provider_rates`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProviderRate{}
	for rows.Next() {
		var r ProviderRate
		if err := rows.Scan(&r.Provider, &r.Component, &r.Unit, &r.MicroCentsPerUnit); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// BillingSummaryRow es una fila del agregado del dashboard. Una fila por
// día × bucket (campaign|bot|provider, según el query del caller).
type BillingSummaryRow struct {
	Day             time.Time `json:"day"`
	BucketID        string    `json:"bucketId"`
	BucketLabel     string    `json:"bucketLabel"`
	Calls           int       `json:"calls"`
	DurationSec     int64     `json:"durationSec"`
	TotalMicroCents int64     `json:"totalMicroCents"`
}

// BillingSummary agrupa call_usage por día y por la dimensión que pida
// el caller (campaign|bot|provider|none). Devuelve hasta los últimos
// rangeDays días (default 30). Útil para el dashboard /portal/billing.
func (s *Store) BillingSummary(ctx context.Context, tenantID, groupBy string, from, to time.Time) ([]BillingSummaryRow, error) {
	// Whitelist de columnas para evitar SQL injection en groupBy.
	// Por ahora "bot" no está disponible — calls no tiene bot_id directo;
	// futura iteración: lookup via campaigns.bot_id.
	var bucketSQL string
	switch groupBy {
	case "campaign":
		bucketSQL = "COALESCE(c.campaign_id, '')"
	case "provider":
		bucketSQL = "COALESCE(u.provider, '')"
	default:
		bucketSQL = "''"
	}
	q := `
		SELECT date_trunc('day', u.created_at) AS day,
		       ` + bucketSQL + ` AS bucket_id,
		       '' AS bucket_label,
		       COUNT(*) AS calls,
		       COALESCE(SUM(u.duration_sec), 0) AS duration_sec,
		       COALESCE(SUM(u.total_micro_cents), 0) AS total_micro_cents
		FROM call_usage u
		JOIN calls c ON c.id = u.call_id
		WHERE u.tenant_id = $1 AND u.created_at >= $2 AND u.created_at < $3
		GROUP BY day, bucket_id
		ORDER BY day ASC, bucket_id ASC`
	rows, err := s.pool.Query(ctx, q, tenantID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BillingSummaryRow{}
	for rows.Next() {
		var r BillingSummaryRow
		if err := rows.Scan(&r.Day, &r.BucketID, &r.BucketLabel, &r.Calls, &r.DurationSec, &r.TotalMicroCents); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
