package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lain-the-coder/ea-qms-backend/internal/database"
	"github.com/lain-the-coder/ea-qms-backend/internal/logging"
)

type ChangeControlResponse struct {
	// Identification (BRD 1–6)
	ID                uuid.UUID `json:"id"`
	CcID              string    `json:"cc_id"`
	CurrentState      string    `json:"current_state"`
	ChangeOwnerID     uuid.UUID `json:"change_owner_id"`
	ChangeOwnerName   string    `json:"change_owner_name"`
	LastUpdatedByID   uuid.UUID `json:"last_updated_by_id"`
	LastUpdatedByName string    `json:"last_updated_by_name"`
	CreatedOn         time.Time `json:"created_on"`
	LastUpdatedOn     time.Time `json:"last_updated_on"`

	// Change Definition (BRD 7–12)
	ChangeTitle            *string `json:"change_title"`
	ChangeDescription      *string `json:"change_description"`
	ChangeType             *string `json:"change_type"`
	ChangeCategory         *string `json:"change_category"`
	DepartmentFunction     *string `json:"department_function"`
	AffectedSystemsModules *string `json:"affected_systems_modules"`

	// Planning (BRD 13–16)
	ProposedImplementationDate *time.Time `json:"proposed_implementation_date"`
	TargetClosureDate          *time.Time `json:"target_closure_date"`
	ImplementationWindowStart  *time.Time `json:"implementation_window_start"`
	ImplementationWindowEnd    *time.Time `json:"implementation_window_end"`

	// Impact & Risk (BRD 17–23; 24 supporting_documents is a file attachment)
	ReasonForChange     *string `json:"reason_for_change"`
	BusinessImpact      *string `json:"business_impact"`
	ExpectedDowntime    *string `json:"expected_downtime"`
	RequiresTesting     *string `json:"requires_testing"`
	RequiresTraining    *string `json:"requires_training"`
	RiskRationale       *string `json:"risk_rationale"`
	KeyRisksMitigations *string `json:"key_risks_mitigations"`

	// Implementation Plan (BRD 25–28)
	HighLevelImplementationPlan *string `json:"high_level_implementation_plan"`
	ValidationApproach          *string `json:"validation_approach"`
	SuccessCriteria             *string `json:"success_criteria"`
	RollbackBackoutPlan         *string `json:"rollback_backout_plan"`

	// Implementation Details (BRD 29–33; 34 implementation_evidence is a file)
	ActualImplementationDate *time.Time `json:"actual_implementation_date"`
	PostImplementationIssues *string    `json:"post_implementation_issues"`
	ImplementationSummary    *string    `json:"implementation_summary"`
	DeviationsFromPlan       *string    `json:"deviations_from_plan"`
	ValidationPerformed      *string    `json:"validation_performed"`

	// Approvals — Initiation (BRD 35–36)
	AssignedApproverID   *uuid.UUID `json:"assigned_approver_id"`
	AssignedApproverName *string    `json:"assigned_approver_name"`
	CommentsForApprover  *string    `json:"comments_for_approver"`

	// Implementation Approval (BRD 37–41)
	Decision                     *string    `json:"decision"`
	RiskLevel                    *string    `json:"risk_level"`
	DecisionComments             *string    `json:"decision_comments"`
	ImplementationApprovalByID   *uuid.UUID `json:"implementation_approval_by_id"`
	ImplementationApprovalByName *string    `json:"implementation_approval_by_name"`
	ImplementationApprovalOn     *time.Time `json:"implementation_approval_on"`

	// Final Approval (BRD 42–45)
	FinalDecision       *string    `json:"final_decision"`
	FinalComments       *string    `json:"final_comments"`
	FinalApprovalByID   *uuid.UUID `json:"final_approval_by_id"`
	FinalApprovalByName *string    `json:"final_approval_by_name"`
	FinalApprovalOn     *time.Time `json:"final_approval_on"`

	// Status (BRD 46–48)
	ImplementationApprovalStatus string     `json:"implementation_approval_status"`
	FinalApprovalStatus          string     `json:"final_approval_status"`
	ActualClosureDate            *time.Time `json:"actual_closure_date"`

	// Additional (BRD 49–50)
	Comments           *string `json:"comments"`
	CancellationReason *string `json:"cancellation_reason"`
}

