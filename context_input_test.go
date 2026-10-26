package ezclassify

import (
	"errors"
	"testing"
)

func TestExclusiveInputPresence(t *testing.T) {
	format := ""
	for _, tc := range []struct {
		name         string
		input, facts bool
		format       *string
		refusal      string
	}{
		{"text", true, false, nil, ""},
		{"empty text with format", true, false, &format, ""},
		{"facts", false, true, nil, ""},
		{"neither", false, false, nil, "input-or-assertions"},
		{"both", true, true, nil, "input-or-assertions"},
		{"facts with empty format", false, true, &format, "input-format-with-assertions"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := exclusiveInputError("scheme.suggest", tc.input, tc.facts, tc.format)
			if tc.refusal == "" {
				if err != nil {
					t.Fatalf("error = %v, want none", err)
				}
				return
			}
			var invalid *InvalidInputError
			if !errors.As(err, &invalid) || invalid.Op != "scheme.suggest" || invalid.Detail != tc.refusal {
				t.Fatalf("error = %T(%v), want an *InvalidInputError with detail %q", err, err, tc.refusal)
			}
		})
	}
}
