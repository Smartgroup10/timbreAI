package api

import (
	"context"
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"strings"

	"atrium-calls/backend/internal/store"
)

type importResult struct {
	Created int      `json:"created"`
	Skipped int      `json:"skipped"`
	Invalid int      `json:"invalid"`
	Errors  []string `json:"errors,omitempty"`
}

// handleImportLeads accepts a CSV body and creates leads from it. Limits:
//   - 5 MB max body
//   - First row must be a header with column names; supported: name, phone, email, type, source, consent
//   - name and phone are required; rows without them count as invalid
//   - Phones already in the tenant's do_not_call list are skipped
//   - Phones that match an existing lead (same tenant + phone) are skipped as duplicates
//
// Headers can come in any order; case-insensitive matching.
func (s *Server) handleImportLeads(w http.ResponseWriter, r *http.Request) {
	tenantID, err := s.tenantScope(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5*1024*1024)

	reader := csv.NewReader(r.Body)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing_header")
		return
	}
	idx := indexHeaders(header)
	if idx["name"] < 0 || idx["phone"] < 0 {
		writeError(w, http.StatusBadRequest, "header_requires_name_and_phone")
		return
	}

	result := importResult{Errors: []string{}}
	existingPhones, err := s.loadTenantPhones(r.Context(), tenantID)
	if err != nil {
		s.logger.Error("import: load phones", "error", err)
		writeError(w, http.StatusInternalServerError, "load_failed")
		return
	}

	lineNo := 1
	for {
		lineNo++
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			if errors.Is(err, http.ErrBodyReadAfterClose) {
				break
			}
			result.Invalid++
			if len(result.Errors) < 20 {
				result.Errors = append(result.Errors, "line "+itoa(lineNo)+": "+err.Error())
			}
			continue
		}
		name := strings.TrimSpace(field(row, idx, "name"))
		phone := strings.TrimSpace(field(row, idx, "phone"))
		if name == "" || phone == "" {
			result.Invalid++
			continue
		}
		if _, ok := existingPhones[phone]; ok {
			result.Skipped++
			continue
		}
		blocked, _ := s.store.IsBlockedPhone(r.Context(), tenantID, phone)
		if blocked {
			result.Skipped++
			continue
		}

		lead := store.Lead{
			TenantID: tenantID,
			Name:     name,
			Phone:    phone,
			Email:    strings.TrimSpace(field(row, idx, "email")),
			Type:     defaultStr(field(row, idx, "type"), "renter"),
			Source:   defaultStr(field(row, idx, "source"), "import"),
			Consent:  defaultStr(field(row, idx, "consent"), "imported"),
			Status:   "new",
		}
		if _, err := s.store.CreateLead(r.Context(), lead); err != nil {
			result.Invalid++
			if len(result.Errors) < 20 {
				result.Errors = append(result.Errors, "line "+itoa(lineNo)+": "+err.Error())
			}
			continue
		}
		existingPhones[phone] = struct{}{}
		result.Created++
	}

	s.audit(r, "leads.import", "leads", "csv", map[string]any{
		"created": result.Created, "skipped": result.Skipped, "invalid": result.Invalid,
	})
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) loadTenantPhones(ctx context.Context, tenantID string) (map[string]struct{}, error) {
	leads, err := s.store.ListLeads(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(leads))
	for _, l := range leads {
		out[strings.TrimSpace(l.Phone)] = struct{}{}
	}
	return out, nil
}

func indexHeaders(row []string) map[string]int {
	idx := map[string]int{"name": -1, "phone": -1, "email": -1, "type": -1, "source": -1, "consent": -1}
	for i, h := range row {
		key := strings.ToLower(strings.TrimSpace(h))
		if _, ok := idx[key]; ok {
			idx[key] = i
		}
	}
	return idx
}

func field(row []string, idx map[string]int, key string) string {
	i := idx[key]
	if i < 0 || i >= len(row) {
		return ""
	}
	return row[i]
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
