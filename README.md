Events capture with a Black Box ToolKit 
=======================================

> [!WARNING] > This is a **Work In Progress**. The program may not work as advertised, the documentation may not be up to date, etc.  You can contribute by reorting issues, either by contacting me (`<christophe@pallier.org>`) or by opening an issue on the [github bbtkv3 repository](http://github.com/chrplr/bbtkv3).


<p align="center" width="100%">
![](bbtkv3.jpg)
</p>

The **Black Box ToolKit v3** is a device that allows psychologists to
 measure the timing of audio-visual stimuli with sub-millisecond
 accuracy. It replaces a digital oscilloscope (capturing activity on
 sound and visual sensors, or TTL signals) and a signal generator
 (generating sound or TTL signal). (See
<https://www.blackboxtoolkit.com/bbtkv3.html for more information>)

To operate, three pieces of equipement are needed:

1. A stimulation device (typically a computer, but not necessarily) 
2. The bbtkv3 with input sensors (photodiodes, sound detectors, TTL detectors) attached to the stimulation device.
3. A host computer driving the bbtkv2 (hooked to it via a USB cable).

> [!TIP] > The stimulation PC and the host PC *can* be the same computer (but do not have to). As data are recorded asynchronously by the BBTKv2, it is possible for a single PC to switch the BBTKv2 into “capture mode”, launch the stimulation program and, when done, download the timing data from the BBTKv2 memory.
   
> [!IMPORTANT] >  Under Windows, you may need to install a driver to communicate with the BBTK. You can install the mbed-cli available from <https://os.mbed.com/docs/mbed-os/v6.16/quick-start/build-with-mbed-cli.html> and check install driver during the setup.

> [!IMPORTANT] > Linux: the kernel module `ftdi_sio` needs to be loaded (e.g. `sudo modeprobe ftdi_sio`)


The BBTKv3 and the host PC communicate via a serial protocol over
USB. 

Here, we provide:

- a *bbtkv3* Go module which encapsulates some of
the commands documented in *The BBTKv2 API Guide* sold by the parent
company.
- an execuctable program, `bbtk`, that will drive the acquisition of events on the bbtkv3 and saves them in a csv file.



# Installation

Compiled versions for MACOSX, Windows and Linux, and intel (amd64) or arm are avavailable in [binaries](binaries)

Get the version for your OS and architecture, and copy it in any folder listed in the PATH variable of your OS (e.g. `%windir%/system32` for Windows)



# Compiling from source

The source code is at <https://github.com/chrplr/bbtkv3>

To compile it to an executable, you need the [Go development tools](https://go.dev/) (and [Git](https://git-scm.com/downloads) if you want to clone the github repository rather than downloading the sc as a zip file)


```
git clone https://github.com/chrplr/bbtkv3.git
cd bbtkv3  
go build ./... 
```

This will generate the `bbtk` executable in the folder `cmd/bbtk`

# Usage

Open a terminal, and type `bbtk`

`bbtk -h` will display the usage, e.g.:


```
Usage of cmd/bbtk/bbtk:
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

# Troubleshooting

## Linux:

For the bbtk to be detected as a serial device, the module `ftdi_sio` must be loaded in the kernel. You may need to do it manually:


    sudo modprobe ftdi_sio

To know the serial port to which the bbtk is attached, you can run `sudo dmesg -w` before attaching it. 

---

AUTHOR: christophe@pallier.org
LICENSE: GPL-3.0
