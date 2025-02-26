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
