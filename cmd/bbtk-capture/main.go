// Drive a BlackBoxToolKit to capture events
// Author: Christophe Pallier <christophe@pallier.org>
// LICENSE: GPL-3.0

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

var defaultSmoothingMask = bbtkv3.SmoothingMask{
	Mic1:  true,
	Mic2:  true,
	Opto4: false,
	Opto3: false,
	Opto2: false,
	Opto1: false,
}

func main() {

	portPtr := flag.String("p", bbtkv3.PortAddress, "device (serial port name)")
	speedPtr := flag.Int("b", bbtkv3.Baudrate, "baudrate (speed in bps)")
	durationPtr := flag.Int("d", bbtkv3.Duration, "duration of capture (in s)")
	outputFilenamePtr := flag.String("o", bbtkv3.OutputFileName, "output file name for captured data")
	debugPtr := flag.Bool("D", bbtkv3.DEBUG, "Debug mode")
	versionPtr := flag.Bool("V", false, "Display version")

	flag.Parse()

	if *versionPtr {
		fmt.Printf("Version: %s  Build: %s\n", Version, Build[:8])
		os.Exit(0)
	}

	bbtkv3.DEBUG = *debugPtr

	// Initialisation
	b, err := bbtkv3.NewBbtkv3(*portPtr, *speedPtr)
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
		log.Fatalf("Connect returned: %w\n", err)
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
		log.Printf("%w", err)
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
	data := b.CaptureEvents(*durationPtr)
	bbtkv3.WriteText(*outputFilenamePtr, data)
	fmt.Println(data)

	// Not necessary as defer will take care of it
	//if err = b.Disconnect(); err != nil {
	//	log.Println(err)
	//}

}
