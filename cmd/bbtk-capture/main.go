// Drive a BlackBoxToolKit to capture events
// Author: Christophe Pallier <christophe@pallier.org>
// LICENSE: GPL-3.0

// Package main provides a command-line tool to capture events using the BlackBoxToolKit (bbtkv3).
// It allows setting various parameters such as port address, baud rate, capture duration, and output base filename.
// The tool also supports a debug mode and displays version information if requested.
//
// The main functionality includes initializing the bbtkv3 device, setting parameters, clearing internal memory,
// capturing events, and saving the captured data to files in both raw and CSV formats.
//
// Usage:
//   bbtk-capture [options] <basefilename> [-- command [args...]]
//
//   -p string
//         device (serial port name) (default "/dev/ttyUSB0")
//   -b int
//         baudrate (speed in bps) (default 115200)
//   -d int
//         duration of capture (in s) (default 30)
//   -D
//         Debug mode (default false)
//   -V
//         Display version
//
// Output files are named <basefilename>-001.dat, <basefilename>-001-dscevents.csv, etc.
// The sequence number is incremented automatically to avoid overwriting previous recordings.
//
// Everything after "--" is run as a child process, started at the instant the
// device begins recording. This removes the need for a wrapper script to launch
// bbtk-capture in the background and watch its output for the readiness marker:
//
//   bbtk-capture -d 516 session/av -- ./Timing-Tests -test av -cycles 1000
//
// In that mode bbtk-capture's own progress moves to stderr so stdout carries
// only the child's output, the countdown and the Esc/Ctrl-C handler are turned
// off (the terminal belongs to the child), and the child's exit status decides
// what happens to the recording — see runChild below.

// TODO: implement adjustable thresholds, reading the thresholds form the command line or from a configuration file
// TODO: better handle errors
// TODO: The way I handle DEBUG is a disaster, implement verbose and debug with 2 level logs.
//        THe module bbtkv3 uses the env at

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chrplr/bbtkv3"
)

// Variables to be passed on the compilation command line with "-X main.Version=${VERSION} -X main.Build=${BUILD}"
var (
	Version string
	Build   string
)

var (
	PortAddress = "/dev/ttyUSB0"
	Baudrate    = 115200
	Duration    = 30
	DEBUG       = false
)

var defaultSmoothingMask = bbtkv3.SmoothingMask{
	Mic1:  true,
	Mic2:  true,
	Opto4: false,
	Opto3: false,
	Opto2: true,
	Opto1: true,
}

