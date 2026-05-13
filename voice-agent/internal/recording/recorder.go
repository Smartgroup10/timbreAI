package recording

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net/http"
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

func (u *Uploader) Upload(ctx context.Context, sessionID string, wav []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		u.BaseURL+"/api/internal/voice/recordings?sessionId="+sessionID,
		bytes.NewReader(wav))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "audio/wav")
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
