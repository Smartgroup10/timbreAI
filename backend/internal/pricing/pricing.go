// Package pricing computes the estimated cost of a voice call.
//
// Models the cost as duration_sec × rate where rate is a per-provider
// constant expressed in centi-cents per second (i.e. millicents per
// second, 1e-5 USD/s). We keep the unit small so that 30 seconds of a
// $0.30/min provider rounds cleanly: 30s × 30_000_000 / 60_000_000 = 15
// (cents). Float math is avoided on purpose — money should never round
// twice.
//
// Rates default to public list prices at time of writing; override via
// env vars (PRICING_<PROVIDER>_CENTS_PER_MIN, integer). Real billing
// should use the provider's metering API for accuracy; this is a
// "estimate cost on the dashboard" not "charge the customer".
package pricing

import (
	"os"
	"strconv"
	"strings"
)

// Table holds the rate (cents per minute) per provider id. Cents are
// integers so cost arithmetic is exact. Use Cost() to compute per-call.
type Table struct {
	rates map[string]int // provider → cents per minute
}

// NewTable builds the pricing table, reading optional overrides from env.
// Defaults match the public list prices at time of writing for the
// providers we integrate; tweak in env when they change.
//
//	PRICING_OPENAI_REALTIME_CENTS_PER_MIN=30
//	PRICING_DEEPGRAM_CENTS_PER_MIN=8
//	PRICING_ASSEMBLYAI_CENTS_PER_MIN=10
//	PRICING_ECHO_CENTS_PER_MIN=0
func NewTable() *Table {
	t := &Table{rates: map[string]int{
		// gpt-realtime GA (ago 2025): $32/1M audio in + $64/1M audio out.
		// A ~31 tok/sec → ~$0.06/min in + ~$0.12/min out. En conversación
		// realista (un lado habla a la vez) ≈ $0.18-0.24/min. Usamos 24
		// como estimación conservadora coherente con el 20% de descuento
		// vs gpt-4o-realtime-preview que era 30 c/min.
		"openai_realtime": 24,
		// Deepgram Voice Agent — Standard tier Pay As You Go: $0.075/min.
		// Lo redondeamos a 8 c/min hacia arriba (8 c × 60s/60 = 8 c/min ≈
		// $0.08, ligeramente conservador). Si el operador usa el tier
		// Custom BYO LLM ($0.056) o Advanced ($0.163), debería ajustar
		// PRICING_DEEPGRAM_CENTS_PER_MIN por env. NOTA: con BYO LLM además
		// se paga aparte al LLM provider (OpenAI/Anthropic) — eso no entra
		// en este flat rate.
		"deepgram": 8,
		// AssemblyAI Voice Agent API: $4.50/hr = $0.075/min Pay As You Go,
		// igual que Deepgram Standard. Custom tier por contrato (variable).
		// El antiguo valor 10 c/min era una estimación obsoleta.
		"assemblyai": 8,
		// ElevenLabs Conversational AI Agents: $0.08-$0.12/min según
		// LLM del agente. Default 10 c/min (mid-range). Para agentes que
		// usan GPT-4o premium debería ser 12 c/min; los voice-only 8 c/min.
		"elevenlabs": 10,
		"echo":       0, // sandbox local, no provider cost
	}}
	for prov := range t.rates {
		envKey := "PRICING_" + strings.ToUpper(prov) + "_CENTS_PER_MIN"
		if raw := os.Getenv(envKey); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
				t.rates[prov] = n
			}
		}
	}
	return t
}

// Cost returns the estimated cost of a call in cents, given the provider
// and total duration in seconds. Unknown providers return 0 (don't fail
// — we'd rather under-report than block the call list from rendering).
//
// Math: durationSec × centsPerMin / 60. Integer division; we always
// round DOWN to avoid over-stating cost to the customer.
func (t *Table) Cost(provider string, durationSec int) int {
	if durationSec <= 0 || t == nil {
		return 0
	}
	rate, ok := t.rates[provider]
	if !ok {
		return 0
	}
	return (durationSec * rate) / 60
}

// CentsPerMin returns the configured rate for a provider, or 0 if unknown.
// Exposed for UI display ("Tarifa: 30¢/min").
func (t *Table) CentsPerMin(provider string) int {
	if t == nil {
		return 0
	}
	return t.rates[provider]
}

// All returns a snapshot of the rates map, for /api/pricing exposure.
func (t *Table) All() map[string]int {
	if t == nil {
		return nil
	}
	out := make(map[string]int, len(t.rates))
	for k, v := range t.rates {
		out[k] = v
	}
	return out
}
