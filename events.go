package bbtkv3

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"

	//"os"
	//"path/filepath"
	"strings"
)

// Port name constants
var InputPortNames = []string{
	"Keypad4", "Keypad3", "Keypad2", "Keypad1",
	"Opto4", "Opto3", "Opto2", "Opto1",
	"TTLin2", "TTLin1", "Mic2", "Mic1",
}

var OutputPortNames = []string{
	"ActClose4", "ActClose3", "ActClose2", "ActClose1",
	"TTLout2", "TTLout1", "Sounder2", "Sounder1",
}

// DSCLineNames combines all column names for the output DataFrame
var DSCLineNames = append([]string{"timestamp"}, append(InputPortNames, OutputPortNames...)...)

// PortState represents the state of input or output ports
type PortState map[string]int

// DSCEvent represents a single event (transition) with timestamp and port states
type DSCEvent struct {
	Timestamp  float64
	PortStates map[string]int
}

// Edge represents a binary signal edge with its position
type Edge struct {
	Position int
	Time     float64
}

// Event represents a complete event with type, onset time, and duration
type Event struct {
	Type     string
	Onset    float64
	Duration float64
}

// OutputPortMask8ToSeries converts an 8-bit string to a map of port states
func OutputPortMask8ToSeries(mask8 string) (PortState, error) {
	if len(mask8) != 8 {
		return nil, errors.New("mask must be exactly 8 bits long")
	}

	result := make(PortState)
	for i, name := range OutputPortNames {
		bit, err := strconv.Atoi(string(mask8[i]))
		if err != nil {
			return nil, errors.New("mask must contain only binary digits (0 or 1)")
		}
		result[name] = bit
	}
	return result, nil
}

func Txt2DSCEvent(txt string) (*DSCEvent, error) {

	timestamp, err := strconv.ParseFloat(txt[20:], 64)
	if err != nil {
		return nil, errors.New("invalid timestamp format")
	}
	timestamp /= 1000.0 // Convert to milliseconds

	// Process port states
	portStates := make(map[string]int)
	for i, name := range append(InputPortNames, OutputPortNames...) {
		bit, err := strconv.Atoi(string(txt[i]))
		if err != nil {
			return nil, errors.New("invalid port state format")
		}
		portStates[name] = bit
	}

	return &DSCEvent{Timestamp: timestamp, PortStates: portStates}, nil
}

// CaptureOutputToEvents converts DSC command output text to a slice of events
func CaptureOutputToEvents(text string) ([]DSCEvent, error) {
	var events []DSCEvent

	lines := strings.Split(text, ";")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 32 {
			// Extract timestamp (converting microseconds to milliseconds)
			timestamp, err := strconv.ParseFloat(line[20:], 64)
			if err != nil {
				return nil, errors.New("invalid timestamp format")
			}
			timestamp /= 1000.0 // Convert to milliseconds

			// Process port states
			portStates := make(map[string]int)
			for i, name := range append(InputPortNames, OutputPortNames...) {
				bit, err := strconv.Atoi(string(line[i]))
				if err != nil {
					return nil, errors.New("invalid port state format")
				}
				portStates[name] = bit
			}

			events = append(events, DSCEvent{
				Timestamp:  timestamp,
				PortStates: portStates,
			})
		}
	}

	return events, nil
}

// SaveDSCEventsToCSV saves a slice of DSCEvents to a CSV file
func SaveDSCEventsToCSV(events []DSCEvent, filename string) error {
	// Create or truncate the file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer file.Close()

	// Create CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write(DSCLineNames); err != nil {
		return fmt.Errorf("error writing header: %w", err)
	}

	// Write each event
	for _, event := range events {
		// Create a row slice with capacity for all fields
		row := make([]string, len(DSCLineNames))

		// First column is timestamp
		row[0] = strconv.FormatFloat(event.Timestamp, 'f', 3, 64)

		// Fill in port states in the correct order
		for i, portName := range InputPortNames {
			row[i+1] = strconv.Itoa(event.PortStates[portName])
		}
		for i, portName := range OutputPortNames {
			row[i+1+len(InputPortNames)] = strconv.Itoa(event.PortStates[portName])
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("error writing row: %w", err)
		}
	}

	return nil
}

