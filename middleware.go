package main

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/lain-the-coder/ea-qms-backend/internal/logging"
)

func (cfg *apiConfig) middlewareLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// generate unique ID and derive child logger with attached attribute
		requestID := uuid.NewString()
		reqLogger := cfg.logger.With("request_id", requestID)

		ctx := logging.ContextWithLogger(r.Context(), reqLogger)
		r = r.WithContext(ctx)

		reqLogger.Info("request started", "method", r.Method, "path", r.URL.Path)
		// Pass control downstream
		next.ServeHTTP(w, r)

		reqLogger.Info("request finished",
			"method", r.Method,
			"path", r.URL.Path,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}
