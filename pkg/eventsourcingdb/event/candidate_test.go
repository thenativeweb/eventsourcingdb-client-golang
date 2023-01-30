package event_test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/thenativeweb/eventsourcingdb-client-golang/internal/test/events"
	"github.com/thenativeweb/eventsourcingdb-client-golang/pkg/eventsourcingdb/event"
	"testing"
	"time"
	"unsafe"
)

type NestedDataWithJSONMarshaler struct {
	private string
}

func (data NestedDataWithJSONMarshaler) MarshalJSON() ([]byte, error) {
	return []byte("test"), nil
}

type NestedDataWithoutJSONMarshaler struct {
	private string
}
type TestDataWithPublicFields struct {
	Public any
}
type TestDataWithPrivateFields struct {
	private string
}
type TestDataWithPrivateFieldsAndJSONMarshaler struct {
	private string
}

func (data TestDataWithPrivateFieldsAndJSONMarshaler) MarshalJSON() ([]byte, error) {
	return []byte("test"), nil
}

type MyComplex128 complex128

func (m MyComplex128) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%e", complex128(m))), nil
}

type MyInterface interface {
	Foo() string
}

type MyInterfaceImplementation complex128

func (m MyInterfaceImplementation) Foo() string {
	return "Foo"
}

type MyStringAlias string
type MyIntAlias int

func TestNewCandidate(t *testing.T) {
	tests := []struct {
		timestamp event.Timestamp
		subject   string
		eventType string
		data      event.Data
	}{
		{
			timestamp: event.Timestamp{Time: time.Now()},
			subject:   "/account/user",
			eventType: "registered",
			data:      map[string]interface{}{"username": "jane.doe", "password": "secret"},
		},
	}

	for _, test := range tests {
		createdEvent := event.NewCandidate(events.TestSource, test.subject, test.eventType, test.data)

		assert.Equal(t, test.subject, createdEvent.Subject)
		assert.Equal(t, test.eventType, createdEvent.Type)
		assert.Equal(t, test.data, createdEvent.Data)
	}
}

