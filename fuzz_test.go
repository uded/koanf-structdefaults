package structdefaults

import (
	"errors"
	"strings"
	"testing"
)

// FuzzSubstituteEnv verifies the "never panic" contract of substituteEnv and
// enforces the basic correctness property: when the lookup returns ok=true for
// the known name, the returned string must contain that value.
//
// The fuzz target intentionally avoids os.LookupEnv; the in-memory lookup is
// fully deterministic so corpus entries are reproducible across machines.
func FuzzSubstituteEnv(f *testing.F) {
	// Seed corpus — one entry per distinct input shape exercised in the unit
	// tests plus a handful of edge cases the parser handles specially.
	seeds := []string{
		// plain string — fast-path shortcut (no "${")
		"plain string",
		// single variable, set
		"${KNOWN}",
		// single variable with fallback, variable unset
		"${MISSING:-fallback}",
		// single variable with fallback, variable set
		"${KNOWN:-ignored}",
		// fallback with empty string
		"${MISSING:-}",
		// multiple variables in one string
		"${KNOWN}:${OTHER}",
		// unmatched open — kept literal
		"${UNFINISHED",
		// invalid var name starting with digit — kept literal
		"${1ABC}",
		// empty braces — kept literal
		"${}",
		// fallback whose value looks like a nested ref (non-recursive)
		"${MISSING:-${NESTED}}",
		// fallback with dots and dashes (semver-like)
		"${X:-1.2.3-rc.4}",
		// bare dollar, no brace
		"$PLAIN",
		// empty input
		"",
		// only whitespace
		"   ",
		// multiple adjacent refs, one unset-with-fallback
		"host=${KNOWN} port=${PORT:-8080}",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	// deterministic in-memory lookup:
	//   "KNOWN" → ("fuzz_value", true)
	//   everything else → ("", false)
	const knownName = "KNOWN"
	const knownValue = "fuzz_value"

	lookup := func(name string) (string, bool) {
		if name == knownName {
			return knownValue, true
		}
		return "", false
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Contract 1 — must not panic (guaranteed by the defer in safeLookup,
		// but the parser itself must not panic on arbitrary byte sequences).
		result, err := substituteEnv(input, lookup)

		// Contract 2 — the only allowed non-nil error is one that wraps
		// ErrUnsetEnv (or ErrLookupPanic for a panicking lookup, though our
		// lookup never panics). Any other error category is a defect.
		if err != nil {
			if !errors.Is(err, ErrUnsetEnv) && !errors.Is(err, ErrLookupPanic) {
				t.Fatalf("substituteEnv(%q) returned unexpected error: %v", input, err)
			}
			// When an error is returned, result should be empty string (the
			// function returns "" on the error path).
			if result != "" {
				t.Fatalf("substituteEnv(%q) returned non-empty result %q alongside error %v", input, result, err)
			}
			return
		}

		// Contract 3 — positive correctness: if the input contains a
		// top-level ${KNOWN} reference (one that the single-pass parser will
		// actually consume as a variable reference rather than as part of the
		// inner text of an outer invalid token), the output must contain
		// knownValue.
		//
		// We determine "top-level" by running the same forward scan the parser
		// uses: if the parser has already consumed the bytes at the ${KNOWN}
		// position as part of an earlier "${...}" token (valid or invalid), the
		// occurrence is not top-level.
		if containsTopLevelRef(input, knownName) && !strings.Contains(result, knownValue) {
			t.Fatalf("substituteEnv(%q) = %q; expected it to contain %q", input, result, knownValue)
		}
	})
}

// containsTopLevelRef reports whether raw contains a top-level ${name}
// reference — one that the substituteEnv single-pass parser will process as a
// variable reference rather than as text embedded inside the inner content of
// some outer "${...}" token.
//
// The logic mirrors the parser's forward scan: we advance index i, and whenever
// we see "${", we jump past the matching "}" (or to end-of-string if there is
// none). Only positions not consumed by an outer token are considered.
func containsTopLevelRef(raw, name string) bool {
	target := "${" + name + "}"
	for i := 0; i < len(raw); {
		if i+1 < len(raw) && raw[i] == '$' && raw[i+1] == '{' {
			closeRel := strings.IndexByte(raw[i+2:], '}')
			if closeRel < 0 {
				// No closing brace — the rest of the string is consumed literally.
				return false
			}
			next := i + 2 + closeRel + 1
			// Check if this exact token is our target reference.
			if raw[i:next] == target {
				return true
			}
			// Not our target; skip past this whole token.
			i = next
			continue
		}
		// Check if target starts exactly here (it would need raw[i]=='$',
		// which the above branch already handles; reaching here means raw[i]!='$'
		// or raw[i+1]!='{', so no match possible at i).
		i++
	}
	return false
}
