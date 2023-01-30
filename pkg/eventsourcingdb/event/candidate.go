package event

import (
	"fmt"
	customErrors "github.com/thenativeweb/eventsourcingdb-client-golang/pkg/errors"
	"reflect"
)

type CandidateContext struct {
	Source  string `json:"source"`
	Subject string `json:"subject"`
	Type    string `json:"type"`
}

type Candidate struct {
	CandidateContext
	Data Data `json:"data"`
}

func NewCandidate(
	source string,
	subject string,
	eventType string,
	data Data,
) Candidate {
	return Candidate{
		CandidateContext{
			Source:  source,
			Subject: subject,
			Type:    eventType,
		},
		data,
	}
}

type valueWithPath struct {
	path  string
	value reflect.Value
}

func (candidate Candidate) validateData() error {
	dataValue := reflect.ValueOf(candidate.Data)
	if dataValue.Kind() != reflect.Struct {
		return fmt.Errorf("data must be a struct, but received '%s'", dataValue.Kind().String())
	}

	itemsToValidate := []valueWithPath{
		{path: "[root element]", value: reflect.ValueOf(candidate.Data)},
	}
	seenPointers := map[any]struct{}{}
	var currentItem valueWithPath

	for len(itemsToValidate) > 0 {
		currentItem, itemsToValidate = itemsToValidate[0], itemsToValidate[1:]

		currentItem.value.CanAddr()

		switch currentItem.value.Kind() {
		// error cases
		case reflect.Invalid:
			fallthrough
		case reflect.Uintptr:
			fallthrough
		case reflect.UnsafePointer:
			fallthrough
		case reflect.Chan:
			fallthrough
		case reflect.Complex128:
			fallthrough
		case reflect.Complex64:
			fallthrough
		case reflect.Func:
			return fmt.Errorf("function at path '%s' is not supported, data must not contain functions", currentItem.path)

		// indirections
		case reflect.Interface:
			fallthrough
		case reflect.Pointer:
			pointer := currentItem.value.UnsafeAddr()
			if _, ok := seenPointers[pointer]; ok {
				return fmt.Errorf("pointer at path '%s' is circular, data must not contain circular references", currentItem.path)
			}
			seenPointers[pointer] = struct{}{}

			itemsToValidate = append(itemsToValidate, valueWithPath{
				value: currentItem.value.Elem(),
				path:  currentItem.path,
			})

		// containers
		case reflect.Map:
			pointer := currentItem.value.UnsafeAddr()
			if _, ok := seenPointers[pointer]; ok {
				return fmt.Errorf("map at path '%s' is circular, data must not contain circular references", currentItem.path)
			}
			seenPointers[pointer] = struct{}{}

			mapKeys := currentItem.value.MapKeys()
			keyKind := mapKeys[0].Kind()

			switch keyKind {
			case reflect.Int:
			case reflect.Int8:
			case reflect.Int16:
			case reflect.Int32:
			case reflect.Int64:
			case reflect.Uint:
			case reflect.Uint8:
			case reflect.Uint16:
			case reflect.Uint32:
			case reflect.Uint64:
			case reflect.String:
			default:
				return fmt.Errorf("map at path '%s' has keys of kind '%s', but only integers and strings are supported as map keys", currentItem.path, keyKind.String())
			}

			for _, key := range mapKeys {
				itemsToValidate = append(itemsToValidate, valueWithPath{
					path:  fmt.Sprintf("%s.%s", currentItem.path, key),
					value: currentItem.value.MapIndex(key),
				})
			}

		case reflect.Struct:
			for i := 0; i < currentItem.value.NumField(); i++ {
				field := currentItem.value.Type().Field(i)
				if !field.IsExported() {
					return fmt.Errorf("unexported field '%s' at path '%s' is not supported, data must only contain exported fields", field.Name, currentItem.path)
				}

				itemsToValidate = append(itemsToValidate, valueWithPath{
					path:  fmt.Sprintf("%s.%s", currentItem.path, field.Name),
					value: currentItem.value.Field(i),
				})
			}

		case reflect.Slice:
			pointer := currentItem.value.UnsafeAddr()
			if _, ok := seenPointers[pointer]; ok {
				return fmt.Errorf("slice at path '%s' is circular, data must not be circular", currentItem.path)
			}
			seenPointers[pointer] = struct{}{}
			fallthrough

		case reflect.Array:
			for i := 0; i < currentItem.value.Len(); i++ {
				itemsToValidate = append(itemsToValidate, valueWithPath{
					path:  fmt.Sprintf("%s.%d", currentItem.path, i),
					value: currentItem.value.Index(i),
				})
			}

		// primitives
		case reflect.Bool:
		case reflect.Int:
		case reflect.Int8:
		case reflect.Int16:
		case reflect.Int32:
		case reflect.Int64:
		case reflect.Uint:
		case reflect.Uint8:
		case reflect.Uint16:
		case reflect.Uint32:
		case reflect.Uint64:
		case reflect.Float32:
		case reflect.Float64:
		case reflect.String:
		default:
			// Should never happen, because the switch is exhaustive.
			return customErrors.NewInternalError(fmt.Errorf("unexpected Kind '%s' encountered", currentItem.value.Kind().String()))
		}
	}

	return nil
}

func (candidate Candidate) Validate() error {
	if err := ValidateSource(candidate.Source); err != nil {
		return fmt.Errorf("event candidate failed to validate: %w", err)
	}

	if err := ValidateSubject(candidate.Subject); err != nil {
		return fmt.Errorf("event candidate failed to validate: %w", err)
	}

	if err := ValidateType(candidate.Type); err != nil {
		return fmt.Errorf("event candidate failed to validate: %w", err)
	}

	if err := candidate.validateData(); err != nil {
		return fmt.Errorf("event candidate failed to validate: event data is unsupported: %w", err)
	}

	return nil
}
