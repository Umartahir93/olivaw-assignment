package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// HTTPLogging logs every request with its method, path, status and
// duration. The cause of a 500 is logged where it is turned into the
// response (the REST server's NewError).
func HTTPLogging(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start),
		}
		switch {
		case rec.status >= http.StatusInternalServerError:
			log.ErrorContext(r.Context(), "http request failed", attrs...)
		case rec.status >= http.StatusBadRequest:
			log.WarnContext(r.Context(), "http request rejected", attrs...)
		default:
			log.InfoContext(r.Context(), "http request", attrs...)
		}
	})
}

// HTTPRecovery answers 500 when a handler panics and logs the panic. Without
// it net/http drops the connection and logs outside the JSON log.
func HTTPRecovery(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rv := recover(); rv != nil {
				log.ErrorContext(r.Context(), "http handler panicked",
					"method", r.Method,
					"path", r.URL.Path,
					"error", fmt.Sprintf("panic: %v\n%s", rv, debug.Stack()),
				)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statusRecorder remembers the status code a handler writes.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
