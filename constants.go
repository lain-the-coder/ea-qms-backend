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

// Workflow transitions — ck_esignatures_transition.
// T1 (record creation) is deliberately absent: it requires no signature (BR-8.8.1).
const (
	transitionT2 = "T2" // Initiated → Pending Implementation Approval
	transitionT3 = "T3" // Initiated → Cancelled
	transitionT4 = "T4" // Pending Implementation Approval → In Implementation (approve)
	transitionT5 = "T5" // Pending Implementation Approval → Initiated (reject)
	transitionT6 = "T6" // In Implementation → Pending Final Approval
	transitionT7 = "T7" // Pending Final Approval → Closed (approve)
	transitionT8 = "T8" // Pending Final Approval → In Implementation (reject)
)

// Signature meanings — ck_esignatures_meaning (BR-8.8.4). Closed set of seven,
// one per transition.
const (
	meaningSubmittedImplApproval  = "Submitted for Implementation Approval" // T2
	meaningCancelled              = "Cancelled"                             // T3
	meaningApprovedImplApproval   = "Approved - Implementation Approval"    // T4
	meaningRejectedImplApproval   = "Rejected - Implementation Approval"    // T5
	meaningSubmittedFinalApproval = "Submitted for Final Approval"          // T6
	meaningApprovedFinalApproval  = "Approved - Final Approval"             // T7
	meaningRejectedFinalApproval  = "Rejected - Final Approval"             // T8
)

// Decision — ck_cc_decision / ck_cc_final_decision
const (
	decisionApprove = "Approve"
	decisionReject  = "Reject"
)

// Risk level — ck_cc_risk_level
const (
	riskLow    = "Low"
	riskMedium = "Medium"
	riskHigh   = "High"
)

// Notification types — Phase 1 logs these; SMTP is deferred (FR-6.4.1)
const (
	notifySubmittedForApproval      = "submitted_for_approval"       // T2 → approver
	notifyCCCancelled               = "cc_cancelled"                 // T3 → approver, if assigned
	notifyCCApproved                = "cc_approved"                  // T4 → owner
	notifyCCRejected                = "cc_rejected"                  // T5 → owner
	notifySubmittedForFinalApproval = "submitted_for_final_approval" // T6 -> approver
)

// File upload — BR-8.2.14 (10 MB), BR-8.2.13 narrowed to PDF only
const (
	maxUploadBytes     = 10 << 20 // 10 MB
	maxMultipartMemory = 1 << 20  // 1 MB stays in RAM; larger spills to a temp file
	contentTypePDF     = "application/pdf"
)

// File attachment field names — ck_file_attachments_field_name.
// 'supporting_documents' is deliberately NOT accepted in release 1. The CHECK
// constraint still permits it, so THIS whitelist is the only thing enforcing that.
const fieldImplementationEvidence = "implementation_evidence"

// Post-implementation issues — ck_cc_post_impl_issues
const (
	postImplNone             = "None"
	postImplMinorResolved    = "Minor issues resolved"
	postImplRequiresFollowUp = "Issues requiring follow-up"
)
