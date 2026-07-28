// Interface to [Black Box Toolkit BBTKv3](https://www.blackboxtoolkit.com/bbtkv3.html)
// Author: Christophe Pallier <christophe@pallier.org>
// LICENSE: GPL-3.0

package bbtkv3

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"strconv"

	"os"
	"path/filepath"
	"strings"
	"time"

	"go.bug.st/serial"
	"golang.org/x/term"
)

// Variables to be passed on the compilation command line with "-X main.Version=${VERSION} -X main.Build=${BUILD}"
var (
	Version string
	Build   string
)

// default parameters
var (
	verbose = false
	DEBUG   = false
)

type bbtkv3 struct {
	port   serial.Port
	reader *bufio.Reader
}

// init checks if the DEBUG environment variable is set.
func init() {
	if _, ok := os.LookupEnv("DEBUG"); ok {
		DEBUG = true
		log.Println("DEBUG mode enabled.")
	} else {
		DEBUG = false
	}
}

func GetPortFromEnv() string {
	if val, ok := os.LookupEnv("BBTK_PORT"); ok {
		return val
	} else {
		return ""
	}
}

// bbtkByIDGlob matches the udev by-id symlink the BBTK presents on Linux, e.g.
// /dev/serial/by-id/usb-BBTK_BBTK_BBTK_V3_BBTKBBTKV3-if00-port0. On platforms
// without /dev/serial/by-id the glob simply matches nothing.
const bbtkByIDGlob = "/dev/serial/by-id/*BBTK*"

// ResolvePort returns the serial port to talk to, or "" if it cannot tell.
//
// Order: BBTK_PORT, then the by-id symlink. The latter is derived from the
// device's USB descriptors, so unlike /dev/ttyUSBn — which is handed out in
// enumeration order — it survives replugging and power-cycling the box. That
// matters in practice: numbering shifts exactly when you have just rebooted a
// wedged device and least want to go hunting for its new name.
//
// Preferred over scanning (as bbtk-detect-port does), which has to open every
// serial port and send CONN to it, disturbing whatever else is attached.
//
// Callers keep their own final fallback, so behaviour is unchanged when neither
// source yields anything.
func ResolvePort() string {
	if p := GetPortFromEnv(); p != "" {
		return p
	}
	matches, err := filepath.Glob(bbtkByIDGlob)
	if err != nil || len(matches) == 0 {
		return ""
	}
	if len(matches) > 1 && verbose {
		fmt.Printf("note: %d BBTK devices found, using %s\n", len(matches), matches[0])
	}
	return matches[0]
}

