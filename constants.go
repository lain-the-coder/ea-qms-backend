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

// Change type — ck_cc_change_type
const (
	changeTypeApplication    = "Application"
	changeTypeInfrastructure = "Infrastructure"
	changeTypeDatabase       = "Database"
	changeTypeSecurity       = "Security"
	changeTypeNetwork        = "Network"
	changeTypeHardware       = "Hardware"
	changeTypeProcess        = "Process"
	changeTypeOther          = "Other"
)

// Change category — ck_cc_change_category ("Emergency" excluded, Phase 1 / BRD L1)
const (
	changeCategoryNormal   = "Normal"
	changeCategoryStandard = "Standard"
)

// Department / function — ck_cc_department_function
const (
	deptIT         = "IT"
	deptOperations = "Operations"
	deptSecurity   = "Security"
	deptQA         = "QA"
	deptFacilities = "Facilities"
	deptOther      = "Other"
)

// Expected downtime — ck_cc_expected_downtime
const (
	downtimeYes     = "Yes"
	downtimeNo      = "No"
	downtimeUnknown = "Unknown"
)

// Requires testing — ck_cc_requires_testing
const (
	testingFull    = "Yes - Full testing"
	testingPartial = "Yes - Partial testing"
	testingNone    = "No"
)

// Requires training — ck_cc_requires_training
const (
	trainingYes           = "Yes"
	trainingNo            = "No"
	trainingNotApplicable = "Not applicable"
)
