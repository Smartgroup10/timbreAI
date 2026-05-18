package audio

import "encoding/binary"

// Resampling slin ↔ slin entre 8 kHz y 24 kHz, ambos PCM16 LE mono.
//
// Por qué existe esto: AudioSocket nos entrega audio nativo a 8 kHz
// (el codec del trunk SIP). Algunos providers (AssemblyAI Voice Agent)
// requieren 24 kHz como entrada y emiten 24 kHz como salida. Sin
// resampling el audio se interpreta a la frecuencia incorrecta y suena
// acelerado/ralentizado 3×, lo que rompe completamente la conversación.
//
// La ratio es exacta (24 / 8 = 3), así que usamos:
//   - Upsample 8k→24k: interpolación lineal entre samples consecutivos.
//     Inserta 2 muestras intermedias por cada par del original.
//   - Downsample 24k→8k: promedio simple de 3 samples consecutivos.
//     El promedio actúa como filtro low-pass de orden 0 — suficiente
//     para evitar aliasing audible en voz telefónica (banda <4 kHz).
//
// Calidad: estos métodos no son perceptualmente perfectos comparados con
// filtros polifásicos / sinc-windowed, pero son ~10× más baratos en CPU
// (sin tablas, sin allocs intermedios) y para una llamada telefónica el
// ancho de banda útil ya cae a 3-4 kHz, así que el aliasing por encima
// es inaudible.

// UpsampleSlin8kTo24k convierte slin 8 kHz a 24 kHz mediante
// interpolación lineal. La salida tiene exactamente 3× el número de
// samples del input. Bytes in / out: ambos PCM16 LE.
//
// Allocs: una sola, del slice de salida. CPU: O(n) sin tablas.
func UpsampleSlin8kTo24k(src []byte) []byte {
	n := len(src) / 2
	if n == 0 {
		return nil
	}
	out := make([]byte, n*3*2)
	for i := 0; i < n; i++ {
		s0 := int16(binary.LittleEndian.Uint16(src[2*i : 2*i+2]))
		var s1 int16
		if i+1 < n {
			s1 = int16(binary.LittleEndian.Uint16(src[2*(i+1) : 2*(i+1)+2]))
		} else {
			s1 = s0 // último sample: repetir
		}
		// 3 samples de salida por cada uno de entrada:
		//   pos 0: s0
		//   pos 1: (2·s0 + s1) / 3
		//   pos 2: (s0 + 2·s1) / 3
		samples := [3]int16{
			s0,
			int16((int32(s0)*2 + int32(s1)) / 3),
			int16((int32(s0) + int32(s1)*2) / 3),
		}
		base := i * 3 * 2
		for k := 0; k < 3; k++ {
			binary.LittleEndian.PutUint16(out[base+k*2:base+k*2+2], uint16(samples[k]))
		}
	}
	return out
}

// DownsampleSlin24kTo8k convierte slin 24 kHz a 8 kHz promediando
// grupos de 3 samples. La salida tiene exactamente 1/3 del número de
// samples del input (si el input no es múltiplo de 3, los samples
// finales se descartan — los frames del Voice Agent siempre vienen en
// chunks múltiplos de 3 porque están alineados a 20ms = 480 samples a
// 24 kHz = 160 a 8 kHz).
//
// El promedio sobre 3 actúa como filtro low-pass que atenúa lo que
// haya por encima de los 4 kHz (Nyquist a 8k), suficiente para voz
// telefónica.
func DownsampleSlin24kTo8k(src []byte) []byte {
	n := len(src) / 2
	outN := n / 3
	if outN == 0 {
		return nil
	}
	out := make([]byte, outN*2)
	for i := 0; i < outN; i++ {
		s0 := int32(int16(binary.LittleEndian.Uint16(src[(3*i+0)*2 : (3*i+0)*2+2])))
		s1 := int32(int16(binary.LittleEndian.Uint16(src[(3*i+1)*2 : (3*i+1)*2+2])))
		s2 := int32(int16(binary.LittleEndian.Uint16(src[(3*i+2)*2 : (3*i+2)*2+2])))
		avg := int16((s0 + s1 + s2) / 3)
		binary.LittleEndian.PutUint16(out[i*2:i*2+2], uint16(avg))
	}
	return out
}

// UpsampleSlin8kTo16k convierte slin 8 kHz a 16 kHz mediante interpolación
// lineal. Ratio 1:2 — más simple que el 1:3 de 24k. La salida tiene 2× los
// samples del input. ElevenLabs Conversational AI espera PCM 16k.
func UpsampleSlin8kTo16k(src []byte) []byte {
	n := len(src) / 2
	if n == 0 {
		return nil
	}
	out := make([]byte, n*2*2)
	for i := 0; i < n; i++ {
		s0 := int16(binary.LittleEndian.Uint16(src[2*i : 2*i+2]))
		var s1 int16
		if i+1 < n {
			s1 = int16(binary.LittleEndian.Uint16(src[2*(i+1) : 2*(i+1)+2]))
		} else {
			s1 = s0
		}
		// pos 0: s0
		// pos 1: (s0 + s1) / 2 — punto medio
		base := i * 2 * 2
		binary.LittleEndian.PutUint16(out[base:base+2], uint16(s0))
		mid := int16((int32(s0) + int32(s1)) / 2)
		binary.LittleEndian.PutUint16(out[base+2:base+4], uint16(mid))
	}
	return out
}

// DownsampleSlin16kTo8k convierte slin 16 kHz a 8 kHz promediando pares de
// samples. La salida tiene la mitad de samples del input.
func DownsampleSlin16kTo8k(src []byte) []byte {
	n := len(src) / 2
	outN := n / 2
	if outN == 0 {
		return nil
	}
	out := make([]byte, outN*2)
	for i := 0; i < outN; i++ {
		s0 := int32(int16(binary.LittleEndian.Uint16(src[(2*i+0)*2 : (2*i+0)*2+2])))
		s1 := int32(int16(binary.LittleEndian.Uint16(src[(2*i+1)*2 : (2*i+1)*2+2])))
		avg := int16((s0 + s1) / 2)
		binary.LittleEndian.PutUint16(out[i*2:i*2+2], uint16(avg))
	}
	return out
}
