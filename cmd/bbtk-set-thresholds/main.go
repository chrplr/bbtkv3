// Drive a BlackBoxToolKit to capture events
// Author: Christophe Pallier <christophe@pallier.org>
// LICENSE: GPL-3.0

// Package main provides a command-line tool to write the eight activation
// thresholds of a BlackBoxToolKit (bbtkv3).
//
// The values are sensitivities, not trigger levels: the HIGHER the value, the
// more sensitive that sensor. Raise a channel until it false-alarms, then back
// off — see the "Typical session" section of the README.
//
// Usage:
//
//	bbtk-set-thresholds [options] <mic1,mic2,sounder1,sounder2,opto1,opto2,opto3,opto4>
//
//	-p string
//	      device (serial port name); overrides BBTK_PORT (default: autodetect)
//	-b int
//	      baudrate (speed in bps) (default 115200)
//	-V
//	      Display version

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
	Baudrate = 115200
)

func myUsage() {
	fmt.Printf("Usage: %s [OPTIONS] thresholds\n", os.Args[0])
	fmt.Println("Where thresholds is a string of 8 comma-separated 0-127 values, .e.g., '63,63,32,32,100,100,100,100'")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = myUsage
	portPtr := flag.String("p", "", "device (serial port name); overrides BBTK_PORT (default: autodetect)")
	speedPtr := flag.Int("b", Baudrate, "baudrate (speed in bps)")
	versionPtr := flag.Bool("V", false, "Display version")

	flag.Parse()

	if *versionPtr {
		fmt.Println(bbtkv3.VersionString(Version, Build))
		os.Exit(0)
	}

	// Port resolution, highest precedence first: -p, then BBTK_PORT, then
	// autodetection (the /dev/serial/by-id symlink on Linux, else a scan).
	serPort := *portPtr
	if serPort == "" {
		serPort = bbtkv3.ResolvePort()
	}
	if serPort == "" {
		log.Fatal("no serial port specified: use -p <port> or set BBTK_PORT")
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

	fmt.Println("Getting current thresholds...")
	if thresholds, err := b.GetThresholds(); err != nil {
		log.Printf("GetThresholds: %v\n", err)
	} else {
		fmt.Printf("%+v\n", thresholds)
	}

	fmt.Printf("Setting new thresholds...: %s\n", t.ToString())
	if err := b.SetThresholds(t); err != nil {
		log.Printf("SetThresholds: %v\n", err)
	}

	fmt.Println("Getting new thresholds...")
	if thresholds, err := b.GetThresholds(); err != nil {
		log.Printf("GetThresholds: %v\n", err)
	} else {
		fmt.Printf("%+v\n", thresholds)
	}
}
