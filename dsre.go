package bbtkv3

// Digital Stimulus Response Echo (DSRE).
//
// DSRE is the BBTK firmware mode that answers an event on an input line with a
// pulse on one or more output lines, after a programmable delay and for a
// programmable duration, repeating for as long as the program runs.  The whole
// trigger→delay→pulse cycle happens inside the device, so its timing does not
// depend on the host, the USB link, or the operating system's scheduler.
//
// The programming sequence is
//
//	PDCR / STYP / PATT / TIML / <duration> / <8 pattern rows> / PCCR / RUSR
//
// mirroring the event-marking sequence in EventMarking, and a TIML of 0 means
// "run until interrupted".  Each pattern row has the form
//
//	trigger1,trigger2,trigger3,RT,portout,DURATION
//
// where the three triggers are 12-bit input masks in InputPortNames order,
// portout is an 8-bit output mask in OutputPortNames order, and RT and DURATION
// are milliseconds.  When any of the three triggers fires, the lines set in
// portout go high RT ms later and stay high for DURATION ms.
//
// This is the mode that drives the Robotic Key Actuator: the RKA solenoid is
// wired to TTL Out 1 through the 3.5 mm lead on the TTL/ASC extension port, so
// "press the key" is simply "raise TTLout1".

import (
	"fmt"
	"strings"
	"time"
)

// DSREUnusedTrigger is the all-9s placeholder for a trigger slot that is not
// used.  A row must always carry three trigger fields.
const DSREUnusedTrigger = "999999999999"

// DSREPatternRows is the number of pattern rows the PATT payload must contain,
// as in event marking.  Rows beyond the ones you need are padded by
// DSREProgram with DSREPadRow.
const DSREPatternRows = 8

// DSREPadRow fills the unused rows of the PATT payload.
const DSREPadRow = "999999999999,999999999999,999999999999,9,99999999,9"

// InputMask returns the 12-bit trigger mask, in InputPortNames order, with the
// named input ports set to 1.  Names are matched case-insensitively.
func InputMask(names ...string) (string, error) {
	return portMask(InputPortNames, names)
}

// OutputMask returns the 8-bit output mask, in OutputPortNames order, with the
// named output ports set to 1.  Names are matched case-insensitively.
func OutputMask(names ...string) (string, error) {
	return portMask(OutputPortNames, names)
}

func portMask(ports []string, names []string) (string, error) {
	bits := make([]byte, len(ports))
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

// DSRERow builds one PATT row.  triggers holds between one and three 12-bit
// input masks (as returned by InputMask); the unused slots are filled with
// DSREUnusedTrigger.  rtMs is the delay between the trigger and the leading
// edge of the pulse, durationMs the length of the pulse, both in milliseconds.
func DSRERow(triggers []string, rtMs int, outputs string, durationMs int) (string, error) {
	if len(triggers) == 0 || len(triggers) > 3 {
		return "", fmt.Errorf("DSRERow: need 1 to 3 triggers, got %d", len(triggers))
	}
	for _, t := range triggers {
		if len(t) != len(InputPortNames) {
			return "", fmt.Errorf("DSRERow: trigger mask %q is %d bits, want %d",
				t, len(t), len(InputPortNames))
		}
	}
	if len(outputs) != len(OutputPortNames) {
		return "", fmt.Errorf("DSRERow: output mask %q is %d bits, want %d",
			outputs, len(outputs), len(OutputPortNames))
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
			three[i] = DSREUnusedTrigger
		}
	}

	return fmt.Sprintf("%s,%d,%s,%d", strings.Join(three, ","), rtMs, outputs, durationMs), nil
}

// DSRESequence returns the exact list of commands DSREProgram would send for
// the given rows, without touching the device.  It is what the -n flag of
// bbtk-trigger-response prints, and it is the thing to check against the BBTK
// API Guide before running anything that moves a solenoid.
//
// durationUs is the TIML value in microseconds; 0 means "run until interrupted".
func DSRESequence(rows []string, durationUs int) ([]string, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("DSRESequence: no pattern rows")
	}
	if len(rows) > DSREPatternRows {
		return nil, fmt.Errorf("DSRESequence: %d pattern rows, at most %d fit in a PATT payload",
			len(rows), DSREPatternRows)
	}

	seq := []string{"PDCR", "STYP", "PATT", "TIML", fmt.Sprint(durationUs)}
	seq = append(seq, rows...)
	for i := len(rows); i < DSREPatternRows; i++ {
		seq = append(seq, DSREPadRow)
	}
	return append(seq, "PCCR", "RUSR"), nil
}

// DSREProgram uploads a DSRE program and starts it, then blocks until the user
// presses Esc (or Ctrl-C), at which point it sends the break character to stop
// the device.
//
// The pacing matches EventMarking: one second before every command, and an
// extra 500 ms before the RUSR that starts the program, so the device has time
// to digest each step.
func (b *bbtkv3) DSREProgram(rows []string, durationUs int) error {
	seq, err := DSRESequence(rows, durationUs)
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
	}

	time.Sleep(time.Second)
	time.Sleep(500 * time.Millisecond)
	if err := b.SendCommand(run); err != nil {
		return fmt.Errorf("DSREProgram: %s: %w", run, err)
	}

	eol := waitForStopKey("Trigger-response program running.")
	fmt.Printf("%sStopping...%s", eol, eol)

	if err := b.SendBreakChar(); err != nil {
		return fmt.Errorf("DSREProgram: SendBreakChar: %w", err)
	}

	return nil
}
