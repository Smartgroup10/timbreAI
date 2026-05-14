package audio

import "testing"

func TestUlawRoundtripPreservesShape(t *testing.T) {
	// Una señal senoidal grosera para verificar que el roundtrip slin→ulaw→slin
	// no destruye la dinámica (G.711 es lossy pero conserva forma).
	samples := []int16{0, 100, 500, 1000, 5000, 10000, 20000, 32000, -32000, -10000, -100, 0}
	in := make([]byte, len(samples)*2)
	for i, s := range samples {
		in[i*2] = byte(s)
		in[i*2+1] = byte(s >> 8)
	}
	ulaw := SlinToUlaw(in)
	if len(ulaw) != len(samples) {
		t.Fatalf("ulaw len %d != samples %d", len(ulaw), len(samples))
	}
	back := UlawToSlin(ulaw)
	if len(back) != len(in) {
		t.Fatalf("back len %d != in %d", len(back), len(in))
	}
	for i, s := range samples {
		got := int16(back[i*2]) | int16(back[i*2+1])<<8
		// G.711 cuantiza fuerte en magnitudes pequeñas; tolerancia generosa.
		diff := int(got) - int(s)
		if diff < 0 {
			diff = -diff
		}
		// 8% del rango total como margen máximo.
		if diff > 2600 {
			t.Errorf("sample %d: %d → roundtrip → %d (diff %d, fuera de tolerancia)", i, s, got, diff)
		}
	}
}

func TestSilenceRoundtrip(t *testing.T) {
	in := make([]byte, 320) // 20 ms de silencio
	ulaw := SlinToUlaw(in)
	back := UlawToSlin(ulaw)
	for i, b := range back {
		// Silencio puede no llegar exacto a 0 pero debe ser cercano.
		if b != 0 && b != 0xff && b != 0x01 && b != 0xfe {
			t.Errorf("silencio[%d] = 0x%02x, esperado ~0", i, b)
		}
	}
}
