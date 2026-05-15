package recording

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Recorder collects raw PCM16 audio for the duration of a session and
// uploads it as WAV/Opus at session close. Audio in (caller) y audio
// out (agent) están ya mezclados en mono por el caller via Append.
//
// Backing: ARCHIVO TEMPORAL en disco. Antes acumulábamos en memoria con
// cap 20 MB → llamadas >10 min se truncaban silenciosamente. Ahora
// streamea a un .pcm temp y solo se carga a memoria en el momento de
// codificar (Opus) o construir el WAV. Para una llamada de 1h a 16 kHz
// mono PCM16 el archivo pesa ~115 MB en disco — barato.
//
// Si CreateTemp falla (disco lleno, permisos), caemos a un buffer en
// memoria con el cap antiguo de 20 MB. Mejor degradación que perder
// toda la grabación.
type Recorder struct {
	sessionID  string
	sampleRate int

	mu           sync.Mutex
	file         *os.File // backing en disco; nil si fallback a memoria
	path         string   // path del temp para Close()
	memoryBuf    []byte   // fallback si el archivo no se pudo crear
	bytesWritten int64    // contador para metrics
	closed       bool
}

// MemoryFallbackCap es el cap del buffer en memoria cuando el backing
// por archivo no se pudo crear. Mismo umbral que el comportamiento viejo.
const MemoryFallbackCap = 20 * 1024 * 1024

func New(sessionID string, sampleRate int) *Recorder {
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	r := &Recorder{sessionID: sessionID, sampleRate: sampleRate}
	// os.CreateTemp con prefix identificable — facilita el cleanup
	// manual si por alguna razón Close() no se llamó (kill -9, panic).
	f, err := os.CreateTemp("", "timbre-rec-"+sessionID+"-*.pcm")
	if err == nil {
		r.file = f
		r.path = f.Name()
	}
	// Sin file: r.memoryBuf actúa como fallback. Append lo respeta.
	return r
}

// Append stores a chunk of PCM16 samples. Non-blocking, copia la slice
// para que el caller pueda reusar el buffer.
//
// Errores de escritura a disco se logueanal final via stat() pero NO
// propagamos el error — el caller (audio loop) no debe romperse por un
// fallo de grabación. La grabación es secundaria a la llamada.
func (r *Recorder) Append(buf []byte) {
	if len(buf) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return // append después de Close() — ignorar.
	}
	if r.file != nil {
		n, err := r.file.Write(buf)
		if err != nil {
			// Disco lleno o I/O error. Cerramos el file y pasamos a
			// fallback de memoria con cap pequeño — al menos
			// preservamos los últimos segundos.
			_ = r.file.Close()
			_ = os.Remove(r.path)
			r.file = nil
			r.path = ""
			return
		}
		r.bytesWritten += int64(n)
		return
	}
	// Fallback en memoria con cap.
	c := make([]byte, len(buf))
	copy(c, buf)
	r.memoryBuf = append(r.memoryBuf, c...)
	if len(r.memoryBuf) > MemoryFallbackCap {
		// Descartar inicio — preservamos el final (lo último que se dijo
		// suele ser lo más útil: outcome, despedida).
		excess := len(r.memoryBuf) - MemoryFallbackCap
		r.memoryBuf = r.memoryBuf[excess:]
	}
	r.bytesWritten = int64(len(r.memoryBuf))
}

// Bytes devuelve TODO el PCM acumulado. Para llamadas largas esto
// puede ser >100 MB en memoria temporalmente — solo lo usan los
// encoders al cerrar sesión, no path caliente.
//
// Si el backing es archivo, hace io.ReadAll del file (que cerramos
// abajo si era necesario para hacer seek). Si es memoria, copia el slice.
func (r *Recorder) Bytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.bytesLocked()
}

// bytesLocked es la versión interna que asume el lock tomado. Sirve a
// WAV() para no doble-lockar.
func (r *Recorder) bytesLocked() []byte {
	if r.file != nil {
		// Forzamos sync por si el SO no había flusheado al disco aún
		// (poco probable a estas alturas, pero más vale).
		_ = r.file.Sync()
		// Seek al principio y leer todo. Si Seek o ReadAll fallan,
		// retornamos nil — no podemos recuperarnos.
		if _, err := r.file.Seek(0, io.SeekStart); err != nil {
			return nil
		}
		data, err := io.ReadAll(r.file)
		if err != nil {
			return nil
		}
		return data
	}
	if len(r.memoryBuf) == 0 {
		return nil
	}
	out := make([]byte, len(r.memoryBuf))
	copy(out, r.memoryBuf)
	return out
}

