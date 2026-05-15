// Package amd implementa Answering Machine Detection sobre el audio
// entrante (slin 8 kHz mono, 16-bit, frames de 20ms = 320 B = 160 samples).
//
// Estrategia (heurística, no ML — barato y suficiente para outbound):
//
// Observamos los primeros DecisionWindow segundos del audio recibido y
// medimos:
//   - voicedRatio: % de frames con energía RMS por encima del umbral.
//   - firstBurst: duración del primer tramo continuo de voz antes de un
//     silencio significativo (≥ SilenceGap).
//   - totalVoiced: tiempo total con voz acumulado.
//
// Reglas:
//   - firstBurst ≥ MachineBurstSecs → machine (greeting típico de buzón
//     "Has llamado a 6XX… deja tu mensaje tras la señal" suelen ser
//     locuciones largas sin pausa).
//   - voicedRatio ≥ MachineVoicedRatio (en toda la ventana) → machine
//     (no para de hablar).
//   - firstBurst ≤ HumanBurstSecs y totalVoiced ≤ MachineBurstSecs →
//     human (dice "Hola?" corto y espera).
//   - Si pasa la ventana entera sin voz: unknown (cuelga el bot o llama
//     a un buzón silencioso — el caller decide qué hacer).
//
// Es heurística — los buzones de Movistar/Vodafone son distintos al de
// Orange, y un "Hola, ¿quién es? ¿Está? No te oigo" puede parecer
// machine. Más adelante podríamos entrenar un clasificador, pero esto
// captura el 80% de los casos sin coste extra.
package amd

import (
	"math"
	"sync"
	"time"
)

// Result es el veredicto del detector.
type Result string

const (
	ResultHuman   Result = "human"
	ResultMachine Result = "machine"
	ResultUnknown Result = "unknown"
)

// Constantes sintonizadas a oído para slin 8 kHz. Si vemos muchos
// falsos positivos en producción, las exponemos por tenant.
const (
	SampleRate          = 8000
	FrameSamples        = 160  // 20ms
	EnergyThreshold     = 600  // RMS por debajo de esto = silencio.
	SilenceGapMs        = 400  // gap consecutivo que cierra un "burst" de voz.
	DecisionWindowMs    = 4500 // ventana máxima antes de forzar veredicto.
	MinDecisionMs       = 1500 // antes no decidimos para evitar ruido.
	MachineBurstMs      = 2200 // burst >= esto → machine.
	HumanBurstMs        = 1300 // burst <= esto y queda silencio → human.
	MachineVoicedRatio  = 0.75 // % voiced en la ventana → machine.
)

// Detector mantiene el estado a lo largo de la llamada hasta decidir.
// Es thread-safe — el audiosocket lo alimenta desde el read loop y el
// dueño consulta Result() desde otra goroutine.
type Detector struct {
	mu             sync.Mutex
	startedAt      time.Time
	totalFrames    int
	voicedFrames   int
	currentBurstMs int
	maxBurstMs     int
	silenceRunMs   int
	hasResult      bool
	result         Result
}

func New() *Detector {
	return &Detector{startedAt: time.Now()}
}

// FeedPCM consume un frame de 20ms (160 samples slin 8 kHz LE).
// Si el detector ya tiene veredicto, ignora silenciosamente.
func (d *Detector) FeedPCM(pcm []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.hasResult {
		return
	}
	d.totalFrames++
	voiced := frameVoiced(pcm)
	if voiced {
		d.voicedFrames++
		d.currentBurstMs += 20
		d.silenceRunMs = 0
	} else {
		d.silenceRunMs += 20
		// Si tenemos un burst en curso y el silencio supera el gap,
		// lo cerramos y lo guardamos como candidato a maxBurst.
		if d.currentBurstMs > 0 && d.silenceRunMs >= SilenceGapMs {
			if d.currentBurstMs > d.maxBurstMs {
				d.maxBurstMs = d.currentBurstMs
			}
			d.currentBurstMs = 0
		}
	}
	// Decidir si tenemos suficientes datos.
	elapsedMs := d.totalFrames * 20
	if elapsedMs < MinDecisionMs {
		return
	}
	// Si el burst actual ya supera el umbral, decidimos antes (no hace
	// falta esperar a que se cierre — el buzón sigue hablando).
	if d.currentBurstMs >= MachineBurstMs {
		d.result = ResultMachine
		d.hasResult = true
		return
	}
	// Si tras el primer silencio largo el primer burst fue corto,
	// es muy probable humano (típico "¿Hola?", "Dime?"). Solo decidimos
	// human después de ver al menos un burst cerrado.
	if d.maxBurstMs > 0 && d.maxBurstMs <= HumanBurstMs && d.silenceRunMs >= SilenceGapMs {
		d.result = ResultHuman
		d.hasResult = true
		return
	}
	if elapsedMs >= DecisionWindowMs {
		// Forzamos veredicto al cerrar la ventana.
		voicedRatio := float64(d.voicedFrames) / float64(d.totalFrames)
		bestBurst := d.maxBurstMs
		if d.currentBurstMs > bestBurst {
			bestBurst = d.currentBurstMs
		}
		switch {
		case voicedRatio >= MachineVoicedRatio, bestBurst >= MachineBurstMs:
			d.result = ResultMachine
		case bestBurst > 0 && bestBurst <= HumanBurstMs:
			d.result = ResultHuman
		case d.voicedFrames == 0:
			d.result = ResultUnknown // silencio total (no contestó realmente)
		default:
			// Hubo voz pero no encaja en ningún patrón claro → human por defecto.
			// Preferimos falso negativo (gastar un poco) a colgar a un cliente real.
			d.result = ResultHuman
		}
		d.hasResult = true
	}
}

// Result devuelve el veredicto si ya lo tiene, o ("", false) si sigue
// analizando.
func (d *Detector) Result() (Result, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.result, d.hasResult
}

// Done indica si ya hay veredicto.
func (d *Detector) Done() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.hasResult
}

// Elapsed devuelve cuánto lleva el detector activo.
func (d *Detector) Elapsed() time.Duration {
	return time.Since(d.startedAt)
}

// frameVoiced calcula RMS del frame slin 8 kHz LE y devuelve si supera
// el umbral. Para 160 samples es ~1 µs.
func frameVoiced(pcm []byte) bool {
	if len(pcm) < 2 {
		return false
	}
	var sumSq float64
	n := len(pcm) / 2
	for i := 0; i < n; i++ {
		// PCM little-endian signed 16-bit.
		s := int16(uint16(pcm[2*i]) | uint16(pcm[2*i+1])<<8)
		sumSq += float64(s) * float64(s)
	}
	rms := math.Sqrt(sumSq / float64(n))
	return rms >= EnergyThreshold
}
