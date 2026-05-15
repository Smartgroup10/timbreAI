// Package audiosocket implementa el servidor TCP que recibe audio PCM directo
// de Asterisk vía el módulo res_audiosocket. Es el reemplazo de External Media
// + RTP — mismo objetivo (puente bidireccional de audio) pero sobre TCP, sin
// transcoding en el bridge de Asterisk, y con el formato fijo slin (16-bit
// signed linear, 8 kHz mono).
//
// Protocolo (matchea Asterisk res_audiosocket.h):
//
//	[1 byte type][2 bytes length BE][payload]
//
// Tipos:
//
//	0x00 Hangup   — el caller colgó
//	0x01 UUID     — primer frame de Asterisk, identifica la sesión
//	0x10 Audio    — slin 8 kHz, típicamente 320 B/frame (20 ms)
//	0xFF Error    — error fatal
//
// Asterisk se conecta, envía un frame UUID, y a partir de ahí audio bidirec.
// Nosotros usamos ese UUID como el ID de sesión del voice-agent — el backend
// crea la sesión, le pasa el UUID a Asterisk via dialplan, y AudioSocket() se
// conecta aquí con ese mismo ID.
//
// Patrón inspirado en Smartgroup10/SmartSIP.
package audiosocket

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"timbre/voice-agent/internal/session"
)

const (
	TypeHangup byte = 0x00
	TypeUUID   byte = 0x01
	TypeAudio  byte = 0x10
	TypeError  byte = 0xFF

	// SampleRate slin nativo de AudioSocket = 8 kHz mono 16-bit.
	SampleRate     = 8000
	FrameSamples   = 160 // 20 ms a 8 kHz
	FrameBytes     = FrameSamples * 2

	ReadTimeout  = 30 * time.Second
	WriteTimeout = 5 * time.Second
	FrameTick    = 20 * time.Millisecond
)

// Server escucha TCP, acepta conexiones de Asterisk y enlaza cada una con
// su session.Session correspondiente (encontrada por UUID).
type Server struct {
	addr     string
	registry *session.Registry
	logger   *slog.Logger

	listener net.Listener
	wg       sync.WaitGroup
}

func New(addr string, reg *session.Registry, logger *slog.Logger) *Server {
	return &Server{addr: addr, registry: reg, logger: logger}
}

func (s *Server) Run(ctx context.Context) error {
	lc := &net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp", s.addr)
	if err != nil {
		return fmt.Errorf("audiosocket listen %s: %w", s.addr, err)
	}
	s.listener = ln
	s.logger.Info("audiosocket listening", "addr", s.addr)

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				s.wg.Wait()
				return nil
			}
			s.logger.Warn("audiosocket accept", "error", err)
			continue
		}
		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			s.handleConnection(ctx, c)
		}(conn)
	}
}

func (s *Server) handleConnection(parentCtx context.Context, conn net.Conn) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()

	sessionID, err := readUUIDFrame(conn)
	if err != nil {
		s.logger.Warn("audiosocket: read uuid", "remote", remote, "error", err)
		return
	}
	s.logger.Info("audiosocket: connection identified", "remote", remote, "session", sessionID)

	sess, ok := s.registry.Get(sessionID)
	if !ok {
		s.logger.Warn("audiosocket: unknown session", "session", sessionID, "remote", remote)
		sendError(conn)
		return
	}

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()
	// Si el provider de la sesión termina antes, cerramos esta conexión.
	go func() {
		<-sess.Context().Done()
		cancel()
	}()

	// Read loop: TCP frames → sess.AudioIn (slin 8kHz 16-bit, 320B/20ms).
	go s.readLoop(ctx, conn, sess)

	// Write loop: sess.AudioOut → TCP frames a Asterisk con pacing 20ms.
	s.writeLoop(ctx, conn, sess)
}

