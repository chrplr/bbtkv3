// scan all available serial ports for a BBTK device
// because some ports can be blocking,
package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"time"

	"go.bug.st/serial"
)

var (
	Baudrate = 115200
	DEBUG    = false
)

func ReadData(port serial.Port) string {
	byteBuff := bytes.NewBufferString("")
	buff := make([]byte, 100)

	err := port.SetReadTimeout(time.Second)
	if err != nil {
		panic(err)
	}

	for {

		n, err := port.Read(buff)
		if err != nil {
			panic(err)
		}

		//0 means we hit the timeout.
		if n == 0 {
			return byteBuff.String()
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
	}
	resp := ReadData(port)
	return resp[:len(resp)-1] == "BBTK;"
}

func ScanSerialPortForBBTK(portName string) {
	mode := &serial.Mode{
		BaudRate: Baudrate,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	p, err := serial.Open(portName, mode)
	if err != nil {
		fmt.Println("Error while trying to open", portName, " at ", Baudrate, "bps", err)
	}
	defer p.Close()
	if DEBUG {
		fmt.Println("Opened port", portName)
	}
	if CheckIfBBTKConnectedAt(p) {
		fmt.Printf("BBTK found at %v\n", portName)
	}
}

func main() {
	var err error

	if _, ok := os.LookupEnv("DEBUG"); ok {
		DEBUG = true
		log.Println("DEBUG mode enabled.")
	} else {
		DEBUG = false
	}

	portlist := os.Args[1:]

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
		for _, p := range portlist {
			go ScanSerialPortForBBTK(p)
		}
	}
	time.Sleep(2. * time.Second)
}
