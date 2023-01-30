package event

import (
	"encoding/json"
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

func implementsJSONMarshaler(value reflect.Value) bool {
	if !value.CanInterface() {
		return false
	}

	_, ok := value.Interface().(json.Marshaler)

	return ok
}

func (candidate Candidate) validateData() error {
	dataValue := reflect.ValueOf(candidate.Data)
	if dataValue.Kind() != reflect.Struct && dataValue.Kind() != reflect.Map {
		return fmt.Errorf("data must be a struct or map, but received '%s'", dataValue.Kind().String())
	}

	itemsToValidate := []valueWithPath{
		{path: "Data", value: reflect.ValueOf(candidate.Data)},
	}
	seenPointers := map[any]struct{}{}
	var currentItem valueWithPath

	for len(itemsToValidate) > 0 {
		currentItem, itemsToValidate = itemsToValidate[0], itemsToValidate[1:]

		switch currentItem.value.Kind() {
		// Unsupported data types, i.e. types that can't be json.Marshal'ed without
		// custom types and a custom json.Marshaler implementation.
		// Since we switch over value.Kind() and not value.Type(), a custom type
		// with matching json.Marshaler implementation may exist for the value.
		// Hence, we check if the value can be cast to json.Marshaler before erroring out.
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
			if !implementsJSONMarshaler(currentItem.value) {
				return fmt.Errorf("value of type '%s' at path '%s' is not supported, either implement json.Marshaler on this type, or remove it from the struct", currentItem.value.Type().String(), currentItem.path)
			}

		// Indirections i.e. values that point to other values.
		case reflect.Pointer:
			// Pointers can cause circular references, so we memorize pointers we have seen.
			pointer := currentItem.value.UnsafePointer()
			if _, ok := seenPointers[pointer]; ok {
				return fmt.Errorf("pointer at path '%s' is circular, data must not contain circular references", currentItem.path)
			}
			seenPointers[pointer] = struct{}{}

			// Deal with pointers and interfaces in the same way by unpacking
			// the underlying value using value.Elem().
			fallthrough

		case reflect.Interface:
			itemsToValidate = append(itemsToValidate, valueWithPath{
				value: currentItem.value.Elem(),
				path:  currentItem.path,
			})

		// Containers i.e. types that contain other values, but are not just indirections.
		case reflect.Map:
			// Maps can be circular (this fact was discovered by looking through the
			// JSON encoding code in the standard library, which this circularity check
			// is a copy of), so we also record them in the seen pointers.
			pointer := currentItem.value.UnsafePointer()
			if _, ok := seenPointers[pointer]; ok {
				return fmt.Errorf("map at path '%s' is circular, data must not contain circular references", currentItem.path)
			}
			seenPointers[pointer] = struct{}{}

			mapKeys := currentItem.value.MapKeys()
			keyKind := mapKeys[0].Kind()

			// Only maps that use integers and strings as keys can be marshaled to JSON.
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
				// Note the absence of fallthrough statements.
			default:
				return fmt.Errorf("map at path '%s' has keys of type '%s', but only integers and strings are supported as map keys", currentItem.path, mapKeys[0].Type().String())
			}

			for _, key := range mapKeys {
				itemsToValidate = append(itemsToValidate, valueWithPath{
					path:  fmt.Sprintf("%s.%s", currentItem.path, key),
					value: currentItem.value.MapIndex(key),
				})
			}

		case reflect.Struct:
			// Only plain structs, i.e. structs that don't contain unexported fields are
			// supported without implementing json.Marshaler.
			for i := 0; i < currentItem.value.NumField(); i++ {
				field := currentItem.value.Type().Field(i)
				if !field.IsExported() && !implementsJSONMarshaler(currentItem.value) {
					return fmt.Errorf("unexported field '%s' at path '%s' is not supported, data must only contain exported fields, or json.Marshaler must be implement on '%s'", field.Name, currentItem.path, currentItem.value.Type().String())
				}

				itemsToValidate = append(itemsToValidate, valueWithPath{
					path:  fmt.Sprintf("%s.%s", currentItem.path, field.Name),
					value: currentItem.value.Field(i),
				})
			}

		case reflect.Slice:
			// Slice can be circular (this fact was discovered by looking through the
			// JSON encoding code in the standard library, which this circularity check
			// is a copy of), so we also record them in the seen pointers.
			pointer := currentItem.value.UnsafePointer()
			if _, ok := seenPointers[pointer]; ok {
				return fmt.Errorf("slice at path '%s' is circular, data must not be circular", currentItem.path)
			}
			seenPointers[pointer] = struct{}{}

			// Since arrays can't be circular, we skip the circularity check for them.
			// Otherwise, we treat slices and arrays the same.
			fallthrough

		case reflect.Array:
			for i := 0; i < currentItem.value.Len(); i++ {
				itemsToValidate = append(itemsToValidate, valueWithPath{
					path:  fmt.Sprintf("%s.%d", currentItem.path, i),
					value: currentItem.value.Index(i),
				})
			}

		// Primitives i.e. data types that are natively supported in json.Marshal.
		case reflect.Invalid:
			// i.e. nil
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
			// Note the absence of fallthrough statements.
		default:
			// This should never happen, because the switch is exhaustive.
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
