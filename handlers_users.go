package main

import (
	"database/sql"
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
		log.Warn("user creation failed", "reason", "malformed request body", "error", err)
		respondWithError(w, "Invalid request body", http.StatusBadRequest)
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

func (cfg *apiConfig) HandlerListApprovers(w http.ResponseWriter, r *http.Request, _ database.User) {
	type ApproverResponse struct {
		ID       uuid.UUID `json:"id"`
		FullName string    `json:"full_name"`
	}

	type ListApproversResponse struct {
		Approvers []ApproverResponse `json:"approvers"`
	}
	log := logging.LoggerFrom(r.Context())
	approvers, err := cfg.db.ListApprovers(r.Context())
	if err != nil {
		log.Error("approver retrieval failed", "reason", "db approver list retrieval failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	approverResponses := make([]ApproverResponse, 0, len(approvers))
	for _, a := range approvers {
		approverResponses = append(approverResponses, ApproverResponse{
			ID:       a.ID,
			FullName: a.FullName,
		})
	}
	log.Info("approvers listed", "count", len(approvers))
	respondWithJSON(w, http.StatusOK, ListApproversResponse{
		Approvers: approverResponses,
	})
}

func (cfg *apiConfig) HandlerUpdateUserStatus(w http.ResponseWriter, r *http.Request, admin database.User) {
	type UpdateUserStatusRequest struct {
		IsActive *bool `json:"is_active"` // using *bool to capture empty body from user
	}
	type blockedResponse struct {
		Error   string   `json:"error"`
		Blocked []string `json:"blocked_cc_ids"`
	}
	type UserStatusResponse struct {
		ID        uuid.UUID `json:"id"`
		FullName  string    `json:"full_name"`
		Email     string    `json:"email"`
		Role      string    `json:"role"`
		IsActive  bool      `json:"is_active"`
		UpdatedOn time.Time `json:"updated_on"`
	}
	log := logging.LoggerFrom(r.Context())
	// extract path parameter
	userIDStr := r.PathValue("userID")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		log.Warn("user status update failed", "reason", "invalid userID parameter value", "user_id_param", userIDStr)
		respondWithError(w, "Invalid Path Parameter for User ID", http.StatusBadRequest)
		return
	}
	// decode request body
	reqBody := UpdateUserStatusRequest{}
	err = json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		log.Warn("user status update failed", "reason", "malformed request body", "error", err)
		respondWithError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	// empty body check
	if reqBody.IsActive == nil {
		log.Warn("user status update failed", "reason", "empty is_active value")
		respondWithError(w, "Empty is_active value", http.StatusBadRequest)
		return
	}
	isActive := boolValue(reqBody.IsActive)
	// self deactivation check
	if userID == admin.ID && !isActive {
		log.Warn("user status update failed", "reason", "self deactivation")
		respondWithError(w, "Self Deactivation is not allowed", http.StatusBadRequest)
		return
	}
	// open transaction
	tx, err := cfg.rawDB.BeginTx(r.Context(), nil)
	if err != nil {
		log.Error("user status update failed", "reason", "could not begin transaction", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	qtx := cfg.db.WithTx(tx)
	user, err := qtx.GetUserForUpdate(r.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("user status update failed", "reason", "user not found", "target_user_id", userID)
			respondWithError(w, "User Not Found", http.StatusNotFound)
			return
		}
		log.Error("user status update failed", "reason", "user lookup failed", "target_user_id", userID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// no-op if current user status and request status is same aka no delta
	if user.IsActive == isActive {
		err = tx.Commit()
		if err != nil {
			log.Error("user status update failed", "reason", "db commit failed", "error", err)
			respondWithError(w, "Something went wrong", http.StatusInternalServerError)
			return
		}
		log.Info("user status unchanged", "target_user_id", user.ID, "is_active", isActive)
		respondWithJSON(w, http.StatusOK, UserStatusResponse{
			ID:        user.ID,
			FullName:  user.FullName,
			Email:     user.Email,
			Role:      user.Role,
			IsActive:  user.IsActive,
			UpdatedOn: user.UpdatedOn,
		})
		return
	}
	// Active CC guard, deactivation only
	if !isActive {
		activeCCsForUser, err := qtx.ListActiveCCIDsForUser(r.Context(), userID)
		if err != nil {
			log.Error("user status update failed", "reason", "cc lookup failed", "error", err)
			respondWithError(w, "Something went wrong", http.StatusInternalServerError)
			return
		}
		if len(activeCCsForUser) > 0 {
			log.Warn("user status update failed", "reason", "user has active change controls",
				"target_user_id", userID, "blocking_count", len(activeCCsForUser))
			respondWithJSON(w, http.StatusConflict, blockedResponse{
				Error:   "Cannot deactivate a user with active CCs",
				Blocked: activeCCsForUser,
			})
			return
		}
	}
	updatedUser, err := qtx.SetUserActiveStatus(r.Context(), database.SetUserActiveStatusParams{
		ID:       user.ID,
		IsActive: isActive,
	})
	if err != nil {
		log.Error("user status update failed", "reason", "user update failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// audit row — purpose-built action type for deactivation, generic for reactivation
	action := actionUserDeactivated
	if isActive {
		action = actionUserUpdated
	}
	err = qtx.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
		EntityType:      entityUser,
		EntityID:        updatedUser.ID,
		ActionType:      action,
		FieldName:       strPtr("is_active"),
		OldValue:        strPtr(strconv.FormatBool(user.IsActive)),
		NewValue:        strPtr(strconv.FormatBool(updatedUser.IsActive)),
		PerformedByID:   admin.ID,
		PerformedByName: admin.FullName,
		CreatedOn:       time.Now().UTC(),
	})
	if err != nil {
		log.Error("user status update failed", "reason", "audit entry failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	// commit
	err = tx.Commit()
	if err != nil {
		log.Error("user status update failed", "reason", "db commit failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	log.Info("user status updated", "target_user_id", updatedUser.ID, "is_active", updatedUser.IsActive)
	respondWithJSON(w, http.StatusOK, UserStatusResponse{
		ID:        updatedUser.ID,
		FullName:  updatedUser.FullName,
		Email:     updatedUser.Email,
		Role:      updatedUser.Role,
		IsActive:  updatedUser.IsActive,
		UpdatedOn: updatedUser.UpdatedOn,
	})
}

func (cfg *apiConfig) HandlerUpdateUserDetails(w http.ResponseWriter, r *http.Request, admin database.User) {
	type UpdateUserRequest struct {
		FullName *string `json:"full_name"` // *string to capture/allow empty fields since sometimes only one of these will be updated
		Role     *string `json:"role"`      // *string same as above
	}
	type blockedResponse struct {
		Error   string   `json:"error"`
		Blocked []string `json:"blocked_cc_ids"`
	}
	type UserStatusResponse struct {
		ID        uuid.UUID `json:"id"`
		FullName  string    `json:"full_name"`
		Email     string    `json:"email"`
		Role      string    `json:"role"`
		IsActive  bool      `json:"is_active"`
		UpdatedOn time.Time `json:"updated_on"`
	}
	log := logging.LoggerFrom(r.Context())
	// extract path parameter
	userIDStr := r.PathValue("userID")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		log.Warn("user details update failed", "reason", "invalid userID parameter value", "user_id_param", userIDStr)
		respondWithError(w, "Invalid Path Parameter for User ID", http.StatusBadRequest)
		return
	}
	// decode request body
	reqBody := UpdateUserRequest{}
	err = json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		log.Warn("user details update failed", "reason", "malformed request body", "error", err)
		respondWithError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	// empty body check
	if reqBody.FullName == nil && reqBody.Role == nil {
		log.Warn("user details update failed", "reason", "empty Full Name and Role values")
		respondWithError(w, "Empty Full Name and Role values", http.StatusBadRequest)
		return
	}
	// request validation
	fullName := strValue(reqBody.FullName)
	role := strValue(reqBody.Role)
	fullName = strings.TrimSpace(fullName)
	role = strings.TrimSpace(role)
	if reqBody.FullName != nil && fullName == "" {
		log.Warn("user details update failed", "reason", "full name blank")
		respondWithError(w, "Full Name cannot be blank", http.StatusBadRequest)
		return
	}
	if reqBody.Role != nil && role == "" {
		log.Warn("user details update failed", "reason", "role blank")
		respondWithError(w, "Role cannot be blank", http.StatusBadRequest)
		return
	}
	if reqBody.Role != nil {
		isValid := false
		switch role {
		case roleAdmin, roleCCOwner, roleApprover, roleViewer:
			isValid = true
		}
		if !isValid {
			log.Warn("user details update failed", "reason", "invalid role")
			respondWithError(w, "Invalid Role", http.StatusBadRequest)
			return
		}
	}
	// self guard check
	if userID == admin.ID && reqBody.Role != nil && role != admin.Role {
		log.Warn("user details update failed", "reason", "self role change")
		respondWithError(w, "Self Role Change is not allowed", http.StatusBadRequest)
		return
	}
	// open transaction
	tx, err := cfg.rawDB.BeginTx(r.Context(), nil)
	if err != nil {
		log.Error("user details update failed", "reason", "could not begin transaction", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	qtx := cfg.db.WithTx(tx)
	current, err := qtx.GetUserForUpdate(r.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("user details update failed", "reason", "user not found", "target_user_id", userID)
			respondWithError(w, "User Not Found", http.StatusNotFound)
			return
		}
		log.Error("user details update failed", "reason", "user lookup failed", "target_user_id", userID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	nameChanged := reqBody.FullName != nil && fullName != current.FullName
	roleChanged := reqBody.Role != nil && role != current.Role
	// no-op if current user name and role is same as requested to change aka no delta; catches also a name-only request
	if !nameChanged && !roleChanged {
		err = tx.Commit()
		if err != nil {
			log.Error("user details update failed", "reason", "db commit failed", "error", err)
			respondWithError(w, "Something went wrong", http.StatusInternalServerError)
			return
		}
		log.Info("user details unchanged", "target_user_id", current.ID, "full_name", current.FullName, "role", current.Role)
		respondWithJSON(w, http.StatusOK, UserStatusResponse{
			ID:        current.ID,
			FullName:  current.FullName,
			Email:     current.Email,
			Role:      current.Role,
			IsActive:  current.IsActive,
			UpdatedOn: current.UpdatedOn,
		})
		return
	}
	// Active CC guard for role change
	if roleChanged {
		activeCCsForUser, err := qtx.ListActiveCCIDsForUser(r.Context(), userID)
		if err != nil {
			log.Error("user details update failed", "reason", "cc lookup failed", "error", err)
			respondWithError(w, "Something went wrong", http.StatusInternalServerError)
			return
		}
		if len(activeCCsForUser) > 0 {
			log.Warn("user details update failed", "reason", "user has active change controls and role cannot be updated",
				"target_user_id", userID, "blocking_count", len(activeCCsForUser))
			respondWithJSON(w, http.StatusConflict, blockedResponse{
				Error:   "Cannot update role of a user with active CCs",
				Blocked: activeCCsForUser,
			})
			return
		}
	}
	now := time.Now().UTC() // captured once, before both inserts
	// full name update
	if nameChanged {
		updatedUser, err := qtx.UpdateUserName(r.Context(), database.UpdateUserNameParams{
			ID:       current.ID,
			FullName: fullName,
		})
		if err != nil {
			log.Error("user details update failed", "reason", "user full name update failed", "target_user_id", userID, "error", err)
			respondWithError(w, "Something went wrong", http.StatusInternalServerError)
			return
		}
		err = qtx.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
			EntityType:      entityUser,
			EntityID:        updatedUser.ID,
			ActionType:      actionUserUpdated,
			FieldName:       strPtr("full_name"),
			OldValue:        strPtr(current.FullName),
			NewValue:        strPtr(updatedUser.FullName),
			PerformedByID:   admin.ID,
			PerformedByName: admin.FullName,
			CreatedOn:       now,
		})
		if err != nil {
			log.Error("user details update failed", "reason", "audit entry for user full name update failed", "error", err)
			respondWithError(w, "Something went wrong", http.StatusInternalServerError)
			return
		}
		current.FullName = fullName
		current.UpdatedOn = updatedUser.UpdatedOn
	}
	// role update
	if roleChanged {
		updatedUser, err := qtx.UpdateUserRole(r.Context(), database.UpdateUserRoleParams{
			ID:   current.ID,
			Role: role,
		})
		if err != nil {
			log.Error("user details update failed", "reason", "user role update failed", "target_user_id", userID, "error", err)
			respondWithError(w, "Something went wrong", http.StatusInternalServerError)
			return
		}
		err = qtx.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
			EntityType:      entityUser,
			EntityID:        updatedUser.ID,
			ActionType:      actionUserRoleChanged,
			FieldName:       strPtr("role"),
			OldValue:        strPtr(current.Role),
			NewValue:        strPtr(updatedUser.Role),
			PerformedByID:   admin.ID,
			PerformedByName: admin.FullName,
			CreatedOn:       now,
		})
		if err != nil {
			log.Error("user details update failed", "reason", "audit entry for user role update failed", "error", err)
			respondWithError(w, "Something went wrong", http.StatusInternalServerError)
			return
		}
		current.Role = role
		current.UpdatedOn = updatedUser.UpdatedOn
	}
	// commit
	err = tx.Commit()
	if err != nil {
		log.Error("user details update failed", "reason", "db commit failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	log.Info("user details updated", "target_user_id", current.ID, "full_name", current.FullName, "role", current.Role)
	respondWithJSON(w, http.StatusOK, UserStatusResponse{
		ID:        current.ID,
		FullName:  current.FullName,
		Email:     current.Email,
		Role:      current.Role,
		IsActive:  current.IsActive,
		UpdatedOn: current.UpdatedOn,
	})
}
