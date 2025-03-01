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

// TODO: implement adjustable thresholds, reading the thresholds form the command line or from a configuration file
// TODO: better handle errors
// TODO: The way I handle DEBUG is a disaster, implement verbose and debug with 2 level logs.
//        THe module bbtkv3 uses the env at

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
	PortAddress = "/dev/ttyUSB0"
	Baudrate    = 115200
)

var defaultSmoothingMask = bbtkv3.SmoothingMask{
	Mic1:  true,
	Mic2:  true,
	Opto4: false,
	Opto3: false,
	Opto2: true,
	Opto1: true,
}

func myUsage() {
	fmt.Printf("Usage: %s [OPTIONS] thresholds\n", os.Args[0])
	fmt.Println("Where thresholds is a string of 8 comma-separated 0-127 values, .e.g., '63,63,32,32,100,100,100,100'")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = myUsage
	portPtr := flag.String("p", PortAddress, "device (serial port name)")
	speedPtr := flag.Int("b", Baudrate, "baudrate (speed in bps)")
	versionPtr := flag.Bool("V", false, "Display version")

	flag.Parse()

	if *versionPtr {
		fmt.Printf("Version: %s  Build: %s\n", Version, Build[:8])
		os.Exit(0)
	}

	newthresholds := flag.Arg(0)
	if newthresholds == "" {
		myUsage()
		os.Exit(1)
	}

	t, err := bbtkv3.ThresholdsFromString(newthresholds)
	fmt.Printf("Will try to set thresholds to: %s\n", t.ToString())

	if err != nil {
		log.Fatalf("Error parsing thresholds: %v\n", err)
	}

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

	fmt.Println("Getting current thresholds...")
	fmt.Printf("%+v\n", b.GetThresholds())

	fmt.Printf("Setting new thresholds...: %s\n", t.ToString())
	b.SetThresholds(t)

	fmt.Println("Getting new thresholds...")
	fmt.Printf("%+v\n", b.GetThresholds())
}
