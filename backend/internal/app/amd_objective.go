package app

import "strings"

// amdAugmentObjective añade al objective del bot una instrucción extra
// para que el LLM reconozca un buzón de voz y suelte un mensaje. Solo
// se aplica si AMD está habilitado y la acción es drop_message con un
// mensaje configurado.
//
// La detección heurística del voice-agent corre en paralelo — esta
// instrucción es defensa en profundidad: si nuestro detector falla
// (silencios atípicos), el LLM todavía puede reconocer un greeting de
// buzón por contexto semántico ("Has llamado al teléfono…", "Deja tu
// mensaje tras la señal"). Si el LLM lo detecta, dice el mensaje y
// cuelga via tool end_call (si la tiene) o terminando la frase.
//
// Para hangup/continue/disabled: devolvemos el objective sin tocar.
func amdAugmentObjective(objective string, enabled bool, action, message string) string {
	if !enabled || action != "drop_message" || strings.TrimSpace(message) == "" {
		return objective
	}
	addon := "\n\nIMPORTANTE — Detección de buzón de voz:\n" +
		"Si al inicio de la llamada detectas que estás hablando con un buzón de voz " +
		"(greeting automático tipo \"Has llamado al teléfono…\", \"Deja tu mensaje tras la señal\", " +
		"\"El usuario al que llama no está disponible\", o cualquier locución pre-grabada larga sin pausas), " +
		"NO intentes mantener una conversación. Espera al pitido y di EXACTAMENTE este mensaje, después termina la llamada:\n\n" +
		"« " + strings.TrimSpace(message) + " »"
	return strings.TrimSpace(objective) + addon
}
