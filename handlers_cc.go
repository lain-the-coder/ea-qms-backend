package main

import (
	"database/sql"
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
	resBody := ChangeControlResponse{
		// Identification (BRD 1–6)
		ID:                row.ChangeControl.ID,
		CcID:              row.ChangeControl.CcID,
		CurrentState:      row.ChangeControl.CurrentState,
		ChangeOwnerID:     row.ChangeControl.ChangeOwnerID,
		ChangeOwnerName:   row.OwnerName,
		LastUpdatedByID:   row.ChangeControl.LastUpdatedByID,
		LastUpdatedByName: row.UpdaterName,
		CreatedOn:         row.ChangeControl.CreatedOn,
		LastUpdatedOn:     row.ChangeControl.LastUpdatedOn,

		// Change Definition (BRD 7–12)
		ChangeTitle:            row.ChangeControl.ChangeTitle,
		ChangeDescription:      row.ChangeControl.ChangeDescription,
		ChangeType:             row.ChangeControl.ChangeType,
		ChangeCategory:         row.ChangeControl.ChangeCategory,
		DepartmentFunction:     row.ChangeControl.DepartmentFunction,
		AffectedSystemsModules: row.ChangeControl.AffectedSystemsModules,

		// Planning (BRD 13–16)
		ProposedImplementationDate: row.ChangeControl.ProposedImplementationDate,
		TargetClosureDate:          row.ChangeControl.TargetClosureDate,
		ImplementationWindowStart:  row.ChangeControl.ImplementationWindowStart,
		ImplementationWindowEnd:    row.ChangeControl.ImplementationWindowEnd,

		// Impact & Risk (BRD 17–23; 24 supporting_documents is a file attachment)
		ReasonForChange:     row.ChangeControl.ReasonForChange,
		BusinessImpact:      row.ChangeControl.BusinessImpact,
		ExpectedDowntime:    row.ChangeControl.ExpectedDowntime,
		RequiresTesting:     row.ChangeControl.RequiresTesting,
		RequiresTraining:    row.ChangeControl.RequiresTraining,
		RiskRationale:       row.ChangeControl.RiskRationale,
		KeyRisksMitigations: row.ChangeControl.KeyRisksMitigations,

		// Implementation Plan (BRD 25–28)
		HighLevelImplementationPlan: row.ChangeControl.HighLevelImplementationPlan,
		ValidationApproach:          row.ChangeControl.ValidationApproach,
		SuccessCriteria:             row.ChangeControl.SuccessCriteria,
		RollbackBackoutPlan:         row.ChangeControl.RollbackBackoutPlan,

		// Implementation Details (BRD 29–33; 34 implementation_evidence is a file)
		ActualImplementationDate: row.ChangeControl.ActualImplementationDate,
		PostImplementationIssues: row.ChangeControl.PostImplementationIssues,
		ImplementationSummary:    row.ChangeControl.ImplementationSummary,
		DeviationsFromPlan:       row.ChangeControl.DeviationsFromPlan,
		ValidationPerformed:      row.ChangeControl.ValidationPerformed,

		// Approvals — Initiation (BRD 35–36)
		AssignedApproverID:   row.ChangeControl.AssignedApproverID,
		AssignedApproverName: row.ApproverName,
		CommentsForApprover:  row.ChangeControl.CommentsForApprover,

		// Implementation Approval (BRD 37–41)
		Decision:                     row.ChangeControl.Decision,
		RiskLevel:                    row.ChangeControl.RiskLevel,
		DecisionComments:             row.ChangeControl.DecisionComments,
		ImplementationApprovalByID:   row.ChangeControl.ImplementationApprovalByID,
		ImplementationApprovalByName: row.ImplApprovalByName,
		ImplementationApprovalOn:     row.ChangeControl.ImplementationApprovalOn,

		// Final Approval (BRD 42–45)
		FinalDecision:       row.ChangeControl.FinalDecision,
		FinalComments:       row.ChangeControl.FinalComments,
		FinalApprovalByID:   row.ChangeControl.FinalApprovalByID,
		FinalApprovalByName: row.FinalApprovalByName,
		FinalApprovalOn:     row.ChangeControl.FinalApprovalOn,

		// Status (BRD 46–48)
		ImplementationApprovalStatus: row.ChangeControl.ImplementationApprovalStatus,
		FinalApprovalStatus:          row.ChangeControl.FinalApprovalStatus,
		ActualClosureDate:            row.ChangeControl.ActualClosureDate,

		// Additional (BRD 49–50)
		Comments:           row.ChangeControl.Comments,
		CancellationReason: row.ChangeControl.CancellationReason,
	}
	log.Info("change control retrieved", "cc_id", row.ChangeControl.CcID, "state", row.ChangeControl.CurrentState)
	respondWithJSON(w, http.StatusOK, resBody)
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
			respondWithError(w, "created_after must be YYYY-MM-DD", http.StatusBadRequest)
			return
		}
		createdAfter = &t
	}
	var createdBefore *time.Time
	if s := strings.TrimSpace(q.Get("created_before")); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			log.Warn("CCs retrieval failed", "reason", "invalid created_before parameter value", "created_before", s, "error", err)
			respondWithError(w, "created_before must be YYYY-MM-DD", http.StatusBadRequest)
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
