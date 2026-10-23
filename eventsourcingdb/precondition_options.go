package eventsourcingdb

// PreconditionOptions configures a precondition on a subject, as created by
// NewIsSubjectPristinePrecondition, NewIsSubjectPopulatedPrecondition and
// NewIsSubjectOnEventIDPrecondition. Passing them is optional: a precondition
// without them applies to the subject alone. Pass them at most once, since a
// precondition takes the first and ignores every other.
type PreconditionOptions struct {
	// Recursive tells whether the precondition applies to the subject
	// together with all subjects nested below it, or to the subject alone.
	//
	// Only the v2 API of the server can check a recursive precondition, so
	// against a server without it, WriteEvents fails with
	// ErrIncompatibleServer before it sends anything, and writes nothing.
	Recursive bool
}

// preconditionOptionsOf returns the options that a precondition on a subject
// was given, or nil if it was given none, so that it is sent as one without
// options.
func preconditionOptionsOf(options []PreconditionOptions) *PreconditionOptions {
	if len(options) == 0 {
		return nil
	}

	// A copy, so that the precondition does not change if the caller changes
	// the slice it passed.
	first := options[0]

	return &first
}
