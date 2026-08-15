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
	log.Info("notification pending", "type", notifySubmittedForApproval,
		"cc_id", ccID, "recipient_id", cc.AssignedApproverID)

	respondWithJSON(w, http.StatusOK, toChangeControlResponse(row))
}

func (cfg *apiConfig) HandlerCancelChangeControl(w http.ResponseWriter, r *http.Request, user database.User) {
	type CancelRequest struct {
		CancellationReason string `json:"cancellation_reason"`
		Email              string `json:"email"`
		Password           string `json:"password"`
	}
	log := logging.LoggerFrom(r.Context())
	// extract and validate path parameter
	ccIDRawStr := r.PathValue("ccID")
	ccID := strings.TrimSpace(ccIDRawStr)
	if ccID == "" {
		log.Warn("cc cancellation failed", "reason", "CC-ID blank")
		respondWithError(w, "CC-ID cannot be blank", http.StatusBadRequest)
		return
	}
	// decode request body
	reqBody := CancelRequest{}
	err := json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		log.Warn("cc cancellation failed", "reason", "malformed request body", "error", err)
		respondWithError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	// request validation
	reqBody.CancellationReason = strings.TrimSpace(reqBody.CancellationReason)
	if reqBody.CancellationReason == "" {
		log.Warn("cc cancellation failed", "reason", "cancellation reason blank")
		respondWithError(w, "Cancellation Reason cannot be blank", http.StatusBadRequest)
		return
	}
	reqBody.Email = strings.TrimSpace(reqBody.Email)
	if reqBody.Email == "" {
		log.Warn("cc cancellation failed", "reason", "email blank")
		respondWithError(w, "Email cannot be blank", http.StatusBadRequest)
		return
	}
	if reqBody.Password == "" {
		log.Warn("cc cancellation failed", "reason", "password blank")
		respondWithError(w, "Password cannot be blank", http.StatusBadRequest)
		return
	}
	// max length — 500 runes
	if len([]rune(reqBody.CancellationReason)) > 500 {
		log.Warn("cc cancellation failed", "reason", "cancellation reason must be 500 characters or fewer", "cc_id", ccID)
		respondWithError(w, "Cancellation Reason must be 500 characters or fewer", http.StatusBadRequest)
		return
	}
	// open transaction
	tx, err := cfg.rawDB.BeginTx(r.Context(), nil)
	if err != nil {
		log.Error("cc cancellation failed", "reason", "could not begin transaction", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	qtx := cfg.db.WithTx(tx)
	// retrieve cc details with intent of updating
	cc, err := qtx.GetChangeControlForUpdate(r.Context(), ccID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("cc cancellation failed", "reason", "cc not found", "cc_id", ccID)
			respondWithError(w, "Change Control not found", http.StatusNotFound)
			return
		}
		log.Error("cc cancellation failed", "reason", "cc lookup failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// ownership check
	if user.ID != cc.ChangeOwnerID {
		log.Warn("cc cancellation failed", "reason", "user is not owner of cc", "cc_id", ccID)
		respondWithError(w, "Forbidden", http.StatusForbidden)
		return
	}
	// state check
	if cc.CurrentState != stateInitiated {
		log.Warn("cc cancellation failed", "reason",
			"cancellation only allowed from initiated state", "cc_id", ccID)
		respondWithError(w, "Cancellation only allowed from Initiated state of CC", http.StatusConflict)
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
		log.Warn("cc cancellation failed", "reason", "email does not match")
		if err != nil {
			log.Error("cc cancellation failed", "reason", "audit entry for signature failure failed", "error", err)
		}
		respondWithError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	// e-sig credentials password check
	match, err := auth.CheckPasswordHash(reqBody.Password, user.HashedPassword)
	if err != nil {
		log.Error("cc cancellation failed", "reason", "password verification error", "error", err)
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
		log.Warn("cc cancellation failed", "reason", "password mismatch")
		if err != nil {
			log.Error("cc cancellation failed", "reason", "audit entry for signature failure failed", "error", err)
		}
		respondWithError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	// cancel change control
	_, err = qtx.CancelChangeControl(r.Context(), database.CancelChangeControlParams{
		CcID:                         ccID,
		CurrentState:                 stateCancelled,
		ImplementationApprovalStatus: approvalNA,
		FinalApprovalStatus:          approvalNA,
		CancellationReason:           strPtr(reqBody.CancellationReason),
		LastUpdatedByID:              user.ID,
	})
	if err != nil {
		log.Error("cc cancellation failed", "reason", "cc state update failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// e-sig table entry
	err = qtx.InsertESignature(r.Context(), database.InsertESignatureParams{
		ChangeControlID: cc.ID,
		SignerID:        user.ID,
		SignerName:      user.FullName,
		Transition:      transitionT3,
		Meaning:         meaningCancelled,
		SignedOn:        now,
	})
	if err != nil {
		log.Error("cc cancellation failed", "reason", "e-sig row insertion failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// audit entry for state change
	err = qtx.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
		EntityType:      entityChangeControl,
		EntityID:        cc.ID,
		ActionType:      actionStateChanged,
		FieldName:       strPtr("current_state"),
		OldValue:        strPtr(stateInitiated),
		NewValue:        strPtr(stateCancelled),
		PerformedByID:   user.ID,
		PerformedByName: user.FullName,
		CreatedOn:       now,
	})
	if err != nil {
		log.Error("cc cancellation failed", "reason",
			"audit entry for cc state change to Cancelled failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// audit entry for cancellation reason field
	err = qtx.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
		EntityType:      entityChangeControl,
		EntityID:        cc.ID,
		ActionType:      actionFieldUpdated,
		FieldName:       strPtr("cancellation_reason"),
		OldValue:        nil,
		NewValue:        strPtr(reqBody.CancellationReason),
		PerformedByID:   user.ID,
		PerformedByName: user.FullName,
		CreatedOn:       now,
	})
	if err != nil {
		log.Error("cc cancellation failed", "reason", "audit entry for cancellation_reason field failed",
			"cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// audit entry for successful signature
	err = qtx.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
		EntityType:      entityChangeControl,
		EntityID:        cc.ID,
		ActionType:      actionSignatureCaptured,
		PerformedByID:   user.ID,
		PerformedByName: user.FullName,
		CreatedOn:       now,
	})
	if err != nil {
		log.Error("cc cancellation failed", "reason", "audit entry for signature capture failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// fetch for response
	row, err := qtx.GetChangeControlByCcID(r.Context(), ccID)
	if err != nil {
		log.Error("cc cancellation failed", "reason", "cc re-fetch failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// commit
	err = tx.Commit()
	if err != nil {
		log.Error("cc cancellation failed", "reason", "db commit failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	log.Info("cc cancelled", "cc_id", ccID, "new_state", stateCancelled)
	// receives notification if previously assigned
	if cc.AssignedApproverID != nil {
		log.Info("notification pending", "type", notifyCCCancelled, "cc_id", ccID, "recipient_id", *cc.AssignedApproverID)
	}
	respondWithJSON(w, http.StatusOK, toChangeControlResponse(row))
}

func (cfg *apiConfig) HandlerImplementationDecision(w http.ResponseWriter, r *http.Request, approver database.User) {
	type DecisionRequest struct {
		Decision         string `json:"decision"`
		RiskLevel        string `json:"risk_level"`
		DecisionComments string `json:"decision_comments"`
		Email            string `json:"email"`
		Password         string `json:"password"`
	}
	log := logging.LoggerFrom(r.Context())
	// extract and validate path parameter
	ccIDRawStr := r.PathValue("ccID")
	ccID := strings.TrimSpace(ccIDRawStr)
	if ccID == "" {
		log.Warn("implementation decision failed", "reason", "CC-ID blank")
		respondWithError(w, "CC-ID cannot be blank", http.StatusBadRequest)
		return
	}
	// decode request body
	reqBody := DecisionRequest{}
	err := json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		log.Warn("implementation decision failed", "reason", "malformed request body", "error", err)
		respondWithError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	// request validation
	// decision field validation
	reqBody.Decision = strings.TrimSpace(reqBody.Decision)
	if reqBody.Decision == "" {
		log.Warn("implementation decision failed", "reason", "decision blank")
		respondWithError(w, "Decision cannot be blank", http.StatusBadRequest)
		return
	}
	switch reqBody.Decision {
	case decisionApprove, decisionReject:
	default:
		log.Warn("implementation decision failed", "reason", "invalid decision parameter value", "decision", reqBody.Decision)
		respondWithError(w, "Invalid decision", http.StatusBadRequest)
		return
	}
	// risk level field validation
	reqBody.RiskLevel = strings.TrimSpace(reqBody.RiskLevel)
	if reqBody.RiskLevel == "" {
		log.Warn("implementation decision failed", "reason", "risk level blank")
		respondWithError(w, "Risk Level cannot be blank", http.StatusBadRequest)
		return
	}
	switch reqBody.RiskLevel {
	case riskLow, riskMedium, riskHigh:
	default:
		log.Warn("implementation decision failed", "reason", "invalid risk level parameter value", "risk_level", reqBody.RiskLevel)
		respondWithError(w, "Invalid risk level", http.StatusBadRequest)
		return
	}
	// decision comments field validation
	reqBody.DecisionComments = strings.TrimSpace(reqBody.DecisionComments)
	if reqBody.DecisionComments == "" {
		log.Warn("implementation decision failed", "reason", "decision comments blank")
		respondWithError(w, "Decision Comments cannot be blank", http.StatusBadRequest)
		return
	}
	// max length — 2000 runes
	if len([]rune(reqBody.DecisionComments)) > 2000 {
		log.Warn("implementation decision failed", "reason", "Decision Comments must be 2000 characters or fewer", "cc_id", ccID)
		respondWithError(w, "Decision Comments must be 2000 characters or fewer", http.StatusBadRequest)
		return
	}
	// email validation
	reqBody.Email = strings.TrimSpace(reqBody.Email)
	if reqBody.Email == "" {
		log.Warn("implementation decision failed", "reason", "email blank")
		respondWithError(w, "Email cannot be blank", http.StatusBadRequest)
		return
	}
	// password validation
	if reqBody.Password == "" {
		log.Warn("implementation decision failed", "reason", "password blank")
		respondWithError(w, "Password cannot be blank", http.StatusBadRequest)
		return
	}
	// open transaction
	tx, err := cfg.rawDB.BeginTx(r.Context(), nil)
	if err != nil {
		log.Error("implementation decision failed", "reason", "could not begin transaction", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	qtx := cfg.db.WithTx(tx)
	// retrieve cc details with intent of updating
	cc, err := qtx.GetChangeControlForUpdate(r.Context(), ccID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("implementation decision failed", "reason", "cc not found", "cc_id", ccID)
			respondWithError(w, "Change Control not found", http.StatusNotFound)
			return
		}
		log.Error("implementation decision failed", "reason", "cc lookup failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// approver ownership check
	// nil check first since Assigned Approver field is nullable unlike Change Owner field
	if cc.AssignedApproverID == nil || *cc.AssignedApproverID != approver.ID {
		log.Warn("implementation decision failed", "reason", "user is not approver of cc", "cc_id", ccID)
		respondWithError(w, "Forbidden", http.StatusForbidden)
		return
	}
	// state check
	if cc.CurrentState != statePendingImplApproval {
		log.Warn("implementation decision failed", "reason",
			"implementation approval only allowed from Pending Implementation Approval state", "cc_id", ccID)
		respondWithError(w, "Implementation Approval only allowed from Pending Implementation Approval state of CC",
			http.StatusConflict)
		return
	}
	// use same timestamp for audit entry and e-sig entry
	now := time.Now().UTC()
	// e-sig credentials email check
	if !strings.EqualFold(reqBody.Email, approver.Email) {
		// written with cfg.db, NOT qtx — this must survive the rollback
		err = cfg.db.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
			EntityType:      entityChangeControl,
			EntityID:        cc.ID,
			ActionType:      actionSignatureFailed,
			PerformedByID:   approver.ID,
			PerformedByName: approver.FullName,
			CreatedOn:       now,
		})
		log.Warn("implementation decision failed", "reason", "email does not match")
		if err != nil {
			log.Error("implementation decision failed", "reason", "audit entry for signature failure failed", "error", err)
		}
		respondWithError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	// e-sig credentials password check
	match, err := auth.CheckPasswordHash(reqBody.Password, approver.HashedPassword)
	if err != nil {
		log.Error("implementation decision failed", "reason", "password verification error", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	if !match {
		// written with cfg.db, NOT qtx — this must survive the rollback
		err = cfg.db.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
			EntityType:      entityChangeControl,
			EntityID:        cc.ID,
			ActionType:      actionSignatureFailed,
			PerformedByID:   approver.ID,
			PerformedByName: approver.FullName,
			CreatedOn:       now,
		})
		log.Warn("implementation decision failed", "reason", "password mismatch")
		if err != nil {
			log.Error("implementation decision failed", "reason", "audit entry for signature failure failed", "error", err)
		}
		respondWithError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	// setting values based on approval/reject branches
	meaning := meaningApprovedImplApproval
	newState := stateInImplementation
	implementationApprovalStatus := approvalApproved
	transition := transitionT4
	notifType := notifyCCApproved
	if reqBody.Decision == decisionReject {
		meaning = meaningRejectedImplApproval
		newState = stateInitiated
		implementationApprovalStatus = approvalNotSubmitted
		transition = transitionT5
		notifType = notifyCCRejected
	}
	// update CC branching based on decision - approve/reject
	if reqBody.Decision == decisionApprove {
		_, err := qtx.ApproveImplementation(r.Context(), database.ApproveImplementationParams{
			CcID:                         ccID,
			CurrentState:                 newState,
			ImplementationApprovalStatus: implementationApprovalStatus,
			Decision:                     strPtr(reqBody.Decision),
			RiskLevel:                    strPtr(reqBody.RiskLevel),
			DecisionComments:             strPtr(reqBody.DecisionComments),
			ImplementationApprovalByID:   &approver.ID,
			ImplementationApprovalOn:     &now,
			LastUpdatedByID:              approver.ID,
		})
		if err != nil {
			log.Error("implementation decision failed", "reason", "cc approve update failed", "cc_id", ccID, "error", err)
			respondWithError(w, "Something went wrong", http.StatusInternalServerError)
			return
		}
	} else {
		_, err := qtx.RejectImplementation(r.Context(), database.RejectImplementationParams{
			CcID:                         ccID,
			CurrentState:                 newState,
			ImplementationApprovalStatus: implementationApprovalStatus,
			Decision:                     strPtr(reqBody.Decision),
			RiskLevel:                    strPtr(reqBody.RiskLevel),
			DecisionComments:             strPtr(reqBody.DecisionComments),
			LastUpdatedByID:              approver.ID,
		})
		if err != nil {
			log.Error("implementation decision failed", "reason", "cc reject update failed", "cc_id", ccID, "error", err)
			respondWithError(w, "Something went wrong", http.StatusInternalServerError)
			return
		}
	}
	// e-sig table entry
	err = qtx.InsertESignature(r.Context(), database.InsertESignatureParams{
		ChangeControlID: cc.ID,
		SignerID:        approver.ID,
		SignerName:      approver.FullName,
		Transition:      transition,
		Meaning:         meaning,
		SignedOn:        now,
	})
	if err != nil {
		log.Error("implementation decision failed", "reason", "e-sig row insertion failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// audit entry for state change
	err = qtx.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
		EntityType:      entityChangeControl,
		EntityID:        cc.ID,
		ActionType:      actionStateChanged,
		FieldName:       strPtr("current_state"),
		OldValue:        strPtr(statePendingImplApproval),
		NewValue:        strPtr(newState),
		PerformedByID:   approver.ID,
		PerformedByName: approver.FullName,
		CreatedOn:       now,
	})
	if err != nil {
		log.Error("implementation decision failed", "reason",
			"audit entry for cc state change failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// audit entry for decision, risk level and decision comments fields
	err = qtx.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
		EntityType:      entityChangeControl,
		EntityID:        cc.ID,
		ActionType:      actionFieldUpdated,
		FieldName:       strPtr("decision"),
		OldValue:        cc.Decision,
		NewValue:        strPtr(reqBody.Decision),
		PerformedByID:   approver.ID,
		PerformedByName: approver.FullName,
		CreatedOn:       now,
	})
	if err != nil {
		log.Error("implementation decision failed", "reason", "audit entry for decision field failed",
			"cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	err = qtx.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
		EntityType:      entityChangeControl,
		EntityID:        cc.ID,
		ActionType:      actionFieldUpdated,
		FieldName:       strPtr("risk_level"),
		OldValue:        cc.RiskLevel,
		NewValue:        strPtr(reqBody.RiskLevel),
		PerformedByID:   approver.ID,
		PerformedByName: approver.FullName,
		CreatedOn:       now,
	})
	if err != nil {
		log.Error("implementation decision failed", "reason", "audit entry for risk level field failed",
			"cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	err = qtx.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
		EntityType:      entityChangeControl,
		EntityID:        cc.ID,
		ActionType:      actionFieldUpdated,
		FieldName:       strPtr("decision_comments"),
		OldValue:        cc.DecisionComments,
		NewValue:        strPtr(reqBody.DecisionComments),
		PerformedByID:   approver.ID,
		PerformedByName: approver.FullName,
		CreatedOn:       now,
	})
	if err != nil {
		log.Error("implementation decision failed", "reason", "audit entry for decision comments field failed",
			"cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// audit entry for successful signature
	err = qtx.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
		EntityType:      entityChangeControl,
		EntityID:        cc.ID,
		ActionType:      actionSignatureCaptured,
		PerformedByID:   approver.ID,
		PerformedByName: approver.FullName,
		CreatedOn:       now,
	})
	if err != nil {
		log.Error("implementation decision failed", "reason", "audit entry for signature capture failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// fetch for response
	row, err := qtx.GetChangeControlByCcID(r.Context(), ccID)
	if err != nil {
		log.Error("implementation decision failed", "reason", "cc re-fetch failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// commit
	err = tx.Commit()
	if err != nil {
		log.Error("implementation decision failed", "reason", "db commit failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	log.Info("cc implementation decision recorded", "cc_id", ccID, "new_state", newState, "decision", reqBody.Decision)
	// email notification deferred for first release (FR-6.4.1 — no SMTP in Phase 1)
	log.Info("notification pending", "type", notifType, "cc_id", ccID, "recipient_id", cc.ChangeOwnerID)
	respondWithJSON(w, http.StatusOK, toChangeControlResponse(row))
}

func (cfg *apiConfig) HandlerSubmitForFinalApproval(w http.ResponseWriter, r *http.Request, user database.User) {
	type SubmitFinalRequest struct {
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
		log.Warn("submit for final approval failed", "reason", "CC-ID blank")
		respondWithError(w, "CC-ID cannot be blank", http.StatusBadRequest)
		return
	}
	// decode request body
	reqBody := SubmitFinalRequest{}
	err := json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		log.Warn("submit for final approval failed", "reason", "malformed request body", "error", err)
		respondWithError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	// request validation
	reqBody.Email = strings.TrimSpace(reqBody.Email)
	if reqBody.Email == "" {
		log.Warn("submit for final approval failed", "reason", "email blank")
		respondWithError(w, "Email cannot be blank", http.StatusBadRequest)
		return
	}
	if reqBody.Password == "" {
		log.Warn("submit for final approval failed", "reason", "password blank")
		respondWithError(w, "Password cannot be blank", http.StatusBadRequest)
		return
	}
	// open transaction
	tx, err := cfg.rawDB.BeginTx(r.Context(), nil)
	if err != nil {
		log.Error("submit for final approval failed", "reason", "could not begin transaction", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	qtx := cfg.db.WithTx(tx)
	// retrieve cc details with intent of updating
	cc, err := qtx.GetChangeControlForUpdate(r.Context(), ccID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("submit for final approval failed", "reason", "cc not found", "cc_id", ccID)
			respondWithError(w, "Change Control not found", http.StatusNotFound)
			return
		}
		log.Error("submit for final approval failed", "reason", "cc lookup failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// ownership check
	if user.ID != cc.ChangeOwnerID {
		log.Warn("submit for final approval failed", "reason", "user is not owner of cc", "cc_id", ccID)
		respondWithError(w, "Forbidden", http.StatusForbidden)
		return
	}
	// state check
	if cc.CurrentState != stateInImplementation {
		log.Warn("submit for final approval failed", "reason",
			"submit for final approval allowed only in In Implementation state", "cc_id", ccID)
		respondWithError(w, "Submit for Final Approval only allowed at In Implementation state of CC", http.StatusConflict)
		return
	}
	// collect all issues in this slice
	var problems []string
	// presence + business rules check
	// Implementation Evidence field presence
	hasEvidence, err := qtx.FileAttachmentExists(r.Context(), database.FileAttachmentExistsParams{
		ChangeControlID: cc.ID,
		FieldName:       fieldImplementationEvidence,
	})
	if err != nil {
		log.Error("submit for final approval failed", "reason", "file attachment lookup failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	if !hasEvidence {
		problems = append(problems, "Implementation Evidence")
	}
	// for other fields
	if cc.ActualImplementationDate == nil {
		problems = append(problems, "Actual Implementation Date")
	}
	if cc.PostImplementationIssues == nil {
		problems = append(problems, "Post-Implementation Issues")
	}
	if cc.ImplementationSummary == nil {
		problems = append(problems, "Implementation Summary")
	}
	if cc.ValidationPerformed == nil {
		problems = append(problems, "Validation Performed")
	}
	// for date field; cannot be in the future
	nowUTC := time.Now().UTC()
	today := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)
	if cc.ActualImplementationDate != nil && cc.ActualImplementationDate.After(today) {
		problems = append(problems, "Actual Implementation Date cannot be in the future")
	}
	// output collect issues as one error message
	if len(problems) > 0 {
		log.Warn("submit for final approval failed", "reason", "validation failed",
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
		log.Warn("submit for final approval failed", "reason", "email does not match")
		if err != nil {
			log.Error("submit for final approval failed", "reason", "audit entry for signature failure failed", "error", err)
		}
		respondWithError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	// e-sig credentials password check
	match, err := auth.CheckPasswordHash(reqBody.Password, user.HashedPassword)
	if err != nil {
		log.Error("submit for final approval failed", "reason", "password verification error", "error", err)
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
		log.Warn("submit for final approval failed", "reason", "password mismatch")
		if err != nil {
			log.Error("submit for final approval failed", "reason", "audit entry for signature failure failed", "error", err)
		}
		respondWithError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	_, err = qtx.SubmitForFinalApproval(r.Context(), database.SubmitForFinalApprovalParams{
		CcID:                cc.CcID,
		CurrentState:        statePendingFinalApproval,
		FinalApprovalStatus: approvalPending,
		LastUpdatedByID:     user.ID,
	})
	if err != nil {
		log.Error("submit for final approval failed", "reason", "cc update failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	err = qtx.InsertESignature(r.Context(), database.InsertESignatureParams{
		ChangeControlID: cc.ID,
		SignerID:        user.ID,
		SignerName:      user.FullName,
		Transition:      transitionT6,
		Meaning:         meaningSubmittedFinalApproval,
		SignedOn:        now,
	})
	if err != nil {
		log.Error("submit for final approval failed", "reason", "e-sig row insertion failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	err = qtx.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
		EntityType:      entityChangeControl,
		EntityID:        cc.ID,
		ActionType:      actionStateChanged,
		FieldName:       strPtr("current_state"),
		OldValue:        strPtr(stateInImplementation),
		NewValue:        strPtr(statePendingFinalApproval),
		PerformedByID:   user.ID,
		PerformedByName: user.FullName,
		CreatedOn:       now,
	})
	if err != nil {
		log.Error("submit for final approval failed", "reason",
			"audit entry for cc state change to Pending Final Approval failed", "error", err)
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
		log.Error("submit for final approval failed", "reason", "audit entry for signature capture failed", "error", err)
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
		log.Error("submit for final approval failed", "reason", "cc re-fetch failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// commit
	err = tx.Commit()
	if err != nil {
		log.Error("submit for final approval failed", "reason", "db commit failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	log.Info("submitted for final approval", "cc_id", ccID,
		"new_state", statePendingFinalApproval, "approver_id", cc.AssignedApproverID)
	// email notification deferred for first release (FR-6.4.1 — no SMTP in Phase 1)
	log.Info("notification pending", "type", notifySubmittedForFinalApproval,
		"cc_id", ccID, "recipient_id", cc.AssignedApproverID)
	respondWithJSON(w, http.StatusOK, toChangeControlResponse(row))
}