// toChangeControlResponse maps a joined change control row into the API shape.
// Used by both GET /{ccID} and PUT /{ccID}, which return identical bodies.
func toChangeControlResponse(row database.GetChangeControlByCcIDRow) ChangeControlResponse {
	cc := row.ChangeControl
	return ChangeControlResponse{
		// Identification (BRD 1–6)
		ID:                cc.ID,
		CcID:              cc.CcID,
		CurrentState:      cc.CurrentState,
		ChangeOwnerID:     cc.ChangeOwnerID,
		ChangeOwnerName:   row.OwnerName,
		LastUpdatedByID:   cc.LastUpdatedByID,
		LastUpdatedByName: row.UpdaterName,
		CreatedOn:         cc.CreatedOn,
		LastUpdatedOn:     cc.LastUpdatedOn,

		// Change Definition (BRD 7–12)
		ChangeTitle:            cc.ChangeTitle,
		ChangeDescription:      cc.ChangeDescription,
		ChangeType:             cc.ChangeType,
		ChangeCategory:         cc.ChangeCategory,
		DepartmentFunction:     cc.DepartmentFunction,
		AffectedSystemsModules: cc.AffectedSystemsModules,

		// Planning (BRD 13–16)
		ProposedImplementationDate: cc.ProposedImplementationDate,
		TargetClosureDate:          cc.TargetClosureDate,
		ImplementationWindowStart:  cc.ImplementationWindowStart,
		ImplementationWindowEnd:    cc.ImplementationWindowEnd,

		// Impact & Risk (BRD 17–23)
		ReasonForChange:     cc.ReasonForChange,
		BusinessImpact:      cc.BusinessImpact,
		ExpectedDowntime:    cc.ExpectedDowntime,
		RequiresTesting:     cc.RequiresTesting,
		RequiresTraining:    cc.RequiresTraining,
		RiskRationale:       cc.RiskRationale,
		KeyRisksMitigations: cc.KeyRisksMitigations,

		// Implementation Plan (BRD 25–28)
		HighLevelImplementationPlan: cc.HighLevelImplementationPlan,
		ValidationApproach:          cc.ValidationApproach,
		SuccessCriteria:             cc.SuccessCriteria,
		RollbackBackoutPlan:         cc.RollbackBackoutPlan,

		// Implementation Details (BRD 29–33)
		ActualImplementationDate: cc.ActualImplementationDate,
		PostImplementationIssues: cc.PostImplementationIssues,
		ImplementationSummary:    cc.ImplementationSummary,
		DeviationsFromPlan:       cc.DeviationsFromPlan,
		ValidationPerformed:      cc.ValidationPerformed,

		// Approvals: Initiation (BRD 35–36)
		AssignedApproverID:   cc.AssignedApproverID,
		AssignedApproverName: row.ApproverName,
		CommentsForApprover:  cc.CommentsForApprover,

		// Implementation Approval (BRD 37–41)
		Decision:                     cc.Decision,
		RiskLevel:                    cc.RiskLevel,
		DecisionComments:             cc.DecisionComments,
		ImplementationApprovalByID:   cc.ImplementationApprovalByID,
		ImplementationApprovalByName: row.ImplApprovalByName,
		ImplementationApprovalOn:     cc.ImplementationApprovalOn,

		// Final Approval (BRD 42–45)
		FinalDecision:       cc.FinalDecision,
		FinalComments:       cc.FinalComments,
		FinalApprovalByID:   cc.FinalApprovalByID,
		FinalApprovalByName: row.FinalApprovalByName,
		FinalApprovalOn:     cc.FinalApprovalOn,

		// Status (BRD 46–48)
		ImplementationApprovalStatus: cc.ImplementationApprovalStatus,
		FinalApprovalStatus:          cc.FinalApprovalStatus,
		ActualClosureDate:            cc.ActualClosureDate,

		// Additional (BRD 49–50)
		Comments:           cc.Comments,
		CancellationReason: cc.CancellationReason,
	}
}

type ChangeControlSummary struct {
	ID                   uuid.UUID  `json:"id"`
	CcID                 string     `json:"cc_id"`
	ChangeTitle          *string    `json:"change_title"`
	CurrentState         string     `json:"current_state"`
	ChangeOwnerID        uuid.UUID  `json:"change_owner_id"`
	ChangeOwnerName      string     `json:"change_owner_name"`
	AssignedApproverID   *uuid.UUID `json:"assigned_approver_id"`
	AssignedApproverName *string    `json:"assigned_approver_name"`
	CreatedOn            time.Time  `json:"created_on"`
	LastUpdatedOn        time.Time  `json:"last_updated_on"`
}

type ListChangeControlsResponse struct {
	ChangeControls []ChangeControlSummary `json:"change_controls"`
	Total          int64                  `json:"total"`
	Limit          int32                  `json:"limit"`
	Offset         int32                  `json:"offset"`
}

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

