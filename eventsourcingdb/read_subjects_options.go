package eventsourcingdb

// ReadSubjectsOptions configures which subjects ReadSubjects reads.
type ReadSubjectsOptions struct {
	// Recursive tells whether to read the subject together with all subjects
	// nested below it, or the subject alone. Set it either way, since there is
	// no default: to read every subject, read "/" recursively.
	Recursive bool
}
