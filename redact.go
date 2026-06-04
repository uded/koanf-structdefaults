package structdefaults

import (
	"errors"
	"strconv"
)

// redactCause returns an error suitable for wrapping by wrapInvalidValue
// that does not expose the raw post-substitution value in its Error()
// string. The original cause remains reachable via errors.Unwrap so
// callers using errors.Is / errors.As to inspect the chain continue to
// work; redaction protects the common log.Printf("%v", err) /
// slog.Error("...", "err", err) path where the raw value would otherwise
// leak.
//
// Threat model: a default like koanf-default:"${DB_PASSWORD}" bound to a
// non-string typed field surfaces the resolved secret value through the
// parser's error chain. Callers who deliberately introspect via
// errors.As to a custom TextUnmarshaler error type have opted out of
// redaction; this function only protects the surface Error() string.
func redactCause(cause error) error {
	var ne *strconv.NumError
	if errors.As(cause, &ne) {
		// Rebuild without the value-bearing Num field; Func + Err
		// preserve enough context for errors.As-based diagnostics.
		return &strconv.NumError{Func: ne.Func, Err: ne.Err}
	}
	// Unknown error type (TextUnmarshaler, time.ParseDuration, ...) —
	// substitute a generic surface message but preserve the chain.
	return &redactedError{cause: cause}
}

// redactedError replaces an arbitrary error's Error() string with a
// fixed message while keeping the original reachable via Unwrap for
// errors.Is / errors.As callers.
type redactedError struct {
	cause error
}

func (r *redactedError) Error() string { return "value rejected by parser" }
func (r *redactedError) Unwrap() error { return r.cause }
