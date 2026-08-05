package bbtkv3

import (
	"strings"
	"testing"
)

func TestInputMask(t *testing.T) {
	// Bit position is the whole point: a mask built for the wrong index makes
	// the device answer a different sensor. InputPortNames runs
	// Keypad4..Keypad1, Opto4..Opto1, TTLin2, TTLin1, Mic2, Mic1, so TTLin1 is
	// the tenth of twelve.
	for _, tc := range []struct {
		names []string
		want  string
	}{
		{[]string{"TTLin1"}, "000000000100"},
		{[]string{"ttlin1"}, "000000000100"}, // case-insensitive
		{[]string{"Opto1"}, "000000010000"},
		{[]string{"Mic1"}, "000000000001"},
		{[]string{"Keypad4"}, "100000000000"},
		{[]string{"Opto1", "Mic1"}, "000000010001"},
		{nil, "000000000000"},
	} {
		got, err := InputMask(tc.names...)
		if err != nil {
			t.Fatalf("InputMask(%v): %v", tc.names, err)
		}
		if got != tc.want {
			t.Errorf("InputMask(%v) = %q, want %q", tc.names, got, tc.want)
		}
	}

	if _, err := InputMask("TTLout1"); err == nil {
		t.Error("InputMask(\"TTLout1\"): want error, an output port is not a trigger")
	}
}

func TestOutputMask(t *testing.T) {
	// OutputPortNames runs ActClose4..ActClose1, TTLout2, TTLout1, Sounder2,
	// Sounder1. TTLout1 — the line the Robotic Key Actuator hangs off — is the
	// sixth of eight.
	for _, tc := range []struct {
		names []string
		want  string
	}{
		{[]string{"TTLout1"}, "00000100"},
		{[]string{"TTLout2"}, "00001000"},
		{[]string{"TTLout1", "TTLout2"}, "00001100"},
		{[]string{"Sounder1"}, "00000001"},
	} {
		got, err := OutputMask(tc.names...)
		if err != nil {
			t.Fatalf("OutputMask(%v): %v", tc.names, err)
		}
		if got != tc.want {
			t.Errorf("OutputMask(%v) = %q, want %q", tc.names, got, tc.want)
		}
	}

	if _, err := OutputMask("Opto1"); err == nil {
		t.Error("OutputMask(\"Opto1\"): want error, an input port cannot be pulsed")
	}
}

func TestDSRERow(t *testing.T) {
	trigger, err := InputMask("TTLin1")
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := OutputMask("TTLout1")
	if err != nil {
		t.Fatal(err)
	}

	got, err := DSRERow([]string{trigger}, 200, outputs, 500)
	if err != nil {
		t.Fatalf("DSRERow: %v", err)
	}
	// trigger1,trigger2,trigger3,RT,portout,DURATION — the two unused trigger
	// slots must still be present, filled with 9s.
	want := "000000000100,999999999999,999999999999,200,00000100,500"
	if got != want {
		t.Errorf("DSRERow = %q, want %q", got, want)
	}

	if _, err := DSRERow(nil, 200, outputs, 500); err == nil {
		t.Error("DSRERow with no trigger: want error")
	}
	if _, err := DSRERow([]string{trigger, trigger, trigger, trigger}, 200, outputs, 500); err == nil {
		t.Error("DSRERow with four triggers: want error, a row holds three")
	}
	if _, err := DSRERow([]string{"0100"}, 200, outputs, 500); err == nil {
		t.Error("DSRERow with a short trigger mask: want error")
	}
	if _, err := DSRERow([]string{trigger}, 200, "001", 500); err == nil {
		t.Error("DSRERow with a short output mask: want error")
	}
	if _, err := DSRERow([]string{trigger}, 200, outputs, 0); err == nil {
		t.Error("DSRERow with zero duration: want error")
	}
	if _, err := DSRERow([]string{trigger}, -1, outputs, 500); err == nil {
		t.Error("DSRERow with negative delay: want error")
	}
}

func TestDSRESequence(t *testing.T) {
	row := "000000000100,999999999999,999999999999,200,00000100,500"

	seq, err := DSRESequence([]string{row}, 0)
	if err != nil {
		t.Fatalf("DSRESequence: %v", err)
	}

	// PDCR STYP PATT TIML <dur> + 8 pattern rows + PCCR RUSR.
	if len(seq) != 5+DSREPatternRows+2 {
		t.Fatalf("DSRESequence returned %d commands, want %d", len(seq), 5+DSREPatternRows+2)
	}
	for i, want := range []string{"PDCR", "STYP", "PATT", "TIML", "0", row} {
		if seq[i] != want {
			t.Errorf("seq[%d] = %q, want %q", i, seq[i], want)
		}
	}
	// The seven rows after the real one are padding, and RUSR is what starts it.
	for i := 6; i < 5+DSREPatternRows; i++ {
		if seq[i] != DSREPadRow {
			t.Errorf("seq[%d] = %q, want the pad row %q", i, seq[i], DSREPadRow)
		}
	}
	if seq[len(seq)-2] != "PCCR" || seq[len(seq)-1] != "RUSR" {
		t.Errorf("sequence ends with %v, want [PCCR RUSR]", seq[len(seq)-2:])
	}

	// Every pattern row must have the same field count, padding included, or
	// the device reads the payload out of step.
	for _, cmd := range seq[5 : 5+DSREPatternRows] {
		if n := len(strings.Split(cmd, ",")); n != 6 {
			t.Errorf("pattern row %q has %d fields, want 6", cmd, n)
		}
	}

	if _, err := DSRESequence(nil, 0); err == nil {
		t.Error("DSRESequence with no rows: want error")
	}
	rows := make([]string, DSREPatternRows+1)
	for i := range rows {
		rows[i] = row
	}
	if _, err := DSRESequence(rows, 0); err == nil {
		t.Errorf("DSRESequence with %d rows: want error, a PATT payload holds %d", len(rows), DSREPatternRows)
	}
}
