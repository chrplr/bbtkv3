// bbtk-event-marking: configure the BBTKv3 to mark a command event.
//
// Sends the PDCE / STYP / PATT / TIML sequence to the device followed by
// the pattern rows and PCCR / RUEM to commit and run the event-marking
// program. It then waits for the user to press Enter and sends 'X' to the
// device to stop it.
//
// Usage:
//
//	bbtk-event-marking [-p <port>] [-b <baudrate>]
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/chrplr/bbtkv3"
)

var (
	Version string
	Build   string
)

func main() {
	portPtr := flag.String("p", "", "serial port (or set BBTK_PORT)")
	baudPtr := flag.Int("b", 115200, "baudrate")
	versionPtr := flag.Bool("V", false, "display version and exit")
	flag.Parse()

	if *versionPtr {
		fmt.Printf("Version: %s  Build: %s\n", Version, Build)
		os.Exit(0)
	}

	port := *portPtr
	if port == "" {
		port = bbtkv3.GetPortFromEnv()
	}
	if port == "" {
		log.Fatal("no serial port specified: use -p <port> or set BBTK_PORT")
	}

	b, err := bbtkv3.NewBbtkv3(port, *baudPtr, false)
	if err != nil {
		log.Fatal(err)
	}
	defer b.Disconnect()

	if err := b.Connect(); err != nil {
		log.Fatalf("connect: %v", err)
	}

	commands := []string{
		"PDCE",
		"STYP",
		"PATT",
		"TIML",
		"0",
		"00000001000000000000,0000010000000000",
		"00000000000100000000,0000100000000000",
		"99999999999999999999,9999999999999999",
		"99999999999999999999,9999999999999999",
		"99999999999999999999,9999999999999999",
		"99999999999999999999,9999999999999999",
		"99999999999999999999,9999999999999999",
		"99999999999999999999,9999999999999999",
		"PCCR",
		"RUEM",
	}

	for _, cmd := range commands {
		fmt.Printf("> %s\n", cmd)
		if err := b.SendCommand(cmd); err != nil {
			log.Fatalf("send %q: %v", cmd, err)
		}
	}

	fmt.Println("Event marking running. Press Enter to stop.")
	bufio.NewReader(os.Stdin).ReadString('\n')

	if err := b.SendBreakChar(); err != nil {
		log.Fatalf("sendbreak: %v", err)
	}
	fmt.Println("Stopped.")
}
