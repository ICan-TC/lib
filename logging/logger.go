package logging

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog"
)

var logger zerolog.Logger

func InitLogger(format string) {
	logFile, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		panic(fmt.Sprintf("failed to open log file: %v", err))
	}
	multi := io.MultiWriter(os.Stdout, logFile)
	logger = zerolog.New(multi).With().Timestamp().Logger()
	switch format {
	case "json":
		return
	case "text":
		logger = logger.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	default:
		logger = logger.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}
}

func L() zerolog.Logger {
	return logger
}

// RequestLoggingHandler attaches the logger to the context and logs the request.
func RequestLoggingHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := L()
		excluded := []string{"/openapi.yaml", "/docs"}
		start := time.Now()
		ww := &responseWriter{ResponseWriter: w, status: 200}
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = ulid.Make().String()
		}

		logctx := l.With().Str("requestID", requestID)
		ctx := logctx.Logger().WithContext(r.Context())

		next.ServeHTTP(ww, r.WithContext(ctx))
		if !slices.Contains(excluded, r.URL.Path) {
			duration := time.Since(start)
			zerolog.Ctx(ctx).Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", ww.status).
				Dur("duration", duration).
				Str("agent", r.UserAgent()).
				Any("ctx", ctx).
				Msg("request completed")
		}
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
