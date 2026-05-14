// Package audio: conversores entre los formatos de audio que pasean por el
// voice-agent. Lo único que necesitamos por ahora es slin (Asterisk AudioSocket,
// 8 kHz signed linear 16-bit) ↔ G.711 μ-law (OpenAI Realtime con
// input/output_audio_format=g711_ulaw).
//
// La tabla G.711 μ-law es estándar ITU-T G.711. Implementación derivada del
// algoritmo de referencia que usa cualquier libsamplerate / spandsp / SmartSIP.
package audio

// SlinToUlaw convierte slin (PCM signed 16-bit little endian) a G.711 μ-law.
// Output size = input bytes / 2.
func SlinToUlaw(pcm []byte) []byte {
	out := make([]byte, len(pcm)/2)
	for i := 0; i < len(out); i++ {
		sample := int16(pcm[i*2]) | int16(pcm[i*2+1])<<8
		out[i] = linearToUlaw(sample)
	}
	return out
}

// UlawToSlin convierte G.711 μ-law a slin (PCM signed 16-bit little endian).
// Output size = input bytes * 2.
func UlawToSlin(ulaw []byte) []byte {
	out := make([]byte, len(ulaw)*2)
	for i, b := range ulaw {
		s := ulawToLinear(b)
		out[i*2] = byte(s)
		out[i*2+1] = byte(s >> 8)
	}
	return out
}

func linearToUlaw(pcm int16) byte {
	const bias = 0x84
	const clip = 32635
	var sign byte
	v := int(pcm)
	if v < 0 {
		sign = 0x80
		v = -v
	}
	if v > clip {
		v = clip
	}
	v += bias
	seg := byte(7)
	mask := 0x4000
	for ; seg > 0; seg-- {
		if v >= mask {
			break
		}
		mask >>= 1
	}
	uval := sign | (seg << 4) | byte((v>>uint(seg+3))&0x0f)
	return ^uval
}

func ulawToLinear(u byte) int16 {
	u = ^u
	sign := u & 0x80
	seg := (u >> 4) & 0x07
	mantissa := u & 0x0f
	sample := ((int(mantissa) << 3) + 0x84) << seg
	sample -= 0x84
	if sign != 0 {
		sample = -sample
	}
	return int16(sample)
}
