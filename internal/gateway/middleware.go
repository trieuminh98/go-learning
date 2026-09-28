package gateway

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"go.uber.org/zap"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// RequestIDFromContext retrieves the request ID associated with the context.
func RequestIDFromContext(ctx context.Context) string {
	if val, ok := ctx.Value(requestIDKey).(string); ok {
		return val
	}
	return ""
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// RecoveryMiddleware catches panics, logs structured error with stack trace, and returns 500 JSON.
func RecoveryMiddleware(logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					reqID := RequestIDFromContext(r.Context())
					if logger != nil {
						logger.Errorw("panic recovered",
							"request_id", reqID,
							"method", r.Method,
							"path", r.URL.Path,
							"error", fmt.Sprintf("%v", rec),
							"stack", string(debug.Stack()),
						)
					}
					w.Header().Set("Connection", "close")
					writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// RequestIDMiddleware injects a request ID header and context value.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}
		w.Header().Set("X-Request-ID", reqID)
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LoggingMiddleware records request execution duration and method/path using structured logging.
func LoggingMiddleware(logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			reqID := RequestIDFromContext(r.Context())

			next.ServeHTTP(w, r)

			if logger != nil {
				logger.Infow("request completed",
					"request_id", reqID,
					"method", r.Method,
					"path", r.URL.Path,
					"duration_ms", time.Since(start).Milliseconds(),
				)
			}
		})
	}
}

// TimeoutMiddleware bounds request execution duration using context timeout.
func TimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AuthMiddleware enforces inbound token authentication for protected route groups.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized: missing authorization header"})
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") || strings.TrimPrefix(authHeader, "Bearer ") != "secret-token" {
			writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized: invalid token"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// BasicAuthMiddleware enforces HTTP Basic Authentication for protected route groups.
// It verifies the username and password in constant time using SHA-256 digests
// to prevent timing attacks. On failure, it returns 401 Unauthorized with
// a standard WWW-Authenticate challenge header and a JSON error body.
func BasicAuthMiddleware(expectedUser, expectedPass string) func(http.Handler) http.Handler {
	expectedUserHash := sha256.Sum256([]byte(expectedUser))
	expectedPassHash := sha256.Sum256([]byte(expectedPass))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if expectedUser == "" || expectedPass == "" {
				unauthorizedBasicAuth(w)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				unauthorizedBasicAuth(w)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Basic") {
				unauthorizedBasicAuth(w)
				return
			}

			decoded, err := base64.StdEncoding.DecodeString(parts[1])
			if err != nil {
				unauthorizedBasicAuth(w)
				return
			}

			username, password, found := strings.Cut(string(decoded), ":")
			if !found {
				unauthorizedBasicAuth(w)
				return
			}

			givenUserHash := sha256.Sum256([]byte(username))
			givenPassHash := sha256.Sum256([]byte(password))

			userMatch := subtle.ConstantTimeCompare(givenUserHash[:], expectedUserHash[:]) == 1
			passMatch := subtle.ConstantTimeCompare(givenPassHash[:], expectedPassHash[:]) == 1

			if !userMatch || !passMatch {
				unauthorizedBasicAuth(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func unauthorizedBasicAuth(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Restricted", charset="UTF-8"`)
	writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
}
