package bbtkv3

import (
	"fmt"
	"strconv"
	"strings"
)

type SmoothingMask struct {
	Mic1  bool
	Mic2  bool
	Opto4 bool
	Opto3 bool
	Opto2 bool
	Opto1 bool
}

var defaultSmoothingMask = SmoothingMask{
	Mic1:  true,
	Mic2:  true,
	Opto4: true,
	Opto3: true,
	Opto2: true,
	Opto1: true,
}

// DefaultSmoothingDurationOffsetMs is how much longer an event reads on a
// channel that has smoothing enabled, in milliseconds.
//
// Smoothing holds the line high past the true falling edge, so a recorded
// duration is the stimulus plus a fixed tail. Onsets appear unaffected — see the
// caveat below.
//
// Measured 2026-07-31 on host is158520 (BBTK V3, DSC capture at 0.25 ms
// resolution), against goxpyriment's Timing-Tests stimulus:
//
//	channel   200.2 ms stim   200.2 ms stim   16.7 ms stim   smoothing
//	Opto1        +21.137         +21.011        +20.792        on
//	Opto2        +20.359         +20.529        +19.992        on
//	Mic1         +19.891         +19.863        +20.397        on
//	TTLin1        +0.174          +0.171         +0.175        not in mask
//
// Two things make this a fixed instrument offset rather than anything optical or
// acoustic: it is stable to within 0.5 ms across a 12x change in stimulus length,
// and TTLin1 — the one channel the smoothing mask does not cover — shows none of
// it (a 5.000 ms commanded pulse reads 5.17 ms).
//
// The per-channel spread is systematic, not noise: Opto1 > Opto2 > Mic1 in all
// three captures, about 1 ms end to end. A single constant is used anyway —
// three channels on one device is not enough to justify a hardcoded per-channel
// table, and callers that need better can pass their own offset. Anyone
// tightening this should re-measure on their own device rather than trusting the
// figure above.
//
// Onsets are NOT affected: per the BBTK documentation, smoothing does not delay
// the leading edge. Only the tail is extended, which is why the correction
// belongs on Duration alone and absolute onset latencies measured against a
// smoothed channel need no adjustment.
const DefaultSmoothingDurationOffsetMs = 20.0

// Enabled reports whether smoothing applies to the named input port (as spelled
// in InputPortNames, e.g. "Opto1", "Mic1"). Ports the mask does not cover —
// TTLin1/2 and the keypad — always return false, so their durations are never
// corrected.
func (s SmoothingMask) Enabled(portName string) bool {
	switch portName {
	case "Mic1":
		return s.Mic1
	case "Mic2":
		return s.Mic2
	case "Opto1":
		return s.Opto1
	case "Opto2":
		return s.Opto2
	case "Opto3":
		return s.Opto3
	case "Opto4":
		return s.Opto4
	}
	return false
}

// CorrectedDuration removes the smoothing tail from a duration recorded on
// portName, given the mask in force during the capture. Channels without
// smoothing are returned unchanged.
//
// The result is clamped at zero: an event shorter than the offset would
// otherwise come back negative, which is worse than useless in a data file. A
// clamped value is a signal in itself — the stimulus was shorter than the
// smoothing window, so its recorded duration carried no usable information.
func CorrectedDuration(duration float64, portName string, mask SmoothingMask, offsetMs float64) float64 {
	if !mask.Enabled(portName) {
		return duration
	}
	if corrected := duration - offsetMs; corrected > 0 {
		return corrected
	}
	return 0
}

// ToString converts a SmoothingMask struct to a semicolon-separated string
// Each boolean is represented as "1" for true and "0" for false
func (s SmoothingMask) ToString() string {
	// Convert each boolean to 1 or 0
	mic1 := boolToInt(s.Mic1)
	mic2 := boolToInt(s.Mic2)
	opto4 := boolToInt(s.Opto4)
	opto3 := boolToInt(s.Opto3)
	opto2 := boolToInt(s.Opto2)
	opto1 := boolToInt(s.Opto1)

	return fmt.Sprintf("%d;%d;%d;%d;%d;%d", mic1, mic2, opto4, opto3, opto2, opto1)
}

// FromString parses a semicolon-separated string into a SmoothingMask struct
func SmoothingMaskFromString(s string) (SmoothingMask, error) {
	var mask SmoothingMask
	parts := strings.Split(s, ";")

	if len(parts) != 6 {
		return mask, fmt.Errorf("invalid format: expected 6 values, got %d", len(parts))
	}

	// Parse each value to bool
	boolValues := make([]bool, 6)
	for i, part := range parts {
		val, err := strconv.ParseUint(part, 10, 8)
		if err != nil {
			return mask, fmt.Errorf("invalid value at position %d: %v", i, err)
		}

		// Only 0 and 1 are valid values
		if val != 0 && val != 1 {
			return mask, fmt.Errorf("invalid value at position %d: expected 0 or 1, got %d", i, val)
		}

		boolValues[i] = val == 1
	}

	// Assign values to struct fields
	mask.Mic1 = boolValues[0]
	mask.Mic2 = boolValues[1]
	mask.Opto4 = boolValues[2]
	mask.Opto3 = boolValues[3]
	mask.Opto2 = boolValues[4]
	mask.Opto1 = boolValues[5]

	return mask, nil
}

// Helper function to convert bool to int
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
