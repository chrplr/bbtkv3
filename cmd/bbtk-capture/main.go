// Drive a BlackBoxToolKit to capture events
// Author: Christophe Pallier <christophe@pallier.org>
// LICENSE: GPL-3.0

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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
	OutputFileName = "bbtk-capture-001.dat"
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

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func WriteText(basename string, text string) error {
	var filename string = basename

	ext := filepath.Ext(basename)
	name := strings.TrimSuffix(basename, ext)

	for i := 2; fileExists(filename); i++ {
		filename = fmt.Sprintf("%s-%03d%s", name, i, ext)
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = f.WriteString(text)

	return err
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
	WriteText(*outputFilenamePtr, data)
	fmt.Println(data)

	// Not necessary as defer will take care of it
	//if err = b.Disconnect(); err != nil {
	//	log.Println(err)
	//}

}
