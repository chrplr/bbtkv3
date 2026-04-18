// bbtk-input-check: stream live input state from the BBTKv3.
//
// Sends the ICHK command to the device and prints every line it returns.
// Press Esc (no Enter needed) to stop; the program sends the break character
// to the device and exits.
//
// Usage:
//
//	bbtk-input-check [-p <port>] [-b <baudrate>]
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/chrplr/bbtkv3"
	"golang.org/x/term"
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

	if err := b.SendCommand("ICHK"); err != nil {
		log.Fatalf("ICHK: %v", err)
	}

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)

	// Stream device lines until done is closed.
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
			}
			line, err := b.ReadLine()
			if err != nil {
				continue // read timeout or transient error — keep polling
			}
			fmt.Println(line)
		}
	}()

	// Wait for the user to press Esc.
	// Use raw terminal mode so no Enter is required.
	if oldState, rawErr := term.MakeRaw(int(os.Stdin.Fd())); rawErr == nil {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
		fmt.Print("Streaming input state. Press Esc to stop.")
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				break
			}
			if buf[0] == 27 {
				break
			}
		}
	} else {
		// stdin is not a terminal (piped/scripted): wait for Enter.
		fmt.Println("Streaming input state. Press Enter to stop.")
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				break
			}
			if buf[0] == '\n' || buf[0] == '\r' {
				break
			}
		}
	}

	// Send break first so the device stops streaming and the goroutine's
	// in-flight ReadLine returns promptly.
	fmt.Println("\nStopping...")
	if err := b.SendBreakChar(); err != nil {
		log.Printf("sendbreak: %v", err)
	}
	close(done)
	wg.Wait()
}
