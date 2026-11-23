package stepconf

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNotStructPtr indicates a type is not a pointer to a struct.
var ErrNotStructPtr = errors.New("must be a pointer to a struct")

// ParseError occurs when a struct field cannot be set.
type ParseError struct {
	Field string
	Value string
	Err   error
}

// Error implements builtin errors.Error.
func (e *ParseError) Error() string {
	segments := []string{e.Field}
	if e.Value != "" {
		segments = append(segments, e.Value)
	}
	segments = append(segments, e.Err.Error())
	return strings.Join(segments, ": ")
}

// rangeError is a failed range constraint. The message of the wrapped error quotes
// the validated value, so Secret fields report redacted() instead.
type rangeError struct {
	err        error
	constraint string
}

func (e *rangeError) Error() string {
	return e.err.Error()
}

func (e *rangeError) Unwrap() error {
	return e.err
}

func (e *rangeError) redacted() error {
	return fmt.Errorf("value is invalid or out of range (%s)", e.constraint)
}
