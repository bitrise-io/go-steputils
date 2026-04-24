package step

import (
	"errors"
	"fmt"
	"testing"
)

func TestNewError_fieldsAndErrorString(t *testing.T) {
	cause := errors.New("boom")
	e := NewError("git-clone", "fetch_failed", cause, "short")

	if e.StepID != "git-clone" {
		t.Errorf("StepID = %q, want %q", e.StepID, "git-clone")
	}
	if e.Tag != "fetch_failed" {
		t.Errorf("Tag = %q, want %q", e.Tag, "fetch_failed")
	}
	if e.ShortMsg != "short" {
		t.Errorf("ShortMsg = %q, want %q", e.ShortMsg, "short")
	}
	if !errors.Is(e.Err, cause) {
		t.Errorf("Err = %v, want %v", e.Err, cause)
	}
	if e.Recommendations != nil {
		t.Errorf("Recommendations = %v, want nil", e.Recommendations)
	}
	if got := e.Error(); got != "boom" {
		t.Errorf("Error() = %q, want %q", got, "boom")
	}
}

func TestNewErrorWithRecommendations_setsRecommendations(t *testing.T) {
	rec := Recommendation{"key": []string{"a", "b"}}
	e := NewErrorWithRecommendations("git-clone", "checkout_failed", errors.New("x"), "short", rec)

	if e.Recommendations == nil {
		t.Fatal("Recommendations = nil, want non-nil")
	}
	got, ok := e.Recommendations["key"].([]string)
	if !ok {
		t.Fatalf("Recommendations[key] type = %T, want []string", e.Recommendations["key"])
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("Recommendations[key] = %v, want [a b]", got)
	}
}

func TestError_UnwrapReturnsInnerWrappedError(t *testing.T) {
	inner := errors.New("inner")
	wrapped := fmt.Errorf("wrap: %w", inner)
	e := NewError("step", "tag", wrapped, "short")

	if got := errors.Unwrap(e); got != inner {
		t.Errorf("Unwrap() = %v, want %v", got, inner)
	}
	if !errors.Is(e, inner) {
		t.Errorf("errors.Is(e, inner) = false, want true")
	}
}

func TestError_UnwrapReturnsNilWhenErrNotWrapped(t *testing.T) {
	e := NewError("step", "tag", errors.New("leaf"), "short")
	if got := errors.Unwrap(e); got != nil {
		t.Errorf("Unwrap() = %v, want nil", got)
	}
}

func TestError_SatisfiesErrorInterface(t *testing.T) {
	var _ error = (*Error)(nil)
}
