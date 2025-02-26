package bbtkv3

import (
	"fmt"
	"strconv"
	"strings"
)

type Thresholds struct {
	Mic1     uint8
	Mic2     uint8
	Sounder1 uint8
	Sounder2 uint8
	Opto1    uint8
	Opto2    uint8
	Opto3    uint8
	Opto4    uint8
}

var defaultThresholds = Thresholds{
	Mic1:     63,
	Mic2:     63,
	Sounder1: 63,
	Sounder2: 63,
	Opto1:    63,
	Opto2:    63,
	Opto3:    63,
	Opto4:    63,
}

// ToString converts a Thresholds struct to a comma-separated string
func (t Thresholds) ToString() string {
	return fmt.Sprintf("%d,%d,%d,%d,%d,%d,%d,%d",
		t.Mic1, t.Mic2, t.Sounder1, t.Sounder2, t.Opto1, t.Opto2, t.Opto3, t.Opto4)
}

// FromString parses a comma-separated string into a Thresholds struct
func ThresholdsFromString(s string) (Thresholds, error) {
	var t Thresholds
	parts := strings.Split(s, ",")

	if len(parts) != 8 {
		return t, fmt.Errorf("invalid format: expected 8 values, got %d", len(parts))
	}

	// Parse each value to uint8
	var values [8]uint8
	for i, part := range parts {
		val, err := strconv.ParseUint(part, 10, 8)
		if err != nil {
			return t, fmt.Errorf("invalid value at position %d: %v", i, err)
		}
		values[i] = uint8(val)
	}
	// Assign values to struct fields
	t.Mic1 = values[0]
	t.Mic2 = values[1]
	t.Sounder1 = values[2]
	t.Sounder2 = values[3]
	t.Opto1 = values[4]
	t.Opto2 = values[5]
	t.Opto3 = values[6]
	t.Opto4 = values[7]

	return t, nil
}
