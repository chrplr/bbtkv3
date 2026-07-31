// bbtk-send-break: stop whatever the BBTKv3 is doing and leave it idle.
//
// Sends the break character ('X', bare, no line ending) — the v3 mechanism for
// interrupting an ongoing device operation such as ICHK, OCHK or a running
// event-marking program. This is the recovery tool for a box left streaming,
// which happens whenever such an operation is killed rather than stopped with
// Esc: the device goes on sending, and the next tool to connect reads that
// stream instead of the handshake reply and fails with
//
//	Connect returned: Connect: expected "BBTK;", got "000000000000;"
//
// It deliberately does NOT handshake before sending. Connect() is exactly what
// fails on a wedged device, so requiring it first would make this tool unusable
// in the one situation it exists for. The port is opened, the character is
// written, the buffers are purged, and only then is the device asked whether it
// is back.
//
// Usage:
//
//	bbtk-send-break [-p <port>] [-b <baudrate>] [-n]
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/chrplr/bbtkv3"
)

var (
	Version string
	Build   string
)

func main() {
	portPtr := flag.String("p", "", "serial port (or set BBTK_PORT)")
	baudPtr := flag.Int("b", 115200, "baudrate")
	noVerifyPtr := flag.Bool("n", false, "send the break and exit without checking that the device responds")
	versionPtr := flag.Bool("V", false, "display version and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %s [options]

Sends the break character 'X' to the BBTK, stopping any operation it is running
(ICHK, OCHK, event marking) and leaving it idle.

Use this when a tool was killed instead of being stopped with Esc and the device
kept streaming, so that the next connection fails with

    Connect returned: Connect: expected "BBTK;", got "000000000000;"

No handshake is attempted before the break is sent, which is what makes this
work on a device that is too busy to handshake. Afterwards the device is asked
for an ECHO; exit status is 0 if it answers, 1 if it does not.

Options:
`, os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *versionPtr {
		fmt.Println(bbtkv3.VersionString(Version, Build))
		os.Exit(0)
	}

	port := *portPtr
	if port == "" {
		port = bbtkv3.ResolvePort()
	}
	if port == "" {
		log.Fatal("no serial port specified: use -p <port> or set BBTK_PORT")
	}

	// Opens the port without handshaking — see the package comment.
	b, err := bbtkv3.NewBbtkv3(port, *baudPtr, true)
	if err != nil {
		log.Fatal(err)
	}
	defer b.Disconnect()

	if err := b.SendBreakChar(); err != nil {
		log.Fatalf("sending break character: %v", err)
	}
	fmt.Println("Break character sent.")

	// The device may still be flushing bytes it had already queued. Give it a
	// moment, then purge both directions so the verification below reads the
	// ECHO reply rather than the tail of the interrupted stream.
	time.Sleep(time.Second)
	if err := b.ResetSerialBuffers(); err != nil {
		log.Printf("purging serial buffers: %v", err)
	}

	if *noVerifyPtr {
		return
	}

	if err := b.Connect(); err != nil {
		log.Fatalf("device did not come back: %v\n(try again, or power-cycle the BBTK)", err)
	}
	alive, err := b.IsAlive()
	if err != nil {
		log.Fatalf("device did not come back: %v\n(try again, or power-cycle the BBTK)", err)
	}
	if !alive {
		log.Fatal("device did not respond to ECHO\n(try again, or power-cycle the BBTK)")
	}
	fmt.Println("bbtkv3 is alive")
}
