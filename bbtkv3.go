// Drive a BlackBoxToolKit to capture events
// Author: Christophe Pallier <christophe@pallier.org>
// LICENSE: GPL-3.0

package bbtkv3

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.bug.st/serial"
)

// Variables to be passed on the compilation command line with "-X main.Version=${VERSION} -X main.Build=${BUILD}"
var (
	Version string
	Build   string
)

// default parameters
var (
	PortAddress    = "/dev/ttyUSB0"
	Baudrate       = 115200
	Duration       = 30
	OutputFileName = "bbtk-capture-001.dat"
	DEBUG          = false
)

type bbtkv3 struct {
	port   serial.Port
	reader *bufio.Reader
}

type Thresholds struct {
	Mic1     uint8
	Mic2     uint8
	Sounder1 uint8
	Sounder2 uint8
	Opto1    uint8
	Opto2    uint8
	Opto3    uint8
	Opto4    uint8
}

var defaultThresholds = Thresholds{
	Mic1:     0,
	Mic2:     0,
	Sounder1: 63,
	Sounder2: 63,
	Opto1:    110,
	Opto2:    110,
	Opto3:    110,
	Opto4:    110,
}

type SmoothingMask struct {
	Mic1  bool
	Mic2  bool
	Opto4 bool
	Opto3 bool
	Opto2 bool
	Opto1 bool
}

var defaultSmoothingMask = SmoothingMask{
	Mic1:  true,
	Mic2:  true,
	Opto4: false,
	Opto3: false,
	Opto2: false,
	Opto1: false,
}