// NewBbtkv3 creates a new bbtkv3 object, connecting to the serial device at portAddress.
func NewBbtkv3(portAddress string, baudrate int, verbose_flag bool) (*bbtkv3, error) {
	var box bbtkv3

	verbose = verbose_flag

	mode := &serial.Mode{
		BaudRate: baudrate,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	if verbose {
		fmt.Printf("Trying to open %v at %d bps...\n", portAddress, baudrate)
	}

	port, err := serial.Open(portAddress, mode)
	if err != nil {
		return nil, fmt.Errorf("error while trying to open %s (at %d bps): %w (Under Linux, try `sudo modprobe ftdi_sio`)", portAddress, baudrate, err)
	}

	if verbose {
		fmt.Println("ok!")
	}

	port.SetReadTimeout(time.Second)
	// port.SetDTR(false)
	// port.SetRTS(false)

	box.port = port
	box.reader = bufio.NewReader(port)

	return &box, nil
}

// Connect initiates a connection to the BBTK.
func (b *bbtkv3) Connect() error {

	if verbose {
		fmt.Println("Trying to connect to BBTK...")
	}

	b.SendCommand("CONN")

	time.Sleep(100. * time.Millisecond)

	resp, err := b.ReadLine()
	if err != nil {
		return err
	}
	if resp != "BBTK;" {
		return fmt.Errorf("Connect: expected \"BBTK;\", got \"%v\"", resp)
	}

	if verbose {
		fmt.Println("ok!")
	}
	return nil
}

// Disconnect closes the connection to the bbtkv3.
func (b *bbtkv3) Disconnect() error {
	//b.SendBreak()
	return b.port.Close()
}

// SendBreak send a serial break to the bbtk. Useful on the bbtkv2 when the box is stucked, but HARMFUL on the bbtkv3 !!! So disabled.
func (b *bbtkv3) SendBreak() {
	//if DEBUG {
	//	log.Println("Sending serial break.")
	//}
	//b.port.Break(10. * time.Millisecond)
	time.Sleep(time.Second)
}

// SendBreakChar sends the ASCII character 'X' to the BBTK without any suffix.
// This is the BBTKv3 mechanism for interrupting ongoing device operations such
// as ICHK or OCHK.
func (b *bbtkv3) SendBreakChar() error {
	if DEBUG {
		log.Println("SendBreakChar: sending 'X'")
	}
	_, err := b.port.Write([]byte("X"))
	return err
}

// ResetSerialBuffers purges the input and output buffers of the serial port.
func (b *bbtkv3) ResetSerialBuffers() error {
	if err := b.port.ResetInputBuffer(); err != nil {
		return err
	}

	return b.port.ResetOutputBuffer()
}

// SendCommand adds CRLF to cmd and send it to the BBTK
func (b *bbtkv3) SendCommand(cmd string) error {

	if DEBUG {
		log.Printf("SendCommand: \"%v\"\n", cmd)
	}

	_, err := b.port.Write([]byte(cmd + "\r\n"))

	time.Sleep(50. * time.Millisecond)

	return err
}

// ReadLine returns the next line output by the BBTK
func (b *bbtkv3) ReadLine() (string, error) {
	var s string
	var err error
	if s, err = b.reader.ReadString('\n'); err != nil {
		return "", fmt.Errorf("in Readline(): %w", err)
	}

	if DEBUG {
		log.Printf("In Readline(), got \"%s\"\n", s[:len(s)-1])
	}
	return s[:len(s)-1], err
}

// IsAlive sends an 'ECHO' command to the bbtkv3 and expects 'ECHO' in return.
// This permits to check that the bbtkv3 is up and running.
func (b *bbtkv3) IsAlive() (bool, error) {

	if err := b.SendCommand("ECHO"); err != nil {
		return false, fmt.Errorf("IsAlive: %w", err)
	} else {
		resp, err := b.ReadLine()
		if err != nil {
			return false, fmt.Errorf("IsAlive: %w", err)
		}

		if resp != "ECHO" {
			return false, fmt.Errorf("IsAlive: Expected \"ECHO\", Got \"%v\"", resp)
		} else {
			return true, nil
		}
	}

}

// SetSmoothing on Opto and Mic sensors.
// When smoothing is 'off', the BBTK will detect *all* leading edges, e.g.
// each refresh on a CRT.
// When smoothing is 'on', you need to subtract 20ms from offset times.
func (b *bbtkv3) SetSmoothing(mask SmoothingMask) error {
	if err := b.SendCommand("SMOO"); err != nil {
		return fmt.Errorf("SetSmoothing: %w", err)
	}

	strMask := ""

	if mask.Mic1 {
		strMask += "1"
	} else {
		strMask += "0"
	}

	if mask.Mic2 {
		strMask += "1"
	} else {
		strMask += "0"
	}

	if mask.Opto4 {
		strMask += "1"
	} else {
		strMask += "0"
	}

	if mask.Opto3 {
		strMask += "1"
	} else {
		strMask += "0"
	}

	if mask.Opto2 {
		strMask += "1"
	} else {
		strMask += "0"
	}

	if mask.Opto1 {
		strMask += "1"
	} else {
		strMask += "0"
	}

	strMask += "11"

	err := b.SendCommand(strMask)
	if err != nil {
		return fmt.Errorf("SetSmoothing: %w", err)
	}
	return nil
}

// FLUS command attempts to clear the USB output buffer.
// If this fails you may need to send a Serial Break with SendBreak().
func (b *bbtkv3) Flush() error {
	if err := b.SendCommand("FLUS"); err != nil {
		return err
	}
	time.Sleep(time.Second)
	return nil
}

// Retrieves the version of the BBTK firmware
// currently running in the ARM chip.
func (b *bbtkv3) GetFirmwareVersion() (string, error) {
	if err := b.SendCommand("FIRM"); err != nil {
		return "", fmt.Errorf("GetFirmwareVersion: %w", err)
	}
	resp, err := b.ReadLine()
	if err != nil {
		return "", fmt.Errorf("GetFirmwareVersion: %w", err)
	}
	return resp, nil
}

func str2uint8(s string) uint8 {
	num, err := strconv.ParseUint(s, 10, 8)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	return uint8(num)
}

func (b *bbtkv3) GetThresholds() (Thresholds, error) {
	if err := b.SendCommand("GEPV"); err != nil {
		return Thresholds{}, fmt.Errorf("GetThresholds: %w", err)
	}
	resp, err := b.ReadLine()
	if err != nil {
		return Thresholds{}, fmt.Errorf("GetThresholds: %w", err)
	}
	if DEBUG {
		fmt.Println(resp)
	}
	x, err := ThresholdsFromString(resp)
	if err != nil {
		return Thresholds{}, fmt.Errorf("GetThresholds: %w", err)
	}
	return x, nil
}

// Sets the sensor activation thresholds for the eight
// adjustable lines, i.e. Mic activation threshold,
// Sounder volume (amplitude) and Opto luminance
// activation threshold. Activation thresholds range
// from 0-127.
func (b *bbtkv3) SetThresholds(x Thresholds) error {
	cmds := []string{
		"SEPV",
		fmt.Sprintf("%d", x.Mic1),
		fmt.Sprintf("%d", x.Mic2),
		fmt.Sprintf("%d", x.Sounder1),
		fmt.Sprintf("%d", x.Sounder2),
		fmt.Sprintf("%d", x.Opto1),
		fmt.Sprintf("%d", x.Opto2),
		fmt.Sprintf("%d", x.Opto3),
		fmt.Sprintf("%d", x.Opto4),
	}
	for _, cmd := range cmds {
		if err := b.SendCommand(cmd); err != nil {
			return fmt.Errorf("SetThresholds: %w", err)
		}
	}
	time.Sleep(1 * time.Second)
	return nil
}

// AdjustThresholds launches the procedure to manually set up the thresholds on the BBTK
func (b *bbtkv3) AdjustThresholds() {
	b.SendCommand("AJPV")
	response, _ := b.ReadLine()
	for response != "Done;" {
		if DEBUG {
			fmt.Printf("Adjusting Threshold: expecting \"DONE;\", got \"%v\"", response)
		}
		time.Sleep(100. * time.Millisecond)
		response, _ = b.ReadLine()
	}
}

// ClearTimingData either formats the whole of the BBTK's internal
// RAM (on first power up or after a reset) or erases
// only previously used sectors.
func (b *bbtkv3) ClearTimingData() error {
	if err := b.SendCommand("SPIE"); err != nil {
		return fmt.Errorf("ClearTimingData: %w", err)
	}

	response, err := b.ReadLine()
	if err != nil {
		return fmt.Errorf("ClearTimingData reading first response: %w", err)
	}
	if response != "FRMT;" && response != "ESEC;" {
		log.Printf("Warning: ClearTimingData expected \"FRMT;\" or \"ESEC;\", got %q", response)
	}

	response, err = b.ReadLine()
	if err != nil {
		return fmt.Errorf("ClearTimingData reading DONE: %w", err)
	}

	for response != "DONE;" {
		if DEBUG {
			log.Printf("ClearTimingData: waiting for DONE, got %q", response)
		}

		time.Sleep(100. * time.Millisecond)
		response, err = b.ReadLine()
		if err != nil {
			return fmt.Errorf("ClearTimingData: %w", err)
		}
	}

	time.Sleep(time.Second)
	return nil
}

// DisplayInfoOnBBTK causes the BBTK to display a copyright notice
// and release date of the firmware it is running on its LCD screen.
func (b *bbtkv3) DisplayInfoOnBBTK() {
	b.SendCommand("ABOU")
	time.Sleep(1. * time.Second)
}

// DefaultEventMarkingPattern is the default 8-row PATT payload used by EventMarking.
// Each row is "OOOOOOOOOOOOOOOOOOOO,IIIIIIIIIIIIIIII" (20 output bits, 16 input bits).
// The first two rows encode the command-event stimulus; the remaining six are padding (all 9s).
var DefaultEventMarkingPattern = [8]string{
	"00000001000000000000,0000010000000000",
	"00000000000100000000,0000100000000000",
	"99999999999999999999,9999999999999999",
	"99999999999999999999,9999999999999999",
	"99999999999999999999,9999999999999999",
	"99999999999999999999,9999999999999999",
	"99999999999999999999,9999999999999999",
	"99999999999999999999,9999999999999999",
}

// ErrCaptureAborted is returned by CaptureEvents when a capture was stopped
// early AND no data could be recovered from the device. A capture that was
// stopped early but did yield data returns that data with a nil error and an
// elapsed time shorter than the requested duration — callers should save it.
var ErrCaptureAborted = errors.New("capture aborted; no data recovered from the device")

// ReadyMarker is printed on stdout, on its own line, at the exact moment the
// device starts recording. It is the synchronisation point for an external
// stimulus program: a wrapper script launches bbtk-capture in the background,
// blocks until this line appears, and only then starts the stimulus.
//
// Nothing else in the output identifies that instant. The "Capturing events…"
// message a caller prints beforehand lands ~5.7 s early, because CaptureEvents
// still has DSCM/TIML/duration/RUDS and their pacing sleeps to go.
const ReadyMarker = "BBTK-CAPTURE-READY"

// CaptureEvents records events on the device for a specified duration.
//
// Parameters:
//   - duration: seconds to record. The DEVICE enforces this (it is sent as the
//     TIML argument); the host merely waits it out.
//   - noCountdown: suppress the per-second countdown on stdout.
//   - abort: optional; closing or sending on this channel stops the capture
//     early, as does pressing Esc when stdin is a terminal. Pass nil for none.
//
// Returns the raw device text (SDAT … EDAT), the number of seconds actually
// recorded, and an error.
//
// On an early stop the elapsed value is the true recording window, which is
// shorter than duration. Callers MUST use it rather than the requested duration
// when timestamping the end of the capture, or events still active at the stop
// are closed at the wrong time and their durations come out stretched.
//
// The function performs the following steps:
//  1. Sends "DSCM", "TIML" and the duration (in microseconds) to the device.
//  2. Sends "RUDS" — recording starts here — and prints ReadyMarker.
//  3. Waits out the duration, watching for Esc or the abort channel.
//  4. Reads data from the device until the "EDAT" marker is found.
func (b *bbtkv3) CaptureEvents(duration int, noCountdown bool, abort <-chan struct{}) (string, float64, error) {
	var err error
	time.Sleep(time.Second)
	err = b.SendCommand("DSCM")
	if err != nil {
		log.Printf("CaptureEvents: DSCM %v", err)
	}

	time.Sleep(time.Second)
	err = b.SendCommand("TIML")
	if err != nil {
		log.Printf("CaptureEvents: TIML %v", err)
	}

	time.Sleep(time.Second)
	err = b.SendCommand(fmt.Sprintf("%d", duration*1000000))
	if err != nil {
		log.Printf("CaptureEvents: %v", err)
	}

	time.Sleep(time.Second)
	time.Sleep(500 * time.Millisecond)
	err = b.SendCommand("RUDS")
	if err != nil {
		log.Printf("CaptureEvents: RUDS %v", err)
	}
	startedAt := time.Now()

	// The device is recording from this instant. Printed unconditionally — not
	// gated on verbose, and not on stdin being a terminal — because an external
	// program synchronises on it. The leading newline closes the caller's
	// progress message, which is deliberately left open.
	fmt.Printf("\n%s duration=%d\n", ReadyMarker, duration)

	waitingDuration := time.Duration(duration-1) * time.Second

	abortCh := make(chan struct{}, 1)

	// Put terminal in raw mode so Esc is detected immediately without Enter.
	// If stdin is not a terminal (e.g. piped, or </dev/null under a wrapper
	// script), MakeRaw fails and we skip keypress detection gracefully — the
	// abort channel is then the only way to stop early.
	if oldState, rawErr := term.MakeRaw(int(os.Stdin.Fd())); rawErr == nil {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
		fmt.Print("(press Esc to abort) ")
		go func() {
			buf := make([]byte, 1)
			for {
				n, err := os.Stdin.Read(buf)
				if err != nil || n == 0 {
					return
				}
				if buf[0] == 27 {
					select {
					case abortCh <- struct{}{}:
					default:
					}
					return
				}
			}
		}()
	}

	aborted := false
	for i := int(waitingDuration.Seconds()); i > 0; i-- {
		if !noCountdown {
			fmt.Printf("%d ", i)
		}
		select {
		case <-abortCh:
			aborted = true
		case <-abort:
			aborted = true
		case <-time.After(time.Second):
		}
		if aborted {
			break
		}
	}
	if !noCountdown {
		fmt.Println("0")
	}

	elapsed := time.Since(startedAt).Seconds()

	if aborted {
		// An interrupted capture is lost. The BBTK holds its timing data in
		// internal RAM and only streams it when the programmed TIML window
		// completes; there is no command that stops a run early and still hands
		// back what has been recorded so far. Stopping is therefore worth doing
		// only to leave the device idle and ready for the next capture — never
		// to salvage data — so this deliberately does not try to download.
		//
		// That is also why the capture duration must be worked out in advance:
		// a run that turns out too short cannot be extended, and one that is
		// interrupted has to be repeated from the start.
		fmt.Println("\nStopping capture: sending break to the BBTK...")
		if err := b.SendBreakChar(); err != nil {
			log.Printf("CaptureEvents: SendBreakChar: %v", err)
		}
		// Drain briefly so any bytes the device emits in response do not sit in
		// the buffer and confuse the next session's handshake. Discarded.
		b.drainPort(2 * time.Second)
		return "", elapsed, ErrCaptureAborted
	}

	fmt.Println("")
	fmt.Printf("Downloading data...")

	if DEBUG {
		fmt.Println("Waiting for data...")
	}

	// The device is about to stream, so a long idle gap here means something is
	// wrong rather than merely slow; 30 s is generous for that. The bound is on
	// idle time, not total transfer, so a large capture is not cut short.
	data, rerr := b.readCaptureData(30 * time.Second)
	if rerr != nil {
		return "", elapsed, rerr
	}

	return data, float64(duration), nil
}

// drainPort reads and discards whatever the device emits, until it has been
// quiet for the given period. Used after stopping a capture, so that stray
// bytes do not sit in the buffer and desynchronise the next session's
// handshake.
func (b *bbtkv3) drainPort(quiet time.Duration) {
	buff := make([]byte, 1024)
	var idle time.Duration
	for idle < quiet {
		n, err := b.port.Read(buff)
		if err != nil {
			return
		}
		if n == 0 {
			idle += time.Second // one port read timeout elapsed
			continue
		}
		idle = 0
	}
}

// readCaptureData reads from the device until the EDAT terminator.
//
// idleTimeout bounds how long it waits with NO bytes arriving; the total
// transfer may legitimately take much longer on a long capture, so the bound is
// deliberately on idle time. A zero idleTimeout waits indefinitely. The serial
// port carries a 1 s read timeout (set in New), which surfaces as a zero-length
// read rather than an error, and is what makes the idle accounting work.
func (b *bbtkv3) readCaptureData(idleTimeout time.Duration) (string, error) {
	text := ""
	buff := make([]byte, 1024)
	var idle time.Duration
	for {
		n, err := b.port.Read(buff)
		if err != nil {
			return text, fmt.Errorf("readCaptureData: %w", err)
		}
		if n == 0 {
			idle += time.Second // one port read timeout elapsed
			if idleTimeout > 0 && idle >= idleTimeout {
				return text, fmt.Errorf("readCaptureData: no data from the device for %v", idleTimeout)
			}
			continue
		}
		idle = 0
		text += string(buff[:n])
		// Check the accumulated text, not the current buffer: EDAT can straddle
		// two reads, and an unrewritten tail of buff would otherwise match
		// spuriously and truncate the download.
		if strings.Contains(text, "EDAT") {
			return text, nil
		}
	}
}

// EventMarking sends the event-marking program to the BBTK and runs it until
// the user presses 'x' (or 'X'), at which point it sends a break to the device.
//
// pattern must be exactly 8 rows; use DefaultEventMarkingPattern for the
// standard command-event stimulus.  A 1-second pause is inserted before every
// command (mirroring the timing in CaptureEvents) so the device has time to
// process each step.
func (b *bbtkv3) EventMarking(pattern [8]string) error {

	sequence := []string{"PDCE", "STYP", "PATT", "TIML", "0"}
	for _, row := range pattern {
		sequence = append(sequence, row)
	}
	sequence = append(sequence, "PCCR")

	for _, cmd := range sequence {
		time.Sleep(time.Second)
		if err := b.SendCommand(cmd); err != nil {
			return fmt.Errorf("EventMarking: %q: %w", cmd, err)
		}
	}

	// RUEM needs the same extra 500 ms that RUDS gets in CaptureEvents.
	time.Sleep(time.Second)
	time.Sleep(500 * time.Millisecond)
	if err := b.SendCommand("RUEM"); err != nil {
		return fmt.Errorf("EventMarking: RUEM: %w", err)
	}

	// Wait for the user to press 'x' / 'X'.
	stopCh := make(chan struct{}, 1)

	if oldState, rawErr := term.MakeRaw(int(os.Stdin.Fd())); rawErr == nil {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
		fmt.Print("Event marking running. Press Esc to stop.")
		go func() {
			buf := make([]byte, 1)
			for {
				n, err := os.Stdin.Read(buf)
				if err != nil || n == 0 {
					return
				}
				if buf[0] == 27 {
					select {
					case stopCh <- struct{}{}:
					default:
					}
					return
				}
			}
		}()
	} else {
		// stdin is not a terminal (e.g. piped): fall back to waiting for Enter.
		fmt.Println("Event marking running. Press Enter to stop.")
		go func() {
			buf := make([]byte, 1)
			for {
				n, err := os.Stdin.Read(buf)
				if err != nil || n == 0 {
					return
				}
				if buf[0] == '\n' || buf[0] == '\r' {
					select {
					case stopCh <- struct{}{}:
					default:
					}
					return
				}
			}
		}()
	}

	<-stopCh
	fmt.Println("\nStopping event marking...")

	if err := b.SendBreakChar(); err != nil {
		return fmt.Errorf("EventMarking: SendBreakChar: %w", err)
	}

	return nil
}
