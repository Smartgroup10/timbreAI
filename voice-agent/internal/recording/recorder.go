package recording

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"sync"
	"time"
)

// Recorder collects raw PCM16 audio for the duration of a session and uploads it as a single WAV
// file at session close. Audio in (caller) and audio out (agent) are mixed into one mono track
// by summing samples — fine for previewing; a real product would record stereo with each side on
// its own channel.
type Recorder struct {
	sessionID  string
	mu         sync.Mutex
	chunks     [][]byte
	sampleRate int
}

func New(sessionID string, sampleRate int) *Recorder {
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	return &Recorder{sessionID: sessionID, sampleRate: sampleRate}
}

// Append stores a chunk of PCM16 samples. Non-blocking, copies the slice so the caller can reuse
// the buffer.
func (r *Recorder) Append(buf []byte) {
	if len(buf) == 0 {
		return
	}
	c := make([]byte, len(buf))
	copy(c, buf)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.chunks = append(r.chunks, c)
	// Cap memory at ~20 MB (≈10 min @ 16kHz mono PCM16).
	total := 0
	for _, ch := range r.chunks {
		total += len(ch)
	}
	for total > 20*1024*1024 && len(r.chunks) > 0 {
		total -= len(r.chunks[0])
		r.chunks = r.chunks[1:]
	}
}

func (r *Recorder) Bytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]byte, 0)
	for _, c := range r.chunks {
		out = append(out, c...)
	}
	return out
}

// WAV returns the recorded audio as a 16-bit PCM mono WAV blob. Returns nil if nothing was
// recorded — callers should skip the upload in that case.
func (r *Recorder) WAV() []byte {
	pcm := r.Bytes()
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
	binary.Write(&buf, binary.LittleEndian, uint32(16))                // fmt chunk size
	binary.Write(&buf, binary.LittleEndian, uint16(1))                 // PCM
	binary.Write(&buf, binary.LittleEndian, uint16(1))                 // mono
	binary.Write(&buf, binary.LittleEndian, uint32(r.sampleRate))      // sample rate
	binary.Write(&buf, binary.LittleEndian, uint32(r.sampleRate*2))    // byte rate
	binary.Write(&buf, binary.LittleEndian, uint16(2))                 // block align
	binary.Write(&buf, binary.LittleEndian, uint16(16))                // bits per sample
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, dataLen)
	buf.Write(pcm)
	return buf.Bytes()
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
