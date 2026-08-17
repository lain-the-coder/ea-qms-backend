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

func (cfg *apiConfig) requireRole(role string, next authedHandler) authedHandler {
	return func(w http.ResponseWriter, r *http.Request, user database.User) {
		log := logging.LoggerFrom(r.Context())
		if user.Role != role {
			log.Warn("authorization failed", "reason", "insufficient role", "required", role, "actual", user.Role)
			respondWithError(w, "Forbidden", http.StatusForbidden)
			return
		}
		next(w, r, user)
	}
}

// middlewareCORS answers cross-origin preflights and adds the headers a browser
// needs before it will let JavaScript read a response.
//
// This is a BROWSER rule, not an API security control — Postman and curl ignore
// it entirely. Authentication and authorisation are what secure the API; this
// only decides which web origins the browser will permit to call it.
func (cfg *apiConfig) middlewareCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// Only echo back an origin that is explicitly allowed. Reflecting any
		// origin, or using "*", would let any website call this API from a
		// user's browser.
		if _, ok := cfg.allowedOrigins[origin]; ok && origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			// Tells caches the response varies by origin, so a response for one
			// origin is never served to another.
			w.Header().Add("Vary", "Origin")
		} else if origin != "" {
			logging.LoggerFrom(r.Context()).Warn("cors origin rejected", "origin", origin)
		}
		// Preflight: the browser asks permission before sending anything with an
		// Authorization header. Answer it and stop — never let it reach a handler.
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

			// The browser may skip the preflight for 10 minutes.
			w.Header().Set("Access-Control-Max-Age", "600")

			// Lets JavaScript read these on a download response; without it the
			// browser hides every header except a short safelist.
			w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition, Content-Length")

			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Same exposure for the actual response, so a download can read the filename.
		w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition, Content-Length")
		next.ServeHTTP(w, r)
	})
}
