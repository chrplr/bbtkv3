// Drive a BlackBoxToolKit to capture events
// Author: Christophe Pallier <christophe@pallier.org>
// LICENSE: GPL-3.0

// Package main provides a command-line tool to capture events using the BlackBoxToolKit (bbtkv3).
// It allows setting various parameters such as port address, baud rate, capture duration, and output file name.
// The tool also supports a debug mode and displays version information if requested.
//
// The main functionality includes initializing the bbtkv3 device, setting parameters, clearing internal memory,
// capturing events, and saving the captured data to files in both raw and CSV formats.
//
// Usage:
//   -p string
//         device (serial port name) (default "/dev/ttyUSB0")
//   -b int
//         baudrate (speed in bps) (default 115200)
//   -d int
//         duration of capture (in s) (default 30)
//   -o string
//         output file name for captured data (default "bbtk-capture.dat")
//   -D
//         Debug mode (default false)
//   -V
//         Display version

package main

import (
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
	PortAddress    = "/dev/ttyUSB0"
	Baudrate       = 115200
	Duration       = 30
	OutputFileName = "bbtk-capture.dat"
	DEBUG          = false
)

var defaultSmoothingMask = bbtkv3.SmoothingMask{
	Mic1:  true,
	Mic2:  true,
	Opto4: false,
	Opto3: false,
	Opto2: false,
	Opto1: false,
}

func main() {

	portPtr := flag.String("p", PortAddress, "device (serial port name)")
	speedPtr := flag.Int("b", Baudrate, "baudrate (speed in bps)")
	durationPtr := flag.Int("d", Duration, "duration of capture (in s)")
	outputFilenamePtr := flag.String("o", OutputFileName, "output file name for captured data")
	debugPtr := flag.Bool("D", DEBUG, "Debug mode")
	versionPtr := flag.Bool("V", false, "Display version")

	flag.Parse()

	if *versionPtr {
		fmt.Printf("Version: %s  Build: %s\n", Version, Build[:8])
		os.Exit(0)
	}

	DEBUG = *debugPtr

	// Initialisation
	verbose := true
	b, err := bbtkv3.NewBbtkv3(*portPtr, *speedPtr, verbose)
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

	//fmt.Printf("Setting thresholds: %+v\n", defaultThresholds)
	b.SetDefaultsThresholds()

	// Clearing internal memory
	time.Sleep(time.Second)
	fmt.Printf("Clearing Timing data... ")
	b.ClearTimingData()
	fmt.Println("Ok")

	// Data Capture
	time.Sleep(1 * time.Second)
	fmt.Printf("Capturing event (DSCM) for %v msec... ", *durationPtr)
	data := b.CaptureEvents(*durationPtr)
	fmt.Println("ok!")

	fname, err := WriteText(*outputFilenamePtr, data)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Printf("Raw Data saved to %s\n", fname)

	dscEvents, err := bbtkv3.CaptureOutputToEvents(data)
	if err != nil {
		log.Fatalln(err)
	} else {
		efname := changeExtension(fname, "-dscevents.csv")
		err = bbtkv3.SaveDSCEventsToCSV(dscEvents, efname)
		if err != nil {
			log.Fatalln(err)
		}
	}

	// add a event with all lines set to 0 at the end of dscEvents
	dscEvents = append(dscEvents, bbtkv3.DSCEvent{})

	events, err := bbtkv3.CaptureEventsFromDSCEvents(dscEvents)
	if err != nil {
		log.Fatalln(err)
	}

	err = bbtkv3.SaveEventsToCSV(events, changeExtension(fname, "events.csv"))
	if err != nil {
		log.Fatalln(err)
	}

	// Not necessary as defer will take care of it
	//if err = b.Disconnect(); err != nil {
	//	log.Println(err)
	//}

}
