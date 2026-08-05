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
		width int
		names []string
		want  string
	}{
		{12, []string{"TTLin1"}, "000000000100"},
		{12, []string{"TTLin2"}, "000000001000"},
		{12, []string{"ttlin1"}, "000000000100"}, // case-insensitive
		{12, []string{"Opto1"}, "000000010000"},
		{12, []string{"Mic1"}, "000000000001"},
		{12, []string{"Keypad4"}, "100000000000"},
		{12, []string{"Opto1", "Mic1"}, "000000010001"},
		{12, nil, "000000000000"},
		// On an Elite the standard lines keep their bit positions and the
		// TTLe expansion lines occupy the new ones, so the mask is the
		// standard mask with trailing zeros.
		{20, []string{"TTLin2"}, "00000000100000000000"},
		{20, []string{"Opto1"}, "00000001000000000000"},
	} {
		got, err := InputMask(tc.width, tc.names...)
		if err != nil {
			t.Fatalf("InputMask(%d, %v): %v", tc.width, tc.names, err)
		}
		if got != tc.want {
			t.Errorf("InputMask(%d, %v) = %q, want %q", tc.width, tc.names, got, tc.want)
		}
	}

	if _, err := InputMask(12, "TTLout1"); err == nil {
		t.Error("InputMask(12, \"TTLout1\"): want error, an output port is not a trigger")
	}
	if _, err := InputMask(8, "TTLin1"); err == nil {
		t.Error("InputMask(8, …): want error, 8 bits cannot hold 12 input lines")
	}
}

func TestOutputMask(t *testing.T) {
	// OutputPortNames runs ActClose4..ActClose1, TTLout2, TTLout1, Sounder2,
	// Sounder1. TTLout1 — the line the Robotic Key Actuator hangs off — is the
	// sixth of eight.
	for _, tc := range []struct {
		width int
		names []string
		want  string
	}{
		{8, []string{"TTLout1"}, "00000100"},
		{8, []string{"TTLout2"}, "00001000"},
		{8, []string{"TTLout1", "TTLout2"}, "00001100"},
		{8, []string{"Sounder1"}, "00000001"},
		{16, []string{"TTLout1"}, "0000010000000000"},
	} {
		got, err := OutputMask(tc.width, tc.names...)
		if err != nil {
			t.Fatalf("OutputMask(%d, %v): %v", tc.width, tc.names, err)
		}
		if got != tc.want {
			t.Errorf("OutputMask(%d, %v) = %q, want %q", tc.width, tc.names, got, tc.want)
		}
	}

	if _, err := OutputMask(8, "Opto1"); err == nil {
		t.Error("OutputMask(8, \"Opto1\"): want error, an input port cannot be pulsed")
	}
}

func TestWidthsForModel(t *testing.T) {
	for name, want := range map[string]PortWidths{
		"elite":    EliteWidths,
		"Elite":    EliteWidths,
		"standard": StandardWidths,
		"pro":      StandardWidths,
		"entry":    StandardWidths,
	} {
		got, err := WidthsForModel(name)
		if err != nil {
			t.Fatalf("WidthsForModel(%q): %v", name, err)
		}
		if got != want {
			t.Errorf("WidthsForModel(%q) = %+v, want %+v", name, got, want)
		}
	}
	if _, err := WidthsForModel("deluxe"); err == nil {
		t.Error("WidthsForModel(\"deluxe\"): want error")
	}
}

func TestDSRERow(t *testing.T) {
	// The API Guide's own worked example, section 8.1: Opto1 triggers TTLout1
	// with a 300 ms RT and a 100 ms duration on a standard box.
	trigger, err := InputMask(StandardWidths.Inputs, "Opto1")
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := OutputMask(StandardWidths.Outputs, "TTLout1")
	if err != nil {
		t.Fatal(err)
	}

	got, err := DSRERow([]string{trigger}, 300, outputs, 100, StandardWidths)
	if err != nil {
		t.Fatalf("DSRERow: %v", err)
	}
	want := "000000010000,999999999999,999999999999,300,00000100,100"
	if got != want {
		t.Errorf("DSRERow = %q, want the API Guide's example %q", got, want)
	}

	// The same thing on an Elite: wider masks, wider 9s.
	eTrigger, err := InputMask(EliteWidths.Inputs, "Opto1")
	if err != nil {
		t.Fatal(err)
	}
	eOutputs, err := OutputMask(EliteWidths.Outputs, "TTLout1")
	if err != nil {
		t.Fatal(err)
	}
	got, err = DSRERow([]string{eTrigger}, 300, eOutputs, 100, EliteWidths)
	if err != nil {
		t.Fatalf("DSRERow (elite): %v", err)
	}
	want = "00000001000000000000,99999999999999999999,99999999999999999999,300,0000010000000000,100"
	if got != want {
		t.Errorf("DSRERow (elite) = %q, want %q", got, want)
	}

	// Mixing widths is the mistake that produces a row the device quietly
	// refuses, so each width is checked against the model.
	if _, err := DSRERow([]string{trigger}, 300, outputs, 100, EliteWidths); err == nil {
		t.Error("DSRERow with standard masks and Elite widths: want error")
	}
	if _, err := DSRERow(nil, 200, outputs, 500, StandardWidths); err == nil {
		t.Error("DSRERow with no trigger: want error")
	}
	if _, err := DSRERow([]string{trigger, trigger, trigger, trigger}, 200, outputs, 500, StandardWidths); err == nil {
		t.Error("DSRERow with four triggers: want error, a row holds three")
	}
	if _, err := DSRERow([]string{trigger}, 200, outputs, 0, StandardWidths); err == nil {
		t.Error("DSRERow with zero duration: want error")
	}
	if _, err := DSRERow([]string{trigger}, -1, outputs, 500, StandardWidths); err == nil {
		t.Error("DSRERow with negative delay: want error")
	}
}

func TestDSRESequence(t *testing.T) {
	row := "000000010000,999999999999,999999999999,300,00000100,100"

	seq, err := DSRESequence(row, MatchPattern, 0)
	if err != nil {
		t.Fatalf("DSRESequence: %v", err)
	}

	// Section 8.1 of the API Guide, exactly: one row, no padding. The earlier
	// version of this code padded to eight rows by analogy with event marking,
	// and the device ignored the program.
	want := []string{"PDCR", "STYP", "PATT", "TIML", "0", row, "PCCR", "RUSR"}
	if len(seq) != len(want) {
		t.Fatalf("DSRESequence returned %d commands (%v), want %d", len(seq), seq, len(want))
	}
	for i := range want {
		if seq[i] != want[i] {
			t.Errorf("seq[%d] = %q, want %q", i, seq[i], want[i])
		}
	}

	// INDI takes the place of PATT and nothing else moves.
	seq, err = DSRESequence(row, MatchIndividual, 0)
	if err != nil {
		t.Fatalf("DSRESequence(INDI): %v", err)
	}
	if seq[2] != "INDI" {
		t.Errorf("seq[2] = %q, want INDI", seq[2])
	}

	if _, err := DSRESequence("", MatchPattern, 0); err == nil {
		t.Error("DSRESequence with an empty row: want error")
	}
	if _, err := DSRESequence(row, "NOPE", 0); err == nil {
		t.Error("DSRESequence with an unknown match mode: want error")
	}
	// "Note there should be no spaces after the commas (,)."
	if _, err := DSRESequence(strings.Replace(row, ",", ", ", 1), MatchPattern, 0); err == nil {
		t.Error("DSRESequence with a space after a comma: want error")
	}
}
