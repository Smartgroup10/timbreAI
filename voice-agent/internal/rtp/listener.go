package rtp

import (
	"context"
	"encoding/binary"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net"
	"sync"
	"time"
)

// Listener wraps a UDP socket that talks RTP carrying signed-linear 16-bit mono audio at 16 kHz.
// Asterisk's External Media channel speaks the same protocol when format=slin16.
//
// Direction handling: Asterisk picks its own source port. We discover that address from the first
// inbound packet ("learn-then-reply"), then send our outbound stream back to it.
//
// The listener does NOT own session state — it just shuffles audio bytes between UDP and two
// channels (in/out PCM16) provided by the caller.
type Listener struct {
	conn    *net.UDPConn
	port    int
	logger  *slog.Logger
	closing chan struct{}

	mu         sync.Mutex
	remoteAddr *net.UDPAddr

	// Outbound bookkeeping.
	ssrc        uint32
	sequence    uint16
	timestamp   uint32
	payloadType uint8 // mirrors what Asterisk sent us on the first packet; defaults to 118
}

func NewListener(port int, logger *slog.Logger) (*Listener, error) {
	addr := &net.UDPAddr{IP: net.IPv4zero, Port: port}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, err
	}
	return &Listener{
		conn:        conn,
		port:        port,
		logger:      logger,
		closing:     make(chan struct{}),
		ssrc:        rand.Uint32(),
		sequence:    uint16(rand.Uint32()),
		payloadType: 118, // dynamic slin16 PT (Asterisk default)
	}, nil
}

func (l *Listener) Port() int { return l.port }

// Close shuts the socket. Safe to call multiple times.
func (l *Listener) Close() {
	select {
	case <-l.closing:
		return
	default:
	}
	close(l.closing)
	_ = l.conn.Close()
}

// Run pumps audio between UDP and the in/out channels. Returns when ctx is cancelled or the
// socket is closed.
//
// audioIn:  inbound PCM16 samples from Asterisk → fed into here (we write).
// audioOut: outbound PCM16 samples from the provider → we packetize and send.
func (l *Listener) Run(ctx context.Context, audioIn chan<- []byte, audioOut <-chan []byte) error {
	go l.readLoop(ctx, audioIn)
	go l.writeLoop(ctx, audioOut)
	select {
	case <-ctx.Done():
		l.Close()
		return ctx.Err()
	case <-l.closing:
		return nil
	}
}

func (l *Listener) readLoop(ctx context.Context, audioIn chan<- []byte) {
	buf := make([]byte, 2048)
	// Stats periódicas para diagnosticar problemas tipo "el primer paquete llega
	// y luego silencio" o "el provider está congestionado y droppeamos todo".
	var pktTotal, pktForwarded, pktDropped uint64
	statTick := time.NewTicker(5 * time.Second)
	defer statTick.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-statTick.C:
				if pktTotal == 0 {
					continue
				}
				l.logger.Info("rtp stats", "port", l.port,
					"received", pktTotal, "forwarded", pktForwarded, "dropped", pktDropped)
			}
		}
	}()

	for {
		if ctx.Err() != nil {
			return
		}
		_ = l.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, addr, err := l.conn.ReadFromUDP(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			// Read timeout — loop and check ctx.
			var nerr net.Error
			if errors.As(err, &nerr) && nerr.Timeout() {
				continue
			}
			l.logger.Warn("rtp read", "port", l.port, "error", err)
			continue
		}
		if n < 12 {
			continue
		}
		pktTotal++
		// First packet: learn remote address + payload type.
		l.mu.Lock()
		if l.remoteAddr == nil {
			l.remoteAddr = addr
			l.payloadType = buf[1] & 0x7F
			l.logger.Info("rtp peer learned", "port", l.port, "addr", addr.String(), "pt", l.payloadType)
		}
		l.mu.Unlock()

		header := buf[:12]
		// Skip CSRC + extension if any (we don't expect them from Asterisk, but be safe).
		cc := int(header[0] & 0x0F)
		off := 12 + cc*4
		if (header[0]>>4)&1 == 1 && len(buf) > off+4 {
			extLen := int(binary.BigEndian.Uint16(buf[off+2:off+4])) * 4
			off += 4 + extLen
		}
		if off >= n {
			continue
		}
		payload := buf[off:n]
		// Send a copy: the goroutine pump may outlive this iteration.
		cp := make([]byte, len(payload))
		copy(cp, payload)
		select {
		case audioIn <- cp:
			pktForwarded++
		case <-ctx.Done():
			return
		default:
			// Drop if downstream isn't keeping up. Audio in chan has buffer; if it's full we're
			// already in trouble — better to discard than to block the read loop.
			pktDropped++
		}
	}
}

// writeLoop drains audioOut and emits 20 ms RTP packets at 16 kHz (320 samples = 640 bytes).
// It buffers the upstream chunks (which may come in arbitrary sizes from the provider) and only
// flushes a packet when 640 bytes are queued OR the buffer is non-empty after the inactivity
// flush interval.
func (l *Listener) writeLoop(ctx context.Context, audioOut <-chan []byte) {
	const frameBytes = 640 // 16000 Hz * 2 bytes * 0.020 s
	const samplesPerFrame = 320
	buf := make([]byte, 0, frameBytes*4)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	flush := func(now bool) {
		l.mu.Lock()
		remote := l.remoteAddr
		l.mu.Unlock()
		if remote == nil {
			// Haven't heard from Asterisk yet; drop the audio.
			buf = buf[:0]
			return
		}
		for len(buf) >= frameBytes {
			l.sendPacket(remote, buf[:frameBytes])
			buf = buf[frameBytes:]
		}
		if now && len(buf) > 0 {
			// Pad the trailing fragment with zeros to a full frame so timing stays aligned.
			pad := make([]byte, frameBytes-len(buf))
			frame := append(buf, pad...)
			l.sendPacket(remote, frame)
			buf = buf[:0]
		}
		_ = samplesPerFrame
	}

	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-audioOut:
			if !ok {
				flush(true)
				return
			}
			buf = append(buf, chunk...)
			flush(false)
		case <-ticker.C:
			// Periodic flush: emit whole frames if we have them; do not zero-pad partials here
			// (only when audioOut closes) so we don't introduce silence gaps mid-stream.
			flush(false)
		}
	}
}

func (l *Listener) sendPacket(remote *net.UDPAddr, payload []byte) {
	header := make([]byte, 12)
	header[0] = 0x80 // V=2, P=0, X=0, CC=0
	header[1] = l.payloadType & 0x7F
	binary.BigEndian.PutUint16(header[2:4], l.sequence)
	binary.BigEndian.PutUint32(header[4:8], l.timestamp)
	binary.BigEndian.PutUint32(header[8:12], l.ssrc)

	l.sequence++
	l.timestamp += uint32(len(payload) / 2) // 16-bit samples

	pkt := append(header, payload...)
	if _, err := l.conn.WriteToUDP(pkt, remote); err != nil && !errors.Is(err, net.ErrClosed) {
		l.logger.Warn("rtp write", "port", l.port, "error", err)
	}
}