func NewBbtkv3(portAddress string, baudrate int) (*bbtkv3, error) {
	var box bbtkv3

	mode := &serial.Mode{
		BaudRate: baudrate,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	if DEBUG {
		fmt.Printf("Trying to connect to %v at %dbps...", portAddress, baudrate)
	}
	port, err := serial.Open(portAddress, mode)
	if err != nil {
		return nil, fmt.Errorf("Error while trying to open bbtkv3 at %s at %d bps: %w\n", portAddress, baudrate, err)
	}

	if DEBUG {
		fmt.Println("Success!")
	}

	port.SetReadTimeout(time.Second)
	// port.SetDTR(false)
	// port.SetRTS(false)

	box.port = port
	box.reader = bufio.NewReader(port)

	return &box, nil
}

// Connect initiates a connection to the bbtkv3.
func (b bbtkv3) Connect() error {

	if DEBUG {
		fmt.Printf("Trying to connect to bbtkv3...")
	}

	b.SendCommand("CONN")

	time.Sleep(10. * time.Millisecond)

	resp, err := b.ReadLine()
	if err != nil {
		return err
	}
	if resp != "BBTK;" {
		return fmt.Errorf("Connect: expected \"BBTK;\", got \"%v\"", resp)
	}

	if DEBUG {
		fmt.Println("Success!")
	}
	return nil
}

func (b bbtkv3) Disconnect() error {
	//b.SendBreak()
	return b.port.Close()
}

// SendBreak send a serial break to the bbtk. Useful if the box is stucked.
func (b bbtkv3) SendBreak() {
	//if DEBUG {
	//	log.Println("Sending serial break.")
	//}
	//b.port.Break(10. * time.Millisecond)
	time.Sleep(time.Second)
}

func (b bbtkv3) ResetSerialBuffers() error {
	if err := b.port.ResetInputBuffer(); err != nil {
		return err
	}

	return b.port.ResetOutputBuffer()
}

// SendCommand adds CRLF to cmd and send it to the BBTK
func (b bbtkv3) SendCommand(cmd string) error {

	if DEBUG {
		log.Printf("SendCommand: \"%v\"\n", cmd)
	}

	_, err := b.port.Write([]byte(cmd + "\r\n"))

	time.Sleep(50. * time.Millisecond)

	return err
}

// ReadLine returns the next line output by the BBTK
func (b bbtkv3) ReadLine() (string, error) {
	var s string
	var err error
	if s, err = b.reader.ReadString('\n'); err != nil {
		return "", fmt.Errorf("Readline: %w", err)
	}
	if DEBUG {
		log.Printf("Readline: got \"%s\"\n", s[:len(s)-1])
	}
	return s[:len(s)-1], err
}

// IsAlive sends an 'ECHO' command to the bbtkv3 and expects 'ECHO' in return.
// This permits to check that the bbtkv3 is up and running.
func (b bbtkv3) IsAlive() (bool, error) {

	if err := b.SendCommand("ECHO"); err != nil {
		return false, fmt.Errorf("IsAlive: %w", err)
	} else {
		resp, err := b.ReadLine()
		if err != nil {
			return false, fmt.Errorf("IsAlive: %w", err)
		}

		if resp != "ECHO" {
			return true, fmt.Errorf("IsAlive: Expected \"ECHO\", Got \"%v\"", resp)
		} else {
			return true, nil
		}
	}

}

// SetSmoothing on Opto and Mic sensors.
// When smoothing is 'off', the BBTK will detect *all* leading edges, e.g.
// each refresh on a CRT.
// When smoothing is 'on', you need to subtract 20ms from offset times.
func (b bbtkv3) SetSmoothing(mask SmoothingMask) error {
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
func (b bbtkv3) Flush() error {
	if err := b.SendCommand("FLUS"); err != nil {
		return err
	}
	time.Sleep(time.Second)
	return nil
}

// Retrieves the version of the BBTK firmware
// currently running in the ARM chip.
func (b bbtkv3) GetFirmwareVersion() string {
	b.SendCommand("FIRM")
	resp, err := b.ReadLine()
	if err != nil {
		log.Printf("GetFirmWareVersion: %w", err)
	}
	return resp
}

// AdjustThresholds launches the procedure to manually set up the thresholds on the BBTK
func (b bbtkv3) AdjustThresholds() {
	b.SendCommand("AJPV")
	response, _ := b.ReadLine()
	for response != "Done;" {
		if DEBUG {
			log.Printf("Adjusting Threshold: expecting \"Done;\", got \"%v\"", response)
		}
		time.Sleep(100. * time.Millisecond)
		response, _ = b.ReadLine()
	}
}

// ClearTimingData either formats the whole of the BBTK's internal
// RAM (on first power up or after a reset) or erases
// only previously used sectors.
func (b bbtkv3) ClearTimingData() {
	b.SendCommand("SPIE")

	response, err := b.ReadLine()
	if err != nil {
		log.Fatalf("ClearTimingData: %w", err)
	}
	if response != "FRMT;" && response != "ESEC;" {
		log.Printf("Warning: ClearTimingData expected \"FRMT;\" or \"ESEC;\", got \"%v\"", response)
	}

	response, err = b.ReadLine()
	if err != nil {
		log.Fatalf("ClearTimingData: %w", err)
	}

	for response != "DONE;" {
		if DEBUG {
			log.Printf("Warning: ClearTimingData expected \"DONE;\", got \"%v\"", response)
		}

		time.Sleep(100. * time.Millisecond)
		response, err = b.ReadLine()
		if err != nil {
			log.Fatalf("ClearTimingData: %w", err)
		}
	}

	time.Sleep(time.Second)
}

// DisplayInfoOnBBTK causes the BBTK to display a copyright notice
// and release date of the firmware it is running on its LCD screen.
func (b bbtkv3) DisplayInfoOnBBTK() {
	b.SendCommand("ABOU")
	time.Sleep(1. * time.Second)
}

// Sets the sensor activation thresholds for the eight
// adjustable lines, i.e. Mic activation threshold,
// Sounder volume (amplitude) and Opto luminance
// activation threshold. Activation thresholds range
// from 0-127.
func (b bbtkv3) SetThresholds(x Thresholds) {
	b.SendCommand("SEPV")
	b.SendCommand(fmt.Sprintf("%d", x.Mic1))
	b.SendCommand(fmt.Sprintf("%d", x.Mic2))
	b.SendCommand(fmt.Sprintf("%d", x.Sounder1))
	b.SendCommand(fmt.Sprintf("%d", x.Sounder2))
	b.SendCommand(fmt.Sprintf("%d", x.Opto1))
	b.SendCommand(fmt.Sprintf("%d", x.Opto2))
	b.SendCommand(fmt.Sprintf("%d", x.Opto3))
	b.SendCommand(fmt.Sprintf("%d", x.Opto4))

	time.Sleep(1 * time.Second)
}

func (b bbtkv3) SetDefaultsThresholds() {
	b.SetThresholds(defaultThresholds)
}

// Launches a digital data capture session.
// duration in seconds
func (b bbtkv3) CaptureEvents(duration int) string {
	var err error
	time.Sleep(time.Second)
	err = b.SendCommand("DSCM")
	if err != nil {
		log.Printf("CaptureEvents: DSCM %w", err)
	}

	time.Sleep(time.Second)
	err = b.SendCommand("TIML")
	if err != nil {
		log.Printf("CaptureEvents: TIML %w", err)
	}

	time.Sleep(time.Second)
	err = b.SendCommand(fmt.Sprintf("%d", duration*1000000))
	if err != nil {
		log.Printf("CaptureEvents: %w", err)
	}

	time.Sleep(time.Second)
	time.Sleep(500 * time.Millisecond)
	err = b.SendCommand("RUDS")
	if err != nil {
		log.Printf("CaptureEvents: RUDS %w", err)
	}

	waitingDuration := time.Duration(duration-1) * time.Second
	time.Sleep(waitingDuration)

	if DEBUG {
		fmt.Println("Waiting for data...")
	}

	text := ""
	buff := make([]byte, 1024)
	for {
		n, err := b.port.Read(buff)
		if err != nil {
			log.Fatal(err)
			break
		}
		if n > 0 {
			text += string(buff[:n])
		}
		if strings.Contains(string(buff), "EDAT") {
			break
		}
	}

	return text

}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func WriteText(basename string, text string) error {
	var filename string = basename

	ext := filepath.Ext(basename)
	name := strings.TrimSuffix(basename, ext)

	for i := 2; fileExists(filename); i++ {
		filename = fmt.Sprintf("%s-%03d%s", name, i, ext)
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = f.WriteString(text)

	return err
}

