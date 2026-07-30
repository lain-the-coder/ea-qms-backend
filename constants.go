package main

// User roles — ck_users_role
const (
	roleAdmin    = "Admin"
	roleCCOwner  = "CC Owner"
	roleApprover = "Approver"
	roleViewer   = "Viewer"
)

// Audit entity types — ck_audit_logs_entity_type
const (
	entityChangeControl = "ChangeControl"
	entityUser          = "User"
)

// Audit action types — ck_audit_logs_action_type
const (
	actionCreated           = "Created"
	actionStateChanged      = "StateChanged"
	actionFieldUpdated      = "FieldUpdated"
	actionUserAdded         = "UserAdded"
	actionUserRoleChanged   = "UserRoleChanged"
	actionUserUpdated       = "UserUpdated"
	actionUserDeactivated   = "UserDeactivated"
	actionSignatureCaptured = "SignatureCaptured"
	actionSignatureFailed   = "SignatureFailed"
)

// Change control states — ck_cc_current_state
const (
	stateInitiated            = "Initiated"
	statePendingImplApproval  = "Pending Implementation Approval"
	stateInImplementation     = "In Implementation"
	statePendingFinalApproval = "Pending Final Approval"
	stateClosed               = "Closed"
	stateCancelled            = "Cancelled"
)

// Approval statuses — ck_cc_impl_approval_status / ck_cc_final_approval_status
const (
	approvalNotSubmitted = "Not Submitted"
	approvalPending      = "Pending"
	approvalApproved     = "Approved"
	approvalNA           = "N/A"
)
