Events capture with a Black Box ToolKit 
=======================================

This program drives a [Black Box Toolkit BBTKv3](https://www.blackboxtoolkit.com/bbtkv3.html) to capture events.

When launched, it will connect to the black box toolkit and, if successfull, will capture events for 30s (using the ToolBox' DSCM internal command) and save them in csv files.

NOTE: This is a *Work In Progress*.

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