// WAV returns the recorded audio as a 16-bit PCM mono WAV blob. Returns nil if nothing was
// recorded — callers should skip the upload in that case.
func (r *Recorder) WAV() []byte {
	r.mu.Lock()
	pcm := r.bytesLocked()
	r.mu.Unlock()
	if len(pcm) == 0 {
		return nil
	}
	var buf bytes.Buffer
	dataLen := uint32(len(pcm))
	chunkLen := dataLen + 36

	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, chunkLen)
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))             // fmt chunk size
	binary.Write(&buf, binary.LittleEndian, uint16(1))              // PCM
	binary.Write(&buf, binary.LittleEndian, uint16(1))              // mono
	binary.Write(&buf, binary.LittleEndian, uint32(r.sampleRate))   // sample rate
	binary.Write(&buf, binary.LittleEndian, uint32(r.sampleRate*2)) // byte rate
	binary.Write(&buf, binary.LittleEndian, uint16(2))              // block align
	binary.Write(&buf, binary.LittleEndian, uint16(16))             // bits per sample
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, dataLen)
	buf.Write(pcm)
	return buf.Bytes()
}

// BytesWritten devuelve el total escrito (no afectado por seeks). Útil
// para el log de duración estimada: bytes_written / 2 / sampleRate = seg.
func (r *Recorder) BytesWritten() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.bytesWritten
}

// Close cierra el archivo backing y lo borra. Idempotente. El caller
// (handler de session) lo invoca en defer junto al upload — preferimos
// no dejar archivos huérfanos en /tmp.
func (r *Recorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	if r.file != nil {
		closeErr := r.file.Close()
		removeErr := os.Remove(r.path)
		r.file = nil
		r.path = ""
		// Cerrar primero, borrar después; reportamos el primero que falla.
		if closeErr != nil {
			return fmt.Errorf("close temp: %w", closeErr)
		}
		if removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("remove temp: %w", removeErr)
		}
	}
	r.memoryBuf = nil
	return nil
}

// ErrNoFFmpeg se devuelve cuando ffmpeg no está disponible en PATH —
// el caller hace fallback a WAV. Útil para entornos sin ffmpeg (tests,
// alguna imagen sin la dependencia).
var ErrNoFFmpeg = errors.New("ffmpeg not found in PATH")

// EncodeOpus convierte el WAV interno a OGG/Opus via ffmpeg subprocess.
// Bitrate 24kbps con `voip` application — calidad telefónica buena con
// ~16x menos tamaño que el WAV PCM16 16kHz original.
//
// Retornos:
//   - bytes OGG/Opus, contentType "audio/ogg", nil → éxito
//   - nil, "", ErrNoFFmpeg → ffmpeg no instalado; caller usa WAV
//   - nil, "", err → ffmpeg falló por otra razón
//
// La conversión va por stdin/stdout sin escribir a disco. Timeout 30s
// para no colgar el shutdown de la sesión si ffmpeg se queda.
func (r *Recorder) EncodeOpus(ctx context.Context) ([]byte, string, error) {
	wav := r.WAV()
	if wav == nil {
		return nil, "", nil // nothing to encode
	}
	// Comprobación rápida antes de fork — evita un exec.Cmd inútil.
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, "", ErrNoFFmpeg
	}

	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Args:
	//   -hide_banner -loglevel error → silencia el output de ffmpeg
	//   -f wav -i pipe:0             → entrada WAV desde stdin
	//   -c:a libopus                 → codec opus
	//   -b:a 24k                     → 24 kbps, suficiente para voz IA
	//   -application voip            → preset optimizado para voz
	//   -ac 1 -ar 16000              → preservar mono 16 kHz
	//   -f ogg pipe:1                → salida OGG por stdout
	cmd := exec.CommandContext(cctx, "ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-f", "wav", "-i", "pipe:0",
		"-c:a", "libopus",
		"-b:a", "24k",
		"-application", "voip",
		"-ac", "1", "-ar", "16000",
		"-f", "ogg", "pipe:1")
	cmd.Stdin = bytes.NewReader(wav)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Stderr de ffmpeg suele tener la pista útil ("Unknown encoder
		// libopus" si la build de ffmpeg no incluye libopus, etc.).
		return nil, "", &encodeError{Err: err, Stderr: stderr.String()}
	}
	return stdout.Bytes(), "audio/ogg", nil
}

type encodeError struct {
	Err    error
	Stderr string
}

func (e *encodeError) Error() string {
	if e.Stderr != "" {
		return e.Err.Error() + ": " + e.Stderr
	}
	return e.Err.Error()
}

// Upload posts the WAV to the backend recording webhook. Best-effort: errors are returned but the
// session shutdown path treats them as warnings.
type Uploader struct {
	BaseURL string
	Secret  string
	HTTP    *http.Client
}

func NewUploader(baseURL, secret string) *Uploader {
	return &Uploader{BaseURL: baseURL, Secret: secret, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// Upload envía los bytes con el content-type indicado. El backend lo
// parsea para derivar la extensión del objeto en MinIO (wav/ogg/opus).
// Si contentType es "" usamos audio/wav por compatibilidad histórica.
func (u *Uploader) Upload(ctx context.Context, sessionID string, body []byte, contentType string) error {
	if contentType == "" {
		contentType = "audio/wav"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		u.BaseURL+"/api/internal/voice/recordings?sessionId="+sessionID,
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	if u.Secret != "" {
		req.Header.Set("X-Internal-Secret", u.Secret)
	}
	resp, err := u.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return &uploadError{Status: resp.StatusCode, Body: string(b)}
	}
	return nil
}

type uploadError struct {
	Status int
	Body   string
}

func (e *uploadError) Error() string { return e.Body }
