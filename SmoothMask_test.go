package bbtkv3

import "testing"

func TestSmoothingMaskFromString(t *testing.T) {
	// The field order is the device's, with the Opto channels running
	// downwards: Mic1;Mic2;Opto4;Opto3;Opto2;Opto1. A mask that is not
	// symmetric under reversal is the only kind that catches getting it wrong.
	want := SmoothingMask{Mic1: true, Mic2: true, Opto4: false, Opto3: false, Opto2: true, Opto1: true}

	for _, s := range []string{
		"1;1;0;0;1;1", // wire form, as ToString emits it
		"1,1,0,0,1,1", // comma form, which needs no shell quoting
	} {
		got, err := SmoothingMaskFromString(s)
		if err != nil {
			t.Fatalf("SmoothingMaskFromString(%q): %v", s, err)
		}
		if got != want {
			t.Errorf("SmoothingMaskFromString(%q) = %+v, want %+v", s, got, want)
		}
	}
}

func TestSmoothingMaskRoundTrip(t *testing.T) {
	// ToString must stay the inverse of the parser, since the same string goes
	// on the wire to the device.
	for _, mask := range []SmoothingMask{
		SmoothingAllOn,
		{},
		{Mic1: true, Opto1: true},
		{Opto4: true, Opto3: true},
	} {
		got, err := SmoothingMaskFromString(mask.ToString())
		if err != nil {
			t.Fatalf("round trip of %+v: %v", mask, err)
		}
		if got != mask {
			t.Errorf("round trip of %+v gave %+v (via %q)", mask, got, mask.ToString())
		}
	}
}

func TestSmoothingMaskFromStringRejects(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
	}{
		{"too few fields", "1;1;0;0;1"},
		{"too many fields", "1;1;0;0;1;1;1"},
		{"not a number", "1;1;0;0;1;x"},
		{"out of range", "1;1;0;0;1;2"},
		{"empty", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := SmoothingMaskFromString(tc.in); err == nil {
				t.Errorf("SmoothingMaskFromString(%q) accepted, want an error", tc.in)
			}
		})
	}
}
