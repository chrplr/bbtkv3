// scan all available serial ports for a BBTK device

package main

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
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

func CheckBBTKConnectedAt(port serial.Port) bool {
	_, err := port.Write([]byte("CONN\r\n"))
	if err != nil {
		fmt.Println(err)
	}

	resp := ReadData(port)
	return resp[:len(resp)-1] == "BBTK;"
}

func ScanSerialPortsForBBTK() {
	ports, err := serial.GetPortsList()
	if err != nil {
		log.Fatal(err)
	}

	mode := &serial.Mode{
		BaudRate: Baudrate,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	rand.Shuffle(len(ports), func(i, j int) {
		ports[i], ports[j] = ports[j], ports[i]
	})

	for _, port := range ports {
		fmt.Printf("Checking %s ... ", port)
		p, err := serial.Open(port, mode)
		if err != nil {
			fmt.Println("Error while trying to open", port, " at ", Baudrate, "bps", err)
		}
		if CheckBBTKConnectedAt(p) {
			fmt.Println("BBTK found!")
		} else {
			fmt.Println("no")
		}
		p.Close()
	}
}

func main() {
	if _, ok := os.LookupEnv("DEBUG"); ok {
		DEBUG = true
		log.Println("DEBUG mode enabled.")
	} else {
		DEBUG = false
	}

	portlist, err := serial.GetPortsList()
	if err != nil {
		log.Fatal(err)
	}
	if len(portlist) == 0 {
		fmt.Println("No serial ports found!")
	} else {
		fmt.Printf("Scanning %v for a BBTK...\n", portlist)
		ScanSerialPortsForBBTK()
	}
}
