package http

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/orderfulfillment/fulfillment/internal/infra/config"
	"github.com/orderfulfillment/fulfillment/internal/infra/logging"
	"github.com/orderfulfillment/fulfillment/internal/infra/metrics"
)

type principalKey struct{}

// Principal is the authenticated caller identity.
type Principal struct{ Subject string }

func principalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// correlationMiddleware ensures every request carries a correlation id, both in
// the logging context and in the response header.
func correlationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Correlation-Id")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Correlation-Id", id)
		ctx := logging.WithCorrelationID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// recoveryMiddleware converts panics into a 500 envelope instead of crashing.
func recoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.ErrorContext(r.Context(), "panic recovered", slog.Any("panic", rec))
					correlationID, _ := logging.CorrelationID(r.Context())
					writeJSON(w, http.StatusInternalServerError, errorEnvelope{Error: errorBody{
						Code: "INTERNAL", Message: "an internal error occurred", CorrelationID: correlationID,
					}})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// loggingMiddleware emits one structured access log per request.
func loggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			logger.InfoContext(r.Context(), "http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}
}

// metricsMiddleware records request count and latency, labelled by the matched
// route pattern (not the raw path) to keep cardinality bounded. It runs
// innermost so the pattern is set by the mux by the time it reads it.
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		path := r.Pattern
		if path == "" {
			path = "unmatched"
		}
		metrics.HTTPRequests.WithLabelValues(metrics.Service, r.Method, path, strconv.Itoa(rec.status)).Inc()
		metrics.HTTPDuration.WithLabelValues(metrics.Service, r.Method, path).Observe(time.Since(start).Seconds())
	})
}

// authMiddleware verifies a bearer JWT (HS256) when auth is enabled and injects
// the principal. When disabled it is a pass-through for local development.
func authMiddleware(cfg config.AuthConfig, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}
			raw := bearerToken(r)
			if raw == "" {
				unauthorized(w, r)
				return
			}
			claims := jwt.RegisteredClaims{}
			_, err := jwt.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrTokenSignatureInvalid
				}
				return []byte(cfg.JWTSecret), nil
			}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(cfg.JWTIssuer), jwt.WithAudience(cfg.JWTAudience))
			if err != nil || claims.Subject == "" {
				logger.WarnContext(r.Context(), "auth rejected", slog.String("error", errString(err)))
				unauthorized(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), principalKey{}, Principal{Subject: claims.Subject})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) && h[:len(prefix)] == prefix {
		return h[len(prefix):]
	}
	return ""
}

func unauthorized(w http.ResponseWriter, r *http.Request) {
	correlationID, _ := logging.CorrelationID(r.Context())
	writeJSON(w, http.StatusUnauthorized, errorEnvelope{Error: errorBody{
		Code: "UNAUTHORIZED", Message: "authentication required", CorrelationID: correlationID,
	}})
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
