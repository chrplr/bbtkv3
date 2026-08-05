// Interface to [Black Box Toolkit BBTKv3](https://www.blackboxtoolkit.com/bbtkv3.html)
// Author: Christophe Pallier <christophe@pallier.org>
// LICENSE: GPL-3.0

package bbtkv3

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"

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

	// progressWriter receives the human-readable progress text the verbose flag
	// controls. It sits beside verbose, and is package-level for the same
	// reason: the prints it feeds are scattered across functions that have no
	// other channel back to the caller. nil means os.Stdout.
	progressWriter io.Writer
)

// SetProgressWriter redirects the library's progress text away from stdout. Pass
// os.Stderr when stdout must stay clear for another program's output — a
// stimulus launched as a child of a capture, for instance. Pass nil to restore
// the default.
func SetProgressWriter(w io.Writer) { progressWriter = w }

// ProgressWriter returns where progress text is going, resolving nil to stdout.
func ProgressWriter() io.Writer {
	if progressWriter == nil {
		return os.Stdout
	}
	return progressWriter
}

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
		fmt.Fprintf(ProgressWriter(), "note: %d BBTK devices found, using %s\n", len(matches), matches[0])
	}
	return matches[0]
}

// StablePortName returns the /dev/serial/by-id symlink pointing at dev, or dev
// itself when there is none (no /dev/serial/by-id, or a name that is already a
// symlink there). It is the inverse of what ResolvePort does: tools that find a
// device by scanning end up with a /dev/ttyUSBn name, and should report the
// stable one instead, for the same reason ResolvePort prefers it.
func StablePortName(dev string) string {
	target, err := filepath.EvalSymlinks(dev)
	if err != nil {
		return dev
	}
	links, err := filepath.Glob("/dev/serial/by-id/*")
	if err != nil {
		return dev
	}
	for _, l := range links {
		if t, err := filepath.EvalSymlinks(l); err == nil && t == target {
			return l
		}
	}
	return dev
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
		fmt.Fprintf(ProgressWriter(), "Trying to open %v at %d bps...\n", portAddress, baudrate)
	}

	port, err := serial.Open(portAddress, mode)
	if err != nil {
		return nil, fmt.Errorf("error while trying to open %s (at %d bps): %w (Under Linux, try `sudo modprobe ftdi_sio`)", portAddress, baudrate, err)
	}

	if verbose {
		fmt.Fprintln(ProgressWriter(), "ok!")
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
		fmt.Fprintln(ProgressWriter(), "Trying to connect to BBTK...")
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
		fmt.Fprintln(ProgressWriter(), "ok!")
	}
	return nil
}

// Disconnect closes the connection to the bbtkv3.
func (b *bbtkv3) Disconnect() error {
	return b.port.Close()
}

// A serial break (port.Break) is the bbtkv2 way of unwedging a stuck box and is
// HARMFUL on the bbtkv3, so this package never sends one. There used to be a
// SendBreak method here whose body was commented out, leaving an exported name
// that promised a serial break and only slept for a second; it is gone. Use
// SendBreakChar, which is the v3 mechanism.

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
// If this fails you may need to send the break character with SendBreakChar().
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

// Bytes that request a stop from a terminal held in raw mode. Esc is the
// documented key; Ctrl-C must be handled here as well because term.MakeRaw
// clears ISIG, so Ctrl-C is delivered as this byte rather than raising SIGINT.
// Without it the reader below discards Ctrl-C silently and the process has no
// keyboard escape at all — a SIGINT handler in main() never fires either,
// because no signal is ever generated.
const (
	keyEsc   = 27
	keyCtrlC = 3
)

// waitForStopKey prints prompt, then blocks until the user asks to stop: Esc or
// Ctrl-C when stdin is a terminal, Enter when it is not (piped or scripted).
// It returns the line ending that must be used for anything printed afterwards
// — raw mode clears OPOST and with it the ONLCR translation, so a bare \n would
// leave the cursor in mid-line and staircase the output. See LineEnding.
func waitForStopKey(prompt string) string {
	oldState, rawErr := term.MakeRaw(int(os.Stdin.Fd()))
	if rawErr != nil {
		// stdin is not a terminal: fall back to waiting for Enter.
		fmt.Printf("%s Press Enter to stop.\n", prompt)
		waitForByte(func(c byte) bool { return c == '\n' || c == '\r' })
		return "\n"
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	eol := LineEnding(os.Stdout, true)
	fmt.Printf("%s Press Esc or Ctrl-C to stop.%s", prompt, eol)
	waitForByte(func(c byte) bool { return c == keyEsc || c == keyCtrlC })
	return eol
}

// waitForByte reads stdin one byte at a time until stop says so, or until the
// stream ends.
func waitForByte(stop func(byte) bool) {
	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return
		}
		if stop(buf[0]) {
			return
		}
	}
}

