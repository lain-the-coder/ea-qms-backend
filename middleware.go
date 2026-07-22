package main

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/lain-the-coder/ea-qms-backend/internal/auth"
	"github.com/lain-the-coder/ea-qms-backend/internal/database"
	"github.com/lain-the-coder/ea-qms-backend/internal/logging"
)

type authedHandler func(http.ResponseWriter, *http.Request, database.User)

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

func (cfg *apiConfig) middlewareAuth(next authedHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logging.LoggerFrom(r.Context())
		// extract bearer token
		token, err := auth.GetBearerToken(r.Header)
		if err != nil {
			log.Warn("auth failed", "reason", "jwt token extraction failed", "error", err)
			respondWithError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		// validate jwt
		userID, err := auth.ValidateJWT(token, cfg.secret)
		if err != nil {
			log.Warn("auth failed", "reason", "invalid jwt token passed", "error", err)
			respondWithError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		// get user details
		user, err := cfg.db.GetUserByID(r.Context(), userID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				log.Warn("auth failed", "reason", "user not found", "user_id", userID)
				respondWithError(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			log.Error("auth failed", "reason", "user lookup failed", "user_id", userID, "error", err)
			respondWithError(w, "Something went wrong", http.StatusInternalServerError)
			return
		}
		// active account check
		if !user.IsActive {
			log.Warn("auth failed", "reason", "account deactivated", "email", user.Email, "user_id", user.ID)
			respondWithError(w, "Account is deactivated", http.StatusUnauthorized)
			return
		}
		log = log.With("user_id", user.ID)
		log.Info("authenticated", "role", user.Role, "email", user.Email)
		r = r.WithContext(logging.ContextWithLogger(r.Context(), log))
		next(w, r, user)
	})
}
