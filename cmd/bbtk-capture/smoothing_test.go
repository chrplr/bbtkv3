package main

import (
	"testing"

	"github.com/chrplr/bbtkv3"
)

// The -s default is what every capture that does not pass the flag programs
// into the device, and the same mask is handed to the duration correction. It
// stood as an unconfigurable literal before -s existed, so this pins the value
// the flag inherited: mics and Opto1/Opto2 smoothed, Opto3/Opto4 raw.
func TestDefaultSmoothingMaskParses(t *testing.T) {
	got, err := bbtkv3.SmoothingMaskFromString(DefaultSmoothingMask)
	if err != nil {
		t.Fatalf("DefaultSmoothingMask %q does not parse: %v", DefaultSmoothingMask, err)
	}

	want := bbtkv3.SmoothingMask{
		Mic1:  true,
		Mic2:  true,
		Opto4: false,
		Opto3: false,
		Opto2: true,
		Opto1: true,
	}
	if got != want {
		t.Errorf("DefaultSmoothingMask = %+v, want %+v", got, want)
	}
}