func (cfg *apiConfig) HandlerGetChangeControl(w http.ResponseWriter, r *http.Request, user database.User) {
	log := logging.LoggerFrom(r.Context())
	// extract path parameter
	ccIDRawStr := r.PathValue("ccID")
	ccID := strings.TrimSpace(ccIDRawStr)
	if ccID == "" {
		log.Warn("cc retrieval failed", "reason", "CC-ID blank")
		respondWithError(w, "CC-ID cannot be blank", http.StatusBadRequest)
		return
	}
	row, err := cfg.db.GetChangeControlByCcID(r.Context(), ccID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("cc retrieval failed", "reason", "cc not found", "cc_id", ccID)
			respondWithError(w, "Change Control not found", http.StatusNotFound)
			return
		}
		log.Error("cc retrieval failed", "reason", "cc lookup failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	log.Info("change control retrieved", "cc_id", row.ChangeControl.CcID, "state", row.ChangeControl.CurrentState)
	respondWithJSON(w, http.StatusOK, toChangeControlResponse(row))
}

func (cfg *apiConfig) HandlerListChangeControls(w http.ResponseWriter, r *http.Request, user database.User) {
	log := logging.LoggerFrom(r.Context())
	q := r.URL.Query()
	limit, offset, err := parsePagination(q)
	if err != nil {
		log.Warn("CCs retrieval failed", "reason", "invalid pagination", "error", err,
			"limit_param", q.Get("limit"), "offset_param", q.Get("offset"))
		respondWithError(w, err.Error(), http.StatusBadRequest)
		return
	}
	var ownerID *uuid.UUID
	if q.Get("owner") == "me" {
		ownerID = &user.ID
	}
	var assignedID *uuid.UUID
	if q.Get("assigned") == "me" {
		assignedID = &user.ID
	}
	var state *string
	if s := strings.TrimSpace(q.Get("state")); s != "" {
		switch s {
		case stateInitiated, statePendingImplApproval, stateInImplementation, statePendingFinalApproval, stateClosed, stateCancelled:
			state = &s
		default:
			log.Warn("CCs retrieval failed", "reason", "invalid state parameter value", "state", s)
			respondWithError(w, "Invalid state", http.StatusBadRequest)
			return
		}
	}
	var createdAfter *time.Time
	if s := strings.TrimSpace(q.Get("created_after")); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			log.Warn("CCs retrieval failed", "reason", "invalid created_after parameter value", "created_after", s, "error", err)
			respondWithError(w, "Created After date must be YYYY-MM-DD", http.StatusBadRequest)
			return
		}
		createdAfter = &t
	}
	var createdBefore *time.Time
	if s := strings.TrimSpace(q.Get("created_before")); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			log.Warn("CCs retrieval failed", "reason", "invalid created_before parameter value", "created_before", s, "error", err)
			respondWithError(w, "Created Before date must be YYYY-MM-DD", http.StatusBadRequest)
			return
		}
		createdBefore = &t
	}
	var search *string
	if s := strings.TrimSpace(q.Get("search")); s != "" {
		search = &s
	}
	params := database.ListChangeControlsParams{
		OwnerID:            ownerID,
		AssignedApproverID: assignedID,
		State:              state,
		CreatedAfter:       createdAfter,
		CreatedBefore:      createdBefore,
		Search:             search,
		Offset:             offset,
		Limit:              limit,
	}
	ccs, err := cfg.db.ListChangeControls(r.Context(), params)
	if err != nil {
		log.Error("CCs retrieval failed", "reason", "db ccs list retrieval failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	total, err := cfg.db.CountChangeControls(r.Context(), database.CountChangeControlsParams{
		OwnerID:            ownerID,
		AssignedApproverID: assignedID,
		State:              state,
		CreatedAfter:       createdAfter,
		CreatedBefore:      createdBefore,
		Search:             search,
	})
	if err != nil {
		log.Error("CCs retrieval failed", "reason", "db cc count failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	ccResponses := make([]ChangeControlSummary, 0, len(ccs))
	for _, cc := range ccs {
		ccResponses = append(ccResponses, ChangeControlSummary{
			ID:                   cc.ID,
			CcID:                 cc.CcID,
			ChangeTitle:          cc.ChangeTitle,
			CurrentState:         cc.CurrentState,
			ChangeOwnerID:        cc.ChangeOwnerID,
			ChangeOwnerName:      cc.OwnerName,
			AssignedApproverID:   cc.AssignedApproverID,
			AssignedApproverName: cc.ApproverName,
			CreatedOn:            cc.CreatedOn,
			LastUpdatedOn:        cc.LastUpdatedOn,
		})
	}
	log.Info("change controls listed", "count", len(ccs), "total", total,
		"limit", limit, "offset", offset,
		"filtered", ownerID != nil || assignedID != nil || state != nil ||
			createdAfter != nil || createdBefore != nil || search != nil)
	respondWithJSON(w, http.StatusOK, ListChangeControlsResponse{
		ChangeControls: ccResponses,
		Total:          total,
		Limit:          limit,
		Offset:         offset,
	})
}

func (cfg *apiConfig) HandlerSaveDraft(w http.ResponseWriter, r *http.Request, user database.User) {
	log := logging.LoggerFrom(r.Context())
	// extract path parameter
	ccIDRawStr := r.PathValue("ccID")
	ccID := strings.TrimSpace(ccIDRawStr)
	if ccID == "" {
		log.Warn("save draft failed", "reason", "CC-ID blank")
		respondWithError(w, "CC-ID cannot be blank", http.StatusBadRequest)
		return
	}
	// use json.RawMessage to delay decoding and saving json value as byte array
	// helps for three way check - if key/value is sent (no update), if null is sent (clear), if value is sent (normal update)
	// since i can do key present check for body map
	var body map[string]json.RawMessage
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		log.Warn("save draft failed", "reason", "malformed request body", "error", err)
		respondWithError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if len(body) == 0 {
		log.Warn("save draft failed", "reason", "no fields to update")
		respondWithError(w, "No fields to update", http.StatusBadRequest)
		return
	}
	// open transaction
	tx, err := cfg.rawDB.BeginTx(r.Context(), nil)
	if err != nil {
		log.Error("save draft failed", "reason", "could not begin transaction", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	qtx := cfg.db.WithTx(tx)
	// retrieve cc details with intent of updating
	cc, err := qtx.GetChangeControlForUpdate(r.Context(), ccID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("save draft failed", "reason", "cc not found", "cc_id", ccID)
			respondWithError(w, "Change Control not found", http.StatusNotFound)
			return
		}
		log.Error("save draft failed", "reason", "cc lookup failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// ownership check
	if user.ID != cc.ChangeOwnerID {
		log.Warn("save draft failed", "reason", "user is not owner of cc", "cc_id", ccID)
		respondWithError(w, "Forbidden", http.StatusForbidden)
		return
	}
	// state check
	if cc.CurrentState != stateInitiated {
		log.Warn("save draft failed", "reason", "save only allowed in initiated state", "cc_id", ccID)
		respondWithError(w, "Save action only allowed at Initiated state of CC", http.StatusConflict)
		return
	}
	// Seed every parameter with the record's CURRENT value. The UPDATE assigns all
	// 24 columns unconditionally, so anything the client did not send must be
	// re-written as-is — otherwise it would be nulled. Each field block below
	// overwrites only its own entry, so "leave unchanged" is the default.
	params := database.UpdateChangeControlDraftParams{
		CcID:            ccID,
		LastUpdatedByID: user.ID,

		// Change Definition
		ChangeTitle:            cc.ChangeTitle,
		ChangeDescription:      cc.ChangeDescription,
		ChangeType:             cc.ChangeType,
		ChangeCategory:         cc.ChangeCategory,
		DepartmentFunction:     cc.DepartmentFunction,
		AffectedSystemsModules: cc.AffectedSystemsModules,

		// Planning
		ProposedImplementationDate: cc.ProposedImplementationDate,
		TargetClosureDate:          cc.TargetClosureDate,
		ImplementationWindowStart:  cc.ImplementationWindowStart,
		ImplementationWindowEnd:    cc.ImplementationWindowEnd,

		// Impact & Risk
		ReasonForChange:     cc.ReasonForChange,
		BusinessImpact:      cc.BusinessImpact,
		ExpectedDowntime:    cc.ExpectedDowntime,
		RequiresTesting:     cc.RequiresTesting,
		RequiresTraining:    cc.RequiresTraining,
		RiskRationale:       cc.RiskRationale,
		KeyRisksMitigations: cc.KeyRisksMitigations,

		// Implementation Plan
		HighLevelImplementationPlan: cc.HighLevelImplementationPlan,
		ValidationApproach:          cc.ValidationApproach,
		SuccessCriteria:             cc.SuccessCriteria,
		RollbackBackoutPlan:         cc.RollbackBackoutPlan,

		// Approvals: Initiation
		AssignedApproverID:  cc.AssignedApproverID,
		CommentsForApprover: cc.CommentsForApprover,

		// Additional
		Comments: cc.Comments,
	}
	// multiple entries from the same action shall share the same timestamp for audit not cc table
	now := time.Now().UTC()
	// bool flag to find no-op case
	changed := false
	// Change Definition
	if raw, present := body["change_title"]; present {
		// unmarshal
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "change_title must be a string or null", "cc_id", ccID)
			respondWithError(w, "Change Title must be a string or null", http.StatusBadRequest)
			return
		}
		// normalize
		if v != nil {
			t := strings.TrimSpace(*v)
			if t == "" {
				v = nil
			} else {
				v = &t
			}
		}
		// max length — 200 runes
		if v != nil && len([]rune(*v)) > 200 {
			log.Warn("save draft failed", "reason", "change_title must be 200 characters or fewer", "cc_id", ccID)
			respondWithError(w, "Change Title must be 200 characters or fewer", http.StatusBadRequest)
			return
		}
		params.ChangeTitle = v
		if !sameStrPtr(v, cc.ChangeTitle) { // compare only for the flag
			changed = true
		}
	}
	if raw, present := body["change_description"]; present {
		// unmarshal
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "change_description must be a string or null", "cc_id", ccID)
			respondWithError(w, "Change Description must be a string or null", http.StatusBadRequest)
			return
		}
		// normalize
		if v != nil {
			t := strings.TrimSpace(*v)
			if t == "" {
				v = nil
			} else {
				v = &t
			}
		}
		// max length — 2000 runes
		if v != nil && len([]rune(*v)) > 2000 {
			log.Warn("save draft failed", "reason", "change_description must be 2000 characters or fewer", "cc_id", ccID)
			respondWithError(w, "Change Description must be 2000 characters or fewer", http.StatusBadRequest)
			return
		}
		params.ChangeDescription = v
		if !sameStrPtr(v, cc.ChangeDescription) {
			changed = true
		}
	}
	if raw, present := body["change_type"]; present {
		// unmarshal
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "change_type must be a string or null", "cc_id", ccID)
			respondWithError(w, "Change Type must be a string or null", http.StatusBadRequest)
			return
		}
		// normalize
		if v != nil {
			t := strings.TrimSpace(*v)
			if t == "" {
				v = nil
			} else {
				v = &t
			}
		}
		// constraint check
		if v != nil {
			switch *v {
			case changeTypeApplication, changeTypeInfrastructure, changeTypeDatabase,
				changeTypeSecurity, changeTypeNetwork, changeTypeHardware,
				changeTypeProcess, changeTypeOther:
			default:
				log.Warn("save draft failed", "reason", "Invalid change_type", "cc_id", ccID)
				respondWithError(w, "Invalid Change Type", http.StatusBadRequest)
				return
			}
		}
		params.ChangeType = v
		if !sameStrPtr(v, cc.ChangeType) {
			changed = true
		}
	}
	if raw, present := body["change_category"]; present {
		// unmarshal
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "change_category must be a string or null", "cc_id", ccID)
			respondWithError(w, "Change Category must be a string or null", http.StatusBadRequest)
			return
		}
		// normalize
		if v != nil {
			t := strings.TrimSpace(*v)
			if t == "" {
				v = nil
			} else {
				v = &t
			}
		}
		// constraint check
		if v != nil {
			switch *v {
			case changeCategoryNormal, changeCategoryStandard:
			default:
				log.Warn("save draft failed", "reason", "Invalid change_category", "cc_id", ccID)
				respondWithError(w, "Invalid Change Category", http.StatusBadRequest)
				return
			}
		}
		params.ChangeCategory = v
		if !sameStrPtr(v, cc.ChangeCategory) {
			changed = true
		}
	}
	if raw, present := body["department_function"]; present {
		// unmarshal
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "department_function must be a string or null", "cc_id", ccID)
			respondWithError(w, "Department Function must be a string or null", http.StatusBadRequest)
			return
		}
		// normalize
		if v != nil {
			t := strings.TrimSpace(*v)
			if t == "" {
				v = nil
			} else {
				v = &t
			}
		}
		// constraint check
		if v != nil {
			switch *v {
			case deptIT, deptOperations, deptSecurity, deptQA, deptFacilities, deptOther:
			default:
				log.Warn("save draft failed", "reason", "Invalid department_function", "cc_id", ccID)
				respondWithError(w, "Invalid Department Function", http.StatusBadRequest)
				return
			}
		}
		params.DepartmentFunction = v
		if !sameStrPtr(v, cc.DepartmentFunction) {
			changed = true
		}
	}
	if raw, present := body["affected_systems_modules"]; present {
		// unmarshal
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "affected_systems_modules must be a string or null", "cc_id", ccID)
			respondWithError(w, "Affected System Modules must be a string or null", http.StatusBadRequest)
			return
		}
		// normalize
		if v != nil {
			t := strings.TrimSpace(*v)
			if t == "" {
				v = nil
			} else {
				v = &t
			}
		}
		// max length — 500 runes
		if v != nil && len([]rune(*v)) > 500 {
			log.Warn("save draft failed", "reason", "affected_systems_modules must be 500 characters or fewer", "cc_id", ccID)
			respondWithError(w, "Affected System Modules must be 500 characters or fewer", http.StatusBadRequest)
			return
		}
		params.AffectedSystemsModules = v
		if !sameStrPtr(v, cc.AffectedSystemsModules) {
			changed = true
		}
	}
	// Planning
	if raw, present := body["implementation_window_start"]; present {
		// unmarshal — *time.Time accepts RFC 3339 or null, so format validation is free
		var v *time.Time
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "implementation_window_start must be an RFC 3339 timestamp or null", "cc_id", ccID)
			respondWithError(w, "Implementation Window Start must be an RFC 3339 timestamp or null", http.StatusBadRequest)
			return
		}
		params.ImplementationWindowStart = v
		if !sameTimePtr(v, cc.ImplementationWindowStart) {
			changed = true
		}
	}
	if raw, present := body["implementation_window_end"]; present {
		// unmarshal
		var v *time.Time
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "implementation_window_end must be an RFC 3339 timestamp or null", "cc_id", ccID)
			respondWithError(w, "Implementation Window End must be an RFC 3339 timestamp or null", http.StatusBadRequest)
			return
		}
		params.ImplementationWindowEnd = v
		if !sameTimePtr(v, cc.ImplementationWindowEnd) {
			changed = true
		}
	}
	if raw, present := body["proposed_implementation_date"]; present {
		// unmarshal — *time.Time accepts RFC 3339 or null
		var v *time.Time
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "proposed_implementation_date must be an RFC 3339 timestamp or null", "cc_id", ccID)
			respondWithError(w, "Proposed Implementation Date must be an RFC 3339 timestamp or null", http.StatusBadRequest)
			return
		}
		params.ProposedImplementationDate = v
		// audit-tracked (BRD §6.6.2) — write a row only when the value actually changes
		if !sameTimePtr(v, cc.ProposedImplementationDate) {
			changed = true
			err = qtx.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
				EntityType:      entityChangeControl,
				EntityID:        cc.ID,
				ActionType:      actionFieldUpdated,
				FieldName:       strPtr("proposed_implementation_date"),
				OldValue:        dateToAuditStr(cc.ProposedImplementationDate),
				NewValue:        dateToAuditStr(v),
				PerformedByID:   user.ID,
				PerformedByName: user.FullName,
				CreatedOn:       now,
			})
			if err != nil {
				log.Error("save draft failed", "reason", "audit entry for proposed_implementation_date failed", "cc_id", ccID, "error", err)
				respondWithError(w, "Something went wrong", http.StatusInternalServerError)
				return
			}
		}
	}
	if raw, present := body["target_closure_date"]; present {
		// unmarshal
		var v *time.Time
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "target_closure_date must be an RFC 3339 timestamp or null", "cc_id", ccID)
			respondWithError(w, "Target Closure Date must be an RFC 3339 timestamp or null", http.StatusBadRequest)
			return
		}
		params.TargetClosureDate = v
		// audit-tracked (BRD §6.6.2)
		if !sameTimePtr(v, cc.TargetClosureDate) {
			changed = true
			err = qtx.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
				EntityType:      entityChangeControl,
				EntityID:        cc.ID,
				ActionType:      actionFieldUpdated,
				FieldName:       strPtr("target_closure_date"),
				OldValue:        dateToAuditStr(cc.TargetClosureDate),
				NewValue:        dateToAuditStr(v),
				PerformedByID:   user.ID,
				PerformedByName: user.FullName,
				CreatedOn:       now,
			})
			if err != nil {
				log.Error("save draft failed", "reason", "audit entry for target_closure_date failed", "cc_id", ccID, "error", err)
				respondWithError(w, "Something went wrong", http.StatusInternalServerError)
				return
			}
		}
	}
	// Impact & Risk Assessment
	if raw, present := body["reason_for_change"]; present {
		// unmarshal
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "reason_for_change must be a string or null", "cc_id", ccID)
			respondWithError(w, "Reason for change must be a string or null", http.StatusBadRequest)
			return
		}
		// normalize
		if v != nil {
			t := strings.TrimSpace(*v)
			if t == "" {
				v = nil
			} else {
				v = &t
			}
		}
		// max length — 2000 runes
		if v != nil && len([]rune(*v)) > 2000 {
			log.Warn("save draft failed", "reason", "reason_for_change must be 2000 characters or fewer", "cc_id", ccID)
			respondWithError(w, "Reason For Change must be 2000 characters or fewer", http.StatusBadRequest)
			return
		}
		params.ReasonForChange = v
		if !sameStrPtr(v, cc.ReasonForChange) {
			changed = true
		}
	}
	if raw, present := body["business_impact"]; present {
		// unmarshal
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "business_impact must be a string or null", "cc_id", ccID)
			respondWithError(w, "Business Impact must be a string or null", http.StatusBadRequest)
			return
		}
		// normalize
		if v != nil {
			t := strings.TrimSpace(*v)
			if t == "" {
				v = nil
			} else {
				v = &t
			}
		}
		// max length — 2000 runes
		if v != nil && len([]rune(*v)) > 2000 {
			log.Warn("save draft failed", "reason", "business_impact must be 2000 characters or fewer", "cc_id", ccID)
			respondWithError(w, "Business Impact must be 2000 characters or fewer", http.StatusBadRequest)
			return
		}
		params.BusinessImpact = v
		if !sameStrPtr(v, cc.BusinessImpact) {
			changed = true
		}
	}
	if raw, present := body["expected_downtime"]; present {
		// unmarshal
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "expected_downtime must be a string or null", "cc_id", ccID)
			respondWithError(w, "Expected Downtime must be a string or null", http.StatusBadRequest)
			return
		}
		// normalize
		if v != nil {
			t := strings.TrimSpace(*v)
			if t == "" {
				v = nil
			} else {
				v = &t
			}
		}
		// constraint check
		if v != nil {
			switch *v {
			case downtimeYes, downtimeNo, downtimeUnknown:
			default:
				log.Warn("save draft failed", "reason", "Invalid expected_downtime", "cc_id", ccID)
				respondWithError(w, "Invalid expected downtime", http.StatusBadRequest)
				return
			}
		}
		params.ExpectedDowntime = v
		if !sameStrPtr(v, cc.ExpectedDowntime) {
			changed = true
		}
	}
	if raw, present := body["requires_testing"]; present {
		// unmarshal
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "requires_testing must be a string or null", "cc_id", ccID)
			respondWithError(w, "Requires testing must be a string or null", http.StatusBadRequest)
			return
		}
		// normalize
		if v != nil {
			t := strings.TrimSpace(*v)
			if t == "" {
				v = nil
			} else {
				v = &t
			}
		}
		// constraint check — values use an ASCII hyphen, not an en-dash (DB §6.5)
		if v != nil {
			switch *v {
			case testingFull, testingPartial, testingNone:
			default:
				log.Warn("save draft failed", "reason", "Invalid requires_testing", "cc_id", ccID)
				respondWithError(w, "Invalid Requires Testing", http.StatusBadRequest)
				return
			}
		}
		params.RequiresTesting = v
		if !sameStrPtr(v, cc.RequiresTesting) {
			changed = true
		}
	}
	if raw, present := body["requires_training"]; present {
		// unmarshal
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "requires_training must be a string or null", "cc_id", ccID)
			respondWithError(w, "Requires Training must be a string or null", http.StatusBadRequest)
			return
		}
		// normalize
		if v != nil {
			t := strings.TrimSpace(*v)
			if t == "" {
				v = nil
			} else {
				v = &t
			}
		}
		// constraint check
		if v != nil {
			switch *v {
			case trainingYes, trainingNo, trainingNotApplicable:
			default:
				log.Warn("save draft failed", "reason", "Invalid requires_training", "cc_id", ccID)
				respondWithError(w, "Invalid Requires Training", http.StatusBadRequest)
				return
			}
		}
		params.RequiresTraining = v
		if !sameStrPtr(v, cc.RequiresTraining) {
			changed = true
		}
	}
	if raw, present := body["risk_rationale"]; present {
		// unmarshal
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "risk_rationale must be a string or null", "cc_id", ccID)
			respondWithError(w, "Risk Rationale must be a string or null", http.StatusBadRequest)
			return
		}
		// normalize
		if v != nil {
			t := strings.TrimSpace(*v)
			if t == "" {
				v = nil
			} else {
				v = &t
			}
		}
		// max length — 2000 runes
		if v != nil && len([]rune(*v)) > 2000 {
			log.Warn("save draft failed", "reason", "risk_rationale must be 2000 characters or fewer", "cc_id", ccID)
			respondWithError(w, "Risk Rationale must be 2000 characters or fewer", http.StatusBadRequest)
			return
		}
		params.RiskRationale = v
		if !sameStrPtr(v, cc.RiskRationale) {
			changed = true
		}
	}
	if raw, present := body["key_risks_mitigations"]; present {
		// unmarshal
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "key_risks_mitigations must be a string or null", "cc_id", ccID)
			respondWithError(w, "Key Risk Mitigation must be a string or null", http.StatusBadRequest)
			return
		}
		// normalize
		if v != nil {
			t := strings.TrimSpace(*v)
			if t == "" {
				v = nil
			} else {
				v = &t
			}
		}
		// max length — 2000 runes
		if v != nil && len([]rune(*v)) > 2000 {
			log.Warn("save draft failed", "reason", "key_risks_mitigations must be 2000 characters or fewer", "cc_id", ccID)
			respondWithError(w, "Key Risk Mitigation must be 2000 characters or fewer", http.StatusBadRequest)
			return
		}
		params.KeyRisksMitigations = v
		if !sameStrPtr(v, cc.KeyRisksMitigations) {
			changed = true
		}
	}
	// Implementation Plan & Validation
	if raw, present := body["high_level_implementation_plan"]; present {
		// unmarshal
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "high_level_implementation_plan must be a string or null", "cc_id", ccID)
			respondWithError(w, "High Level Implementation Plan must be a string or null", http.StatusBadRequest)
			return
		}
		// normalize
		if v != nil {
			t := strings.TrimSpace(*v)
			if t == "" {
				v = nil
			} else {
				v = &t
			}
		}
		// max length — 2000 runes
		if v != nil && len([]rune(*v)) > 2000 {
			log.Warn("save draft failed", "reason", "high_level_implementation_plan must be 2000 characters or fewer", "cc_id", ccID)
			respondWithError(w, "High Level Implementation Plan must be 2000 characters or fewer", http.StatusBadRequest)
			return
		}
		params.HighLevelImplementationPlan = v
		if !sameStrPtr(v, cc.HighLevelImplementationPlan) {
			changed = true
		}
	}
	if raw, present := body["validation_approach"]; present {
		// unmarshal
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "validation_approach must be a string or null", "cc_id", ccID)
			respondWithError(w, "Validation Approach must be a string or null", http.StatusBadRequest)
			return
		}
		// normalize
		if v != nil {
			t := strings.TrimSpace(*v)
			if t == "" {
				v = nil
			} else {
				v = &t
			}
		}
		// max length — 2000 runes
		if v != nil && len([]rune(*v)) > 2000 {
			log.Warn("save draft failed", "reason", "validation_approach must be 2000 characters or fewer", "cc_id", ccID)
			respondWithError(w, "Validation Approach must be 2000 characters or fewer", http.StatusBadRequest)
			return
		}
		params.ValidationApproach = v
		if !sameStrPtr(v, cc.ValidationApproach) {
			changed = true
		}
	}
	if raw, present := body["success_criteria"]; present {
		// unmarshal
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "success_criteria must be a string or null", "cc_id", ccID)
			respondWithError(w, "Success Criteria must be a string or null", http.StatusBadRequest)
			return
		}
		// normalize
		if v != nil {
			t := strings.TrimSpace(*v)
			if t == "" {
				v = nil
			} else {
				v = &t
			}
		}
		// max length — 2000 runes
		if v != nil && len([]rune(*v)) > 2000 {
			log.Warn("save draft failed", "reason", "success_criteria must be 2000 characters or fewer", "cc_id", ccID)
			respondWithError(w, "Success Criteria must be 2000 characters or fewer", http.StatusBadRequest)
			return
		}
		params.SuccessCriteria = v
		if !sameStrPtr(v, cc.SuccessCriteria) {
			changed = true
		}
	}
	if raw, present := body["rollback_backout_plan"]; present {
		// unmarshal
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "rollback_backout_plan must be a string or null", "cc_id", ccID)
			respondWithError(w, "Rollback Backout Plan must be a string or null", http.StatusBadRequest)
			return
		}
		// normalize
		if v != nil {
			t := strings.TrimSpace(*v)
			if t == "" {
				v = nil
			} else {
				v = &t
			}
		}
		// max length — 2000 runes
		if v != nil && len([]rune(*v)) > 2000 {
			log.Warn("save draft failed", "reason", "rollback_backout_plan must be 2000 characters or fewer", "cc_id", ccID)
			respondWithError(w, "Rollback Backout Plan must be 2000 characters or fewer", http.StatusBadRequest)
			return
		}
		params.RollbackBackoutPlan = v
		if !sameStrPtr(v, cc.RollbackBackoutPlan) {
			changed = true
		}
	}
	// Approvals: Initiation
	if raw, present := body["assigned_approver_id"]; present {
		// unmarshal
		var v *uuid.UUID
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "assigned_approver_id must be a UUID or null", "cc_id", ccID)
			respondWithError(w, "Assigned Approver ID must be a UUID or null", http.StatusBadRequest)
			return
		}
		// validate the assignee: must exist, be active, and hold the Approver role.
		// The single-role model means an owner can never appear here (field ref 35).
		// newApproverName is captured for the audit row.
		var newApproverName *string
		if v != nil {
			approver, err := qtx.GetUserByID(r.Context(), *v)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					log.Warn("save draft failed", "reason", "assigned approver not found", "cc_id", ccID, "approver_id", *v)
					respondWithError(w, "Assigned approver not found", http.StatusBadRequest)
					return
				}
				log.Error("save draft failed", "reason", "approver lookup failed", "cc_id", ccID, "error", err)
				respondWithError(w, "Something went wrong", http.StatusInternalServerError)
				return
			}
			if !approver.IsActive {
				log.Warn("save draft failed", "reason", "assigned approver is deactivated", "cc_id", ccID, "approver_id", *v)
				respondWithError(w, "Assigned approver is deactivated", http.StatusBadRequest)
				return
			}
			if approver.Role != roleApprover {
				log.Warn("save draft failed", "reason", "assigned user does not hold the Approver role", "cc_id", ccID, "approver_id", *v, "actual_role", approver.Role)
				respondWithError(w, "Assigned user must hold the Approver role", http.StatusBadRequest)
				return
			}
			newApproverName = strPtr(approver.FullName)
		}
		params.AssignedApproverID = v
		// audit-tracked (BRD §6.6.2) — names stored, not UUIDs, so the trail is
		// readable without joins (same rationale as performed_by_name, DB §2.3)
		if !sameUUIDPtr(v, cc.AssignedApproverID) {
			changed = true
			// the previous approver's name needs its own lookup
			var oldApproverName *string
			if cc.AssignedApproverID != nil {
				oldApprover, err := qtx.GetUserByID(r.Context(), *cc.AssignedApproverID)
				if err != nil {
					log.Error("save draft failed", "reason", "previous approver lookup failed", "cc_id", ccID, "error", err)
					respondWithError(w, "Something went wrong", http.StatusInternalServerError)
					return
				}
				oldApproverName = strPtr(oldApprover.FullName)
			}
			err = qtx.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
				EntityType:      entityChangeControl,
				EntityID:        cc.ID,
				ActionType:      actionFieldUpdated,
				FieldName:       strPtr("assign_approver"),
				OldValue:        oldApproverName,
				NewValue:        newApproverName,
				PerformedByID:   user.ID,
				PerformedByName: user.FullName,
				CreatedOn:       now,
			})
			if err != nil {
				log.Error("save draft failed", "reason", "audit entry for assign_approver failed", "cc_id", ccID, "error", err)
				respondWithError(w, "Something went wrong", http.StatusInternalServerError)
				return
			}
		}
	}
	if raw, present := body["comments_for_approver"]; present {
		// unmarshal
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "comments_for_approver must be a string or null", "cc_id", ccID)
			respondWithError(w, "Comments for Approver must be a string or null", http.StatusBadRequest)
			return
		}
		// normalize
		if v != nil {
			t := strings.TrimSpace(*v)
			if t == "" {
				v = nil
			} else {
				v = &t
			}
		}
		// max length — 2000 runes
		if v != nil && len([]rune(*v)) > 2000 {
			log.Warn("save draft failed", "reason", "comments_for_approver must be 2000 characters or fewer", "cc_id", ccID)
			respondWithError(w, "Comments for Approver must be 2000 characters or fewer", http.StatusBadRequest)
			return
		}
		params.CommentsForApprover = v
		if !sameStrPtr(v, cc.CommentsForApprover) {
			changed = true
		}
	}
	// Additional
	if raw, present := body["comments"]; present {
		// unmarshal
		var v *string
		if err := json.Unmarshal(raw, &v); err != nil {
			log.Warn("save draft failed", "reason", "comments must be a string or null", "cc_id", ccID)
			respondWithError(w, "comments must be a string or null", http.StatusBadRequest)
			return
		}
		// normalize
		if v != nil {
			t := strings.TrimSpace(*v)
			if t == "" {
				v = nil
			} else {
				v = &t
			}
		}
		// max length — 2000 runes
		if v != nil && len([]rune(*v)) > 2000 {
			log.Warn("save draft failed", "reason", "comments must be 2000 characters or fewer", "cc_id", ccID)
			respondWithError(w, "comments must be 2000 characters or fewer", http.StatusBadRequest)
			return
		}
		params.Comments = v
		if !sameStrPtr(v, cc.Comments) {
			changed = true
		}
	}
	// The update (if any) runs first, then the re-fetch, then the commit — all
	// inside the transaction. Reading before the commit means a failure at any
	// point leaves nothing written, so the error and the record's state agree.
	if changed {
		_, err = qtx.UpdateChangeControlDraft(r.Context(), params)
		if err != nil {
			log.Error("save draft failed", "reason", "cc update failed", "cc_id", ccID, "error", err)
			respondWithError(w, "Something went wrong", http.StatusInternalServerError)
			return
		}
	}
	// re-fetch with the five user joins so the response matches GET /{ccID}.
	// A transaction reads its own uncommitted writes, so this returns the
	// post-update values.
	row, err := qtx.GetChangeControlByCcID(r.Context(), ccID)
	if err != nil {
		log.Error("save draft failed", "reason", "cc re-fetch failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	err = tx.Commit()
	if err != nil {
		log.Error("save draft failed", "reason", "db commit failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	if changed {
		log.Info("cc draft saved", "cc_id", ccID)
	} else {
		log.Info("cc record unchanged", "cc_id", ccID)
	}
	respondWithJSON(w, http.StatusOK, toChangeControlResponse(row))
}
