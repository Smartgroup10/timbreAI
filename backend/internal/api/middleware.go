package api

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"timbre/backend/internal/auth"
)

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && s.originAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originAllowed(origin string) bool {
	if slices.Contains(s.cfg.AllowedOrigins, "*") {
		return true
	}
	return slices.Contains(s.cfg.AllowedOrigins, origin)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := auth.ParseBearer(r.Header.Get("Authorization"))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "missing_bearer_token")
			return
		}
		claims, err := auth.Verify(s.cfg.JWTSecret, token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_token")
			return
		}
		next(w, r.WithContext(auth.WithClaims(r.Context(), claims)))
	}
}

func (s *Server) requireRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.FromContext(r.Context())
		if !ok || !strings.EqualFold(claims.Role, role) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		next(w, r)
	})
}
