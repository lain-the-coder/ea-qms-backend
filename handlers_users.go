package main

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/lain-the-coder/ea-qms-backend/internal/database"
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
