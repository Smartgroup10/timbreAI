package recording

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"testing"
)

// TestWAVHeaderShape verifica que el WAV producido es un fichero RIFF/WAV
// PCM16 mono parseable. Sin ffmpeg podemos al menos comprobar la forma
// del WAV — es la base que luego comprime el encoder.
func TestWAVHeaderShape(t *testing.T) {
	r := New("test", 16000)
	// 2 chunks de 320 bytes = 320 samples = 20ms a 16kHz.
	chunk := bytes.Repeat([]byte{0x01, 0x00}, 160)
	r.Append(chunk)
	r.Append(chunk)

	wav := r.WAV()
	if len(wav) == 0 {
		t.Fatal("WAV() returned empty")
	}
	if !bytes.HasPrefix(wav, []byte("RIFF")) {
		t.Error("WAV does not start with RIFF")
	}
	if !bytes.Contains(wav[:12], []byte("WAVE")) {
		t.Error("WAVE marker missing in first 12 bytes")
	}
	if !bytes.Contains(wav, []byte("fmt ")) {
		t.Error("fmt chunk missing")
	}
	if !bytes.Contains(wav, []byte("data")) {
		t.Error("data chunk missing")
	}
	// PCM samples = chunks total = 640 bytes; +44 header = 684 esperados.
	want := 44 + 640
	if len(wav) != want {
		t.Errorf("WAV size: got %d, want %d", len(wav), want)
	}
}

func TestWAVEmptyIsNil(t *testing.T) {
	r := New("test", 16000)
	if got := r.WAV(); got != nil {
		t.Errorf("empty recorder should WAV() = nil, got %d bytes", len(got))
	}
}

// TestEncodeOpusFallsBackWhenNoFFmpeg verifica el comportamiento cuando
// ffmpeg NO está en PATH — es el contrato que el caller usa para hacer
// fallback a WAV sin romper el upload.
//
// Saltamos el test si ffmpeg SÍ está, porque entonces queremos probar
// el camino feliz en TestEncodeOpusRoundtrip.
func TestEncodeOpusFallsBackWhenNoFFmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		t.Skip("ffmpeg present, skipping fallback test")
	}
	r := New("test", 16000)
	r.Append(bytes.Repeat([]byte{0x01, 0x00}, 160))

	body, ct, err := r.EncodeOpus(context.Background())
	if !errors.Is(err, ErrNoFFmpeg) {
		t.Errorf("want ErrNoFFmpeg, got %v", err)
	}
	if body != nil || ct != "" {
		t.Errorf("on fallback want nil/empty, got %d bytes, ct=%q", len(body), ct)
	}
}

// TestEncodeOpusRoundtrip prueba el camino feliz si ffmpeg está
// disponible. Verifica que la salida es un OGG válido (firma "OggS")
// y que pesa significativamente menos que el WAV original.
func TestEncodeOpusRoundtrip(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not in PATH, skipping encode test")
	}
	r := New("test", 16000)
	// 5 segundos de "silencio" estructurado para que ffmpeg tenga algo
	// que codificar. 5s * 16000 samples/s * 2 bytes/sample = 160 KB PCM.
	silence := make([]byte, 160*1000)
	r.Append(silence)

	body, ct, err := r.EncodeOpus(context.Background())
	if err != nil {
		t.Fatalf("EncodeOpus: %v", err)
	}
	if ct != "audio/ogg" {
		t.Errorf("content type: got %q, want audio/ogg", ct)
	}
	if !bytes.HasPrefix(body, []byte("OggS")) {
		t.Errorf("output is not OGG (first 4 bytes: %x)", body[:4])
	}
	// Compresión mínima esperada: silence se comprime muchísimo. 5x es
	// el suelo defensivo; en práctica suele estar en 50x+.
	wavSize := len(r.WAV())
	if wavSize == 0 || len(body) >= wavSize/5 {
		t.Errorf("compression too weak: wav=%d, opus=%d", wavSize, len(body))
	}
}
