package bbtkv3

// Digital Stimulus Response Echo (DSRE).
//
// DSRE is the BBTK firmware mode that answers an event on an input line with a
// pulse on one or more output lines, after a programmable delay and for a
// programmable duration, repeating for as long as the program runs.  The whole
// trigger→delay→pulse cycle happens inside the device, so its timing does not
// depend on the host, the USB link, or the operating system's scheduler.
//
// The programming sequence, from section 8.1 of the BBTKv2 API Guide, is
//
//	PDCR / STYP / (PATT|INDI) / TIML / <time limit> / <one row> / PCCR / RUSR
//
// A TIML of 0 means "run until stopped".  The single row has the form
//
//	triggerA,triggerB,triggerC,RT,portout,DURATION
//
// where the three triggers are input masks, portout is an output mask, and RT
// and DURATION are milliseconds.  When a trigger fires, the lines set in
// portout go high RT ms later and stay high for DURATION ms.
//
// Two details cost a wasted session on the hardware if you get them wrong:
//
//   - DSRE takes EXACTLY ONE row.  The API Guide's summary is explicit — "Only
//     one trigger and one response can be defined" — and section 8.1 says the
//     key difference from DSCAR is "programming one Stimulus-Response pair
//     rather than multiple".  Do not pad the payload out to eight rows the way
//     event marking does; the extra rows are not ignored.
//
//   - Mask widths follow the MODEL, not the protocol.  An Entry or Pro box has
//     12 input and 8 output lines; an Elite has 20 and 16, the extra ones being
//     the TTLe expansion board.  The masks must be as wide as the model, which
//     is why EliteWidths exists and why the event-marking rows elsewhere in
//     this repo are 20 and 16 bits wide.
//
// This is the mode that drives the Robotic Key Actuator: the RKA solenoid is
// wired to TTL Out 1 through the 3.5 mm lead on the TTL/ASC extension port, so
// "press the key" is simply "raise TTLout1".

import (
	"fmt"
	"strings"
	"time"
)

// UnusedTriggerBit is the digit that fills a trigger slot that is not used; a
// row must always carry three trigger fields.
const unusedTriggerBit = '9'

// PortWidths is the number of input and output lines a model exposes, and so
// the width of the masks its firmware expects.
type PortWidths struct {
	Inputs  int
	Outputs int
}

// The two models' line counts. StandardWidths covers the Entry and Pro boxes,
// which is what the API Guide's examples show. EliteWidths adds the TTLe
// expansion board's lines: the standard lines keep their bit positions and the
// expansion lines occupy the new high-numbered ones, so an Elite mask is a
// standard mask with trailing zeros — which is exactly the shape of the
// event-marking rows in this repo (20 bits and 16 bits, "all last 8 set to 0").
var (
	StandardWidths = PortWidths{Inputs: 12, Outputs: 8}
	EliteWidths    = PortWidths{Inputs: 20, Outputs: 16}
)

// WidthsForModel maps a model name to its widths. The names are what the -model
// flag of bbtk-trigger-response accepts.
func WidthsForModel(name string) (PortWidths, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "elite":
		return EliteWidths, nil
	case "standard", "pro", "entry":
		return StandardWidths, nil
	}
	return PortWidths{}, fmt.Errorf("unknown model %q (expected elite, standard, pro or entry)", name)
}

// UnusedTrigger returns the all-9s placeholder for an unused trigger slot, at
// the given width.
func UnusedTrigger(width int) string {
	return strings.Repeat(string(unusedTriggerBit), width)
}

// InputMask returns a trigger mask of the given width with the named input
// ports set to 1.  Names are matched case-insensitively.  width must be at
// least len(InputPortNames); the extra bits are the expansion-board lines,
// which this package cannot name and therefore leaves at 0.
func InputMask(width int, names ...string) (string, error) {
	return portMask(InputPortNames, width, names)
}

// OutputMask returns an output mask of the given width with the named output
// ports set to 1.  Names are matched case-insensitively.
func OutputMask(width int, names ...string) (string, error) {
	return portMask(OutputPortNames, width, names)
}

func portMask(ports []string, width int, names []string) (string, error) {
	if width < len(ports) {
		return "", fmt.Errorf("mask width %d is narrower than the %d named ports", width, len(ports))
	}
	bits := make([]byte, width)
	for i := range bits {
		bits[i] = '0'
	}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		found := false
		for i, p := range ports {
			if strings.EqualFold(p, name) {
				bits[i] = '1'
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("unknown port %q (expected one of %s)",
				name, strings.Join(ports, ", "))
		}
	}
	return string(bits), nil
}

