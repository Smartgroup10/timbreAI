package pricing

import (
	"strings"
)

// Detailed calcula el coste por componente (STT/LLM/TTS) en
// micro-céntimos (1e-6 USD). Recibe los contadores que reporta el
// voice-agent (segundos, tokens, chars) y un lookup de tarifas que el
// caller carga al inicio (de provider_rates o defaults).
//
// La estrategia: si tenemos tarifa específica para (provider, component)
// la usamos; si no, el caller puede caer al cents/min flat del paquete
// pricing original (Cost). Así, providers sin tarifa específica siguen
// mostrando un coste estimado en el dashboard.
type Detailed struct {
	rates map[string]map[string]Rate // provider → component → rate
}

// Rate es una tarifa unitaria. unit ∈ {sec, min, token, 1k_token, char, 1k_char}.
type Rate struct {
	Unit              string
	MicroCentsPerUnit int64
}

// NewDetailed construye el lookup a partir de una lista plana de
// (provider, component, unit, microCentsPerUnit). Soporta override
// por env vars del paquete original (compatibility).
func NewDetailed(entries []DetailedEntry) *Detailed {
	d := &Detailed{rates: map[string]map[string]Rate{}}
	for _, e := range entries {
		if _, ok := d.rates[e.Provider]; !ok {
			d.rates[e.Provider] = map[string]Rate{}
		}
		d.rates[e.Provider][e.Component] = Rate{
			Unit:              e.Unit,
			MicroCentsPerUnit: e.MicroCentsPerUnit,
		}
	}
	return d
}

// DetailedEntry es el shape para alimentar NewDetailed desde la BD.
type DetailedEntry struct {
	Provider          string
	Component         string
	Unit              string
	MicroCentsPerUnit int64
}

// CostByComponent calcula el coste de un único componente (stt/llm_input/
// llm_output/tts) dado el contador (segundos, tokens, chars). Devuelve
// micro-céntimos. Si no hay tarifa configurada, devuelve 0.
func (d *Detailed) CostByComponent(provider, component string, count int) int64 {
	if d == nil || count <= 0 {
		return 0
	}
	provRates, ok := d.rates[provider]
	if !ok {
		return 0
	}
	rate, ok := provRates[component]
	if !ok {
		return 0
	}
	return applyUnit(rate, int64(count))
}

// applyUnit convierte count + unit en micro-céntimos.
//
//	sec       → count * micro_cents
//	min       → count / 60 * micro_cents (count en segundos)
//	token     → count * micro_cents
//	1k_token  → count / 1000 * micro_cents
//	char      → count * micro_cents
//	1k_char   → count / 1000 * micro_cents
func applyUnit(r Rate, count int64) int64 {
	switch strings.ToLower(r.Unit) {
	case "sec":
		return count * r.MicroCentsPerUnit
	case "min":
		return (count * r.MicroCentsPerUnit) / 60
	case "token", "char":
		return count * r.MicroCentsPerUnit
	case "1k_token", "1k_char":
		return (count * r.MicroCentsPerUnit) / 1000
	default:
		return 0
	}
}