func TestCandidate_Validate(t *testing.T) {
	t.Run("Returns an error if the source is malformed.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "$%&/(",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct{}{},
		}.Validate()

		assert.ErrorContains(t, err, "event candidate failed to validate: malformed event source '$%&/(': source must be a valid URI")
	})

	t.Run("Returns an error if the subject is malformed.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "barbaz",
				Type:    "invalid.foobar.event",
			},
			Data: struct{}{},
		}.Validate()

		assert.ErrorContains(t, err, "event candidate failed to validate: malformed event subject 'barbaz': subject must be an absolute, slash-separated path")
	})

	t.Run("Returns an error if the type is malformed.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid",
			},
			Data: struct{}{},
		}.Validate()

		assert.ErrorContains(t, err, "event candidate failed to validate: malformed event type 'invalid': type must be a reverse domain name")
	})

	t.Run("Returns an error if the data is not a struct.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: nil,
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: data must be a struct or map, but received 'invalid'")

		err = event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: []string{"foo", "bar"},
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: data must be a struct or map, but received 'slice'")

		err = event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: "foobar",
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: data must be a struct or map, but received 'string'")

		err = event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: &struct{}{},
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: data must be a struct or map, but received 'ptr'")
	})

	t.Run("Returns an error if the data contains private fields and does not implement json.Marshaler.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: TestDataWithPrivateFields{private: "foo"},
		}.Validate()

		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: unexported field 'private' at path 'Data' is not supported, data must only contain exported fields, or json.Marshaler must be implement on 'event_test.TestDataWithPrivateFields'")

	})

	t.Run("Returns an error if the data contains private fields in a nested struct and the nested struct does not implement json.Marshaler.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: TestDataWithPublicFields{Public: NestedDataWithoutJSONMarshaler{private: "foo"}},
		}.Validate()

		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: unexported field 'private' at path 'Data.Public' is not supported, data must only contain exported fields, or json.Marshaler must be implement on 'event_test.NestedDataWithoutJSONMarshaler'")
	})

	t.Run("Does not return an error if the data contains private fields but implements json.Marshaler.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: TestDataWithPrivateFieldsAndJSONMarshaler{private: "foo"},
		}.Validate()

		assert.NoError(t, err)

	})

	t.Run("Does not return an error if the data contains private fields in a nested struct but the nested struct implements json.Marshaler.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: TestDataWithPublicFields{Public: NestedDataWithJSONMarshaler{private: "foo"}},
		}.Validate()

		assert.NoError(t, err)
	})

	t.Run("Returns an error if a field contains an unsupported value and the value's type does not implement json.Marshaler.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct {
				Public any
			}{
				Public: 1 + 2i,
			},
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: value of type 'complex128' at path 'Data.Public' is not supported, either implement json.Marshaler on this type, or remove it from the struct")

		err = event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct {
				Public any
			}{
				Public: complex64(1 + 2i),
			},
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: value of type 'complex64' at path 'Data.Public' is not supported, either implement json.Marshaler on this type, or remove it from the struct")

		err = event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct {
				Public any
			}{
				Public: make(chan bool),
			},
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: value of type 'chan bool' at path 'Data.Public' is not supported, either implement json.Marshaler on this type, or remove it from the struct")

		err = event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct {
				Public any
			}{
				Public: func() {},
			},
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: value of type 'func()' at path 'Data.Public' is not supported, either implement json.Marshaler on this type, or remove it from the struct")

		err = event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct {
				Public any
			}{
				Public: unsafe.Pointer(&struct{}{}),
			},
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: value of type 'unsafe.Pointer' at path 'Data.Public' is not supported, either implement json.Marshaler on this type, or remove it from the struct")

		err = event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct {
				Public any
			}{
				Public: uintptr(unsafe.Pointer(&struct{}{})),
			},
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: value of type 'uintptr' at path 'Data.Public' is not supported, either implement json.Marshaler on this type, or remove it from the struct")
	})

	t.Run("Does not return an error if a field contains an unsupported value but the value's type implements json.Marshaler.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct {
				Public any
			}{
				Public: MyComplex128(1 + 2i),
			},
		}.Validate()
		assert.NoError(t, err)
	})

	t.Run("Resolves pointers.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct {
				Public any
			}{
				Public: &struct {
					Invalid complex128
				}{
					Invalid: 1 + 1i,
				},
			},
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: value of type 'complex128' at path 'Data.Public.Invalid' is not supported, either implement json.Marshaler on this type, or remove it from the struct")
	})

	t.Run("Resolves interfaces.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct {
				Public MyInterface
			}{
				Public: MyInterfaceImplementation(1 + 1i),
			},
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: value of type 'event_test.MyInterfaceImplementation' at path 'Data.Public' is not supported, either implement json.Marshaler on this type, or remove it from the struct")
	})

	t.Run("Works with type aliases.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct {
				Public MyStringAlias
			}{
				Public: "",
			},
		}.Validate()
		assert.NoError(t, err)
	})

	t.Run("Works with slices.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct {
				Public []string
			}{
				Public: []string{"foo", "bar"},
			},
		}.Validate()
		assert.NoError(t, err)
	})

	t.Run("Works with maps.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct {
				Public map[string]string
			}{
				Public: map[string]string{"foo": "bar"},
			},
		}.Validate()
		assert.NoError(t, err)
	})

	t.Run("Does not return an error if all fields in the data are supported by json.Marshal.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct {
				Public map[string]any
			}{
				Public: map[string]any{
					"foo": "bar",
					"bar": 2,
					"baz": []string{"quux"},
					"nil": nil,
					"qux": &struct {
						Public string
					}{
						Public: "party",
					},
				},
			},
		}.Validate()
		assert.NoError(t, err)
	})

	t.Run("Returns an error if data contains a map with keys that are not strings or integers.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct {
				Public map[float32]string
			}{
				Public: map[float32]string{
					3.14: "Almost Pi",
				},
			},
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: map at path 'Data.Public' has keys of type 'float32', but only integers and strings are supported as map keys")

		err = event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct {
				Public map[complex128]string
			}{
				Public: map[complex128]string{
					3.14: "Almost Pi",
				},
			},
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: map at path 'Data.Public' has keys of type 'complex128', but only integers and strings are supported as map keys")

		err = event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct {
				Public map[struct{}]string
			}{
				Public: map[struct{}]string{
					struct{}{}: "What",
				},
			},
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: map at path 'Data.Public' has keys of type 'struct {}', but only integers and strings are supported as map keys")
	})

	t.Run("Does not return an error if data contains a map with keys that are aliases of string or integer.", func(t *testing.T) {
		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct {
				Public map[MyStringAlias]string
			}{
				Public: map[MyStringAlias]string{
					"3.14": "Almost Pi",
				},
			},
		}.Validate()
		assert.NoError(t, err)

		err = event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: struct {
				Public map[MyIntAlias]string
			}{
				Public: map[MyIntAlias]string{
					3: "Almost Pi",
				},
			},
		}.Validate()
		assert.NoError(t, err)
	})

	t.Run("Returns an error if the data contains a circular pointer.", func(t *testing.T) {
		type CyclicStruct struct {
			Cycle any
		}

		circularStruct := CyclicStruct{}
		circularStruct.Cycle = &circularStruct

		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: circularStruct,
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: pointer at path 'Data.Cycle.Cycle' is circular, data must not contain circular references")
	})

	t.Run("Returns an error if the data contains a circular map.", func(t *testing.T) {
		circularMap := map[string]any{}
		circularMap["x"] = circularMap

		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: circularMap,
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: map at path 'Data.x' is circular, data must not contain circular references")
	})

	t.Run("Returns an error if the data contains a circular slice.", func(t *testing.T) {
		circularSlice := make([]any, 1)
		circularSlice[0] = circularSlice

		err := event.Candidate{
			CandidateContext: event.CandidateContext{
				Source:  "tag:foobar.invalid,2023:service",
				Subject: "/foo/bar",
				Type:    "invalid.foobar.event",
			},
			Data: map[string]any{
				"foo": circularSlice,
			},
		}.Validate()
		assert.ErrorContains(t, err, "event candidate failed to validate: event data is unsupported: slice at path 'Data.foo.0' is circular, data must not contain circular references")
	})
}
