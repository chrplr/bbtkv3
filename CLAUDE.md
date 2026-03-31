# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Command-line tools suite for the **Black Box ToolKit v3 (BBTK v3)** — a hardware device used in psychology research to measure the timing of audio-visual stimuli with sub-millisecond accuracy. Communicates with the device over USB/serial.

## Build & Test Commands

```bash
make build       # Build the application
make test        # Run tests (verbose)
make all         # Build, test, and clean
make clean       # Remove binaries
go test ./...    # Run all tests directly
```

**Cross-platform builds** (outputs to `binaries/` as `{cmd}-{os}-{arch}-{version}`):
```bash
./build-multiplatforms.sh 1.2.3    # Build for all platforms/architectures
```

## Architecture

The module is `github.com/chrplr/bbtkv3`. Root-level `.go` files form a shared library; each subdirectory under `cmd/` is its own binary.

**Core library (root level)**:
- `communication_with_bbtk.go` — Serial protocol: opening the port, sending commands, reading raw event data from the device
- `events.go` — Data structures (`DSCEvent`, `Event`), parsing raw device output, and CSV export/import
- `thresholds.go` — `Thresholds` struct (8 × uint8, range 0–127) and parsing
- `SmoothMask.go` — `SmoothingMask` struct (6 boolean sensor channels) and parsing

**CLI tools (`cmd/`)**:
| Tool | Purpose |
|------|---------|
| `bbtk-capture` | Main tool — records events for a given duration, writes `.dat`, `.dscevents.csv`, `.events.csv` |
| `bbtk-detect-port` | Scans serial ports to locate the connected BBTK |
| `bbtk-adjust-thresholds` | Interactive threshold adjustment menu |
| `bbtk-get-thresholds` | Reads current thresholds from device |
| `bbtk-set-thresholds` | Writes 8 threshold values to device |
| `bbtk-set-smoothing` | Configures sensor smoothing |
| `get-serial-port-list` | Lists available serial ports |

**Data flow**:
```
BBTK Device (USB/serial)
  → communication_with_bbtk.go  (serial I/O, raw data)
  → events.go                   (parse + process)
  → .dat → .dscevents.csv → .events.csv
```

**Port definitions** (in `events.go`): 12 input ports (Keypad1-4, Opto1-4, TTLin1-2, Mic1-2) and 8 output ports (ActClose1-4, TTLout1-2, Sounder1-2).

## Environment Variables

- `BBTK_PORT` — Serial port path (overrides `-p` flag)
- `DEBUG` — Enable debug logging

## Key Dependency

`go.bug.st/serial v1.6.2` — the sole external library for serial communication.