// CaptureOptions tunes a single call to CaptureEvents. The zero value is the
// interactive default: countdown on, keyboard abort on, progress on stdout.
type CaptureOptions struct {
	// NoCountdown suppresses the per-second countdown.
	NoCountdown bool

	// NoKeyAbort skips raw terminal mode entirely, so Esc and Ctrl-C are not
	// watched for. Set it when another program shares this terminal: raw mode
	// clears ISIG and ONLCR process-wide, which kills that program's Ctrl-C and
	// staircases its line-oriented output.
	NoKeyAbort bool

	// Progress receives the human-readable progress text, including ReadyMarker.
	// nil means os.Stdout. Point it at os.Stderr to keep stdout clear for a
	// child process's own output.
	Progress io.Writer

	// Abort stops the capture early when closed or sent on. nil means none.
	Abort <-chan struct{}

	// OnRecording, if set, is called once, synchronously, at the instant the
	// device starts recording — the same instant ReadyMarker is written. It is
	// the in-process equivalent of watching for that marker, for a caller that
	// starts a stimulus itself rather than being driven by a wrapper script.
	//
	// It runs on the capture's own goroutine, immediately before the wait loop,
	// so it must not block: whatever it delays is recorded as dead time at the
	// head of the capture window.
	OnRecording func()
}

// progress returns the writer to report to, defaulting to the package-wide one.
func (o CaptureOptions) progress() io.Writer {
	if o.Progress == nil {
		return ProgressWriter()
	}
	return o.Progress
}

// maybeMakeRaw puts stdin in raw mode unless skip is set, returning the state to
// restore. A nil state with a nil error means raw mode was not entered — either
// because it was skipped or because stdin is not a terminal. Keeping the guard
// in front of the call is what stops a skipped caller from silently leaving the
// terminal raw.
func maybeMakeRaw(skip bool) (*term.State, error) {
	if skip {
		return nil, nil
	}
	return term.MakeRaw(int(os.Stdin.Fd()))
}

// LineEnding returns the terminator to end a line with on w while stdin is in
// raw mode.
//
// term.MakeRaw clears OPOST, and with it the ONLCR translation that turns \n
// into CR-LF. A bare \n then moves the cursor down a line without returning it
// to column 0, so successive lines walk off to the right in a growing
// staircase. Restoring the carriage return by hand is the fix.
//
// It is deliberately not enough to know that raw mode is on: stdin can be a
// terminal while the output is redirected to a file, and there a CR would be
// junk in the data. So the CR is added only when w is itself a terminal.
func LineEnding(w io.Writer, rawMode bool) string {
	if !rawMode {
		return "\n"
	}
	if f, ok := w.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		return "\r\n"
	}
	return "\n"
}

// CaptureEvents records events on the device for a specified duration.
//
// duration is in seconds. The DEVICE enforces it (it is sent as the TIML
// argument); the host merely waits it out. See CaptureOptions for the rest.
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
func (b *bbtkv3) CaptureEvents(duration int, opts CaptureOptions) (string, float64, error) {
	out := opts.progress()
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
	fmt.Fprintf(out, "\n%s duration=%d\n", ReadyMarker, duration)

	if opts.OnRecording != nil {
		opts.OnRecording()
	}

	waitingDuration := time.Duration(duration-1) * time.Second

	abortCh := make(chan struct{}, 1)

	// Put terminal in raw mode so Esc is detected immediately without Enter.
	// If stdin is not a terminal (e.g. piped, or </dev/null under a wrapper
	// script), MakeRaw fails and we skip keypress detection gracefully — the
	// abort channel is then the only way to stop early.
	//
	// NoKeyAbort must be tested BEFORE calling MakeRaw, not alongside its error:
	// the call has already changed the terminal by the time the condition is
	// evaluated, and the matching Restore is deferred inside the branch.
	// Every line printed between here and the deferred Restore below needs eol
	// rather than a bare \n; see LineEnding.
	eol := "\n"
	if oldState, rawErr := maybeMakeRaw(opts.NoKeyAbort); rawErr == nil && oldState != nil {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
		eol = LineEnding(out, true)
		fmt.Fprint(out, "(press Esc or Ctrl-C to abort) ")
		go func() {
			buf := make([]byte, 1)
			for {
				n, err := os.Stdin.Read(buf)
				if err != nil || n == 0 {
					return
				}
				if buf[0] == keyEsc || buf[0] == keyCtrlC {
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
		if !opts.NoCountdown {
			fmt.Fprintf(out, "%d ", i)
		}
		select {
		case <-abortCh:
			aborted = true
		case <-opts.Abort:
			aborted = true
		case <-time.After(time.Second):
		}
		if aborted {
			break
		}
	}
	if !opts.NoCountdown {
		fmt.Fprintf(out, "0%s", eol)
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
		fmt.Fprintf(out, "%sStopping capture: sending break to the BBTK...%s", eol, eol)
		if err := b.SendBreakChar(); err != nil {
			log.Printf("CaptureEvents: SendBreakChar: %v", err)
		}
		// Drain briefly so any bytes the device emits in response do not sit in
		// the buffer and confuse the next session's handshake. Discarded.
		b.drainPort(2 * time.Second)
		return "", elapsed, ErrCaptureAborted
	}

	fmt.Fprint(out, eol)
	fmt.Fprintf(out, "Downloading data...")

	if DEBUG {
		fmt.Fprintf(out, "Waiting for data...%s", eol)
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

	eol := waitForStopKey("Event marking running.")
	fmt.Printf("%sStopping event marking...%s", eol, eol)

	if err := b.SendBreakChar(); err != nil {
		return fmt.Errorf("EventMarking: SendBreakChar: %w", err)
	}

	return nil
}
