Command-line interface for the Black Box ToolKit v3
===================================================

The [Black Box ToolKit](https://www.blackboxtoolkit.com/bbtkv3.html)  is a device that allows psychologists to measure the timing of audio-visual stimuli with sub-millisecond accuracy. It replaces a digital oscilloscope, capturing activity on sound and visual sensors and TTL signals, and a signal generator,
 generating sounds or TTL signals.

This page describes a set of command-line tools that streamline the testing of time-critical psychology experiments:

| Tool | Description |
|------|-------------|
| `bbtk-detect-port` | Scans serial ports to locate the connected BBTK |
| `bbtk-capture` | Captures events for a given duration and exports them to `.dat`, `.dscevents.csv`, and `.events.csv` files |
| `bbtk-input-check` | Streams live input state from the device (ICHK); press `x` to stop |
| `bbtk-event-marking` | Configures and runs the command-event marking program on the device; press `x` to stop |
| `bbtk-adjust-thresholds` | Opens the interactive sensor threshold adjustment menu on the device |
| `bbtk-get-thresholds` | Reads and prints the current sensor thresholds |
| `bbtk-set-thresholds` | Writes eight threshold values to the device |
| `bbtk-set-smoothing` | Configures sensor smoothing |
| `bbtk-send-command` | Reads raw protocol commands from stdin, sends each to the BBTK, and prints responses to stdout |
| `get-serial-port-list` | Lists all available serial ports on the host machine |
| `ibbtk` | Interactive menu-driven shell — keeps a persistent connection and exposes all of the above in a nested menu |
| `events-stats` | Computes descriptive statistics and ASCII histograms from the `.events.csv` files produced by `bbtk-capture` |


Binaries for different operating systems are available at <https://github.com/chrplr/bbtkv3/releases>,

The source code (under a GPL-3.0 License) at <https://github.com/chrplr/bbtkv3>

This program relies on a Go module, `github.com/chrplr/bbtkv3`, which encapsulates a small subset of the commands documented in *The BBTKv2 API Guide* (in the future, we might implement more functions). This go module can be used to drive the BBTK from programs written in Go.


# Principle of operation

![](images/bbtkv3.jpg)

To operate, three pieces of equipement are needed:

1. A stimulation device (typically a computer, but not necessarily) 
2. The BBTK with input sensors (photodiodes, sound detectors, TTL detectors) linked to the stimulation device.
3. A host computer driving the BBTK (linked to it via a USB cable).

| :point_up:  The stimulation PC and the host PC *can* be the same computer |
|---------------------------------------------------------------------------| 

As data are recorded asynchronously by the BBTKvr3, it is possible for a single PC to switch the BBTKv2 into “capture mode”, launch the stimulation program and, when done, download the timing data from the BBTKv3 memory.


# Usage

The tools are meant to be ran on the command line. You must therefore open a Terminal to execute them (e.g., under Windows, start `cmd` or `Powershell`). 

Provided the tools are in the PATH (see below), you can just type:

```bash
$ bbtk-detect-port
BBTK found at  COM4
$ bbtk-adjust-thresholds -p COM4
$ bbtk-capture -p COM4 -d 120
... 
``` 

To launch a 2min acquisition. 
When completed, `.dat` and `.events.csv` files will contain the information about detected events.


```bash
bbtk-capture -h
```
 will yield some help:

```
Usage of bbtk-capture:
  -D	Debug mode
  -V	Display version
  -b int
    	baudrate (speed in bps) (default 115200)
  -d int
    	duration of capture (in s) (default 30)
  -o string
    	output file name for captured data (default "bbtk-capture.dat")
  -p string
    	device (serial port name) (default "/dev/ttyUSB0")
```

During the countdown, press `x` (no Enter needed) to abort the capture early. The program sends a stop command to the device and exits cleanly.



# Installation

Compiled versions for MACOSX, Windows and Linux, and intel (amd64) or arm are available at <https://github.com/chrplr/bbtkv3/releases>.

Get the versions for your OS and architecture, rename them to your liking (I would strip the OS-PLATFORM-VERSION), and copy them to some folder listed in the `PATH` variable of your OS. 

| :zap: Windows |
|---------------|

In the command line terminal application, CMD, type:

```bash
cd Downloads

% rename the executable
ren bbtk-capture-windows-arm64-1.0.1  bbtk-capture.exe
ren bbtk-adjust-thresholds-windows-arm64-1.0.1  bbtk-adjust-thresholds.exe
```

Then copy the new`*.exe` files into a folder, say /home/user/bin, and add this folder to the system's PATH environment variable (see <https://www.eukhost.com/kb/how-to-add-to-the-path-on-windows-10-and-windows-11/>).

Now , when you launch CMd, you should be able to execute any of these program by typing its name and pressing 'Enter'.


| :zap: MacOS X |
|---------------|

Assuming that you downloaded the programs in `~/Downloads` and want to install them in `~/bin`:


```zsh
mkdir -p ~/bin
cd ~/Downloads
for f in bbtk*; do chmod +x $f; mv $f ~/bin/${f%-linux-amd64-1.0.1}; done
```

(replace the version number by the current one)


| :zap: Linux |
|-------------|

Make sure that you are part of the `dialout` group. 

```
sudo usermod -a -G dialout $USER
```

Assuming that you downloaded the programs in `~/Downloads` and want to install them in `~/bin`:

```bash
mkdir -p ~/bin
cd ~/Downloads
for f in bbtk*; do chmod +x $f; mv $f ~/bin/${f%-linux-amd64-1.0.1}; done

#run
bbtk -p /dev/ttypACM0
```

(replace the version number by the current one)
# ibbtk — interactive shell

`ibbtk` is an interactive menu-driven shell for communicating with the BBTKv3. Rather than running separate commands, it keeps a persistent connection open and lets you issue commands one by one.

## Starting ibbtk

```bash
ibbtk -p /dev/ttyUSB0        # Linux
ibbtk -p COM4                 # Windows
ibbtk -p /dev/cu.usbserial-BBTKXXXX   # macOS
```

The port can also be set via the `BBTK_PORT` environment variable, in which case `-p` can be omitted.

```
Options:
  -p string   serial port (or set BBTK_PORT)
  -b int      baudrate (default 115200)
  -v          verbose connection output
  -V          display version and exit
```

## Command structure

Once connected, `ibbtk` presents a prompt. Type `menu` at any prompt to list available commands, and `exit` to leave the current level (or quit the program from the top level).

### Top-level commands

| Command | Description |
|---------|-------------|
| `status` | Check whether the device is alive and show firmware version |
| `info` | Display copyright/firmware info on the device LCD |
| `thresholds` | Enter the thresholds sub-menu |
| `smoothing` | Enter the smoothing sub-menu |
| `capture` | Enter the capture sub-menu |
| `inputcheck` | Send `ICHK` and stream input-state lines (type `X` + Enter to stop) |
| `outputcheck` | Send `OCHK` and stream output-state lines (type `X` + Enter to stop) |
| `flush` | Flush the serial output buffer |
| `reset` | Reset serial input/output buffers |
| `raw <CMD>` | Send a raw protocol command and print all response lines (stops after 1 s of silence) |
| `reconnect` | Purge serial buffers and re-issue `CONN` — use this to recover after a bad command desynchronises the dialog |
| `sendbreak` | Send the break character `X` to the device (interrupts ongoing operations) |

### `thresholds` sub-menu

| Command | Description |
|---------|-------------|
| `get` | Read the current thresholds from the device |
| `set <values>` | Set all eight thresholds, e.g. `set 63,63,32,32,100,100,100,100` (Mic1, Mic2, Sounder1, Sounder2, Opto1–4; range 0–127) |
| `adjust` | Launch the interactive threshold adjustment procedure on the device |

### `smoothing` sub-menu

| Command | Description |
|---------|-------------|
| `set <mask>` | Set the smoothing mask, e.g. `set 1;1;0;0;1;1` (fields: Mic1;Mic2;Opto4;Opto3;Opto2;Opto1) |
| `default` | Enable smoothing on all sensors |

Beware: When smoothing is on for a given input line, one must subtract 20ms to durations reported by the bbtk for this input line.

### `capture` sub-menu

| Command | Description |
|---------|-------------|
| `run <seconds> [output.dat]` | Clear device memory, capture for the given duration, and save `.dat`, `.dscevents.csv`, and `.events.csv` files (default output name: `ibbtk-capture.dat`) |
| `clear` | Erase the device timing memory |

## Example session

```
$ ibbtk -p /dev/ttyUSB0
Connected to BBTKv3. Type 'menu' for commands, 'exit' to quit.
ibbtk> status
BBTKv3 is alive
Firmware: 3.14
ibbtk> thresholds
thresholds> get
{Mic1:63 Mic2:63 Sounder1:32 Sounder2:32 Opto1:100 Opto2:100 Opto3:100 Opto4:100}
thresholds> set 63,63,32,32,80,80,80,80
Thresholds now: {Mic1:63 Mic2:63 Sounder1:32 Sounder2:32 Opto1:80 Opto2:80 Opto3:80 Opto4:80}
thresholds> exit
Exiting...
ibbtk> capture
capture> run 30 myexp.dat
Clearing timing data... ok
Capturing for 30 seconds...
Raw data saved to myexp.dat
DSC events saved to myexp.dscevents.csv
Events saved to myexp.events.csv (12 events detected)
capture> exit
Exiting...
ibbtk> exit
Exiting...
```

# bbtk-input-check — stream live input state

`bbtk-input-check` sends the `ICHK` command to the device and continuously prints every line returned, showing the state of the 12 input lines (Keypad1–4, Opto1–4, TTLin1–2, Mic1–2) in real time. Press `x` (no Enter needed) to stop; the program sends the break character `X` to the device and exits.

```
Options:
  -p string   serial port (or set BBTK_PORT)
  -b int      baudrate (default 115200)
  -V          display version and exit
```

## Example

```bash
$ bbtk-input-check -p /dev/ttyUSB0
Streaming input state. Press 'x' to stop.
000000000000;
000000010000;
000000000000;
000000010000;
x
Stopping...
```

Each line is a 12-bit snapshot of the input ports, terminated with `;`. A `1` in a position means that sensor is currently active.

# bbtk-event-marking — run the command-event marking program

`bbtk-event-marking` programs the BBTK to operate in event-marking mode: it sends the `PDCE / STYP / PATT / TIML` setup sequence together with the pattern table, commits with `PCCR`, and starts execution with `RUEM`. The device then runs indefinitely, marking command events as they occur. Press `x` (no Enter needed) to stop; the program sends `X` to the device and exits.

A one-second pause is inserted before each protocol command (matching the timing used by `bbtk-capture`) to give the device time to process each step.

```
Options:
  -p string   serial port (or set BBTK_PORT)
  -b int      baudrate (default 115200)
  -V          display version and exit
```

# bbtk-send-command — pipe raw commands to the device

`bbtk-send-command` reads commands from stdin (one per line), sends each to the BBTK as a raw protocol command, and prints the device's responses to stdout. No handshake is performed automatically, so you have full control over the command sequence.

```
Options:
  -p string   serial port (or set BBTK_PORT)
  -b int      baudrate (default 115200)
  -timeout int  seconds of silence after last response line before moving to
                the next command (default 1)
  -V          display version and exit
```

## Examples

```bash
# Check that the device echoes back
echo "ECHO" | bbtk-send-command -p /dev/ttyUSB0

# Full handshake then query firmware version
printf "CONN\nFIRM\n" | bbtk-send-command -p /dev/ttyUSB0

# Use the BBTK_PORT environment variable
printf "CONN\nECHO\n" | BBTK_PORT=/dev/ttyUSB0 bbtk-send-command -timeout 2
```

After sending each command the tool waits up to `-timeout` seconds for the device to stop replying before sending the next command. Increase `-timeout` for commands that trigger longer device operations.

# events-stats — descriptive statistics on captured events

`events-stats` reads one or more `.events.csv` files produced by `bbtk-capture` and prints three blocks of statistics, each followed by an ASCII histogram:

1. **Duration statistics** — distribution of event durations for each sensor channel.
2. **Inter-onset interval (jitter) statistics** — distribution of the time between successive events of the same type. This measures the regularity (jitter) of the stimulus presentation.
3. **Paired-event onset differences** — for each occurrence of a reference event (default: `TTLin1`), the tool finds the nearest following event of a second type (default: `Opto1`) and reports the distribution of those onset differences. This is the main measure of audio-visual latency: it tells you how long after a TTL trigger the corresponding visual or audio event was actually detected by the sensor.

All time values are in milliseconds.

## Usage

```
events-stats [-event1 TYPE] [-event2 TYPE] [-detect-outliers K] file1.events.csv [file2.events.csv ...]
```

Multiple files are pooled together before computing statistics, which is useful when you have repeated capture sessions.

## Options

| Flag | Default | Description |
|------|---------|-------------|
| `-event1 TYPE` | `TTLin1` | Reference event type for the paired-difference analysis |
| `-event2 TYPE` | `Opto1` | Target event type for the paired-difference analysis (difference = `event2.Onset − event1.Onset`) |
| `-detect-outliers MS` | `0` | Exclude data points more than MS milliseconds away from the median. A warning line is printed for each type where outliers are removed. Set to `0` (the default) to disable outlier filtering. |

## Output format

Each of the three sections contains a table with the following columns:

| Column | Description |
|--------|-------------|
| `Type` | Event type (sensor channel name) |
| `N` | Number of data points (after outlier removal) |
| `Min` / `P10` … `P90` / `Max` | Percentiles 0 %, 10 %, … 100 % |
| `Range` | Max − Min |
| `P99.5-P0.5` | Near-full spread (robust range) |
| `P95-P05` | 90 % central interval |
| `SD` | Sample standard deviation (Bessel-corrected) |

Below each table, a 10-bin ASCII histogram is printed for every event type. If outliers were removed, a warning line of the form

```
Warning: N outliers detected in TYPE (> D.DDD ms away from the median): V1, V2, ...
```

is printed between the table and the histograms.

## Example

```
$ events-stats -event1 TTLin1 -event2 Opto1 -detect-outliers 10 bbtk-capture-001.events.csv

=== Duration Statistics (ms) ===

Type    N   Min     P10     ...  Max     Range  P99.5-P0.5  P95-P05  SD
------  --  ------  ------  ...  ------  -----  ----------  -------  -----
TTLin1  50   1.000   1.000  ...   1.250  0.250       0.240    0.200  0.050
Opto1   49  513.250 513.650 ...  514.500 1.250       1.160    0.750  0.240
Warning: 1 outliers detected in Opto1 (> 10.000 ms away from the median): -39547.750

  Opto1:
  histogram (10 bins):
  [ 513.250,  513.375) ms :     1  **
  [ 513.375,  513.500) ms :     0
  ...

=== Inter-Onset Interval / Jitter Statistics (ms) ===

...

=== Paired-Event Onset Differences: TTLin1 → Opto1 (ms) ===

Type           N   Min    P10    P50    P90    Max    Range  SD
-------------  --  -----  -----  -----  -----  -----  -----  -----
TTLin1→Opto1   50  12.50  12.80  13.20  13.90  14.10   1.60  0.420

  TTLin1→Opto1:
  histogram (10 bins):
  [  12.500,  12.660) ms :    3  ****
  ...
```

## Pairing algorithm

For each `event1` occurrence (sorted by onset time), `events-stats` locates the **nearest following** `event2` onset using binary search. Multiple `event1` events can pair with the same `event2` (non-exclusive), which handles edge cases where two triggers fire in rapid succession before the sensor responds.

## Typical workflow

```bash
# 1. Record events
bbtk-capture -p /dev/ttyUSB0 -d 60 -o session1.dat

# 2. Inspect statistics, excluding events more than 10 ms from the median
events-stats -detect-outliers 10 session1.events.csv

# 3. Check TTL-to-audio latency instead
events-stats -event1 TTLin1 -event2 Mic1 -detect-outliers 10 session1.events.csv

# 4. Disable outlier filtering to see raw data
events-stats session1.events.csv

# 5. Pool several sessions
events-stats session1.events.csv session2.events.csv session3.events.csv
```

# Troubleshooting

> [!WARNING]
> The BBTK and the host PC communicate via a serial protocol over
USB. Depending on your computer, you may need to install an additional drivers to handle this. 

   
| :zap: Windows |
|---------------|

To determine the (virtual) serial port to which the BBTK is attached, check the "Ports (COM & LPT)" section of the Computer Management console.

For the BBTK v2, you may need to install a driver to communicate with the BBTK. You can install the mbed-cli available from <https://os.mbed.com/docs/mbed-os/v6.16/quick-start/build-with-mbed-cli.html> and check install driver during the setup.

For the BBTK v3, you may need to install the <https://ftdichip.com/drivers/vcp-drivers/> following instructions at <https://ftdichip.com/document/installation-guides/>


| :zap: Linux  |
|--------------|


For the BBTK to be recognized as a serial device, the module `ftdi_sio` must be loaded in the linux kernel. To do so manually:

    sudo modprobe ftdi_sio

To determine which serial port the BBTK is attached toi (`/dev/ttyACM0`, `/dev/ttyUSB0`, ...), run: 

    sudo dmesg -w 

| :zap: MacOS X |
|---------------|

The BBTK may appear as `/dev/cu.usbserial-BBTKXXXX`. The page at <https://ftdichip.com/drivers/vcp-drivers/> contains drivers for various MacOS X versions.


# Compiling from source

The source code is available at <https://github.com/chrplr/bbtkv3>

To build the executable, you need the [Go development tools](https://go.dev/) (and [Git](https://git-scm.com/downloads) if you want to clone the github repository rather than downloading the src as a zip file)


```
git clone https://github.com/chrplr/bbtkv3.git
cd bbtkv3  
make build
```

This will generate executables in `_build/`.

For cross-compiling:

```bash
./build-multiplatforms.sh X.X.X
```

where X.X.X is a version number

The outcome will be in `binaries/`

> [!NOTE]
> You can set the `PLATFORMS` and `ARCHITECTURES` to target a subset of OS and ARCH, e.g.:

```bash
export PLATFORMS=linux
export ARCHITECTURES=amd64
./build-multiplatforms.sh X.X.X
```

---

AUTHOR: christophe@pallier.org

LICENSE: GPL-3.0

REFERENCES:

* Plant, R., Hammond, N., & Turner, G. (2004). Self-validating presentation and response timing in cognitive paradigms: How and why? Behavior Research Methods, Instruments, & Computers : A Journal of the Psychonomic Society, Inc, 36, 291–303. https://doi.org/10.3758/BF03195575
* Plant, R. (2016). The Black Box Toolkit v2. API Guide. Revision RC4.
