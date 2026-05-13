// Package voicecatalog expone la lista de providers de voz soportados, con sus
// modelos y voces disponibles. Es ESTÁTICO (compilado dentro del binario) —
// cuando un provider añada una voz nueva basta con tocar este archivo y
// redeployar. No depende de la BD ni necesita migraciones.
//
// El frontend consume /api/voice-catalog para poblar los dropdowns del editor
// de bots: provider → modelo → voz.
package voicecatalog

// Option representa una opción en un dropdown: ID interno + etiqueta humana.
type Option struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Provider es la entrada de catálogo de un proveedor de voz.
//
// Cada proveedor tiene un conjunto de Modelos y Voces. Los dos pueden combinarse
// libremente (no hay restricciones cruzadas que valga la pena modelar aquí).
type Provider struct {
	ID     string   `json:"id"`     // "openai_realtime" | "deepgram" | "assemblyai"
	Label  string   `json:"label"`  // "OpenAI Realtime"
	Models []Option `json:"models"` // listen models (Deepgram) o realtime models (OpenAI)
	Voices []Option `json:"voices"`
	// ExtraFields documenta inputs adicionales que la UI debe pedir para este
	// provider (p.ej. Deepgram pide listen/think/speak por separado).
	ExtraFields []string `json:"extraFields,omitempty"`
}

// All es el catálogo completo. Versiona aquí cuando un provider actualice
// su lista — el frontend lo refresca solo.
var All = []Provider{
	{
		ID:    "openai_realtime",
		Label: "OpenAI Realtime",
		Models: []Option{
			{ID: "gpt-realtime", Label: "gpt-realtime (Sep 2025, recomendado)"},
			{ID: "gpt-4o-realtime-preview", Label: "gpt-4o-realtime-preview"},
			{ID: "gpt-4o-mini-realtime-preview", Label: "gpt-4o-mini-realtime-preview (más barato)"},
		},
		Voices: []Option{
			{ID: "alloy", Label: "Alloy (neutral)"},
			{ID: "ash", Label: "Ash (cálido)"},
			{ID: "ballad", Label: "Ballad (suave)"},
			{ID: "coral", Label: "Coral (cálido femenino)"},
			{ID: "echo", Label: "Echo (claro masculino)"},
			{ID: "sage", Label: "Sage (profesional)"},
			{ID: "shimmer", Label: "Shimmer (femenino)"},
			{ID: "verse", Label: "Verse (expresivo)"},
		},
	},
	{
		ID:    "deepgram",
		Label: "Deepgram Voice Agent",
		Models: []Option{
			// "listen" models — ASR
			{ID: "nova-3", Label: "Nova-3 (ASR multilenguaje)"},
			{ID: "nova-2", Label: "Nova-2"},
			{ID: "flux-general-en", Label: "Flux General EN (latencia baja)"},
		},
		Voices: []Option{
			// Aura-2 multilenguaje
			{ID: "aura-2-thalia-en", Label: "Aura-2 Thalia (EN, femenino cálido)"},
			{ID: "aura-2-celeste-es", Label: "Aura-2 Celeste (ES, femenino)"},
			{ID: "aura-2-arcas-en", Label: "Aura-2 Arcas (EN, masculino)"},
			{ID: "aura-2-asteria-en", Label: "Aura-2 Asteria (EN, femenino)"},
			{ID: "aura-2-luna-en", Label: "Aura-2 Luna (EN, femenino joven)"},
			{ID: "aura-2-orion-en", Label: "Aura-2 Orion (EN, masculino profundo)"},
			{ID: "aura-2-orpheus-en", Label: "Aura-2 Orpheus (EN, masculino)"},
			{ID: "aura-2-stella-en", Label: "Aura-2 Stella (EN, femenino joven)"},
			// Aura-1 legacy
			{ID: "aura-asteria-en", Label: "Aura Asteria (EN, legacy)"},
		},
		ExtraFields: []string{"think_provider", "think_model"},
	},
	{
		ID:    "assemblyai",
		Label: "AssemblyAI Voice Agent",
		Models: []Option{
			{ID: "universal", Label: "Universal Streaming"},
		},
		Voices: []Option{
			{ID: "ivy", Label: "Ivy (femenino)"},
			{ID: "james", Label: "James (masculino)"},
			{ID: "tyler", Label: "Tyler (masculino joven)"},
		},
	},
}
