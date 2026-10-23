package eventsourcingdb

import "fmt"

// PreconditionOptions configures a precondition on a subject, as created by
// NewIsSubjectPristinePrecondition, NewIsSubjectPopulatedPrecondition and
// NewIsSubjectOnEventIDPrecondition. Passing them is optional: a precondition
// without them applies to the subject alone. Pass them at most once: a
// constructor panics if it is given more than one, since that is a mistake in
// the code that calls it rather than something to handle at run time.
type PreconditionOptions struct {
	// Recursive tells whether the precondition applies to the subject
	// together with all subjects nested below it, or to the subject alone.
	//
	// Only the v2 API of the server can check a recursive precondition. If
	// the client found out before that the server does not offer it, it asks
	// the server again before it writes, as the server may have been upgraded
	// since. Against a server without it, WriteEvents then fails with
	// ErrIncompatibleServer before it sends the write, and writes nothing.
	Recursive bool
}

// preconditionOptionsOf returns the options that a precondition on a subject
// was given, or nil if it was given none, so that it is sent as one without
// options. It panics if the precondition was given more than one.
func preconditionOptionsOf(options []PreconditionOptions) *PreconditionOptions {
	switch len(options) {
	case 0:
		return nil
	case 1:
		// A copy, so that the precondition does not change if the caller
		// changes the slice it passed.
		given := options[0]

		return &given
	default:
		panic(fmt.Sprintf("eventsourcingdb: a precondition takes at most one PreconditionOptions, got %d", len(options)))
	}
}