// DSRERow builds the single PATT row.  triggers holds between one and three
// input masks (as returned by InputMask); the unused slots are filled with
// UnusedTrigger.  rtMs is the delay between the trigger and the leading edge of
// the pulse, durationMs the length of the pulse, both in milliseconds.
func DSRERow(triggers []string, rtMs int, outputs string, durationMs int, w PortWidths) (string, error) {
	if len(triggers) == 0 || len(triggers) > 3 {
		return "", fmt.Errorf("DSRERow: need 1 to 3 triggers, got %d", len(triggers))
	}
	for _, t := range triggers {
		if len(t) != w.Inputs {
			return "", fmt.Errorf("DSRERow: trigger mask %q is %d bits, want %d",
				t, len(t), w.Inputs)
		}
	}
	if len(outputs) != w.Outputs {
		return "", fmt.Errorf("DSRERow: output mask %q is %d bits, want %d",
			outputs, len(outputs), w.Outputs)
	}
	if rtMs < 0 {
		return "", fmt.Errorf("DSRERow: negative delay %d ms", rtMs)
	}
	if durationMs <= 0 {
		return "", fmt.Errorf("DSRERow: duration must be positive, got %d ms", durationMs)
	}

	three := make([]string, 3)
	for i := range three {
		if i < len(triggers) {
			three[i] = triggers[i]
		} else {
			three[i] = UnusedTrigger(w.Inputs)
		}
	}

	return fmt.Sprintf("%s,%d,%s,%d", strings.Join(three, ","), rtMs, outputs, durationMs), nil
}

// DSREMatch selects how the firmware compares the live input state with the
// trigger masks.
type DSREMatch string

const (
	// MatchPattern (STYP PATT) fires only on an exact match of the whole input
	// port: every line set in the mask high AND every other line low.  Activity
	// anywhere else on the port suppresses the response.
	MatchPattern DSREMatch = "PATT"

	// MatchIndividual (STYP INDI) fires when any line set in the mask is active,
	// whatever the other lines are doing.  This is the API Guide's "respond to
	// individual lines within any stimulus pattern", and it is the setting to
	// use when you are not certain the rest of the port is quiet.
	MatchIndividual DSREMatch = "INDI"
)

// DSRESequence returns the exact list of commands DSREProgram would send,
// without touching the device.  It is what the -n flag of
// bbtk-trigger-response prints.
//
// timeLimitUs is the TIML value in microseconds; 0 means "run until stopped".
func DSRESequence(row string, match DSREMatch, timeLimitUs int) ([]string, error) {
	if row == "" {
		return nil, fmt.Errorf("DSRESequence: empty pattern row")
	}
	if match != MatchPattern && match != MatchIndividual {
		return nil, fmt.Errorf("DSRESequence: match must be %s or %s, got %q",
			MatchPattern, MatchIndividual, match)
	}
	if strings.ContainsAny(row, " \t") {
		// The API Guide is explicit: "there should be no spaces after the
		// commas". A row that looks right but carries a space is rejected by
		// the device with no clue as to why.
		return nil, fmt.Errorf("DSRESequence: pattern row %q contains whitespace", row)
	}

	return []string{
		"PDCR",
		"STYP",
		string(match),
		"TIML",
		fmt.Sprint(timeLimitUs),
		row,
		"PCCR",
		"RUSR",
	}, nil
}

// DSREProgram uploads a DSRE program and starts it, then blocks until the user
// presses Esc (or Ctrl-C), at which point it sends the break character.
//
// Every reply the device sends is echoed to ProgressWriter, prefixed with the
// command that provoked it.  PCCR in particular answers with the sequence it
// understood, which is the only way to tell a program the box accepted from one
// it silently discarded — so read that line before concluding the hardware is
// at fault.
//
// The pacing matches EventMarking: one second per command, and an extra 500 ms
// before RUSR.
func (b *bbtkv3) DSREProgram(row string, match DSREMatch, timeLimitUs int) error {
	seq, err := DSRESequence(row, match, timeLimitUs)
	if err != nil {
		return err
	}

	// RUSR is the last element and needs the extra pause CaptureEvents gives
	// RUDS, so send it separately.
	body, run := seq[:len(seq)-1], seq[len(seq)-1]

	for _, cmd := range body {
		time.Sleep(time.Second)
		if err := b.SendCommand(cmd); err != nil {
			return fmt.Errorf("DSREProgram: %q: %w", cmd, err)
		}
		b.reportReplies(cmd)
	}

	time.Sleep(time.Second)
	time.Sleep(500 * time.Millisecond)
	if err := b.SendCommand(run); err != nil {
		return fmt.Errorf("DSREProgram: %s: %w", run, err)
	}
	b.reportReplies(run)

	eol := waitForStopKey("Trigger-response program running.")
	fmt.Printf("%sStopping...%s", eol, eol)

	if err := b.SendBreakChar(); err != nil {
		return fmt.Errorf("DSREProgram: SendBreakChar: %w", err)
	}

	return nil
}

// reportReplies prints whatever the device has to say about cmd. It stops at
// the first read timeout, so a command that answers nothing costs one timeout
// (a second) and prints nothing.
func (b *bbtkv3) reportReplies(cmd string) {
	for {
		line, err := b.ReadLine()
		if err != nil {
			return
		}
		if line = strings.TrimSpace(line); line != "" {
			fmt.Fprintf(ProgressWriter(), "  %s → %s\n", cmd, line)
		}
	}
}
