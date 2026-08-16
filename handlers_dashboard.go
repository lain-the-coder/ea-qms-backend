package main

import (
	"net/http"
	"time"

	"github.com/lain-the-coder/ea-qms-backend/internal/database"
	"github.com/lain-the-coder/ea-qms-backend/internal/logging"
)

// DashboardOverview is the system-wide count per state (BRD §9.5.2). A fixed
// struct rather than a map: GROUP BY returns only states that HAVE records, so
// a state with none would simply be absent. Starting from zero and overwriting
// from the rows is what guarantees all five cards always appear.
//
// Cancelled is deliberately absent — §9.5.2 lists five "active" states for the
// Overview. A cancelled record can still surface in recent activity.
type DashboardOverview struct {
	Initiated                     int64 `json:"initiated"`
	PendingImplementationApproval int64 `json:"pending_implementation_approval"`
	InImplementation              int64 `json:"in_implementation"`
	PendingFinalApproval          int64 `json:"pending_final_approval"`
	Closed                        int64 `json:"closed"`
}

// DashboardCCItem is one row in the Pending Approvals or My Drafts card.
// change_title is nullable — a draft may have none — and is returned as null
// rather than a placeholder, since presentation is the frontend's decision.
type DashboardCCItem struct {
	CcID         string  `json:"cc_id"`
	ChangeTitle  *string `json:"change_title"`
	CurrentState string  `json:"current_state"`
}

// DashboardActivityItem is one row in the Recent Activity table.
type DashboardActivityItem struct {
	CcID              string    `json:"cc_id"`
	ChangeTitle       *string   `json:"change_title"`
	CurrentState      string    `json:"current_state"`
	LastUpdatedOn     time.Time `json:"last_updated_on"`
	LastUpdatedByName string    `json:"last_updated_by_name"`
}

// DashboardResponse is the whole landing page in one call. Each card shows a
// total alongside a capped list, so the counts are separate from len(items).
// Recent Activity has no total — the card is a list plus a "View All" link.
type DashboardResponse struct {
	Overview              DashboardOverview       `json:"overview"`
	PendingApprovals      []DashboardCCItem       `json:"pending_approvals"`
	PendingApprovalsTotal int64                   `json:"pending_approvals_total"`
	MyDrafts              []DashboardCCItem       `json:"my_drafts"`
	MyDraftsTotal         int64                   `json:"my_drafts_total"`
	RecentActivity        []DashboardActivityItem `json:"recent_activity"`
}

// HandlerDashboard serves the landing page for every authenticated role
// (BRD §9.5.2 — all four cards are shown to all roles, with empty states when
// the user has nothing pending). Six reads, no writes, so no transaction.
func (cfg *apiConfig) HandlerDashboard(w http.ResponseWriter, r *http.Request, user database.User) {
	log := logging.LoggerFrom(r.Context())

	// 1. Overview — system-wide count per state
	stateCounts, err := cfg.db.CountChangeControlsByState(r.Context())
	if err != nil {
		log.Error("dashboard retrieval failed", "reason", "state count query failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	// The struct starts at zero, so a state with no records reports 0 rather
	// than vanishing from the response. No default case is needed: Cancelled is
	// excluded in SQL, so nothing else can arrive.
	overview := DashboardOverview{}
	for _, row := range stateCounts {
		switch row.CurrentState {
		case stateInitiated:
			overview.Initiated = row.Count
		case statePendingImplApproval:
			overview.PendingImplementationApproval = row.Count
		case stateInImplementation:
			overview.InImplementation = row.Count
		case statePendingFinalApproval:
			overview.PendingFinalApproval = row.Count
		case stateClosed:
			overview.Closed = row.Count
		}
	}

	// 2. Pending approvals — assigned to me, in either pending state.
	// assigned_approver_id is nullable, so the parameter is *uuid.UUID.
	pendingTotal, err := cfg.db.CountPendingApprovalsForUser(r.Context(), &user.ID)
	if err != nil {
		log.Error("dashboard retrieval failed", "reason", "pending approvals count failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	pendingRows, err := cfg.db.ListPendingApprovalsForUser(r.Context(), database.ListPendingApprovalsForUserParams{
		AssignedApproverID: &user.ID,
		Limit:              dashboardCardItems,
	})
	if err != nil {
		log.Error("dashboard retrieval failed", "reason", "pending approvals list failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	// 3. My drafts — owned by me, Initiated. change_owner_id is NOT NULL, so
	// this parameter is a plain uuid.UUID, unlike the approver one above.
	draftsTotal, err := cfg.db.CountDraftsForUser(r.Context(), user.ID)
	if err != nil {
		log.Error("dashboard retrieval failed", "reason", "drafts count failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	draftRows, err := cfg.db.ListDraftsForUser(r.Context(), database.ListDraftsForUserParams{
		ChangeOwnerID: user.ID,
		Limit:         dashboardCardItems,
	})
	if err != nil {
		log.Error("dashboard retrieval failed", "reason", "drafts list failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	// 4. Recent activity — system-wide, including Cancelled records
	recentRows, err := cfg.db.ListRecentActivity(r.Context(), dashboardRecentItems)
	if err != nil {
		log.Error("dashboard retrieval failed", "reason", "recent activity query failed", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	// map the three lists. make(..., 0, len) so an empty card marshals as []
	// and not null — two of these three will be empty for most users.
	pendingApprovals := make([]DashboardCCItem, 0, len(pendingRows))
	for _, row := range pendingRows {
		pendingApprovals = append(pendingApprovals, DashboardCCItem{
			CcID:         row.CcID,
			ChangeTitle:  row.ChangeTitle,
			CurrentState: row.CurrentState,
		})
	}

	myDrafts := make([]DashboardCCItem, 0, len(draftRows))
	for _, row := range draftRows {
		myDrafts = append(myDrafts, DashboardCCItem{
			CcID:         row.CcID,
			ChangeTitle:  row.ChangeTitle,
			CurrentState: row.CurrentState,
		})
	}

	recentActivity := make([]DashboardActivityItem, 0, len(recentRows))
	for _, row := range recentRows {
		recentActivity = append(recentActivity, DashboardActivityItem{
			CcID:              row.CcID,
			ChangeTitle:       row.ChangeTitle,
			CurrentState:      row.CurrentState,
			LastUpdatedOn:     row.LastUpdatedOn,
			LastUpdatedByName: row.LastUpdatedByName,
		})
	}

	log.Info("dashboard retrieved",
		"pending_approvals_total", pendingTotal,
		"drafts_total", draftsTotal,
		"recent_count", len(recentActivity))

	respondWithJSON(w, http.StatusOK, DashboardResponse{
		Overview:              overview,
		PendingApprovals:      pendingApprovals,
		PendingApprovalsTotal: pendingTotal,
		MyDrafts:              myDrafts,
		MyDraftsTotal:         draftsTotal,
		RecentActivity:        recentActivity,
	})
}
