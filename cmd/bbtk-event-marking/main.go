// bbtk-event-marking: configure the BBTKv3 to mark a command event.
//
// Sends the PDCE / STYP / PATT / TIML sequence to the device followed by
// the pattern rows and PCCR / RUEM to commit and run the event-marking
// program. It then waits for the user to press Esc and sends the break
// character to the device to stop it.
//
// Usage:
//
//	bbtk-event-marking [-p <port>] [-b <baudrate>]
package main

import (
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
		fmt.Println(bbtkv3.VersionString(Version, Build))
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

	if err := b.EventMarking(bbtkv3.DefaultEventMarkingPattern); err != nil {
		log.Fatalf("event marking: %v", err)
	}
}