func main() {

	portPtr := flag.String("p", "", "device (serial port name); overrides BBTK_PORT (default \""+PortAddress+"\")")
	speedPtr := flag.Int("b", Baudrate, "baudrate (speed in bps)")
	durationPtr := flag.Int("d", Duration, "duration of capture (in s)")
	debugPtr := flag.Bool("D", DEBUG, "Debug mode")
	versionPtr := flag.Bool("V", false, "Display version")
	noCountdownPtr := flag.Bool("no-countdown", false, "Disable second-by-second countdown display")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <basefilename> [-- command [args...]]\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nOutput files: <basefilename>-001.dat, <basefilename>-001-dscevents.csv, <basefilename>-001-events.csv\nSequence number is incremented automatically to avoid overwriting previous recordings.\n")
		fmt.Fprintf(os.Stderr, "\nAnything after -- is run as a child process, started the instant the device\nbegins recording. Progress then moves to stderr so stdout carries only the\nchild's output. A child that exits non-zero aborts the capture.\n")
	}

	// Split off the child command before parsing, so its flags are never taken
	// for ours. See splitArgv in child.go.
	hostArgv, childArgv := splitArgv(os.Args)
	os.Args = hostArgv
	flag.Parse()

	// With a child to run, stdout belongs to it alone: our own progress goes to
	// stderr, and the countdown and Esc handler are switched off because the
	// terminal is the child's (raw mode would take away its Ctrl-C).
	out := io.Writer(os.Stdout)
	if len(childArgv) > 0 {
		out = os.Stderr
		// The library prints its own connection progress; send that to the same
		// place, or "Trying to open /dev/... ok!" would still land on the
		// child's stdout.
		bbtkv3.SetProgressWriter(out)
	}

	if *versionPtr {
		fmt.Println(bbtkv3.VersionString(Version, Build))
		// Advertise the synchronisation marker, so a wrapper script can check
		// for handshake support without touching the device. A binary built
		// before the marker existed simply never prints it during a capture,
		// which strands the caller waiting for a line that will never come.
		fmt.Printf("ready-marker: %s\n", bbtkv3.ReadyMarker)
		os.Exit(0)
	}

	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Error: a basefilename argument is required.\n\n")
		flag.Usage()
		os.Exit(1)
	}
	baseFilename := flag.Arg(0)

	DEBUG = *debugPtr

	// Port resolution, highest precedence first: -p, then BBTK_PORT, then the
	// built-in default.
	serPort := *portPtr
	if serPort == "" {
		serPort = bbtkv3.ResolvePort()
	}
	if serPort == "" {
		serPort = PortAddress
	}

	// Initialisation
	verbose := true
	b, err := bbtkv3.NewBbtkv3(serPort, *speedPtr, verbose)
	if err != nil {
		log.Fatalln(err)
	}
	defer b.Disconnect()

	time.Sleep(time.Second)

	err = b.ResetSerialBuffers()
	if err != nil {
		log.Printf("ResetSerialIOBuff %v\n", err)
	}

	// HandShaking
	if err = b.Connect(); err != nil {
		log.Fatalf("Connect returned: %v\n", err)
	}
	time.Sleep(time.Second)

	err = b.ResetSerialBuffers()
	if err != nil {
		log.Printf("ResetSerialIOBuff %v\n", err)
	}

	var alive bool
	if alive, err = b.IsAlive(); err != nil {
		log.Println(err)
	} else {
		if alive {
			fmt.Fprintln(out, "bbtkv3 is alive")
		} else {
			fmt.Fprintln(out, "bbtkv3 not responding to ECHO")
		}
	}
	time.Sleep(time.Second)

	// Parameters setting
	fmt.Fprintf(out, "Setting Smoothing mask to %+v\n", defaultSmoothingMask)
	if err = b.SetSmoothing(defaultSmoothingMask); err != nil {
		log.Printf("%v", err)
	}
	time.Sleep(time.Second)

	fmt.Fprintln(out, "Getting thresholds...")
	thresholds, err := b.GetThresholds()
	if err != nil {
		log.Printf("GetThresholds: %v\n", err)
	} else {
		fmt.Fprintf(out, "%+v\n", thresholds)
	}

	// Clearing internal memory
	time.Sleep(time.Second)
	fmt.Fprintf(out, "Clearing Timing data... ")
	if err := b.ClearTimingData(); err != nil {
		log.Fatalf("ClearTimingData: %v\n", err)
	}
	fmt.Fprintln(out, "Ok")

	// Stop cleanly on Ctrl-C or SIGTERM instead of dying with the recording
	// still in the device's RAM and nothing written to disk. The capture is
	// cut short, the device is asked for whatever it has, and the files below
	// are written from that — a truncated recording beats no recording at all.
	//
	// Only the FIRST signal is handled; the goroutine then returns and
	// signal.Notify's registration no longer has a reader, so a second Ctrl-C
	// force-quits in the usual way if the recovery read is taking too long.
	abort := make(chan struct{}, 1)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		select {
		case abort <- struct{}{}:
		default:
		}
	}()

	// Data Capture
	time.Sleep(1 * time.Second)
	fmt.Fprintf(out, "Capturing events (with DSCM) for %v seconds... ", *durationPtr)

	opts := bbtkv3.CaptureOptions{
		NoCountdown: *noCountdownPtr,
		Abort:       abort,
		Progress:    out,
	}
	var stim *child
	if len(childArgv) > 0 {
		stim = newChild(childArgv, out, abort)
		// Started from inside CaptureEvents, at the instant RUDS is
		// acknowledged, which is the whole point: the setup before it takes a
		// variable 11-40 s that no caller can predict.
		opts.OnRecording = stim.start
		// The countdown would interleave with the child's output, and raw mode
		// would take the child's Ctrl-C away and staircase its lines.
		opts.NoCountdown = true
		opts.NoKeyAbort = true
	}

	data, elapsedS, err := b.CaptureEvents(*durationPtr, opts)
	if errors.Is(err, bbtkv3.ErrCaptureAborted) {
		fmt.Fprintln(out, "Capture stopped early and the device returned no data.")
		fmt.Fprintln(out, "The recording is still in the BBTK's RAM; it will be cleared by the next capture.")
		if stim != nil {
			// Reap it either way: an abort triggered by something else (a signal,
			// Esc) leaves the stimulus running with no window left to record it.
			stim.finish()
			os.Exit(stim.exitCode())
		}
		os.Exit(1)
	}
	if err != nil {
		log.Fatalf("CaptureEvents: %v\n", err)
	}

	// The window has closed and the data is valid. A stimulus still running at
	// this point outran the capture: it is stopped and the run reported as
	// failed, but the files below are still written — see child.finish.
	stimFailed := false
	if stim != nil {
		stimFailed = stim.finish()
	}
	if elapsedS < float64(*durationPtr)-1 {
		fmt.Fprintf(out, "ok! (stopped early: %.1f s of the %d s requested)\n", elapsedS, *durationPtr)
	} else {
		fmt.Fprintln(out, "ok!")
	}

	base := GetNextBase(baseFilename)
	datFile := base + ".dat"
	dscFile := base + "-dscevents.csv"
	eventsFile := base + "-events.csv"

	if err := os.WriteFile(datFile, []byte(data), 0644); err != nil {
		log.Fatalln(err)
	}
	fmt.Fprintf(out, "Raw Data saved to %s\n", datFile)

	dscEvents, err := bbtkv3.CaptureOutputToEvents(data)
	if err != nil {
		log.Fatalln(err)
	}
	err = bbtkv3.SaveDSCEventsToCSV(dscEvents, dscFile)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Fprintf(out, "DSC Events saved to %s\n", dscFile)

	// Add an event with all lines set to 0 at the end of dscEvents, so that a
	// port still active when the capture stopped gets a falling edge and is
	// reported with its duration truncated to the capture window. The sentinel
	// must carry the end-of-capture timestamp: a zero-valued DSCEvent closes
	// those events at t=0 and yields a negative duration. Its PortStates map
	// is left nil, which reads as 0 for every port.
	// Use the time actually recorded, not the requested duration: after an early
	// stop they differ, and closing a still-active port at the requested end
	// would report a duration longer than the capture itself.
	endOfCapture := elapsedS * 1000
	if n := len(dscEvents); n > 0 && dscEvents[n-1].Timestamp > endOfCapture {
		endOfCapture = dscEvents[n-1].Timestamp
	}
	dscEvents = append(dscEvents, bbtkv3.DSCEvent{Timestamp: endOfCapture})

	events, err := bbtkv3.CaptureEventsFromDSCEvents(dscEvents)
	if err != nil {
		log.Fatalln(err)
	}

	err = bbtkv3.SaveEventsToCSV(events, eventsFile)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Fprintf(out, "Events saved to %s\n", eventsFile)

	// A capture in which no sensor ever fired is a legitimate result, and a
	// common one: a photodiode aimed off its square, a threshold set too high, or
	// a stimulus that opened on a different display all produce it. Say so
	// plainly — the header-only CSV above is otherwise indistinguishable from a
	// tooling failure.
	if len(events) == 0 {
		fmt.Fprintln(out, "WARNING: no events detected on any port.")
		fmt.Fprintln(out, "  The device recorded the full window but nothing crossed a threshold.")
		fmt.Fprintln(out, "  Check that the photodiodes sit on the stimulus, that the stimulus")
		fmt.Fprintln(out, "  opened on the display they are attached to, and the thresholds")
		fmt.Fprintln(out, "  (bbtk-adjust-thresholds).")
	}

	// The recording is on disk, so exit last rather than earlier: a caller must
	// still learn the stimulus failed, but not at the cost of the data. Disconnect
	// is deferred, and os.Exit skips defers, hence the explicit call.
	if stimFailed {
		b.Disconnect()
		os.Exit(stim.exitCode())
	}

	// Not necessary as defer will take care of it
	//if err = b.Disconnect(); err != nil {
	//	log.Println(err)
	//}

}
