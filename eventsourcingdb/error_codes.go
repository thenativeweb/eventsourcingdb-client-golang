package eventsourcingdb

// The codes of the errors that the server reports, which DBAPIError.Code holds,
// so that a caller can tell errors apart without comparing descriptions, e.g.
// a precondition that does not hold from an event that does not match its
// schema:
//
//	var dbAPIError *eventsourcingdb.DBAPIError
//	if errors.As(err, &dbAPIError) && dbAPIError.Code == eventsourcingdb.CodePreconditionViolated {
//		// ...
//	}
//
// Each code is the URL of the page that documents the error, exactly as the
// server sends it. The name of a constant follows the last part of the URL,
// e.g. CodePreconditionViolated for precondition-violated.
const (
	CodeSubjectMissing               = "https://docs.eventsourcingdb.io/errors/subject-missing"
	CodeSubjectMalformed             = "https://docs.eventsourcingdb.io/errors/subject-malformed"
	CodeOrderUnknown                 = "https://docs.eventsourcingdb.io/errors/order-unknown"
	CodeEventIDMissing               = "https://docs.eventsourcingdb.io/errors/event-id-missing"
	CodeEventIDMalformed             = "https://docs.eventsourcingdb.io/errors/event-id-malformed"
	CodeBoundTypeMissing             = "https://docs.eventsourcingdb.io/errors/bound-type-missing"
	CodeBoundTypeUnknown             = "https://docs.eventsourcingdb.io/errors/bound-type-unknown"
	CodeRangeUnsatisfiable           = "https://docs.eventsourcingdb.io/errors/range-unsatisfiable"
	CodeFromLatestEventIncomplete    = "https://docs.eventsourcingdb.io/errors/from-latest-event-incomplete"
	CodeIfEventIsMissingUnknown      = "https://docs.eventsourcingdb.io/errors/if-event-is-missing-unknown"
	CodeSelectionConflicting         = "https://docs.eventsourcingdb.io/errors/selection-conflicting"
	CodeEventTypeMissing             = "https://docs.eventsourcingdb.io/errors/event-type-missing"
	CodeEventTypeMalformed           = "https://docs.eventsourcingdb.io/errors/event-type-malformed"
	CodeEventTypeUnknown             = "https://docs.eventsourcingdb.io/errors/event-type-unknown"
	CodeSourceMissing                = "https://docs.eventsourcingdb.io/errors/source-missing"
	CodeSourceMalformed              = "https://docs.eventsourcingdb.io/errors/source-malformed"
	CodeDataMissing                  = "https://docs.eventsourcingdb.io/errors/data-missing"
	CodeTraceparentMalformed         = "https://docs.eventsourcingdb.io/errors/traceparent-malformed"
	CodeTracestateMalformed          = "https://docs.eventsourcingdb.io/errors/tracestate-malformed"
	CodeTracestateWithoutTraceparent = "https://docs.eventsourcingdb.io/errors/tracestate-without-traceparent"
	CodeEventsMissing                = "https://docs.eventsourcingdb.io/errors/events-missing"
	CodePreconditionTypeMissing      = "https://docs.eventsourcingdb.io/errors/precondition-type-missing"
	CodePreconditionUnknown          = "https://docs.eventsourcingdb.io/errors/precondition-unknown"
	CodePayloadMissing               = "https://docs.eventsourcingdb.io/errors/payload-missing"
	CodeLicenseInvalid               = "https://docs.eventsourcingdb.io/errors/license-invalid"
	CodeCapacityExceeded             = "https://docs.eventsourcingdb.io/errors/capacity-exceeded"
	CodeEventSchemaViolated          = "https://docs.eventsourcingdb.io/errors/event-schema-violated"
	CodeEventSchemaUnregistered      = "https://docs.eventsourcingdb.io/errors/event-schema-unregistered"
	CodeEventSchemaMissing           = "https://docs.eventsourcingdb.io/errors/event-schema-missing"
	CodeEventSchemaMalformed         = "https://docs.eventsourcingdb.io/errors/event-schema-malformed"
	CodeEventSchemaNotObject         = "https://docs.eventsourcingdb.io/errors/event-schema-not-object"
	CodeEventSchemaAlreadyRegistered = "https://docs.eventsourcingdb.io/errors/event-schema-already-registered"
	CodeLastAuditedEventMissing      = "https://docs.eventsourcingdb.io/errors/last-audited-event-missing"
	CodeLastAuditedEventMismatched   = "https://docs.eventsourcingdb.io/errors/last-audited-event-mismatched"
	CodeLastAuditedEventUnknown      = "https://docs.eventsourcingdb.io/errors/last-audited-event-unknown"
	CodeEventHashMissing             = "https://docs.eventsourcingdb.io/errors/event-hash-missing"
	CodeEventHashMalformed           = "https://docs.eventsourcingdb.io/errors/event-hash-malformed"
	CodeEventChainBroken             = "https://docs.eventsourcingdb.io/errors/event-chain-broken"
	CodeLastBackedUpEventMissing     = "https://docs.eventsourcingdb.io/errors/last-backed-up-event-missing"
	CodeLastBackedUpEventMismatched  = "https://docs.eventsourcingdb.io/errors/last-backed-up-event-mismatched"
	CodeLastBackedUpEventUnknown     = "https://docs.eventsourcingdb.io/errors/last-backed-up-event-unknown"
	CodeStoreNotEmpty                = "https://docs.eventsourcingdb.io/errors/store-not-empty"
	CodeEventHashVersionUnknown      = "https://docs.eventsourcingdb.io/errors/event-hash-version-unknown"
	CodeBackupMalformed              = "https://docs.eventsourcingdb.io/errors/backup-malformed"
	CodeBackupIncomplete             = "https://docs.eventsourcingdb.io/errors/backup-incomplete"
	CodeBackupFormatUnknown          = "https://docs.eventsourcingdb.io/errors/backup-format-unknown"
	CodeFullBackupMissing            = "https://docs.eventsourcingdb.io/errors/full-backup-missing"
	CodeBackupIncrementDisconnected  = "https://docs.eventsourcingdb.io/errors/backup-increment-disconnected"
	CodeBackupFailed                 = "https://docs.eventsourcingdb.io/errors/backup-failed"
	CodeBackupUnreadable             = "https://docs.eventsourcingdb.io/errors/backup-unreadable"
	CodeEventTooLarge                = "https://docs.eventsourcingdb.io/errors/event-too-large"
	CodeDataMalformed                = "https://docs.eventsourcingdb.io/errors/data-malformed"
	CodeQueryMissing                 = "https://docs.eventsourcingdb.io/errors/query-missing"
	CodeQueryMalformed               = "https://docs.eventsourcingdb.io/errors/query-malformed"
	CodeQueryUnevaluable             = "https://docs.eventsourcingdb.io/errors/query-unevaluable"
	CodeQueryNotBoolean              = "https://docs.eventsourcingdb.io/errors/query-not-boolean"
	CodePreconditionViolated         = "https://docs.eventsourcingdb.io/errors/precondition-violated"
	CodeStoreReadOnly                = "https://docs.eventsourcingdb.io/errors/store-read-only"
	CodeReadOnlyModeAlreadyEnabled   = "https://docs.eventsourcingdb.io/errors/read-only-mode-already-enabled"
	CodeDiskFull                     = "https://docs.eventsourcingdb.io/errors/disk-full"
	CodeRestoreInProgress            = "https://docs.eventsourcingdb.io/errors/restore-in-progress"
	CodeStoreFailed                  = "https://docs.eventsourcingdb.io/errors/store-failed"
	CodeIndexesRebuilding            = "https://docs.eventsourcingdb.io/errors/indexes-rebuilding"
	CodeServerShuttingDown           = "https://docs.eventsourcingdb.io/errors/server-shutting-down"
)
