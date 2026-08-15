// Drive a BlackBoxToolKit to capture events
// Author: Christophe Pallier <christophe@pallier.org>
// LICENSE: GPL-3.0

// Package main provides a command-line tool to set the smoothing mask of a
// BlackBoxToolKit (bbtkv3).
//
// Smoothing tells the device to ignore short transitions on a channel. With it
// off, every leading edge is reported — including each refresh of a CRT. With it
// on, the channel reads about 20 ms longer than the stimulus actually lasted,
// because the line is held past the true falling edge; see
// bbtkv3.DefaultSmoothingDurationOffsetMs.
//
// The mask is given as six 0/1 values, one per channel. See myUsage below for
// the order, which is the device's own and is not the obvious one.
//
// Usage:
//
//	bbtk-set-smoothing [OPTIONS] <mic1,mic2,opto4,opto3,opto2,opto1>

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
	fmt.Fprintf(os.Stderr, `Usage: %s [OPTIONS] <mask>

Sets which channels the BBTK smooths. <mask> is six 0/1 values, 1 enabling
smoothing on that channel. Separate them with commas or semicolons.

The order is the device's own, and the Opto channels run DOWNWARDS:

    mic1,mic2,opto4,opto3,opto2,opto1
      |    |     |     |     |     |
      |    |     |     |     |     +-- Opto1
      |    |     |     |     +-------- Opto2
      |    |     |     +-------------- Opto3
      |    |     +-------------------- Opto4
      |    +-------------------------- Mic2
      +------------------------------- Mic1

Examples:
    %s 1,1,1,1,1,1     smoothing on every channel
    %s 0,0,0,0,0,0     smoothing off everywhere
    %s 1,1,0,0,1,1     mics and Opto1/Opto2 only, Opto3/Opto4 off

Smoothing suppresses spurious edges — without it a CRT reports every refresh —
at the cost of about 20 ms added to each recorded duration on the channels it
covers. TTLin and the keypad are outside the mask and are never affected.

The parsed mask is echoed by channel name before it is sent, so a mistake in
the ordering above is visible before it reaches the device.

Options:
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0])
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

	maskArg := flag.Arg(0)
	if maskArg == "" {
		myUsage()
		os.Exit(1)
	}

	mask, err := bbtkv3.SmoothingMaskFromString(maskArg)
	if err != nil {
		log.Fatalf("Error parsing smoothing mask %q: %v\n", maskArg, err)
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

	fmt.Printf("Setting smoothing mask to %+v\n", mask)
	if err = b.SetSmoothing(mask); err != nil {
		log.Fatalf("failed to set smoothing mask: %v", err)
	}
	fmt.Println("ok!")

}
