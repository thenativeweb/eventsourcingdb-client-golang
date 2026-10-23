package eventsourcingdb_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
)

func TestPreconditionOptions(t *testing.T) {
	t.Run("a precondition on a subject panics if it is given more than one options value", func(t *testing.T) {
		recursive := eventsourcingdb.PreconditionOptions{Recursive: true}
		notRecursive := eventsourcingdb.PreconditionOptions{Recursive: false}

		constructors := map[string]func(options ...eventsourcingdb.PreconditionOptions) eventsourcingdb.Precondition{
			"isSubjectPristine": func(options ...eventsourcingdb.PreconditionOptions) eventsourcingdb.Precondition {
				return eventsourcingdb.NewIsSubjectPristinePrecondition("/test", options...)
			},
			"isSubjectPopulated": func(options ...eventsourcingdb.PreconditionOptions) eventsourcingdb.Precondition {
				return eventsourcingdb.NewIsSubjectPopulatedPrecondition("/test", options...)
			},
			"isSubjectOnEventId": func(options ...eventsourcingdb.PreconditionOptions) eventsourcingdb.Precondition {
				return eventsourcingdb.NewIsSubjectOnEventIDPrecondition("/test", "0", options...)
			},
		}

		for name, constructor := range constructors {
			t.Run(name, func(t *testing.T) {
				assert.PanicsWithValue(t, "eventsourcingdb: a precondition takes at most one PreconditionOptions, got 2", func() {
					constructor(recursive, notRecursive)
				})
				assert.PanicsWithValue(t, "eventsourcingdb: a precondition takes at most one PreconditionOptions, got 3", func() {
					constructor(notRecursive, recursive, notRecursive)
				})

				assert.NotPanics(t, func() {
					constructor()
					constructor(recursive)
				})
			})
		}
	})
}