func (s *Server) readLoop(ctx context.Context, conn net.Conn, sess *session.Session) {
	defer s.logger.Info("audiosocket read loop ended", "session", sess.ID)
	header := make([]byte, 3)
	var pktTotal, forwarded, dropped uint64
	statTick := time.NewTicker(5 * time.Second)
	defer statTick.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-statTick.C:
				if pktTotal > 0 {
					s.logger.Info("audiosocket rx stats", "session", sess.ID,
						"received", pktTotal, "forwarded", forwarded, "dropped", dropped)
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(ReadTimeout))
		if _, err := io.ReadFull(conn, header); err != nil {
			if ctx.Err() == nil {
				s.logger.Debug("audiosocket read header", "error", err)
			}
			return
		}
		frameType := header[0]
		length := binary.BigEndian.Uint16(header[1:3])

		switch frameType {
		case TypeHangup:
			s.logger.Info("audiosocket: caller hung up", "session", sess.ID)
			return
		case TypeError:
			s.logger.Warn("audiosocket: error frame", "session", sess.ID)
			return
		case TypeAudio:
			if length == 0 {
				continue
			}
			payload := make([]byte, length)
			if _, err := io.ReadFull(conn, payload); err != nil {
				return
			}
			pktTotal++
			// Tap AMD antes de empujar al provider. Es no-op si AMD está
			// deshabilitado o ya emitió veredicto, así que no penaliza.
			sess.ObserveInbound(payload)
			select {
			case sess.AudioIn <- payload:
				forwarded++
			case <-ctx.Done():
				return
			default:
				dropped++
			}
		default:
			// Skip unknown frame payload.
			if length > 0 {
				skip := make([]byte, length)
				if _, err := io.ReadFull(conn, skip); err != nil {
					return
				}
			}
		}
	}
}

// writeLoop drena sess.AudioOut y manda frames a 20ms. Si no hay audio
// pendiente, NO escribe (Asterisk reproduce silencio nativamente).
func (s *Server) writeLoop(ctx context.Context, conn net.Conn, sess *session.Session) {
	defer s.logger.Info("audiosocket write loop ended", "session", sess.ID)
	var buf []byte
	tick := time.NewTicker(FrameTick)
	defer tick.Stop()
	var sent, recv uint64
	statTick := time.NewTicker(5 * time.Second)
	defer statTick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-statTick.C:
			if sent > 0 || recv > 0 {
				s.logger.Info("audiosocket tx stats", "session", sess.ID, "chunks_recv", recv, "frames_sent", sent)
			}
		case chunk, ok := <-sess.AudioOut:
			if !ok {
				return
			}
			recv++
			buf = append(buf, chunk...)
		case <-tick.C:
			// Mandamos un frame de 320 bytes si hay bastante; si no, silencio (no escribimos).
			if len(buf) >= FrameBytes {
				frame := buf[:FrameBytes]
				if err := writeAudio(conn, frame); err != nil {
					s.logger.Warn("audiosocket write", "session", sess.ID, "error", err)
					return
				}
				buf = buf[FrameBytes:]
				sent++
			}
		}
	}
}

// readUUIDFrame lee el frame inicial que identifica la sesion. Asterisk envia
// el UUID como 16 bytes binarios; aceptamos tambien texto para clientes de test.
func readUUIDFrame(conn net.Conn) (string, error) {
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetReadDeadline(time.Time{})

	header := make([]byte, 3)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", fmt.Errorf("read header: %w", err)
	}
	if header[0] != TypeUUID {
		return "", fmt.Errorf("expected UUID frame (0x01), got 0x%02x", header[0])
	}
	length := binary.BigEndian.Uint16(header[1:3])
	if length == 0 || length > 128 {
		return "", fmt.Errorf("uuid length out of range: %d", length)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return "", err
	}
	if len(payload) == 16 {
		return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
			payload[0:4], payload[4:6], payload[6:8], payload[8:10], payload[10:16]), nil
	}
	id := strings.TrimSpace(string(payload))
	if len(id) == 36 {
		return strings.ToLower(id), nil
	}
	return "", fmt.Errorf("unsupported uuid payload length: %d", len(payload))
}

func writeAudio(conn net.Conn, pcm []byte) error {
	frame := make([]byte, 3+len(pcm))
	frame[0] = TypeAudio
	binary.BigEndian.PutUint16(frame[1:3], uint16(len(pcm)))
	copy(frame[3:], pcm)
	_ = conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	_, err := conn.Write(frame)
	return err
}

func sendError(conn net.Conn) {
	_ = conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	_, _ = conn.Write([]byte{TypeError, 0x00, 0x00})
}
