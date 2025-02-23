package bbtkv3

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
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

// DSCEvent represents a single event with timestamp and port states
type DSCEvent struct {
	Timestamp  float64
	PortStates map[string]int
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

// LocateEdges finds the positions of leading and falling edges in a binary sequence
func LocateEdges(sequence []int) ([]Edge, []Edge, error) {
	if len(sequence) <= 2 {
		return nil, nil, errors.New("sequence too short")
	}

	// Check baseline conditions
	if sequence[0] != 0 {
		return nil, nil, errors.New("signal must start at baseline (0)")
	}
	if sequence[len(sequence)-1] != 0 {
		return nil, nil, errors.New("signal must end at baseline (0)")
	}

	var leadingEdges []Edge
	var fallingEdges []Edge

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

		// Force first value to 0 (baseline)
		sequence[0] = 0
		timestamps[0] = rawEvents[0].Timestamp

		// Fill the rest of the sequence
		for i := 1; i < len(rawEvents); i++ {
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

// SaveEventsToCSV saves detected events to a CSV file
func SaveEventsToCSV(events []Event, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"Type", "Onset", "Duration"}); err != nil {
		return fmt.Errorf("error writing header: %w", err)
	}

	// Write events
	for _, event := range events {
		row := []string{
			event.Type,
			strconv.FormatFloat(event.Onset, 'f', 3, 64),
			strconv.FormatFloat(event.Duration, 'f', 3, 64),
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("error writing row: %w", err)
		}
	}

	return nil
}
