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
