package provider

import (
	"context"
	"log/slog"
	"time"

	"timbre/voice-agent/internal/session"
)

// Echo is a no-API provider that just echoes incoming audio back and emits a fake transcript.
// Useful for plumbing tests, CI, demos without spending API budget, and for the default config.
type Echo struct {
	logger *slog.Logger
}

func (e *Echo) Name() string { return "echo" }

func (e *Echo) Run(ctx context.Context, s *session.Session) error {
	greeting := "Hola, soy el agente de voz en modo echo. Repetiré lo que digas."
	if s.Config.Objective != "" {
		greeting = "Hola. Estoy en modo echo. Mi objetivo configurado es: " + s.Config.Objective
	}
	emit(s, session.Event{Type: "status", State: "ready"})
	emit(s, session.Event{Type: "transcript", Role: "agent", Text: greeting, Final: true})
	s.AppendTranscript("agent", greeting)

	// Heartbeat: every 10s emit a status keep-alive so the client sees the session is alive
	// even when there's no audio activity (echo mode is mostly idle).
	heartbeat := time.NewTicker(10 * time.Second)
	defer heartbeat.Stop()

	// Buffer audio for ~500ms before echoing it back as a "user said: ..." transcript.
	// We don't actually do ASR here; we just echo bytes through and pretend.
	const flushAfter = 500 * time.Millisecond
	var (
		buf       []byte
		lastChunk time.Time
	)
	flush := time.NewTicker(100 * time.Millisecond)
	defer flush.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-heartbeat.C:
			emit(s, session.Event{Type: "status", State: "listening"})
		case chunk, ok := <-s.AudioIn:
			if !ok {
				return nil
			}
			buf = append(buf, chunk...)
			lastChunk = time.Now()
			// Echo the audio bytes back without modification.
			select {
			case s.AudioOut <- append([]byte(nil), chunk...):
			case <-ctx.Done():
				return ctx.Err()
			}
		case <-flush.C:
			if len(buf) > 0 && time.Since(lastChunk) >= flushAfter {
				fake := "(echo): " + humanBytes(len(buf)) + " audio recibido"
				emit(s, session.Event{Type: "transcript", Role: "user", Text: fake, Final: true})
				s.AppendTranscript("user", fake)
				buf = nil
			}
		}
	}
}

func humanBytes(n int) string {
	if n < 1024 {
		return itoa(n) + "B"
	}
	if n < 1024*1024 {
		return itoa(n/1024) + "KB"
	}
	return itoa(n/(1024*1024)) + "MB"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// emit pushes an event to the session, dropping it if the channel is full instead of blocking
// the provider's main loop. Losing a transcript update is preferable to stalling audio.
func emit(s *session.Session, ev session.Event) {
	select {
	case s.Events <- ev:
	default:
	}
}
