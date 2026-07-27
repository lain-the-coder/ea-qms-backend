package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lain-the-coder/ea-qms-backend/internal/auth"
	"github.com/lain-the-coder/ea-qms-backend/internal/database"
	"github.com/lain-the-coder/ea-qms-backend/internal/logging"
	"github.com/lib/pq"
)

func (cfg *apiConfig) HandlerGetMe(w http.ResponseWriter, r *http.Request, user database.User) {
	type GetMeResponse struct {
		ID       uuid.UUID `json:"id"`
		FullName string    `json:"full_name"`
		Email    string    `json:"email"`
		Role     string    `json:"role"`
	}
	resBody := GetMeResponse{
		ID:       user.ID,
		FullName: user.FullName,
		Email:    user.Email,
		Role:     user.Role,
	}
	respondWithJSON(w, http.StatusOK, resBody)
}

func (cfg *apiConfig) HandlerCreateUser(w http.ResponseWriter, r *http.Request, admin database.User) {
	type CreateUserRequest struct {
		FullName string `json:"full_name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	type CreateUserResponse struct {
		ID        uuid.UUID `json:"id"`
		FullName  string    `json:"full_name"`
		Email     string    `json:"email"`
		Role      string    `json:"role"`
		IsActive  bool      `json:"is_active"`
		CreatedOn time.Time `json:"created_on"`
	}
	reqBody := CreateUserRequest{}
	// decode request body
	log := logging.LoggerFrom(r.Context())
	err := json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		log.Error("user creation failed", "reason", "malformed request body", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// request validation
	reqBody.FullName = strings.TrimSpace(reqBody.FullName)
	reqBody.Email = strings.TrimSpace(reqBody.Email)
	reqBody.Role = strings.TrimSpace(reqBody.Role)
	if reqBody.FullName == "" {
		log.Warn("user creation failed", "reason", "full name blank")
		respondWithError(w, "Full Name cannot be blank", http.StatusBadRequest)
		return
	}
	if reqBody.Email == "" {
		log.Warn("user creation failed", "reason", "email blank")
		respondWithError(w, "Email cannot be blank", http.StatusBadRequest)
		return
	}
	if reqBody.Password == "" {
		log.Warn("user creation failed", "reason", "password blank")
		respondWithError(w, "Password cannot be blank", http.StatusBadRequest)
		return
	}
	if problems := validatePassword(reqBody.Password); len(problems) > 0 {
		log.Warn("user creation failed", "reason", "password policy not met", "unmet_rules", len(problems))
		respondWithError(w,
			"Password must contain "+strings.Join(problems, ", "),
			http.StatusBadRequest)
		return
	}
	if reqBody.Role == "" {
		log.Warn("user creation failed", "reason", "role blank")
		respondWithError(w, "Role cannot be blank", http.StatusBadRequest)
		return
	}
	isValid := false
	switch reqBody.Role {
	case roleAdmin, roleCCOwner, roleApprover, roleViewer:
		isValid = true
	}
	if !isValid {
		log.Warn("user creation failed", "reason", "invalid role")
		respondWithError(w, "Invalid Role", http.StatusBadRequest)
		return
	}
	hashedPassword, err := auth.HashPassword(reqBody.Password, cfg.params)
	if err != nil {
		log.Error("user creation failed", "reason", "failed hashing", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	tx, err := cfg.rawDB.BeginTx(r.Context(), nil)
	if err != nil {
		log.Error("user creation failed", "reason", "could not begin transaction", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	qtx := cfg.db.WithTx(tx)
	newUser, err := qtx.CreateUser(r.Context(), database.CreateUserParams{
		FullName:       reqBody.FullName,
		Email:          reqBody.Email,
		HashedPassword: hashedPassword,
		Role:           reqBody.Role,
	})
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			log.Warn("user creation failed", "reason", "unique email constraint violation", "email", reqBody.Email)
			respondWithError(w, "A user with that email already exists", http.StatusConflict)
			return
		}
		log.Error("user creation failed", "reason", "user registration failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	err = qtx.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
		EntityType:      entityUser,
		EntityID:        newUser.ID,
		ActionType:      actionUserAdded,
		PerformedByID:   admin.ID,
		PerformedByName: admin.FullName,
		CreatedOn:       time.Now().UTC(),
	})
	if err != nil {
		log.Error("user creation failed", "reason", "audit entry failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	err = tx.Commit()
	if err != nil {
		log.Error("user creation failed", "reason", "db commit failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	log.Info("user created", "created_user_id", newUser.ID, "created_email", newUser.Email, "created_role", newUser.Role)
	resBody := CreateUserResponse{
		ID:        newUser.ID,
		FullName:  newUser.FullName,
		Email:     newUser.Email,
		Role:      newUser.Role,
		IsActive:  newUser.IsActive,
		CreatedOn: newUser.CreatedOn,
	}
	respondWithJSON(w, http.StatusCreated, resBody)
}

func (cfg *apiConfig) HandlerListUsers(w http.ResponseWriter, r *http.Request, admin database.User) {
	type UserResponse struct {
		ID        uuid.UUID `json:"id"`
		FullName  string    `json:"full_name"`
		Email     string    `json:"email"`
		Role      string    `json:"role"`
		IsActive  bool      `json:"is_active"`
		CreatedOn time.Time `json:"created_on"`
	}

	type ListUsersResponse struct {
		Users  []UserResponse `json:"users"`
		Total  int64          `json:"total"`
		Limit  int32          `json:"limit"`
		Offset int32          `json:"offset"`
	}
	log := logging.LoggerFrom(r.Context())
	q := r.URL.Query()
	limit, offset, err := parsePagination(q)
	if err != nil {
		log.Warn("user retrieval failed", "reason", "invalid pagination", "error", err,
			"limit_param", q.Get("limit"), "offset_param", q.Get("offset"))
		respondWithError(w, err.Error(), http.StatusBadRequest)
		return
	}
	var isActive *bool
	activeStr := q.Get("active")
	if activeStr != "" {
		b, err := strconv.ParseBool(activeStr)
		if err != nil {
			log.Warn("user retrieval failed", "reason", "invalid active parameter value", "active", activeStr)
			respondWithError(w, "Invalid Query Parameter for Active", http.StatusBadRequest)
			return
		}
		isActive = &b
	}
	users, err := cfg.db.ListUsers(r.Context(), database.ListUsersParams{
		Limit:    limit,
		Offset:   offset,
		IsActive: isActive,
	})
	if err != nil {
		log.Error("user retrieval failed", "reason", "db users list retrieval failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	total, err := cfg.db.CountUsers(r.Context(), isActive)
	if err != nil {
		log.Error("user retrieval failed", "reason", "db user count failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	userResponses := make([]UserResponse, 0, len(users))
	for _, u := range users {
		userResponses = append(userResponses, UserResponse{
			ID:        u.ID,
			FullName:  u.FullName,
			Email:     u.Email,
			Role:      u.Role,
			IsActive:  u.IsActive,
			CreatedOn: u.CreatedOn,
		})
	}
	log.Info("users listed", "count", len(users), "total", total, "limit", limit, "offset", offset)
	respondWithJSON(w, http.StatusOK, ListUsersResponse{
		Users:  userResponses,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}
