// Interface to [Black Box Toolkit BBTKv3](https://www.blackboxtoolkit.com/bbtkv3.html)
// Author: Christophe Pallier <christophe@pallier.org>
// LICENSE: GPL-3.0

package bbtkv3

import (
	"bufio"
	"fmt"
	"log"
	"strconv"

	"os"
	//"path/filepath"
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
	verbose = false
	DEBUG   = false
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
	Opto2: true,
	Opto1: true,
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

// Connect initiates a connection to the bbtkv3.
func (b bbtkv3) Connect() error {

	if verbose {
		fmt.Println("Trying to connect to bbtkv3...")
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
func (b bbtkv3) Disconnect() error {
	//b.SendBreak()
	return b.port.Close()
}

// SendBreak send a serial break to the bbtk. Useful on the bbtkv2 when the box is stucked, but HARMFUL on the bbtkv3 !!! So disabled.
func (b bbtkv3) SendBreak() {
	//if DEBUG {
	//	log.Println("Sending serial break.")
	//}
	//b.port.Break(10. * time.Millisecond)
	time.Sleep(time.Second)
}

// ResetSerialBuffers purges the input and output buffers of the serial port.
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
		return "", fmt.Errorf("in Readline(): %w", err)
	}

	if DEBUG {
		log.Printf("In Readline(), got \"%s\"\n", s[:len(s)-1])
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
		fmt.Printf("In GetFirmWareVersion(): %v", err)
	}
	return resp
}

func str2uint8(s string) uint8 {
	num, err := strconv.ParseUint(s, 10, 8)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	return uint8(num)
}

func (b bbtkv3) GetThresholds() Thresholds {
	b.SendCommand("GEPV")
	resp, err := b.ReadLine()
	if err != nil {
		fmt.Printf("In GetThresholds(): %v", err)
	}
	if DEBUG {
		fmt.Println(resp)
	}
	vals := strings.Split(resp, ",")
	var x Thresholds
	/* x := Thresholds{str2uint8(vals[0]),   TODO!!
		str2uint8(vals[1]),
		str2uint8(vals[2]),
		str2uint8(vals[3]),
		str2uint8(vals[4]),
		str2uint8(vals[5]),
		str2uint8(vals[6]),
		str2uint8(vals[7])}
	if DEBUG {
		fmt.Println("+v", x)
	}
	*/
	fmt.Printf("%v", vals)

	return x
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

//func (b bbtkv3) SetDefaultsThresholds() {
//	b.SetThresholds(defaultThresholds)
//}

// AdjustThresholds launches the procedure to manually set up the thresholds on the BBTK
func (b bbtkv3) AdjustThresholds() {
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
func (b bbtkv3) ClearTimingData() {
	b.SendCommand("SPIE")

	response, err := b.ReadLine()
	if err != nil {
		fmt.Printf("ClearTimingData: %v", err)
	}
	if response != "FRMT;" && response != "ESEC;" {
		fmt.Printf("Warning: ClearTimingData expected \"FRMT;\" or \"ESEC;\", got \"%v\"", response)
	}

	response, err = b.ReadLine()
	if err != nil {
		log.Fatalf("ClearTimingData @ call ReadLine(): %v", err)
	}

	for response != "DONE;" {
		if DEBUG {
			fmt.Printf("Warning: ClearTimingData expected \"DONE;\", got \"%v\"", response)
		}

		time.Sleep(100. * time.Millisecond)
		response, err = b.ReadLine()
		if err != nil {
			log.Fatalf("ClearTimingData: %v", err)
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

// Launches a digital data capture session.
// duration in seconds
func (b bbtkv3) CaptureEvents(duration int) string {
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