// LocateEdges finds the positions of leading and falling edges in a binary sequence.
// If the sequence starts at 1 (sensor already active), a leading edge at position 0
// is recorded. If it ends at 1 (sensor still active), there is no corresponding
// falling edge; the caller is responsible for handling that incomplete event.
//
// A sequence with no edges in it is not an error — it yields no edges. Short
// sequences used to be rejected outright, which made a capture that recorded
// nothing at all fail here rather than report itself as empty: the device returns
// a single all-zero record in that case, so every port's sequence was length one
// and "sequence too short" was the only thing the caller ever saw. The loop below
// is correct for any length; only an empty slice needs guarding, because the
// starts-high test would index it.
func LocateEdges(sequence []int) ([]Edge, []Edge, error) {
	if len(sequence) == 0 {
		return nil, nil, nil
	}

	var leadingEdges []Edge
	var fallingEdges []Edge

	// If the signal is already high at the start, record a leading edge at position 0.
	if sequence[0] == 1 {
		leadingEdges = append(leadingEdges, Edge{Position: 0})
	}

	// Find edges by comparing adjacent values
	for i := 1; i < len(sequence); i++ {
		if sequence[i-1] == 0 && sequence[i] == 1 {
			leadingEdges = append(leadingEdges, Edge{Position: i})
		} else if sequence[i-1] == 1 && sequence[i] == 0 {
			fallingEdges = append(fallingEdges, Edge{Position: i})
		}
	}

	return leadingEdges, fallingEdges, nil
}

// CaptureEventsFromDSCEvents converts raw DSC events into a slice of detected events
func CaptureEventsFromDSCEvents(rawEvents []DSCEvent) ([]Event, error) {
	if len(rawEvents) == 0 {
		return nil, errors.New("no events provided")
	}

	var allEvents []Event

	// Process each input port
	for _, portName := range InputPortNames {
		// Extract binary sequence for this port
		sequence := make([]int, len(rawEvents))
		timestamps := make([]float64, len(rawEvents))

		// Fill the sequence with actual port states and timestamps
		for i := 0; i < len(rawEvents); i++ {
			sequence[i] = rawEvents[i].PortStates[portName]
			timestamps[i] = rawEvents[i].Timestamp
		}

		// Locate edges
		leadingEdges, fallingEdges, err := LocateEdges(sequence)
		if err != nil {
			return nil, fmt.Errorf("error processing port %s: %w", portName, err)
		}

		// Skip if no events detected
		if len(leadingEdges) == 0 {
			continue
		}

		// Create events from edges
		for i := 0; i < len(leadingEdges); i++ {
			if i >= len(fallingEdges) {
				// Signal still active at end of capture; no falling edge available.
				break
			}
			event := Event{
				Type:     portName,
				Onset:    timestamps[leadingEdges[i].Position],
				Duration: timestamps[fallingEdges[i].Position] - timestamps[leadingEdges[i].Position],
			}
			allEvents = append(allEvents, event)
		}
	}

	return allEvents, nil
}

// SortEventsByDuration sorts a slice of Event in ascending order of Duration.
func SortEventsByDuration(events []Event) {
	sort.Slice(events, func(i, j int) bool {
		return events[i].Duration < events[j].Duration
	})
}

// SaveEventsToCSV saves detected events to a CSV file with the three historic
// columns: Type, Onset, Duration. Durations are exactly as recorded, smoothing
// tail included.
//
// Prefer SaveEventsToCSVWithCorrection when the smoothing mask is known — its
// extra column is what makes a file say which convention it follows.
func SaveEventsToCSV(events []Event, filename string) error {
	return writeEventsCSV(events, filename, nil, 0)
}

// SaveEventsToCSVWithCorrection saves detected events with a fourth column,
// DurationCorrected, holding the duration with the smoothing tail removed on the
// channels the mask covers (see CorrectedDuration). Channels without smoothing
// repeat the recorded value, so the two columns agree wherever no correction
// applies.
//
// Duration itself is never altered. Keeping the recorded value means captures
// taken before this column existed stay directly comparable, and a mistaken
// offset can be undone — neither of which survives correcting in place.
func SaveEventsToCSVWithCorrection(events []Event, filename string, mask SmoothingMask, offsetMs float64) error {
	return writeEventsCSV(events, filename, &mask, offsetMs)
}

// writeEventsCSV writes the events sorted by onset. A nil mask selects the
// three-column form.
func writeEventsCSV(events []Event, filename string, mask *SmoothingMask, offsetMs float64) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	sort.Slice(events, func(i, j int) bool {
		return events[i].Onset < events[j].Onset
	})

	header := []string{"Type", "Onset", "Duration"}
	if mask != nil {
		header = append(header, "DurationCorrected")
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("error writing header: %w", err)
	}

	for _, event := range events {
		row := []string{
			event.Type,
			strconv.FormatFloat(event.Onset, 'f', 3, 64),
			strconv.FormatFloat(event.Duration, 'f', 3, 64),
		}
		if mask != nil {
			corrected := CorrectedDuration(event.Duration, event.Type, *mask, offsetMs)
			row = append(row, strconv.FormatFloat(corrected, 'f', 3, 64))
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("error writing row: %w", err)
		}
	}

	// csv.Writer buffers; a flush error here is the difference between a
	// truncated capture and a complete one, so it must not be swallowed by the
	// deferred Flush.
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("error flushing %s: %w", filename, err)
	}
	return nil
}
