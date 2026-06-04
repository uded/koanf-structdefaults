package structdefaults

import (
	"fmt"
	"strings"
)

// substituteEnv expands POSIX-style ${VAR} and ${VAR:-fallback} references in
// raw using lookup. Multiple references per string are supported. Unmatched
// ${ sequences and invalid var-name forms (e.g. ${1FOO}, ${}) are kept literal.
//
// Substitution is single-pass and non-recursive: env-var values are not
// re-scanned for ${...}. This is intentional and prevents indirect env-var
// resolution attacks where one variable's value names another to expand.
//
// If a referenced variable is unset and no fallback is provided, the function
// returns ErrUnsetEnv wrapped with the variable name.
func substituteEnv(raw string, lookup EnvLookup) (string, error) {
	if !strings.Contains(raw, "${") {
		return raw, nil
	}

	var b strings.Builder
	b.Grow(len(raw))

	for i := 0; i < len(raw); {
		if i+1 < len(raw) && raw[i] == '$' && raw[i+1] == '{' {
			closeIdx := strings.IndexByte(raw[i+2:], '}')
			if closeIdx < 0 {
				// Unmatched "${" — keep the rest of the string literal.
				b.WriteString(raw[i:])
				break
			}
			inner := raw[i+2 : i+2+closeIdx]
			next := i + 2 + closeIdx + 1

			name, fallback, hasFallback := splitVarSpec(inner)
			if !isValidVarName(name) {
				// Not a valid var spec — keep literal.
				b.WriteString(raw[i:next])
				i = next
				continue
			}

			val, ok, err := safeLookup(lookup, name)
			if err != nil {
				return "", err
			}
			switch {
			case ok:
				b.WriteString(val)
			case hasFallback:
				b.WriteString(fallback)
			default:
				return "", fmt.Errorf("%w: %s", ErrUnsetEnv, name)
			}
			i = next
			continue
		}
		b.WriteByte(raw[i])
		i++
	}

	return b.String(), nil
}

// safeLookup calls lookup(name) and converts any panic into an error so
// a misbehaving custom EnvLookup (e.g. a Vault or AWS Secrets Manager
// adapter that panics on a nil-map dereference or a closed-channel
// write during transient failure) does not crash the caller's process.
// Honors Read's (map, error) return contract.
//
// The recovered panic value is interpolated into the error message via
// %v — implementations whose panic values may contain sensitive data
// should arrange to recover internally and return an error instead.
func safeLookup(lookup EnvLookup, name string) (val string, ok bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w resolving %s: %v", ErrLookupPanic, name, r)
			val = ""
			ok = false
		}
	}()
	val, ok = lookup(name)
	return val, ok, nil
}

// splitVarSpec parses the contents of a ${...} block. Returns
// (varName, fallback, fallbackPresent).
func splitVarSpec(s string) (name, fallback string, hasFallback bool) {
	if before, after, ok := strings.Cut(s, ":-"); ok {
		return before, after, true
	}
	return s, "", false
}

// isValidVarName reports whether s is a valid POSIX-ish env var name:
// non-empty, starts with a letter or underscore, contains only letters,
// digits, and underscores.
func isValidVarName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}
