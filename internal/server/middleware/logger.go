package middleware

import (
	"log"
	"net/http"
	"time"
)

// statusWriter captures the status code written by downstream handlers so the
// logger can report it. It satisfies http.ResponseWriter and delegates writes.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Logger is structured request logging middleware.
type Logger struct {
	*log.Logger
}

// Middleware returns an http middleware that logs each request on completion.
func (l Logger) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		l.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
	})
}
