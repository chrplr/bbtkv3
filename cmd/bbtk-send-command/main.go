// bbtk-send-command — pipe raw commands to the Black Box ToolKit v3 and print its responses.
//
// Usage:
//
//	echo "ECHO" | bbtk-send-command -p /dev/ttyUSB0
//	printf "CONN\nECHO\n" | bbtk-send-command -p /dev/ttyUSB0 -timeout 2
//
// Each line of stdin is sent as a raw command (CRLF appended automatically).
// After each command the tool reads response lines from the device until
// -timeout consecutive seconds of silence are observed, then moves on to the
// next command.
package main

import (
	"bufio"
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
	portPtr := flag.String("p", "", "serial port device (or set BBTK_PORT)")
	baudPtr := flag.Int("b", 115200, "baudrate (bps)")
	timeoutPtr := flag.Int("timeout", 1, "seconds of silence after last response line before moving to next command")
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

	if *timeoutPtr < 1 {
		log.Fatal("-timeout must be >= 1")
	}

	b, err := bbtkv3.NewBbtkv3(port, *baudPtr, false)
	if err != nil {
		log.Fatal(err)
	}
	defer b.Disconnect()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		cmd := scanner.Text()
		if cmd == "" {
			continue
		}

		if err := b.SendCommand(cmd); err != nil {
			log.Printf("send error: %v", err)
			continue
		}

		// Read response lines until -timeout consecutive 1-second silences.
		// The serial port has an internal 1-second read deadline, so each
		// ReadLine call costs at most one second when no data arrives.
		silence := 0
		for silence < *timeoutPtr {
			line, err := b.ReadLine()
			if err != nil {
				// Read timed out (no data) or transient error — count silence.
				silence++
				continue
			}
			silence = 0
			fmt.Println(line)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("stdin error: %v", err)
	}
}
