// Scan serial ports for a BBTK device.
//
// The probing itself lives in the library (bbtkv3.ScanForBBTK), which is what
// the other tools fall back on when no port is given; this command is the
// explicit form of the same scan, and reports the stable name of what it finds.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/chrplr/bbtkv3"
)

// Variables to be passed on the compilation command line with "-X main.Version=${VERSION} -X main.Build=${BUILD}"
var (
	Version string
	Build   string
)

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

	portlist := flag.Args()

	if len(portlist) == 0 {
		portlist, err = bbtkv3.AvailablePorts()
		if err != nil {
			log.Fatal(err)
		}
	}

	if len(portlist) == 0 {
		fmt.Println("No serial ports found!")
		return
	}

	fmt.Printf("Scanning %v for a BBTK...\n", portlist)
	found := bbtkv3.ScanForBBTK(portlist, bbtkv3.DefaultBaudrate, true)
	if len(found) == 0 {
		fmt.Println("No BBTK found.")
		return
	}
	for _, p := range found {
		if stable := bbtkv3.StablePortName(p); stable != p {
			fmt.Printf("BBTK found at %v (%v)\n", stable, p)
		} else {
			fmt.Printf("BBTK found at %v\n", p)
		}
	}
}
