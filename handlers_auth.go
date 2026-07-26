package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lain-the-coder/ea-qms-backend/internal/auth"
	"github.com/lain-the-coder/ea-qms-backend/internal/database"
	"github.com/lain-the-coder/ea-qms-backend/internal/logging"
)

// constants used for access token and refresh token expiry time respectively
const (
	accessTokenTTL          = 30 * time.Minute
	refreshTokenTTL         = 24 * time.Hour
	refreshInactivityWindow = 2 * time.Hour // for idle check via updated_on constraint during refresh touch
)

func (cfg *apiConfig) HandlerRefresh(w http.ResponseWriter, r *http.Request) {
	type RefreshRequest struct {
		RefreshToken string `json:"refresh_token"`
	}
	type RefreshResponse struct {
		Token string `json:"token"`
	}
	reqBody := RefreshRequest{}
	// decode request body
	log := logging.LoggerFrom(r.Context())
	err := json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		log.Error("refresh failed", "reason", "malformed request body", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// request validation
	reqBody.RefreshToken = strings.TrimSpace(reqBody.RefreshToken)
	if reqBody.RefreshToken == "" {
		log.Warn("refresh failed", "reason", "refresh token blank")
		respondWithError(w, "Refresh token cannot be blank", http.StatusBadRequest)
		return
	}
	// get refresh token
	refreshTokenRow, err := cfg.db.GetRefreshToken(r.Context(), reqBody.RefreshToken)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("refresh failed", "reason", "refresh token not found")
			respondWithError(w, "Invalid refresh token", http.StatusUnauthorized)
			return
		}
		log.Error("refresh failed", "reason", "refresh token lookup failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	if refreshTokenRow.RevokedAt != nil {
		log.Warn("refresh failed", "reason", "refresh token revoked")
		respondWithError(w, "Invalid refresh token", http.StatusUnauthorized)
		return
	}
	if time.Now().UTC().After(refreshTokenRow.ExpiresAt) {
		log.Warn("refresh failed", "reason", "refresh token time limit expired")
		respondWithError(w, "Session expired", http.StatusUnauthorized)
		return
	}
	if time.Since(refreshTokenRow.UpdatedOn) > refreshInactivityWindow {
		log.Warn("refresh failed", "reason", "inactivity timeout")
		respondWithError(w, "Session expired", http.StatusUnauthorized)
		return
	}
	// get user details
	user, err := cfg.db.GetUserByID(r.Context(), refreshTokenRow.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("refresh failed", "reason", "user not found", "user_id", refreshTokenRow.UserID)
			respondWithError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		log.Error("refresh failed", "reason", "user lookup failed", "user_id", refreshTokenRow.UserID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// active account check
	if !user.IsActive {
		log.Warn("refresh failed", "reason", "account deactivated", "user_id", user.ID)
		respondWithError(w, "Account is deactivated", http.StatusUnauthorized)
		return
	}
	err = cfg.db.TouchRefreshToken(r.Context(), reqBody.RefreshToken)
	if err != nil {
		log.Error("refresh failed", "reason", "refresh token touch failed", "user_id", user.ID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// create jwt token
	jwtToken, err := auth.MakeJWT(user.ID, cfg.secret, accessTokenTTL)
	if err != nil {
		log.Error("refresh failed", "reason", "jwt generation failed", "user_id", user.ID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	log.Info("refresh successful", "user_id", user.ID)
	resBody := RefreshResponse{
		Token: jwtToken,
	}
	respondWithJSON(w, http.StatusOK, resBody)
}

func (cfg *apiConfig) HandlerRevoke(w http.ResponseWriter, r *http.Request) {
	type RefreshRequest struct {
		RefreshToken string `json:"refresh_token"`
	}
	reqBody := RefreshRequest{}
	// decode request body
	log := logging.LoggerFrom(r.Context())
	err := json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		log.Error("revoke failed", "reason", "malformed request body", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// request validation
	reqBody.RefreshToken = strings.TrimSpace(reqBody.RefreshToken)
	if reqBody.RefreshToken == "" {
		log.Warn("revoke failed", "reason", "refresh token blank")
		respondWithError(w, "Refresh token cannot be blank", http.StatusBadRequest)
		return
	}
	err = cfg.db.RevokeRefreshToken(r.Context(), reqBody.RefreshToken)
	if err != nil {
		log.Error("revoke failed", "reason", "revoking operation in db failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	log.Info("revoke successful")
	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) HandlerLogin(w http.ResponseWriter, r *http.Request) {
	// request/response structs
	type LoginRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type LoginResponse struct {
		ID           uuid.UUID `json:"id"`
		FullName     string    `json:"full_name"`
		Email        string    `json:"email"`
		Role         string    `json:"role"`
		Token        string    `json:"token"`
		RefreshToken string    `json:"refresh_token"`
	}
	reqBody := LoginRequest{}
	// decode request body
	log := logging.LoggerFrom(r.Context())
	err := json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		log.Error("login failed", "reason", "malformed request body", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// request validation
	reqBody.Email = strings.TrimSpace(reqBody.Email)
	if reqBody.Email == "" {
		log.Warn("login failed", "reason", "email blank")
		respondWithError(w, "Email cannot be blank", http.StatusBadRequest)
		return
	}
	if reqBody.Password == "" {
		log.Warn("login failed", "reason", "password blank", "email", reqBody.Email)
		respondWithError(w, "Password cannot be blank", http.StatusBadRequest)
		return
	}
	// get user details
	user, err := cfg.db.GetUserByEmail(r.Context(), reqBody.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Burn the same argon2id cost a real user would, so the response
			// time doesn't reveal whether the account exists.
			_, _ = auth.CheckPasswordHash(reqBody.Password, cfg.dummyHash)
			log.Warn("login failed", "reason", "user not found", "email", reqBody.Email)
			respondWithError(w, "Incorrect email or password", http.StatusUnauthorized)
			return
		}
		log.Error("login failed", "reason", "user lookup failed", "email", reqBody.Email, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// password match check
	match, err := auth.CheckPasswordHash(reqBody.Password, user.HashedPassword)
	if err != nil {
		log.Error("login failed", "reason", "password verification error", "email", reqBody.Email, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	if !match {
		log.Warn("login failed", "reason", "password mismatch", "email", reqBody.Email, "user_id", user.ID)
		respondWithError(w, "Incorrect email or password", http.StatusUnauthorized)
		return
	}
	// active account check
	if !user.IsActive {
		log.Warn("login failed", "reason", "account deactivated", "email", reqBody.Email, "user_id", user.ID)
		respondWithError(w, "Account is deactivated", http.StatusUnauthorized)
		return
	}
	// create jwt token
	jwtToken, err := auth.MakeJWT(user.ID, cfg.secret, accessTokenTTL)
	if err != nil {
		log.Error("login failed", "reason", "jwt generation failed", "user_id", user.ID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// create refresh token and add into db
	refreshToken, err := auth.MakeRefreshToken()
	if err != nil {
		log.Error("login failed", "reason", "refresh token generation failed", "user_id", user.ID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	_, err = cfg.db.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token:     refreshToken,
		UserID:    user.ID,
		ExpiresAt: time.Now().UTC().Add(refreshTokenTTL),
	})
	if err != nil {
		log.Error("login failed", "reason", "refresh token insert failed", "user_id", user.ID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	log.Info("login successful", "user_id", user.ID, "email", user.Email, "role", user.Role) // create and return response body
	resBody := LoginResponse{
		ID:           user.ID,
		FullName:     user.FullName,
		Email:        user.Email,
		Role:         user.Role,
		Token:        jwtToken,
		RefreshToken: refreshToken,
	}
	respondWithJSON(w, http.StatusOK, resBody)
}
