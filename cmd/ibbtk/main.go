// ibbtk — interactive command-line interface for the Black Box ToolKit v3.
//
// Usage:
//
//	ibbtk -p <port> [-b <baudrate>] [-v]
//
// The tool connects to the BBTK and presents a nested menu-driven shell.
// Top-level sub-menus: thresholds, smoothing, capture.
// Type 'menu' at any prompt for the list of commands, 'exit' to go up one level.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/chrplr/bbtkv3"
	"github.com/turret-io/go-menu/menu"
)

// Variables injected at build time via -ldflags.
var (
	Version string
	Build   string
)

func main() {
	portPtr := flag.String("p", "", "serial port (or set BBTK_PORT)")
	baudPtr := flag.Int("b", 115200, "baudrate")
	verbosePtr := flag.Bool("v", false, "verbose connection output")
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

	b, err := bbtkv3.NewBbtkv3(port, *baudPtr, *verbosePtr)
	if err != nil {
		log.Fatal(err)
	}
	defer b.Disconnect()

	if err := b.Connect(); err != nil {
		log.Fatalf("connect: %v", err)
	}
	fmt.Println("Connected to BBTKv3. Type 'menu' for commands, 'exit' to quit.")

	// ── Thresholds sub-menu ──────────────────────────────────────────────────

	thresholdsMenu := func(args ...string) error {
		cmds := []menu.CommandOption{
			{
				Command:     "get",
				Description: "Read current thresholds from device",
				Function: func(args ...string) error {
					t, err := b.GetThresholds()
					if err != nil {
						fmt.Printf("error: %v\n", err)
						return nil
					}
					fmt.Printf("%+v\n", t)
					return nil
				},
			},
			{
				Command:     "set",
				Description: "Set thresholds: set <mic1,mic2,sounder1,sounder2,opto1,opto2,opto3,opto4>  (0-127 each)",
				Function: func(args ...string) error {
					if len(args) == 0 {
						fmt.Println("usage: set <mic1,mic2,sounder1,sounder2,opto1,opto2,opto3,opto4>")
						return nil
					}
					t, err := bbtkv3.ThresholdsFromString(args[0])
					if err != nil {
						fmt.Printf("error parsing thresholds: %v\n", err)
						return nil
					}
					if err := b.SetThresholds(t); err != nil {
						fmt.Printf("error setting thresholds: %v\n", err)
						return nil
					}
					// Read back to confirm
					t, err = b.GetThresholds()
					if err != nil {
						fmt.Printf("error reading back: %v\n", err)
						return nil
					}
					fmt.Printf("Thresholds now: %+v\n", t)
					return nil
				},
			},
			{
				Command:     "adjust",
				Description: "Launch interactive threshold adjustment on device",
				Function: func(args ...string) error {
					fmt.Println("Entering interactive threshold adjustment (use device controls)...")
					b.AdjustThresholds()
					fmt.Println("Done.")
					return nil
				},
			},
		}
		menu.NewMenu(cmds, menu.NewMenuOptions("thresholds> ", 0)).Start()
		return nil
	}

	// ── Smoothing sub-menu ───────────────────────────────────────────────────

	smoothingMenu := func(args ...string) error {
		cmds := []menu.CommandOption{
			{
				Command:     "set",
				Description: "Set smoothing mask: set <mic1;mic2;opto4;opto3;opto2;opto1>  (0 or 1 each)",
				Function: func(args ...string) error {
					if len(args) == 0 {
						fmt.Println("usage: set <mic1;mic2;opto4;opto3;opto2;opto1>")
						return nil
					}
					mask, err := bbtkv3.SmoothingMaskFromString(args[0])
					if err != nil {
						fmt.Printf("error parsing mask: %v\n", err)
						return nil
					}
					if err := b.SetSmoothing(mask); err != nil {
						fmt.Printf("error setting smoothing: %v\n", err)
						return nil
					}
					fmt.Printf("Smoothing mask set: %s\n", mask.ToString())
					return nil
				},
			},
			{
				Command:     "default",
				Description: "Enable smoothing on all sensors",
				Function: func(args ...string) error {
					mask := bbtkv3.SmoothingMask{
						Mic1: true, Mic2: true,
						Opto1: true, Opto2: true, Opto3: true, Opto4: true,
					}
					if err := b.SetSmoothing(mask); err != nil {
						fmt.Printf("error setting smoothing: %v\n", err)
						return nil
					}
					fmt.Printf("Smoothing mask set: %s\n", mask.ToString())
					return nil
				},
			},
		}
		menu.NewMenu(cmds, menu.NewMenuOptions("smoothing> ", 0)).Start()
		return nil
	}

	// ── Capture sub-menu ─────────────────────────────────────────────────────

	captureMenu := func(args ...string) error {
		cmds := []menu.CommandOption{
			{
				Command:     "run",
				Description: "Capture events: run <seconds> [output.dat]",
				Function: func(args ...string) error {
					if len(args) == 0 {
						fmt.Println("usage: run <seconds> [output.dat]")
						return nil
					}
					seconds, err := strconv.Atoi(args[0])
					if err != nil || seconds <= 0 {
						fmt.Printf("invalid duration %q: must be a positive integer\n", args[0])
						return nil
					}

					outFile := "ibbtk-capture.dat"
					if len(args) >= 2 {
						outFile = args[1]
					}

					fmt.Printf("Clearing timing data... ")
					if err := b.ClearTimingData(); err != nil {
						fmt.Printf("error: %v\n", err)
						return nil
					}
					fmt.Println("ok")

					fmt.Printf("Capturing for %d seconds...\n", seconds)
					data, err := b.CaptureEvents(seconds, false)
					if err != nil {
						fmt.Printf("capture error: %v\n", err)
						return nil
					}

					if err := os.WriteFile(outFile, []byte(data), 0644); err != nil {
						fmt.Printf("error writing %s: %v\n", outFile, err)
						return nil
					}
					fmt.Printf("Raw data saved to %s\n", outFile)

					dscEvents, err := bbtkv3.CaptureOutputToEvents(data)
					if err != nil {
						fmt.Printf("error parsing events: %v\n", err)
						return nil
					}

					dscFile := strings.TrimSuffix(outFile, ".dat") + "-dscevents.csv"
					if err := bbtkv3.SaveDSCEventsToCSV(dscEvents, dscFile); err != nil {
						fmt.Printf("error saving DSC events: %v\n", err)
						return nil
					}
					fmt.Printf("DSC events saved to %s\n", dscFile)

					// Append a zero-state sentinel so the last event gets a falling edge
					dscEvents = append(dscEvents, bbtkv3.DSCEvent{})
					events, err := bbtkv3.CaptureEventsFromDSCEvents(dscEvents)
					if err != nil {
						fmt.Printf("error computing events: %v\n", err)
						return nil
					}

					evFile := strings.TrimSuffix(outFile, ".dat") + "-events.csv"
					if err := bbtkv3.SaveEventsToCSV(events, evFile); err != nil {
						fmt.Printf("error saving events: %v\n", err)
						return nil
					}
					fmt.Printf("Events saved to %s (%d events detected)\n", evFile, len(events))
					return nil
				},
			},
			{
				Command:     "clear",
				Description: "Erase device timing memory",
				Function: func(args ...string) error {
					fmt.Printf("Clearing timing data... ")
					if err := b.ClearTimingData(); err != nil {
						fmt.Printf("error: %v\n", err)
						return nil
					}
					fmt.Println("ok")
					return nil
				},
			},
		}
		menu.NewMenu(cmds, menu.NewMenuOptions("capture> ", 0)).Start()
		return nil
	}

	// ── streamUntilBreak ─────────────────────────────────────────────────────
	// Sends cmd to the device, prints every line the device returns, and stops
	// when the user types "X" (+ Enter). It then sends the break character 'X'
	// to the device to interrupt the ongoing operation.

	streamUntilBreak := func(cmd string) {
		if err := b.SendCommand(cmd); err != nil {
			fmt.Printf("send error: %v\n", err)
			return
		}

		done := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)

		// Goroutine: continuously read and print device lines.
		// ReadLine has a 1-second port timeout, so once 'done' is closed the
		// goroutine exits within at most one timeout cycle.
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
					continue // timeout or transient error — keep polling
				}
				fmt.Println(line)
			}
		}()

		fmt.Println("(type X + Enter to stop)")
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			if strings.EqualFold(strings.TrimSpace(scanner.Text()), "x") {
				break
			}
		}

		// Send the break first so the device stops streaming; this lets the
		// goroutine's next ReadLine return quickly instead of waiting a full
		// timeout cycle.  Then close done and wait for the goroutine to exit
		// before returning — otherwise a second call to streamUntilBreak would
		// start a new goroutine that races on b.reader with the old one,
		// corrupting the bufio.Reader and causing a slice-bounds panic.
		if err := b.SendBreakChar(); err != nil {
			fmt.Printf("sendbreak error: %v\n", err)
		}
		close(done)
		wg.Wait()
		fmt.Println("Stopped.")
	}

	// ── Main menu ────────────────────────────────────────────────────────────

	mainCmds := []menu.CommandOption{
		{
			Command:     "status",
			Description: "Check device status and firmware version",
			Function: func(args ...string) error {
				alive, err := b.IsAlive()
				if err != nil {
					fmt.Printf("echo error: %v\n", err)
				} else if alive {
					fmt.Println("BBTKv3 is alive")
				} else {
					fmt.Println("BBTKv3 not responding to ECHO")
				}
				fw, err := b.GetFirmwareVersion()
				if err != nil {
					fmt.Printf("firmware version error: %v\n", err)
				} else {
					fmt.Printf("Firmware: %s\n", fw)
				}
				return nil
			},
		},
		{
			Command:     "info",
			Description: "Display copyright/firmware info on device LCD",
			Function: func(args ...string) error {
				b.DisplayInfoOnBBTK()
				return nil
			},
		},
		{
			Command:     "thresholds",
			Description: "Enter thresholds sub-menu (get / set / adjust)",
			Function:    thresholdsMenu,
		},
		{
			Command:     "smoothing",
			Description: "Enter smoothing sub-menu (set / default)",
			Function:    smoothingMenu,
		},
		{
			Command:     "capture",
			Description: "Enter capture sub-menu (run / clear)",
			Function:    captureMenu,
		},
		{
			Command:     "flush",
			Description: "Flush serial output buffer",
			Function: func(args ...string) error {
				if err := b.Flush(); err != nil {
					fmt.Printf("flush error: %v\n", err)
					return nil
				}
				fmt.Println("Flushed.")
				return nil
			},
		},
		{
			Command:     "reset",
			Description: "Reset serial input/output buffers",
			Function: func(args ...string) error {
				if err := b.ResetSerialBuffers(); err != nil {
					fmt.Printf("reset error: %v\n", err)
					return nil
				}
				fmt.Println("Buffers reset.")
				return nil
			},
		},
		{
			Command:     "raw",
			Description: "Send a raw command and print all response lines (1 s silence = done): raw <CMD>",
			Function: func(args ...string) error {
				if len(args) == 0 {
					fmt.Println("usage: raw <CMD>")
					return nil
				}
				cmd := strings.Join(args, " ")
				if err := b.SendCommand(cmd); err != nil {
					fmt.Printf("send error: %v\n", err)
					return nil
				}
				// Drain all response lines until 1 second of silence.
				for {
					resp, err := b.ReadLine()
					if err != nil {
						break
					}
					fmt.Printf("< %s\n", resp)
				}
				return nil
			},
		},
		{
			Command:     "reconnect",
			Description: "Purge serial buffers and re-establish CONN (use after a bad command desynchronises the dialog)",
			Function: func(args ...string) error {
				fmt.Print("Resetting serial buffers... ")
				if err := b.ResetSerialBuffers(); err != nil {
					fmt.Printf("error: %v\n", err)
				} else {
					fmt.Println("ok")
				}
				fmt.Print("Reconnecting (CONN)... ")
				if err := b.Connect(); err != nil {
					fmt.Printf("error: %v\n", err)
					return nil
				}
				fmt.Println("ok")
				return nil
			},
		},
		{
			Command:     "sendbreak",
			Description: "Send the break character 'X' to the device (interrupts ongoing operations)",
			Function: func(args ...string) error {
				if err := b.SendBreakChar(); err != nil {
					fmt.Printf("sendbreak error: %v\n", err)
					return nil
				}
				fmt.Println("Break sent.")
				return nil
			},
		},
		{
			Command:     "inputcheck",
			Description: "Send ICHK and stream device output (type X + Enter to stop)",
			Function: func(args ...string) error {
				streamUntilBreak("ICHK")
				return nil
			},
		},
		{
			Command:     "outputcheck",
			Description: "Send OCHK and stream device output (type X + Enter to stop)",
			Function: func(args ...string) error {
				streamUntilBreak("OCHK")
				return nil
			},
		},
	}

	menu.NewMenu(mainCmds, menu.NewMenuOptions("ibbtk> ", 0)).Start()
}
