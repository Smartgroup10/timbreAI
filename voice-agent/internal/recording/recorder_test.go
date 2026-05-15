package recording

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
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
	defer r.Close()
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
	wavSize := len(r.WAV())
	if wavSize == 0 || len(body) >= wavSize/5 {
		t.Errorf("compression too weak: wav=%d, opus=%d", wavSize, len(body))
	}
}

// TestLongRecordingDoesNotTruncate verifica que el cambio a backing por
// archivo arregla el truncado silencioso. 30 MB > MemoryFallbackCap (20 MB).
// Si seguimos en RAM-cap, el WAV viene a 20 MB (limit). Con file backing
// debe venir a 30 MB completos.
func TestLongRecordingDoesNotTruncate(t *testing.T) {
	r := New("long", 16000)
	defer r.Close()

	// 30 MB en chunks de 64 KB simula ~3 frames audio por chunk durante
	// ~15 min reales. Patrón secuencial para detectar si descartamos el
	// inicio (los primeros bytes deben aparecer en el resultado).
	chunk := bytes.Repeat([]byte{0xAA, 0x55}, 32*1024) // 64 KB
	for i := 0; i < 480; i++ {                          // 480 * 64 KB = 30 MB
		// El primer byte de cada chunk lo marcamos con un patrón
		// distinto para localizarlo en el resultado.
		chunk[0] = byte(i)
		r.Append(chunk)
	}
	if got := r.BytesWritten(); got != 30*1024*1024 {
		t.Errorf("BytesWritten: got %d, want %d", got, 30*1024*1024)
	}
	all := r.Bytes()
	if len(all) != 30*1024*1024 {
		t.Fatalf("truncated! got %d bytes, want %d", len(all), 30*1024*1024)
	}
	// El primer chunk debe seguir ahí — antes lo descartábamos.
	if all[0] != 0x00 {
		t.Errorf("first byte: got %x, want 0x00 (start of recording lost)", all[0])
	}
	// El último chunk también. byte(479) = 0xDF (wrap natural en byte).
	lastChunkStart := 30*1024*1024 - 64*1024
	if all[lastChunkStart] != byte(479%256) {
		t.Errorf("last chunk marker: got %x, want 0xdf", all[lastChunkStart])
	}
}

// TestCloseRemovesTempFile garantiza que no dejamos basura en /tmp.
func TestCloseRemovesTempFile(t *testing.T) {
	r := New("cleanup", 16000)
	r.Append([]byte{0x00, 0x00})

	// Acceso al path interno via reflection no funciona porque es
	// privado. Indirectamente: listamos /tmp antes y después.
	tmpDir := os.TempDir()
	before, _ := matchTempFiles(tmpDir, "timbre-rec-cleanup-")
	if len(before) == 0 {
		t.Skip("no temp file detected (filesystem snapshot timing)")
	}

	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	after, _ := matchTempFiles(tmpDir, "timbre-rec-cleanup-")
	if len(after) != 0 {
		t.Errorf("temp file leak: %v still present after Close", after)
	}
}

// TestCloseIsIdempotent — segundo Close() no debe romper.
func TestCloseIsIdempotent(t *testing.T) {
	r := New("idem", 16000)
	if err := r.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// TestAppendAfterCloseIsNoop — defensa contra el caso "el audio loop
// sigue enviando chunks después de que el handler hizo Close".
func TestAppendAfterCloseIsNoop(t *testing.T) {
	r := New("after-close", 16000)
	r.Close()
	r.Append([]byte{0x01, 0x02, 0x03, 0x04})
	if got := r.BytesWritten(); got != 0 {
		t.Errorf("BytesWritten after closed Append: got %d, want 0", got)
	}
	if r.Bytes() != nil && len(r.Bytes()) > 0 {
		t.Errorf("Bytes() after close-then-append should be empty")
	}
}

// matchTempFiles devuelve nombres de archivos en dir cuyo prefijo
// matchea. Helper para el test de cleanup.
func matchTempFiles(dir, prefix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) {
			out = append(out, e.Name())
		}
	}
	return out, nil
}
