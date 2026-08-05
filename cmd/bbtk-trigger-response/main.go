// bbtk-trigger-response: answer an input event with a delayed output pulse,
// in a loop, until stopped.
//
// The BBTK waits for an event on the trigger input line(s), waits -rt
// milliseconds, then raises the output line(s) for -d milliseconds, lowers
// them again, and goes back to waiting. The loop runs in the device's firmware
// (Digital Stimulus Response Echo, DSRE), so the host plays no part in the
// timing once the program is running.
//
// The defaults are the Robotic Key Actuator case: the RKA solenoid is wired to
// TTL Out 1 via the 3.5 mm lead on the TTL/ASC extension port, so a pulse on
// TTLout1 is a key press. The trigger defaults to TTLin2 rather than TTLin1
// because TTL In 1 is where the Breakout Board's calibration button sits, and
// the RKA guide says to leave both TTL In 1 and TTL Out 1 unwired while the
// actuator is connected.
//
//	bbtk-trigger-response                      # TTLin2 -> 200 ms -> TTLout1 for 500 ms
//	bbtk-trigger-response -n                   # print the command sequence, touch nothing
//	bbtk-trigger-response -any                 # ignore activity on other input lines
//	bbtk-trigger-response -model standard      # Entry/Pro box: 12/8 lines instead of 20/16
//
// Usage:
//
//	bbtk-trigger-response [-p <port>] [-b <baudrate>] [-i <inputs>] [-o <outputs>]
//	                      [-rt <ms>] [-d <ms>] [-model <name>] [-any] [-n]
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/chrplr/bbtkv3"
)

var (
	Version string
	Build   string
)

// Ranges the Robotic Key Actuator Guide documents for the RKA calibration
// routine. Outside them the RKA is not characterised, but DSRE itself is
// general, so these only produce a warning.
const (
	rkaMinRTms       = 50
	rkaMaxRTms       = 10000
	rkaMinDurationMs = 50
	rkaMaxDurationMs = 1000
)

func main() {
	portPtr := flag.String("p", "", "serial port (or set BBTK_PORT)")
	baudPtr := flag.Int("b", 115200, "baudrate")
	inPtr := flag.String("i", "TTLin2", "comma-separated trigger input port(s): "+strings.Join(bbtkv3.InputPortNames, ", "))
	outPtr := flag.String("o", "TTLout1", "comma-separated output port(s) to pulse: "+strings.Join(bbtkv3.OutputPortNames, ", "))
	rtPtr := flag.Int("rt", 200, "delay from trigger to pulse onset, in ms")
	durPtr := flag.Int("d", 500, "pulse duration, in ms")
	modelPtr := flag.String("model", "elite", "BBTK model, which sets the mask widths: elite (20/16 lines) or standard (12/8)")
	anyPtr := flag.Bool("any", false, "respond to the trigger line(s) whatever the other input lines are doing (STYP INDI); the default requires an exact match of the whole input port (STYP PATT)")
	dryPtr := flag.Bool("n", false, "dry run: print the command sequence and exit without opening the port")
	versionPtr := flag.Bool("V", false, "display version and exit")
	flag.Parse()

	if *versionPtr {
		fmt.Println(bbtkv3.VersionString(Version, Build))
		os.Exit(0)
	}

	widths, err := bbtkv3.WidthsForModel(*modelPtr)
	if err != nil {
		log.Fatalf("-model: %v", err)
	}

	trigger, err := bbtkv3.InputMask(widths.Inputs, strings.Split(*inPtr, ",")...)
	if err != nil {
		log.Fatalf("-i: %v", err)
	}
	if !strings.Contains(trigger, "1") {
		log.Fatal("-i: at least one trigger input port is required")
	}

	outputs, err := bbtkv3.OutputMask(widths.Outputs, strings.Split(*outPtr, ",")...)
	if err != nil {
		log.Fatalf("-o: %v", err)
	}
	if !strings.Contains(outputs, "1") {
		log.Fatal("-o: at least one output port is required")
	}

	match := bbtkv3.MatchPattern
	if *anyPtr {
		match = bbtkv3.MatchIndividual
	}

	row, err := bbtkv3.DSRERow([]string{trigger}, *rtPtr, outputs, *durPtr, widths)
	if err != nil {
		log.Fatal(err)
	}

	// 0 means "run until stopped", which is what makes this a loop.
	seq, err := bbtkv3.DSRESequence(row, match, 0)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Model:   %s (%d input lines, %d output lines)\n", strings.ToLower(*modelPtr), widths.Inputs, widths.Outputs)
	fmt.Printf("Trigger: %s (%s)\n", *inPtr, trigger)
	fmt.Printf("Output:  %s (%s)\n", *outPtr, outputs)
	fmt.Printf("Pulse:   %d ms after the trigger, for %d ms, repeating\n", *rtPtr, *durPtr)
	fmt.Printf("Match:   %s\n", matchDescription(match))

	// TTL In 1 carries the Breakout Board's calibration button and TTL Out 1
	// carries the solenoid, so the RKA guide says to leave both unwired while
	// the actuator is connected. Triggering off TTL In 1 therefore means either
	// no RKA or a disconnected calibration button — legitimate, but worth
	// saying out loud, since DSRE itself has no idea what is plugged in.
	if strings.Contains(strings.ToLower(*inPtr), "ttlin1") {
		fmt.Fprintln(os.Stderr, "warning: TTL In 1 is the RKA calibration button line; the RKA guide advises against wiring it while the actuator is connected")
	}

	if *rtPtr < rkaMinRTms || *rtPtr > rkaMaxRTms {
		fmt.Fprintf(os.Stderr, "warning: -rt %d ms is outside the %d–%d ms range the RKA guide documents\n",
			*rtPtr, rkaMinRTms, rkaMaxRTms)
	}
	if *durPtr < rkaMinDurationMs || *durPtr > rkaMaxDurationMs {
		fmt.Fprintf(os.Stderr, "warning: -d %d ms is outside the %d–%d ms range the RKA guide documents\n",
			*durPtr, rkaMinDurationMs, rkaMaxDurationMs)
	}
	// The RKA is a solenoid: re-triggering it before the plunger has recoiled
	// gives meaningless timings and stresses the mechanism. The guide's own
	// wording is "do not try to trigger while a response is in progress".
	fmt.Fprintf(os.Stderr, "note: leave at least %d ms between triggers — one cycle occupies the actuator that long.\n",
		*rtPtr+*durPtr)

	if *dryPtr {
		fmt.Println("\nCommand sequence (dry run, nothing sent):")
		for _, cmd := range seq {
			fmt.Printf("  %s\n", cmd)
		}
		return
	}

	port := *portPtr
	if port == "" {
		port = bbtkv3.ResolvePort()
	}
	if port == "" {
		log.Fatal("no serial port specified: use -p <port> or set BBTK_PORT")
	}

	b, err := bbtkv3.NewBbtkv3(port, *baudPtr, false)
	if err != nil {
		log.Fatal(err)
	}
	defer b.Disconnect()

	if err := b.Connect(); err != nil {
		log.Fatalf("connect: %v", err)
	}

	fmt.Println("\nProgramming the device; its replies follow.")
	if err := b.DSREProgram(row, match, 0); err != nil {
		log.Fatalf("trigger-response: %v", err)
	}
}

func matchDescription(m bbtkv3.DSREMatch) string {
	if m == bbtkv3.MatchIndividual {
		return "INDI — any activity on the trigger line(s), whatever else is going on"
	}
	return "PATT — exact match of the whole input port (use -any if other lines may be active)"
}
