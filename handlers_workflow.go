package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/lain-the-coder/ea-qms-backend/internal/auth"
	"github.com/lain-the-coder/ea-qms-backend/internal/database"
	"github.com/lain-the-coder/ea-qms-backend/internal/logging"
)

func (cfg *apiConfig) HandlerSubmitForImplApproval(w http.ResponseWriter, r *http.Request, user database.User) {
	// email/password for e-sig verification, no other fields needed for this request from user
	type SubmitRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type validationErrorResponse struct {
		Error  string   `json:"error"`
		Issues []string `json:"issues"`
	}
	log := logging.LoggerFrom(r.Context())
	// extract and validate path parameter
	ccIDRawStr := r.PathValue("ccID")
	ccID := strings.TrimSpace(ccIDRawStr)
	if ccID == "" {
		log.Warn("submit for implementation approval failed", "reason", "CC-ID blank")
		respondWithError(w, "CC-ID cannot be blank", http.StatusBadRequest)
		return
	}
	// decode request body
	reqBody := SubmitRequest{}
	err := json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		log.Warn("submit for implementation approval failed", "reason", "malformed request body", "error", err)
		respondWithError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	// request validation
	reqBody.Email = strings.TrimSpace(reqBody.Email)
	if reqBody.Email == "" {
		log.Warn("submit for implementation approval failed", "reason", "email blank")
		respondWithError(w, "Email cannot be blank", http.StatusBadRequest)
		return
	}
	if reqBody.Password == "" {
		log.Warn("submit for implementation approval failed", "reason", "password blank")
		respondWithError(w, "Password cannot be blank", http.StatusBadRequest)
		return
	}
	// open transaction
	tx, err := cfg.rawDB.BeginTx(r.Context(), nil)
	if err != nil {
		log.Error("submit for implementation approval failed", "reason", "could not begin transaction", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	qtx := cfg.db.WithTx(tx)
	// retrieve cc details with intent of updating
	cc, err := qtx.GetChangeControlForUpdate(r.Context(), ccID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("submit for implementation approval failed", "reason", "cc not found", "cc_id", ccID)
			respondWithError(w, "Change Control not found", http.StatusNotFound)
			return
		}
		log.Error("submit for implementation approval failed", "reason", "cc lookup failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// ownership check
	if user.ID != cc.ChangeOwnerID {
		log.Warn("submit for implementation approval failed", "reason", "user is not owner of cc", "cc_id", ccID)
		respondWithError(w, "Forbidden", http.StatusForbidden)
		return
	}
	// state check
	if cc.CurrentState != stateInitiated {
		log.Warn("submit for implementation approval failed", "reason",
			"submit for implementation approval allowed only in initiated state", "cc_id", ccID)
		respondWithError(w, "Submit for Implementation Approval only allowed at Initiated state of CC", http.StatusConflict)
		return
	}
	// presence check of all required fields needed to transition to Pending Implementation state
	var problems []string
	if cc.ChangeTitle == nil {
		problems = append(problems, "Change Title")
	}
	if cc.ChangeDescription == nil {
		problems = append(problems, "Change Description")
	}
	if cc.ChangeType == nil {
		problems = append(problems, "Change Type")
	}
	if cc.ChangeCategory == nil {
		problems = append(problems, "Change Category")
	}
	if cc.DepartmentFunction == nil {
		problems = append(problems, "Department / Function")
	}
	if cc.AffectedSystemsModules == nil {
		problems = append(problems, "Affected Systems / Modules")
	}
	if cc.ProposedImplementationDate == nil {
		problems = append(problems, "Proposed Implementation Date")
	}
	if cc.TargetClosureDate == nil {
		problems = append(problems, "Target Closure Date")
	}
	if cc.ReasonForChange == nil {
		problems = append(problems, "Reason for Change")
	}
	if cc.BusinessImpact == nil {
		problems = append(problems, "Business Impact")
	}
	if cc.ExpectedDowntime == nil {
		problems = append(problems, "Expected Downtime")
	}
	if cc.RequiresTesting == nil {
		problems = append(problems, "Requires Testing")
	}
	if cc.RequiresTraining == nil {
		problems = append(problems, "Requires Training")
	}
	if cc.RiskRationale == nil {
		problems = append(problems, "Risk Rationale")
	}
	if cc.KeyRisksMitigations == nil {
		problems = append(problems, "Key Risks & Mitigations")
	}
	if cc.HighLevelImplementationPlan == nil {
		problems = append(problems, "High-Level Implementation Plan")
	}
	if cc.ValidationApproach == nil {
		problems = append(problems, "Validation Approach")
	}
	if cc.SuccessCriteria == nil {
		problems = append(problems, "Success Criteria")
	}
	if cc.RollbackBackoutPlan == nil {
		problems = append(problems, "Rollback / Backout Plan")
	}
	if cc.AssignedApproverID == nil {
		problems = append(problems, "Assigned Approver")
	}
	// Business rules — the two date checks. Only run when the date is present;
	// a missing date is already in `problems`
	// Compare DATES, not instants: proposed_implementation_date is a DATE column,
	// so it arrives as midnight UTC. Truncating "now" to midnight UTC puts both
	// sides on the same footing — otherwise a same-day comparison fails by hours.
	nowUTC := time.Now().UTC()
	today := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)
	if cc.ProposedImplementationDate != nil {
		earliest := businessDaysFrom(today, 2)
		if cc.ProposedImplementationDate.Before(earliest) {
			problems = append(problems,
				"Proposed Implementation Date must be at least 2 business days from today")
		}
	}
	if cc.TargetClosureDate != nil {
		earliest := businessDaysFrom(today, 10)
		if cc.TargetClosureDate.Before(earliest) {
			problems = append(problems,
				"Target Closure Date must be at least 10 business days from today")
		}
	}
	if len(problems) > 0 {
		log.Warn("submit for implementation approval failed", "reason", "validation failed",
			"cc_id", ccID, "problem_count", len(problems))
		respondWithJSON(w, http.StatusBadRequest, validationErrorResponse{
			Error:  "Cannot submit: some requirements are not met",
			Issues: problems,
		})
		return
	}
	// use same timestamp for audit entry and e-sig entry
	now := time.Now().UTC()
	// e-sig credentials email check
	if !strings.EqualFold(reqBody.Email, user.Email) {
		// written with cfg.db, NOT qtx — this must survive the rollback
		err = cfg.db.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
			EntityType:      entityChangeControl,
			EntityID:        cc.ID,
			ActionType:      actionSignatureFailed,
			PerformedByID:   user.ID,
			PerformedByName: user.FullName,
			CreatedOn:       now,
		})
		log.Warn("submit for implementation approval failed", "reason", "email does not match")
		if err != nil {
			log.Error("submit for implementation approval failed", "reason", "audit entry for signature failure failed", "error", err)
		}
		respondWithError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	// e-sig credentials password check
	match, err := auth.CheckPasswordHash(reqBody.Password, user.HashedPassword)
	if err != nil {
		log.Error("submit for implementation approval failed", "reason", "password verification error", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	if !match {
		// written with cfg.db, NOT qtx — this must survive the rollback
		err = cfg.db.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
			EntityType:      entityChangeControl,
			EntityID:        cc.ID,
			ActionType:      actionSignatureFailed,
			PerformedByID:   user.ID,
			PerformedByName: user.FullName,
			CreatedOn:       now,
		})
		log.Warn("submit for implementation approval failed", "reason", "password mismatch")
		if err != nil {
			log.Error("submit for implementation approval failed", "reason", "audit entry for signature failure failed", "error", err)
		}
		respondWithError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	_, err = qtx.SubmitForImplApproval(r.Context(), database.SubmitForImplApprovalParams{
		CcID:                         ccID,
		CurrentState:                 statePendingImplApproval,
		ImplementationApprovalStatus: approvalPending,
		LastUpdatedByID:              user.ID,
	})
	if err != nil {
		log.Error("submit for implementation approval failed", "reason", "cc update failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	err = qtx.InsertESignature(r.Context(), database.InsertESignatureParams{
		ChangeControlID: cc.ID,
		SignerID:        user.ID,
		SignerName:      user.FullName,
		Transition:      transitionT2,
		Meaning:         meaningSubmittedImplApproval,
		SignedOn:        now,
	})
	if err != nil {
		log.Error("submit for implementation approval failed", "reason", "e-sig row insertion failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	err = qtx.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
		EntityType:      entityChangeControl,
		EntityID:        cc.ID,
		ActionType:      actionStateChanged,
		FieldName:       strPtr("current_state"),
		OldValue:        strPtr(stateInitiated),
		NewValue:        strPtr(statePendingImplApproval),
		PerformedByID:   user.ID,
		PerformedByName: user.FullName,
		CreatedOn:       now,
	})
	if err != nil {
		log.Error("submit for implementation approval failed", "reason",
			"audit entry for cc state change to Pending Implementation Approval failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	err = qtx.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
		EntityType:      entityChangeControl,
		EntityID:        cc.ID,
		ActionType:      actionSignatureCaptured,
		PerformedByID:   user.ID,
		PerformedByName: user.FullName,
		CreatedOn:       now,
	})
	if err != nil {
		log.Error("submit for implementation approval failed", "reason", "audit entry for signature capture failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// re-fetch with the five user joins so the response matches GET /{ccID}.
	// Read inside the transaction, before the commit: a failure at any point
	// then leaves nothing written, so the error and the record's state agree.
	// A transaction reads its own uncommitted writes, so this returns the
	// post-transition values.
	row, err := qtx.GetChangeControlByCcID(r.Context(), ccID)
	if err != nil {
		log.Error("submit for implementation approval failed", "reason", "cc re-fetch failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// commit
	err = tx.Commit()
	if err != nil {
		log.Error("submit for implementation approval failed", "reason", "db commit failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	log.Info("submitted for implementation approval", "cc_id", ccID,
		"new_state", statePendingImplApproval, "approver_id", cc.AssignedApproverID)

	// email notification deferred for first release (FR-6.4.1 — no SMTP in Phase 1)
	log.Info("notification pending", "type", "submitted_for_approval",
		"cc_id", ccID, "recipient_id", cc.AssignedApproverID)

	respondWithJSON(w, http.StatusOK, toChangeControlResponse(row))
}
