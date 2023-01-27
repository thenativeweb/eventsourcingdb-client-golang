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
	itemsToValidate := []valueWithPath{
		{path: "[root element]", value: reflect.ValueOf(candidate.Data)},
	}

	for len(itemsToValidate) > 0 {
		currentItem, fieldsToValidate := itemsToValidate[0], itemsToValidate[1:]

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
			return fmt.Errorf("path '%s': unsupported kind: %s", currentItem.path, currentItem.value.Kind().String())

		// indirections
		case reflect.Interface:
			fallthrough
		case reflect.Pointer:
			fieldsToValidate = append(fieldsToValidate, valueWithPath{
				value: currentItem.value.Elem(),
				path:  currentItem.path,
			})

		// containers
		case reflect.Map:
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
				return fmt.Errorf("path '%s': unsupported map key kind: %s", currentItem.path, keyKind.String())
			}

			for _, key := range mapKeys {
				fieldsToValidate = append(fieldsToValidate, valueWithPath{
					path:  fmt.Sprintf("%s.%s", currentItem.path, key),
					value: currentItem.value.MapIndex(key),
				})
			}
		case reflect.Struct:
			for i := 0; i < currentItem.value.NumField(); i++ {
				field := currentItem.value.Type().Field(i)
				if !field.IsExported() {
					return fmt.Errorf("path '%s': unsupported unexported field: %s", currentItem.path, field.Name)
				}

				fieldsToValidate = append(fieldsToValidate, valueWithPath{
					path:  fmt.Sprintf("%s.%s", currentItem.path, field.Name),
					value: currentItem.value.Field(i),
				})
			}
		case reflect.Array:
			fallthrough
		case reflect.Slice:
			for i := 0; i < currentItem.value.Len(); i++ {
				fieldsToValidate = append(fieldsToValidate, valueWithPath{
					path:  fmt.Sprintf("%s.%d", i),
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
		return err
	}

	if err := ValidateSubject(candidate.Subject); err != nil {
		return err
	}

	if err := ValidateType(candidate.Type); err != nil {
		return err
	}

	return nil
}
