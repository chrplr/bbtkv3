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
//   bbtk-capture [options] <basefilename>
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
// Output files are named <basefilename>-001.dat, <basefilename>-001.dscevents.csv, etc.
// The sequence number is incremented automatically to avoid overwriting previous recordings.

// TODO: implement adjustable thresholds, reading the thresholds form the command line or from a configuration file
// TODO: better handle errors
// TODO: The way I handle DEBUG is a disaster, implement verbose and debug with 2 level logs.
//        THe module bbtkv3 uses the env at

package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
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

	portPtr := flag.String("p", PortAddress, "device (serial port name)")
	speedPtr := flag.Int("b", Baudrate, "baudrate (speed in bps)")
	durationPtr := flag.Int("d", Duration, "duration of capture (in s)")
	debugPtr := flag.Bool("D", DEBUG, "Debug mode")
	versionPtr := flag.Bool("V", false, "Display version")
	noCountdownPtr := flag.Bool("no-countdown", false, "Disable second-by-second countdown display")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <basefilename>\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nOutput files: <basefilename>-001.dat, <basefilename>-001.dscevents.csv, <basefilename>-001.events.csv\nSequence number is incremented automatically to avoid overwriting previous recordings.\n")
	}

	flag.Parse()

	if *versionPtr {
		fmt.Printf("Version: %s  Build: %s\n", Version, Build[:8])
		os.Exit(0)
	}

	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Error: a basefilename argument is required.\n\n")
		flag.Usage()
		os.Exit(1)
	}
	baseFilename := flag.Arg(0)

	DEBUG = *debugPtr

	serPort := ""
	if *portPtr != "" {
		serPort = *portPtr
	} else {
		serPort = bbtkv3.GetPortFromEnv()
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
			fmt.Println("bbtkv3 is alive")
		} else {
			fmt.Println("bbtkv3 not responding to ECHO")
		}
	}
	time.Sleep(time.Second)

	// Parameters setting
	fmt.Printf("Setting Smoothing mask to %+v\n", defaultSmoothingMask)
	if err = b.SetSmoothing(defaultSmoothingMask); err != nil {
		log.Printf("%v", err)
	}
	time.Sleep(time.Second)

	fmt.Println("Getting thresholds...")
	thresholds, err := b.GetThresholds()
	if err != nil {
		log.Printf("GetThresholds: %v\n", err)
	} else {
		fmt.Printf("%+v\n", thresholds)
	}

	// Clearing internal memory
	time.Sleep(time.Second)
	fmt.Printf("Clearing Timing data... ")
	if err := b.ClearTimingData(); err != nil {
		log.Fatalf("ClearTimingData: %v\n", err)
	}
	fmt.Println("Ok")

	// Data Capture
	time.Sleep(1 * time.Second)
	fmt.Printf("Capturing events (with DSCM) for %v seconds... ", *durationPtr)
	data, err := b.CaptureEvents(*durationPtr, *noCountdownPtr)
	if errors.Is(err, bbtkv3.ErrCaptureAborted) {
		fmt.Println("Capture aborted.")
		os.Exit(0)
	}
	if err != nil {
		log.Fatalf("CaptureEvents: %v\n", err)
	}
	fmt.Println("ok!")

	base := GetNextBase(baseFilename)
	datFile := base + ".dat"
	dscFile := base + "-dscevents.csv"
	eventsFile := base + "-events.csv"

	if err := os.WriteFile(datFile, []byte(data), 0644); err != nil {
		log.Fatalln(err)
	}
	fmt.Printf("Raw Data saved to %s\n", datFile)

	dscEvents, err := bbtkv3.CaptureOutputToEvents(data)
	if err != nil {
		log.Fatalln(err)
	}
	err = bbtkv3.SaveDSCEventsToCSV(dscEvents, dscFile)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Printf("DSC Events saved to %s\n", dscFile)

	// add a event with all lines set to 0 at the end of dscEvents
	dscEvents = append(dscEvents, bbtkv3.DSCEvent{})

	events, err := bbtkv3.CaptureEventsFromDSCEvents(dscEvents)
	if err != nil {
		log.Fatalln(err)
	}

	err = bbtkv3.SaveEventsToCSV(events, eventsFile)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Printf("Events saved to %s\n", eventsFile)

	// Not necessary as defer will take care of it
	//if err = b.Disconnect(); err != nil {
	//	log.Println(err)
	//}

}
