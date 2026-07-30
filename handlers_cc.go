package main

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/lain-the-coder/ea-qms-backend/internal/database"
	"github.com/lain-the-coder/ea-qms-backend/internal/logging"
)

func (cfg *apiConfig) HandlerCreateChangeControl(w http.ResponseWriter, r *http.Request, owner database.User) {
	type CreateCCResponse struct {
		ID                           uuid.UUID `json:"id"`
		CcID                         string    `json:"cc_id"`
		CurrentState                 string    `json:"current_state"`
		ChangeOwnerID                uuid.UUID `json:"change_owner_id"`
		ChangeOwnerName              string    `json:"change_owner_name"`
		LastUpdatedByID              uuid.UUID `json:"last_updated_by_id"`
		LastUpdatedByName            string    `json:"last_updated_by_name"`
		CreatedOn                    time.Time `json:"created_on"`
		LastUpdatedOn                time.Time `json:"last_updated_on"`
		ImplementationApprovalStatus string    `json:"implementation_approval_status"`
		FinalApprovalStatus          string    `json:"final_approval_status"`
	}
	log := logging.LoggerFrom(r.Context())
	// open transaction
	tx, err := cfg.rawDB.BeginTx(r.Context(), nil)
	if err != nil {
		log.Error("cc initiation failed", "reason", "could not begin transaction", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	qtx := cfg.db.WithTx(tx)
	createdCC, err := qtx.CreateChangeControl(r.Context(), database.CreateChangeControlParams{
		ChangeOwnerID:   owner.ID,
		LastUpdatedByID: owner.ID,
	})
	if err != nil {
		log.Error("cc initiation failed", "reason", "cc creation failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	err = qtx.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
		EntityType:      entityChangeControl,
		EntityID:        createdCC.ID,
		ActionType:      actionCreated,
		PerformedByID:   owner.ID,
		PerformedByName: owner.FullName,
		CreatedOn:       time.Now().UTC(),
	})
	if err != nil {
		log.Error("cc initiation failed", "reason", "audit entry for cc initiation failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// commit
	err = tx.Commit()
	if err != nil {
		log.Error("cc initiation failed", "reason", "db commit failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	log.Info("change control created", "cc_id", createdCC.CcID, "cc_uuid", createdCC.ID)
	resBody := CreateCCResponse{
		ID:                           createdCC.ID,
		CcID:                         createdCC.CcID,
		CurrentState:                 createdCC.CurrentState,
		ChangeOwnerID:                createdCC.ChangeOwnerID,
		ChangeOwnerName:              owner.FullName,
		LastUpdatedByID:              createdCC.LastUpdatedByID,
		LastUpdatedByName:            owner.FullName,
		CreatedOn:                    createdCC.CreatedOn,
		LastUpdatedOn:                createdCC.LastUpdatedOn,
		ImplementationApprovalStatus: createdCC.ImplementationApprovalStatus,
		FinalApprovalStatus:          createdCC.FinalApprovalStatus,
	}
	respondWithJSON(w, http.StatusCreated, resBody)

}
