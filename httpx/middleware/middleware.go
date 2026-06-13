// Package middleware provides composable net/http middleware.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

// CORSConfig controls cross-origin request handling.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           time.Duration
}

// CORS returns configurable CORS middleware.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	origins := makeSet(cfg.AllowedOrigins)
	if len(cfg.AllowedMethods) == 0 {
		cfg.AllowedMethods = []string{http.MethodGet, http.MethodHead, http.MethodPost}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && (origins["*"] || origins[origin]) {
				allowedOrigin := origin
				if origins["*"] && !cfg.AllowCredentials {
					allowedOrigin = "*"
				}
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
				if len(cfg.AllowedHeaders) > 0 {
					w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
				}
				if len(cfg.ExposedHeaders) > 0 {
					w.Header().Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposedHeaders, ", "))
				}
				if cfg.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				if cfg.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", fmt.Sprint(int(cfg.MaxAge.Seconds())))
				}
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

type requestIDKey struct{}

// RequestIDConfig controls request ID propagation and generation.
type RequestIDConfig struct {
	Header   string
	Generate func() string
}

// RequestID returns middleware that propagates or generates request IDs.
func RequestID(cfg RequestIDConfig) func(http.Handler) http.Handler {
	if cfg.Header == "" {
		cfg.Header = "X-Request-ID"
	}
	if cfg.Generate == nil {
		cfg.Generate = generateRequestID
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(cfg.Header)
			if id == "" {
				id = cfg.Generate()
			}
			w.Header().Set(cfg.Header, id)
			ctx := context.WithValue(r.Context(), requestIDKey{}, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequestIDFromContext returns the request ID stored by RequestID middleware.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDKey{}).(string)
	return id, ok
}

// RecoveryConfig controls panic recovery.
type RecoveryConfig struct {
	Logger     *slog.Logger
	WriteError func(http.ResponseWriter, *http.Request, error)
}

// Recovery converts panics to HTTP responses and optionally logs stack traces.
func Recovery(cfg RecoveryConfig) func(http.Handler) http.Handler {
	if cfg.WriteError == nil {
		cfg.WriteError = func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					err := fmt.Errorf("panic: %v", recovered)
					if cfg.Logger != nil {
						cfg.Logger.ErrorContext(r.Context(), "recovered panic",
							"error", err,
							"stack", string(debug.Stack()),
						)
					}
					cfg.WriteError(w, r, err)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// RequestLogConfig controls slog request logging.
type RequestLogConfig struct {
	Logger *slog.Logger
	Enrich func(*http.Request) []slog.Attr
}

// RequestLogger logs method, path, status, duration, and optional attributes.
func RequestLogger(cfg RequestLogConfig) func(http.Handler) http.Handler {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			recorder := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(recorder, r)
			attrs := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", recorder.status),
				slog.Duration("duration", time.Since(started)),
			}
			if id, ok := RequestIDFromContext(r.Context()); ok {
				attrs = append(attrs, slog.String("request_id", id))
			}
			if cfg.Enrich != nil {
				attrs = append(attrs, cfg.Enrich(r)...)
			}
			cfg.Logger.LogAttrs(r.Context(), slog.LevelInfo, "http request", attrs...)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *statusWriter) WriteHeader(status int) {
	if w.wrote {
		return
	}
	w.wrote = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(data []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func makeSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func generateRequestID() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(data[:])
}
