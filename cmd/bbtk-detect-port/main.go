// scan all available serial ports for a BBTK device
// because some ports can be blocking,
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/chrplr/bbtkv3"
	"go.bug.st/serial"
)

// Variables to be passed on the compilation command line with "-X main.Version=${VERSION} -X main.Build=${BUILD}"
var (
	Version string
	Build   string
)

var (
	Baudrate = 115200
	DEBUG    = false
)

func ReadData(port serial.Port) (string, error) {
	byteBuff := bytes.NewBufferString("")
	buff := make([]byte, 100)

	err := port.SetReadTimeout(time.Second)
	if err != nil {
		return "", fmt.Errorf("ReadData: %w", err)
	}

	for {

		n, err := port.Read(buff)
		if err != nil {
			return "", fmt.Errorf("ReadData: %w", err)
		}

		//0 means we hit the timeout.
		if n == 0 {
			return byteBuff.String(), nil
		}

		byteBuff.Write(buff[:n])
		if DEBUG {
			fmt.Printf("%s", byteBuff.String())
		}
	}
}

func CheckIfBBTKConnectedAt(port serial.Port) bool {
	_, err := port.Write([]byte("CONN\r\n"))
	if err != nil {
		fmt.Println(err)
		return false
	}
	resp, err := ReadData(port)
	if err != nil || len(resp) == 0 {
		return false
	}
	return resp[:len(resp)-1] == "BBTK;"
}

func ScanSerialPortForBBTK(portName string, wg *sync.WaitGroup) {
	defer wg.Done()

	mode := &serial.Mode{
		BaudRate: Baudrate,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	p, err := serial.Open(portName, mode)
	if err != nil {
		fmt.Println("Error while trying to open", portName, "at", Baudrate, "bps:", err)
		return
	}
	defer p.Close()
	if DEBUG {
		fmt.Println("Opened port", portName)
	}
	if CheckIfBBTKConnectedAt(p) {
		if stable := bbtkv3.StablePortName(portName); stable != portName {
			fmt.Printf("BBTK found at %v (%v)\n", stable, portName)
		} else {
			fmt.Printf("BBTK found at %v\n", portName)
		}
	}
}

func main() {
	var err error

	versionPtr := flag.Bool("V", false, "display version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [port ...]\n\nScans the given serial ports for a BBTK, or every available port when none is given.\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *versionPtr {
		fmt.Println(bbtkv3.VersionString(Version, Build))
		os.Exit(0)
	}

	if _, ok := os.LookupEnv("DEBUG"); ok {
		DEBUG = true
		log.Println("DEBUG mode enabled.")
	} else {
		DEBUG = false
	}

	portlist := flag.Args()

	if len(portlist) == 0 {
		portlist, err = serial.GetPortsList()
		if err != nil {
			log.Fatal(err)
		}
	}

	if len(portlist) == 0 {
		fmt.Println("No serial ports found!")
	} else {
		fmt.Printf("Scanning %v for a BBTK...\n", portlist)
		var wg sync.WaitGroup
		for _, p := range portlist {
			wg.Add(1)
			go ScanSerialPortForBBTK(p, &wg)
		}
		wg.Wait()
	}
}
