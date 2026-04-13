package middleware

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/nurtidev/medcore/internal/shared/logger"
)

// Logger returns a middleware that assigns an X-Request-ID and logs every request.
// If the incoming request already carries an X-Request-ID it is preserved;
// otherwise a new UUID is generated on the gateway.
func Logger(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.New().String()
			}

			// Always set the correlation ID header in the response.
			w.Header().Set("X-Request-ID", requestID)
			// Forward it to upstreams.
			r.Header.Set("X-Request-ID", requestID)

			reqLog := log.With().
				Str("request_id", requestID).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote_addr", r.RemoteAddr).
				Logger()

			ctx, _ := logger.WithRequestID(r.Context(), requestID)
			ctx = logger.WithContext(ctx, reqLog)

			start := time.Now()
			rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r.WithContext(ctx))

			reqLog.Info().
				Int("status", rw.status).
				Dur("duration_ms", time.Since(start)).
				Msg("request")
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}
