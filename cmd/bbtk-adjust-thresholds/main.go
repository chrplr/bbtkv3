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

	portPtr := flag.String("p", PortAddress, "device (serial port name)")
	speedPtr := flag.Int("b", Baudrate, "baudrate (speed in bps)")
	versionPtr := flag.Bool("V", false, "Display version")

	flag.Parse()

	if *versionPtr {
		fmt.Printf("Version: %s  Build: %s\n", Version, Build[:8])
		os.Exit(0)
	}

	b, err := bbtkv3.NewBbtkv3(*portPtr, *speedPtr, false)
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
	b.GetThresholds()

	fmt.Println("The BBTKv3 is now in Threshold setting mode...")
	b.AdjustThresholds()

	// Not necessary as defer will take care of it
	//if err = b.Disconnect(); err != nil {
	//	log.Println(err)
	//}

}
