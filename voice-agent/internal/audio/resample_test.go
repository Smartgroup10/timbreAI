package audio

import "testing"

// TestResampleRatio comprueba que la ratio 1:3 / 3:1 se cumple en bytes.
// Para frames de 20ms:
//   - slin 8k: 160 samples = 320 B
//   - slin 24k: 480 samples = 960 B
func TestResampleRatio(t *testing.T) {
	in8k := make([]byte, 320) // 160 samples
	out24k := UpsampleSlin8kTo24k(in8k)
	if len(out24k) != 960 {
		t.Fatalf("upsample 8k→24k: got %d B, want 960", len(out24k))
	}

	in24k := make([]byte, 960)
	out8k := DownsampleSlin24kTo8k(in24k)
	if len(out8k) != 320 {
		t.Fatalf("downsample 24k→8k: got %d B, want 320", len(out8k))
	}
}

// TestResampleRoundtripSilence garantiza que silencio in → silencio out.
// Cualquier desfase o bit flip sería detectado.
func TestResampleRoundtripSilence(t *testing.T) {
	zeros := make([]byte, 320)
	up := UpsampleSlin8kTo24k(zeros)
	down := DownsampleSlin24kTo8k(up)
	for i, b := range down {
		if b != 0 {
			t.Errorf("silence not preserved at byte %d: got %d", i, b)
		}
	}
}

// TestUpsampleConstantHoldsValue: si los samples de entrada son todos el
// mismo valor, los samples interpolados también deben ser ese valor.
// Si no, hay un bug en la interpolación.
func TestUpsampleConstantHoldsValue(t *testing.T) {
	// 4 samples a valor 12345
	src := []byte{
		0x39, 0x30, // 12345 LE
		0x39, 0x30,
		0x39, 0x30,
		0x39, 0x30,
	}
	out := UpsampleSlin8kTo24k(src)
	for i := 0; i < len(out); i += 2 {
		v := int16(uint16(out[i]) | uint16(out[i+1])<<8)
		if v != 12345 {
			t.Errorf("sample %d/2: got %d, want 12345", i/2, v)
		}
	}
}

// TestEmptyInputs no debe panicar.
func TestEmptyInputs(t *testing.T) {
	if r := UpsampleSlin8kTo24k(nil); r != nil {
		t.Errorf("nil input should return nil, got %d bytes", len(r))
	}
	if r := DownsampleSlin24kTo8k(nil); r != nil {
		t.Errorf("nil input should return nil, got %d bytes", len(r))
	}
}
