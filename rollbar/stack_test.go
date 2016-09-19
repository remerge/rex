package rollbar

import (
	"testing"
)

func TestStackFingerprint(t *testing.T) {
	tests := []struct {
		Fingerprint string
		Stack       Stack
	}{
		{
			"9344290d",
			Stack{
				Frame{"foo.go", "Oops", "", 1},
			},
		},
		{
			"a4d78b7",
			Stack{
				Frame{"foo.go", "Oops", "", 2},
			},
		},
		{
			"50e0fcb3",
			Stack{
				Frame{"foo.go", "Oops", "", 1},
				Frame{"foo.go", "Oops", "", 2},
			},
		},
	}

	for i, test := range tests {
		fingerprint := test.Stack.Fingerprint()
		if fingerprint != test.Fingerprint {
			t.Errorf("tests[%d]: got %s", i, fingerprint)
		}
	}
}
