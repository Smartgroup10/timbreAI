package store

import (
	"context"
	"time"
)

// Analytics is the per-tenant snapshot returned by the analytics endpoint. Time series are
// always 7 daily buckets ending today; counts are aligned in the tenant's timezone.
type Analytics struct {
	Generated   time.Time          `json:"generatedAt"`
	Timezone    string             `json:"timezone"`
	Last7Days   []DailyCount       `json:"last7Days"`
	Outcomes    []CountByLabel     `json:"outcomes"`
	Statuses    []CountByLabel     `json:"statuses"`
	TopBots     []CountByLabel     `json:"topBots"`
	TopCampaigns []CountByLabel    `json:"topCampaigns"`
	TotalsLast7 int                `json:"totalsLast7"`
	TotalsPrev7 int                `json:"totalsPrev7"`
}

type DailyCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type CountByLabel struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

func (s *Store) BuildAnalytics(ctx context.Context, tenantID, timezone string) (Analytics, error) {
	if timezone == "" {
		timezone = "UTC"
	}
	out := Analytics{Generated: time.Now().UTC(), Timezone: timezone}

	// 7-day series in the tenant timezone. We build a complete series (zero-filled) by generating
	// the bucket dates in Go and joining against the SQL result so empty days still show up.
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	today := time.Now().In(loc)
	dates := make([]string, 7)
	for i := 0; i < 7; i++ {
		dates[i] = today.AddDate(0, 0, -6+i).Format("2006-01-02")
	}

	rows, err := s.pool.Query(ctx, `
		SELECT to_char((COALESCE(started_at, created_at) AT TIME ZONE $2)::date, 'YYYY-MM-DD') AS d,
		       count(*)
		FROM calls
		WHERE tenant_id = $1 AND COALESCE(started_at, created_at) >= now() - interval '6 days'
		GROUP BY d`, tenantID, timezone)
	if err != nil {
		return out, err
	}
	counts := map[string]int{}
	for rows.Next() {
		var d string
		var n int
		if err := rows.Scan(&d, &n); err != nil {
			rows.Close()
			return out, err
		}
		counts[d] = n
	}
	rows.Close()
	for _, d := range dates {
		out.Last7Days = append(out.Last7Days, DailyCount{Date: d, Count: counts[d]})
		out.TotalsLast7 += counts[d]
	}

	// Previous 7-day total for the delta indicator.
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM calls
		WHERE tenant_id = $1
		  AND COALESCE(started_at, created_at) >= now() - interval '13 days'
		  AND COALESCE(started_at, created_at) <  now() - interval '6 days'`, tenantID).Scan(&out.TotalsPrev7); err != nil {
		return out, err
	}

	// Outcome / status breakdowns over the last 30 days.
	if out.Outcomes, err = s.bucket(ctx, `
		SELECT outcome, count(*) FROM calls
		WHERE tenant_id = $1 AND COALESCE(started_at, created_at) >= now() - interval '30 days'
		GROUP BY outcome ORDER BY count(*) DESC LIMIT 10`, tenantID); err != nil {
		return out, err
	}
	if out.Statuses, err = s.bucket(ctx, `
		SELECT status, count(*) FROM calls
		WHERE tenant_id = $1 AND COALESCE(started_at, created_at) >= now() - interval '30 days'
		GROUP BY status ORDER BY count(*) DESC LIMIT 10`, tenantID); err != nil {
		return out, err
	}

	// Top 5 bots / campaigns by call volume in the last 30 days.
	if out.TopBots, err = s.bucket(ctx, `
		SELECT b.name, count(*) FROM calls c
		JOIN campaigns cmp ON cmp.id = c.campaign_id
		JOIN bots b ON b.id = cmp.bot_id
		WHERE c.tenant_id = $1 AND COALESCE(c.started_at, c.created_at) >= now() - interval '30 days'
		GROUP BY b.name ORDER BY count(*) DESC LIMIT 5`, tenantID); err != nil {
		return out, err
	}
	if out.TopCampaigns, err = s.bucket(ctx, `
		SELECT campaign_name, count(*) FROM calls
		WHERE tenant_id = $1 AND campaign_name <> ''
		  AND COALESCE(started_at, created_at) >= now() - interval '30 days'
		GROUP BY campaign_name ORDER BY count(*) DESC LIMIT 5`, tenantID); err != nil {
		return out, err
	}
	return out, nil
}

func (s *Store) bucket(ctx context.Context, query, tenantID string) ([]CountByLabel, error) {
	rows, err := s.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CountByLabel{}
	for rows.Next() {
		var r CountByLabel
		if err := rows.Scan(&r.Label, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
