package eventsourcingdb_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// slugsOfServer are the slugs of every error code that the server knows, in
// the order of cmd/server/api/error.go on main of
// github.com/thenativeweb/eventsourcingdb (as of commit 36fa8821). When the
// server adds a code, add its slug here, and the test tells which constant is
// missing.
var slugsOfServer = []string{
	"subject-missing",
	"subject-malformed",
	"order-unknown",
	"event-id-missing",
	"event-id-malformed",
	"bound-type-missing",
	"bound-type-unknown",
	"range-unsatisfiable",
	"from-latest-event-incomplete",
	"if-event-is-missing-unknown",
	"selection-conflicting",
	"event-type-missing",
	"event-type-malformed",
	"event-type-unknown",
	"source-missing",
	"source-malformed",
	"data-missing",
	"traceparent-malformed",
	"tracestate-malformed",
	"tracestate-without-traceparent",
	"events-missing",
	"precondition-type-missing",
	"precondition-unknown",
	"payload-missing",
	"license-invalid",
	"capacity-exceeded",
	"event-schema-violated",
	"event-schema-unregistered",
	"event-schema-missing",
	"event-schema-malformed",
	"event-schema-not-object",
	"event-schema-already-registered",
	"last-audited-event-missing",
	"last-audited-event-mismatched",
	"last-audited-event-unknown",
	"event-hash-missing",
	"event-hash-malformed",
	"event-chain-broken",
	"last-backed-up-event-missing",
	"last-backed-up-event-mismatched",
	"last-backed-up-event-unknown",
	"store-not-empty",
	"event-hash-version-unknown",
	"backup-malformed",
	"backup-incomplete",
	"backup-format-unknown",
	"full-backup-missing",
	"backup-increment-disconnected",
	"backup-failed",
	"backup-unreadable",
	"event-too-large",
	"data-malformed",
	"query-missing",
	"query-malformed",
	"query-unevaluable",
	"query-not-boolean",
	"precondition-violated",
	"store-read-only",
	"read-only-mode-already-enabled",
	"disk-full",
	"restore-in-progress",
	"store-failed",
	"indexes-rebuilding",
	"server-shutting-down",
}

// A caller has to be able to tell errors apart by their code without
// comparing strings it wrote itself, so the client has a constant for every
// code the server knows, and for no other.
func TestErrorCodes(t *testing.T) {
	codes := constantsOf(t, "error_codes.go", "Code")

	t.Run("has a constant for every code of the server, named after its slug", func(t *testing.T) {
		for _, slug := range slugsOfServer {
			name := nameOfCode(slug)

			value, ok := codes[name]
			if assert.True(t, ok, "there is no constant %s for the code %s", name, slug) {
				assert.Equal(t, "https://docs.eventsourcingdb.io/errors/"+slug, value)
			}
		}
	})

	t.Run("has no constant for a code the server does not know", func(t *testing.T) {
		names := map[string]bool{}
		for _, slug := range slugsOfServer {
			names[nameOfCode(slug)] = true
		}

		for name := range codes {
			assert.True(t, names[name], "the server does not know the code of %s", name)
		}
		assert.Len(t, codes, len(slugsOfServer))
	})
}

// nameOfCode returns the name of the constant for the code with the given
// slug: Code, followed by every word of the slug, capitalized, and with ID in
// capitals, as Go writes it.
func nameOfCode(slug string) string {
	name := "Code"
	for word := range strings.SplitSeq(slug, "-") {
		if word == "id" {
			name += "ID"
			continue
		}

		name += strings.ToUpper(word[:1]) + word[1:]
	}

	return name
}

// constantsOf returns the name and the value of every string constant in the
// given file of the package whose name starts with the given prefix.
func constantsOf(t *testing.T, filename string, prefix string) map[string]string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
	require.NoError(t, err)

	constants := map[string]string{}
	for _, declaration := range file.Decls {
		genDecl, ok := declaration.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}

		for _, spec := range genDecl.Specs {
			valueSpec := spec.(*ast.ValueSpec)
			for index, name := range valueSpec.Names {
				if !strings.HasPrefix(name.Name, prefix) {
					continue
				}

				literal, ok := valueSpec.Values[index].(*ast.BasicLit)
				require.True(t, ok, "the value of %s is not a literal", name.Name)

				value, err := strconv.Unquote(literal.Value)
				require.NoError(t, err)

				constants[name.Name] = value
			}
		}
	}

	return constants
}
