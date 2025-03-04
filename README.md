Automated capture of events with a Black Box ToolKit(tm) 
========================================================

The [Black Box ToolKit](https://www.blackboxtoolkit.com/bbtkv3.html)  is a device that allows psychologists to measure the timing of audio-visual stimuli with sub-millisecond accuracy. It replaces a digital oscilloscope, capturing activity on sound and visual sensors and TTL signals, and a signal generator,
 generating sounds or TTL signals.

This page describes a set of command-line tools that streamline the testing of time-critical psychology experiments:  

* `bbtk-adjust-thresholds` which  opens the "sensor menu" on the BBTK 
* `bbtk-set-thresholds` which sets the values of the various thresholds
* `bbtk-capture` which launches the capture of events and export them to `.csv` files.


Binaries for different operating systems are available at <https://github.com/chrplr/bbtkv3/releases>,

The source code (under a GPL-3.0 License) at <https://github.com/chrplr/bbtkv3>

This program relies on a Go module, `github.com/chrplr/bbtkv3`, which encapsulates a small subset of the commands documented in *The BBTKv2 API Guide* (in the future, we might implement more functions). This go module can be used to drive the BBTK from programs written in Go.


| :exclamation: This is a **Work In Progress**. |
|-----------------------------------------------|

The program may not work as advertised, the documentation may not be up to date, etc.  You can contribute by proposing improvements or reporting bugs either by contacting me (`<christophe@pallier.org>`) or by opening an issue at <https://github.com/chrplr/bbtkv3/issues>.

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


Assuming that you downloaded the programs in `~/Downloads` and want to install them in `~/bin`:

```bash
mkdir -p ~/bin
cd ~/Downloads
for f in bbtk*; do chmod +x $f; mv $f ~/bin/${f%-linux-amd64-1.0.1}; done

#run
bbtk -p /dev/ttypACM0
```

(replace the version number by the current one)

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
go build ./... 
```

This should generate executables in each subfolder of `cmd`

For cross-compiling:

```bash
./build-mutliplatforms.sh X.X.X
```

where X.X.X is a version number

The outcome will be in `binaries/`

> [!NOTE]
> You can set the `PLATFORMS` and `ARCHITECTURES` to target a subset of OS and ARCH, e.g.:

```bash
export PLATFORMS=linux
export ARCHITECTURES=amd64
./build-mutliplatforms.sh X.X.X
```

---

AUTHOR: christophe@pallier.org

LICENSE: GPL-3.0
