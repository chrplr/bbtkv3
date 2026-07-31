package bbtkv3

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// --- LocateEdges ---

func TestLocateEdges_Normal(t *testing.T) {
	// Single pulse: 0 0 1 1 0 0
	seq := []int{0, 0, 1, 1, 0, 0}
	leading, falling, err := LocateEdges(seq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(leading) != 1 || leading[0].Position != 2 {
		t.Errorf("expected leading edge at 2, got %v", leading)
	}
	if len(falling) != 1 || falling[0].Position != 4 {
		t.Errorf("expected falling edge at 4, got %v", falling)
	}
}

func TestLocateEdges_MultiplePulses(t *testing.T) {
	// Two pulses
	seq := []int{0, 1, 0, 0, 1, 1, 0}
	leading, falling, err := LocateEdges(seq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(leading) != 2 {
		t.Fatalf("expected 2 leading edges, got %d", len(leading))
	}
	if len(falling) != 2 {
		t.Fatalf("expected 2 falling edges, got %d", len(falling))
	}
	if leading[0].Position != 1 || leading[1].Position != 4 {
		t.Errorf("unexpected leading edge positions: %v", leading)
	}
	if falling[0].Position != 2 || falling[1].Position != 6 {
		t.Errorf("unexpected falling edge positions: %v", falling)
	}
}

func TestLocateEdges_AllZeros(t *testing.T) {
	seq := []int{0, 0, 0, 0, 0}
	leading, falling, err := LocateEdges(seq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(leading) != 0 || len(falling) != 0 {
		t.Errorf("expected no edges, got leading=%v falling=%v", leading, falling)
	}
}

// Short sequences are valid input, not an error. A capture that recorded nothing
// yields a single all-zero record, so every port's sequence has length one — and
// rejecting those made an empty capture fail instead of reporting itself empty.
func TestLocateEdges_ShortSequences(t *testing.T) {
	cases := []struct {
		name            string
		seq             []int
		leading, faling []int // expected edge positions
	}{
		{name: "empty", seq: []int{}},
		{name: "single zero", seq: []int{0}},
		// One sample, already high: an event that opened before the capture and
		// has no falling edge. Reported as a leading edge at 0, per the contract
		// above.
		{name: "single one", seq: []int{1}, leading: []int{0}},
		{name: "two zeros", seq: []int{0, 0}},
		{name: "rising pair", seq: []int{0, 1}, leading: []int{1}},
		{name: "falling pair", seq: []int{1, 0}, leading: []int{0}, faling: []int{1}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			leading, falling, err := LocateEdges(c.seq)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gotL := make([]int, len(leading))
			for i, e := range leading {
				gotL[i] = e.Position
			}
			gotF := make([]int, len(falling))
			for i, e := range falling {
				gotF[i] = e.Position
			}
			if !reflect.DeepEqual(gotL, c.leading) && !(len(gotL) == 0 && len(c.leading) == 0) {
				t.Errorf("leading = %v, want %v", gotL, c.leading)
			}
			if !reflect.DeepEqual(gotF, c.faling) && !(len(gotF) == 0 && len(c.faling) == 0) {
				t.Errorf("falling = %v, want %v", gotF, c.faling)
			}
		})
	}
}

// The exact shape the device returns from a capture in which nothing was
// detected: one all-zero record, plus the end-of-capture sentinel bbtk-capture
// appends. This must yield no events and no error, so the caller can still write
// its (empty) CSV.
func TestCaptureEventsFromDSCEvents_EmptyCapture(t *testing.T) {
	zero := map[string]int{}
	for _, p := range InputPortNames {
		zero[p] = 0
	}
	raw := []DSCEvent{
		{Timestamp: 0, PortStates: zero},
		{Timestamp: 27000}, // sentinel; nil PortStates reads as 0 for every port
	}

	events, err := CaptureEventsFromDSCEvents(raw)
	if err != nil {
		t.Fatalf("an empty capture must not be an error, got: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected no events, got %d", len(events))
	}
}

func TestLocateEdges_StartsHigh(t *testing.T) {
	// Sensor already active at capture start: leading edge at position 0
	seq := []int{1, 1, 0, 0}
	leading, falling, err := LocateEdges(seq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(leading) != 1 || leading[0].Position != 0 {
		t.Errorf("expected leading edge at position 0, got %v", leading)
	}
	if len(falling) != 1 || falling[0].Position != 2 {
		t.Errorf("expected falling edge at position 2, got %v", falling)
	}
}

func TestLocateEdges_EndsHigh(t *testing.T) {
	// Sensor still active at end of capture: no falling edge for last pulse
	seq := []int{0, 0, 1, 1}
	leading, falling, err := LocateEdges(seq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(leading) != 1 || leading[0].Position != 2 {
		t.Errorf("expected leading edge at 2, got %v", leading)
	}
	if len(falling) != 0 {
		t.Errorf("expected no falling edges, got %v", falling)
	}
}

func TestLocateEdges_StartsAndEndsHigh(t *testing.T) {
	// Already active, stays active the whole time
	seq := []int{1, 1, 1, 1}
	leading, falling, err := LocateEdges(seq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(leading) != 1 || leading[0].Position != 0 {
		t.Errorf("expected single leading edge at 0, got %v", leading)
	}
	if len(falling) != 0 {
		t.Errorf("expected no falling edges, got %v", falling)
	}
}

// --- CaptureEventsFromDSCEvents ---

// makeRawEvents builds a minimal []DSCEvent with a single port varying over time.
func makeRawEvents(portName string, states []int, times []float64) []DSCEvent {
	events := make([]DSCEvent, len(states))
	for i, s := range states {
		ps := make(map[string]int)
		// All ports default to 0; set the one we care about.
		for _, name := range InputPortNames {
			ps[name] = 0
		}
		ps[portName] = s
		events[i] = DSCEvent{Timestamp: times[i], PortStates: ps}
	}
	return events
}

func TestCaptureEventsFromDSCEvents_SingleEvent(t *testing.T) {
	states := []int{0, 0, 1, 1, 0, 0}
	times := []float64{0, 10, 20, 30, 40, 50}
	raw := makeRawEvents("Mic1", states, times)

	events, err := CaptureEventsFromDSCEvents(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]
	if e.Type != "Mic1" {
		t.Errorf("expected Type=Mic1, got %q", e.Type)
	}
	if e.Onset != 20 {
		t.Errorf("expected Onset=20, got %v", e.Onset)
	}
	if e.Duration != 20 {
		t.Errorf("expected Duration=20, got %v", e.Duration)
	}
}

func TestCaptureEventsFromDSCEvents_MultipleEvents(t *testing.T) {
	states := []int{0, 1, 0, 1, 0}
	times := []float64{0, 10, 20, 30, 40}
	raw := makeRawEvents("Opto1", states, times)

	events, err := CaptureEventsFromDSCEvents(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Onset != 10 || events[0].Duration != 10 {
		t.Errorf("event 0: got Onset=%v Duration=%v", events[0].Onset, events[0].Duration)
	}
	if events[1].Onset != 30 || events[1].Duration != 10 {
		t.Errorf("event 1: got Onset=%v Duration=%v", events[1].Onset, events[1].Duration)
	}
}

func TestCaptureEventsFromDSCEvents_StartsHigh(t *testing.T) {
	// Sensor was already active at capture start.
	states := []int{1, 1, 0, 0}
	times := []float64{0, 10, 20, 30}
	raw := makeRawEvents("TTLin1", states, times)

	events, err := CaptureEventsFromDSCEvents(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Onset != 0 {
		t.Errorf("expected Onset=0 (first timestamp), got %v", events[0].Onset)
	}
	if events[0].Duration != 20 {
		t.Errorf("expected Duration=20, got %v", events[0].Duration)
	}
}

func TestCaptureEventsFromDSCEvents_EndsHigh(t *testing.T) {
	// Sensor still active at end of capture — incomplete event is silently skipped.
	states := []int{0, 0, 1, 1}
	times := []float64{0, 10, 20, 30}
	raw := makeRawEvents("Keypad1", states, times)

	events, err := CaptureEventsFromDSCEvents(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events (no falling edge), got %d: %v", len(events), events)
	}
}

func TestCaptureEventsFromDSCEvents_StartsAndEndsHigh(t *testing.T) {
	// Active for the entire capture — no falling edge, event is skipped.
	states := []int{1, 1, 1}
	times := []float64{0, 10, 20}
	raw := makeRawEvents("Mic2", states, times)

	events, err := CaptureEventsFromDSCEvents(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestCaptureEventsFromDSCEvents_NoActivity(t *testing.T) {
	states := []int{0, 0, 0, 0}
	times := []float64{0, 10, 20, 30}
	raw := makeRawEvents("Opto2", states, times)

	events, err := CaptureEventsFromDSCEvents(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestCaptureEventsFromDSCEvents_Empty(t *testing.T) {
	_, err := CaptureEventsFromDSCEvents(nil)
	if err == nil {
		t.Error("expected error for empty input")
	}
}

// --- OutputPortMask8ToSeries ---

func TestOutputPortMask8ToSeries_Valid(t *testing.T) {
	ps, err := OutputPortMask8ToSeries("10000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ps["ActClose4"] != 1 {
		t.Errorf("expected ActClose4=1, got %d", ps["ActClose4"])
	}
	if ps["Sounder1"] != 1 {
		t.Errorf("expected Sounder1=1, got %d", ps["Sounder1"])
	}
	if ps["TTLout1"] != 0 {
		t.Errorf("expected TTLout1=0, got %d", ps["TTLout1"])
	}
}

func TestOutputPortMask8ToSeries_WrongLength(t *testing.T) {
	_, err := OutputPortMask8ToSeries("1010")
	if err == nil {
		t.Error("expected error for mask of wrong length")
	}
}

func TestOutputPortMask8ToSeries_InvalidChar(t *testing.T) {
	_, err := OutputPortMask8ToSeries("1010X010")
	if err == nil {
		t.Error("expected error for non-binary character")
	}
}

// --- SaveDSCEventsToCSV ---

func TestSaveDSCEventsToCSV(t *testing.T) {
	events := []DSCEvent{
		{
			Timestamp: 12.345,
			PortStates: func() map[string]int {
				ps := make(map[string]int)
				for _, n := range InputPortNames {
					ps[n] = 0
				}
				for _, n := range OutputPortNames {
					ps[n] = 0
				}
				ps["Mic1"] = 1
				return ps
			}(),
		},
	}

	path := filepath.Join(t.TempDir(), "test.csv")
	if err := SaveDSCEventsToCSV(events, path); err != nil {
		t.Fatalf("SaveDSCEventsToCSV: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open output file: %v", err)
	}
	defer f.Close()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("failed to read CSV: %v", err)
	}

	if len(records) != 2 { // header + 1 data row
		t.Fatalf("expected 2 rows, got %d", len(records))
	}
	if records[0][0] != "timestamp" {
		t.Errorf("expected first header column to be 'timestamp', got %q", records[0][0])
	}
	if records[1][0] != "12.345" {
		t.Errorf("expected timestamp 12.345, got %q", records[1][0])
	}
	// Mic1 is the last InputPort (index 12), so column 13 (1-based)
	mic1Col := len(InputPortNames) // 12 → index 12 in header (0=timestamp, 1..12=inputs)
	if records[1][mic1Col] != "1" {
		t.Errorf("expected Mic1=1 at column %d, got %q", mic1Col, records[1][mic1Col])
	}
}

// --- SaveEventsToCSV ---

func TestSaveEventsToCSV(t *testing.T) {
	events := []Event{
		{Type: "Mic1", Onset: 20.0, Duration: 15.5},
		{Type: "Opto1", Onset: 100.0, Duration: 5.0},
	}

	path := filepath.Join(t.TempDir(), "events.csv")
	if err := SaveEventsToCSV(events, path); err != nil {
		t.Fatalf("SaveEventsToCSV: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open output file: %v", err)
	}
	defer f.Close()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("failed to read CSV: %v", err)
	}

	if len(records) != 3 { // header + 2 rows
		t.Fatalf("expected 3 rows, got %d", len(records))
	}
	if records[0][0] != "Type" || records[0][1] != "Onset" || records[0][2] != "Duration" {
		t.Errorf("unexpected header: %v", records[0])
	}
	if records[1][0] != "Mic1" || records[1][1] != "20.000" || records[1][2] != "15.500" {
		t.Errorf("unexpected row 1: %v", records[1])
	}
	if records[2][0] != "Opto1" || records[2][1] != "100.000" || records[2][2] != "5.000" {
		t.Errorf("unexpected row 2: %v", records[2])
	}
	if len(records[0]) != 3 {
		t.Errorf("three-column form expected, got header %v", records[0])
	}
}

// --- smoothing correction ---

func TestSmoothingMaskEnabled(t *testing.T) {
	mask := SmoothingMask{Mic1: true, Opto1: true, Opto2: false}

	for _, tc := range []struct {
		port string
		want bool
	}{
		{"Mic1", true},
		{"Opto1", true},
		{"Opto2", false},
		{"Mic2", false},
		// TTL and keypad lines are outside the mask entirely, so smoothing can
		// never apply to them however the struct is filled in.
		{"TTLin1", false},
		{"TTLin2", false},
		{"Keypad1", false},
		{"nonsense", false},
	} {
		if got := mask.Enabled(tc.port); got != tc.want {
			t.Errorf("Enabled(%q) = %v, want %v", tc.port, got, tc.want)
		}
	}
}

func TestCorrectedDuration(t *testing.T) {
	mask := SmoothingMask{Mic1: true, Opto1: true}

	for _, tc := range []struct {
		name     string
		duration float64
		port     string
		want     float64
	}{
		{"smoothed channel loses the tail", 220.25, "Mic1", 200.25},
		{"unsmoothed channel untouched", 5.25, "TTLin1", 5.25},
		{"channel off in the mask untouched", 220.25, "Opto2", 220.25},
		// Shorter than the smoothing window: clamped rather than negative. A
		// negative duration in a data file is worse than a zero, which at least
		// reads as "this event carried no usable duration".
		{"shorter than the offset clamps to zero", 12.0, "Opto1", 0},
		{"exactly the offset clamps to zero", 20.0, "Opto1", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := CorrectedDuration(tc.duration, tc.port, mask, DefaultSmoothingDurationOffsetMs)
			if got != tc.want {
				t.Errorf("CorrectedDuration(%v, %q) = %v, want %v", tc.duration, tc.port, got, tc.want)
			}
		})
	}
}

func TestSaveEventsToCSVWithCorrection(t *testing.T) {
	events := []Event{
		{Type: "Mic1", Onset: 20.0, Duration: 220.25},
		{Type: "TTLin1", Onset: 10.0, Duration: 5.25},
		{Type: "Opto2", Onset: 30.0, Duration: 221.5},
	}
	mask := SmoothingMask{Mic1: true, Opto1: true} // Opto2 deliberately off

	path := filepath.Join(t.TempDir(), "events.csv")
	if err := SaveEventsToCSVWithCorrection(events, path, mask, DefaultSmoothingDurationOffsetMs); err != nil {
		t.Fatalf("SaveEventsToCSVWithCorrection: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open output file: %v", err)
	}
	defer f.Close()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("failed to read CSV: %v", err)
	}

	want := [][]string{
		{"Type", "Onset", "Duration", "DurationCorrected"},
		// Sorted by onset, so TTLin1 leads. Its two duration columns agree
		// because no correction applies.
		{"TTLin1", "10.000", "5.250", "5.250"},
		{"Mic1", "20.000", "220.250", "200.250"},
		// Opto2 is off in this mask, so it keeps the recorded duration even
		// though the channel is smoothable.
		{"Opto2", "30.000", "221.500", "221.500"},
	}
	if len(records) != len(want) {
		t.Fatalf("expected %d rows, got %d: %v", len(want), len(records), records)
	}
	for i := range want {
		for j := range want[i] {
			if records[i][j] != want[i][j] {
				t.Errorf("row %d col %d = %q, want %q", i, j, records[i][j], want[i][j])
			}
		}
	}
}
