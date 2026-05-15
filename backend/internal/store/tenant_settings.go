package store

import (
	"context"
	"strings"
)

// GetTenantSettings returns the current settings row for a tenant. If the row is missing (e.g.
// the tenant was created after the seed migration ran) we lazily insert defaults so callers
// always see a populated record.
func (s *Store) GetTenantSettings(ctx context.Context, tenantID string) (TenantSettings, error) {
	var ts TenantSettings
	var startStr, endStr, daysStr string
	err := s.pool.QueryRow(ctx, `
		WITH ensured AS (
		  INSERT INTO tenant_settings (tenant_id)
		  VALUES ($1)
		  ON CONFLICT (tenant_id) DO NOTHING
		  RETURNING tenant_id
		)
		SELECT tenant_id, timezone, caller_id_default,
		       to_char(allowed_hours_start, 'HH24:MI'),
		       to_char(allowed_hours_end, 'HH24:MI'),
		       allowed_days, daily_call_cap, recording_enabled, recording_retention_days, updated_at
		FROM tenant_settings WHERE tenant_id = $1`, tenantID).
		Scan(&ts.TenantID, &ts.Timezone, &ts.CallerIDDefault,
			&startStr, &endStr,
			&daysStr, &ts.DailyCallCap, &ts.RecordingEnabled, &ts.RecordingRetentionDays, &ts.UpdatedAt)
	if err != nil {
		return ts, err
	}
	ts.AllowedHoursStart = startStr
	ts.AllowedHoursEnd = endStr
	ts.AllowedDays = splitDays(daysStr)
	return ts, nil
}

type TenantSettingsPatch struct {
	Timezone               *string
	CallerIDDefault        *string
	AllowedHoursStart      *string
	AllowedHoursEnd        *string
	AllowedDays            *[]string
	DailyCallCap           *int
	RecordingEnabled       *bool
	RecordingRetentionDays *int
}

// UpdateTenantSettings applies a partial patch. Always runs against an existing row (inserted
// lazily by GetTenantSettings if needed).
func (s *Store) UpdateTenantSettings(ctx context.Context, tenantID string, p TenantSettingsPatch) (TenantSettings, error) {
	// Ensure the row exists before patching.
	if _, err := s.GetTenantSettings(ctx, tenantID); err != nil {
		return TenantSettings{}, err
	}

	set := []string{"updated_at = now()"}
	args := []any{tenantID}
	addStr := func(col string, val *string) {
		if val == nil {
			return
		}
		args = append(args, *val)
		set = append(set, col+" = $"+itoa(len(args)))
	}
	addStr("timezone", p.Timezone)
	addStr("caller_id_default", p.CallerIDDefault)
	if p.AllowedHoursStart != nil {
		args = append(args, *p.AllowedHoursStart)
		set = append(set, "allowed_hours_start = $"+itoa(len(args))+"::time")
	}
	if p.AllowedHoursEnd != nil {
		args = append(args, *p.AllowedHoursEnd)
		set = append(set, "allowed_hours_end = $"+itoa(len(args))+"::time")
	}
	if p.AllowedDays != nil {
		args = append(args, joinDays(*p.AllowedDays))
		set = append(set, "allowed_days = $"+itoa(len(args)))
	}
	if p.DailyCallCap != nil {
		args = append(args, *p.DailyCallCap)
		set = append(set, "daily_call_cap = $"+itoa(len(args)))
	}
	if p.RecordingEnabled != nil {
		args = append(args, *p.RecordingEnabled)
		set = append(set, "recording_enabled = $"+itoa(len(args)))
	}
	if p.RecordingRetentionDays != nil {
		days := *p.RecordingRetentionDays
		if days < 0 {
			days = 0
		}
		args = append(args, days)
		set = append(set, "recording_retention_days = $"+itoa(len(args)))
	}

	q := "UPDATE tenant_settings SET " + strings.Join(set, ", ") + " WHERE tenant_id = $1"
	if _, err := s.pool.Exec(ctx, q, args...); err != nil {
		return TenantSettings{}, err
	}
	return s.GetTenantSettings(ctx, tenantID)
}

func splitDays(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func joinDays(days []string) string {
	clean := make([]string, 0, len(days))
	for _, d := range days {
		d = strings.TrimSpace(strings.ToLower(d))
		if d != "" {
			clean = append(clean, d)
		}
	}
	return strings.Join(clean, ",")
}
