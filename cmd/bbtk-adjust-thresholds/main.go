// Drive a BlackBoxToolKit to capture events
// Author: Christophe Pallier <christophe@pallier.org>
// LICENSE: GPL-3.0

// Package main puts a BlackBoxToolKit (bbtkv3) into its interactive threshold
// adjustment mode (the AJPV command) and waits for it to finish.
//
// The adjustment itself happens ON THE DEVICE: its display shows the live
// sensor levels and its buttons change them. This tool starts that mode and
// blocks until the device reports "Done;", so there is nothing to type here.
//
// Thresholds are sensitivities, not trigger levels: the HIGHER the value, the
// more sensitive the sensor. See the "Typical session" section of the README
// for how to choose them, and bbtk-set-thresholds to write values directly.
//
// Usage:
//
//	bbtk-adjust-thresholds [options]
//
//	-p string
//	      device (serial port name); overrides BBTK_PORT (default "/dev/ttyUSB0")
//	-b int
//	      baudrate (speed in bps) (default 115200)
//	-V
//	      Display version

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

func main() {

	portPtr := flag.String("p", "", "device (serial port name); overrides BBTK_PORT (default \""+PortAddress+"\")")
	speedPtr := flag.Int("b", Baudrate, "baudrate (speed in bps)")
	versionPtr := flag.Bool("V", false, "Display version")

	flag.Parse()

	if *versionPtr {
		fmt.Println(bbtkv3.VersionString(Version, Build))
		os.Exit(0)
	}

	// Port resolution, highest precedence first: -p, then BBTK_PORT, then the
	// /dev/serial/by-id symlink (Linux only), then the built-in default.
	serPort := *portPtr
	if serPort == "" {
		serPort = bbtkv3.ResolvePort()
	}
	if serPort == "" {
		serPort = PortAddress
	}

	b, err := bbtkv3.NewBbtkv3(serPort, *speedPtr, false)
	if err != nil {
		log.Fatalln(err)
	}
	defer b.Disconnect()

	time.Sleep(100 * time.Millisecond)

	// HandShaking
	if err = b.Connect(); err != nil {
		log.Fatalf("Connect returned: %v\n", err)
	}
	time.Sleep(100 * time.Millisecond)

	fmt.Println("Connected to the BBTKv3. Getting thresholds...")
	if t, err := b.GetThresholds(); err != nil {
		log.Printf("GetThresholds: %v\n", err)
	} else {
		// Printed, not discarded: these are the values the device is about to
		// let you edit, and the only record of where you started from.
		fmt.Printf("%+v\n", t)
	}

	fmt.Println("The BBTKv3 is now in Threshold setting mode...")
	b.AdjustThresholds()

	// Not necessary as defer will take care of it
	//if err = b.Disconnect(); err != nil {
	//	log.Println(err)
	//}

}
