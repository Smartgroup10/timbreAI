package api

import (
	"net/http"

	"callhub/voice-agent/internal/rtp"
)

// handleAllocateRTP reserves a UDP port for the session and starts the listener. The backend
// calls this from its StasisStart handler, then uses the returned host:port to ask Asterisk to
// open an External Media channel pointing here.
//
// Response: { "host": "voice-agent", "port": 12001 }
func (s *Server) handleAllocateRTP(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, ok := s.registry.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "session_not_found")
		return
	}
	if s.rtpPool == nil {
		writeError(w, http.StatusServiceUnavailable, "rtp_disabled")
		return
	}
	port, err := s.rtpPool.Acquire()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "rtp_port_pool_exhausted")
		return
	}
	listener, err := rtp.NewListener(port, s.logger)
	if err != nil {
		s.rtpPool.Release(port)
		writeError(w, http.StatusInternalServerError, "rtp_listen_failed")
		return
	}

	// Wire the listener to the session's audio channels. The recorder already tees outbound, and
	// transcript events still flow over the management WS (or the provider's own callbacks).
	go func() {
		_ = listener.Run(sess.Context(), sess.AudioIn, sess.AudioOut)
	}()
	// Release the port when the session ends so the pool stays healthy.
	sess.SetOnClose(func() {
		listener.Close()
		s.rtpPool.Release(port)
	})

	s.logger.Info("rtp allocated", "session", sess.ID, "port", port, "advertise", s.cfg.RTP.AdvertiseHost)
	writeJSON(w, http.StatusOK, map[string]any{
		"host": s.cfg.RTP.AdvertiseHost,
		"port": port,
	})
}
