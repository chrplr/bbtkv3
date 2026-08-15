# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Command-line tools suite for the **Black Box ToolKit v3 (BBTK v3)** — a hardware device used in psychology research to measure the timing of audio-visual stimuli with sub-millisecond accuracy. Communicates with the device over USB/serial.

## Build & Test Commands

```bash
make build       # Build all 14 commands into _build/
make test        # Run tests (verbose)
make all         # Build, then test
make clean       # Remove _build/ and binaries/
go test ./...    # Run all tests directly
```

**Installation**: `make install` builds, then copies the 14 binaries into `$(BINDIR)`, default `$(PREFIX)/bin` = `~/.local/bin`, and warns when that directory is absent from `PATH`. Override with `make install PREFIX=/usr/local` or `BINDIR=...`; `make uninstall` takes the same variables.

**Cross-platform distribution** (outputs to `binaries/` as `bbtkv3-{os}-{arch}-{version}.zip`, one zip per platform holding all 14 binaries):
```bash
make dist                 # darwin/linux/windows × amd64/arm64
make dist VERSION=v1.2.3  # override the version (defaults to `git describe --tags --abbrev=0`)
```

`VERSION` and the short git hash are injected via `-ldflags` into `main.Version` / `main.Build` of every command. The build is pure Go (no cgo), so all six targets cross-compile from any host.

## Releases

Pushing a tag matching `v*` triggers `.github/workflows/release.yml`, which runs the tests, builds the six zips, and publishes a GitHub release with auto-generated notes:

```bash
git tag v1.2.3 && git push origin v1.2.3
```

`make release` does the same thing locally via `gh` — use it only to backfill a tag that predates the workflow. Note the workflow passes `VERSION` explicitly from the tag name, because `actions/checkout` makes a shallow clone in which the Makefile's `git describe` would yield `dev`.

## Architecture

The module is `github.com/chrplr/bbtkv3`. Root-level `.go` files form a shared library; each subdirectory under `cmd/` is its own binary.

**Core library (root level)**:
- `communication_with_bbtk.go` — Serial protocol: opening the port, sending commands, reading raw event data from the device
- `events.go` — Data structures (`DSCEvent`, `Event`), parsing raw device output, and CSV export/import
- `thresholds.go` — `Thresholds` struct (8 × uint8, range 0–127) and parsing
- `dsre.go` — Digital Stimulus Response Echo: mask widths (`StandardWidths` 12/8, `EliteWidths` 20/16), port-name→bitmask helpers (`InputMask`, `OutputMask`), row and sequence builders (`DSRERow`, `DSRESequence`), and `DSREProgram`. Verified against a BBTKv3 Elite, firmware `20230405`. Three things that each cost a hardware session: DSRE takes **exactly one** stimulus-response row (unlike event marking's eight — do not pad); the masks are **12/8 even on an Elite**, despite that box having 20/16 lines and its event-marking rows being 20/16 wide; and reading replies must go through the port directly, not `ReadLine` — `bufio` retries a silent port 100 times (~100 s per unanswered command). `STYP PATT` demands an exact match of the whole input port, `STYP INDI` (`-any`) matches individual lines. The v3 answers **nothing** during DSRE programming, so silence is not a symptom
- `SmoothMask.go` — `SmoothingMask` struct (6 boolean sensor channels), parsing, and the smoothing duration correction (`Enabled`, `CorrectedDuration`, `DefaultSmoothingDurationOffsetMs`)

**CLI tools (`cmd/`)**:
| Tool | Purpose |
|------|---------|
| `bbtk-capture` | Main tool — records events for a given duration (`-d`) with a given smoothing mask (`-s`), writes `.dat`, `-dscevents.csv`, `-events.csv` |
| `bbtk-detect-port` | Scans serial ports to locate the connected BBTK |
| `bbtk-adjust-thresholds` | Interactive threshold adjustment menu |
| `bbtk-get-thresholds` | Reads current thresholds from device |
| `bbtk-set-thresholds` | Writes 8 threshold values to device |
| `bbtk-set-smoothing` | Sets the smoothing mask from a required six-value argument (`mic1,mic2,opto4,opto3,opto2,opto1`) |
| `get-serial-port-list` | Lists available serial ports |
| `ibbtk` | Interactive menu-driven shell (thresholds, smoothing, capture sub-menus) |
| `events-stats` | Offline analysis of `-events.csv`: duration/jitter/onset-difference percentiles (of the uncorrected `Duration`); no device needed |
| `bbtk-send-command` | Pipes raw commands from stdin to the device and prints responses |
| `bbtk-send-break` | Sends the break character to stop a running device operation; recovers a box left streaming by a killed tool. Deliberately does not handshake first — `Connect()` is what fails on a wedged device |
| `bbtk-event-marking` | Sends the PDCE/STYP/PATT/TIML sequence to run an event-marking program |
| `bbtk-input-check` | Streams live input state (`ICHK`) until Esc |
| `bbtk-trigger-response` | Runs a DSRE program: trigger input → delay → output pulse, looping until Esc. Defaults drive the Robotic Key Actuator on TTLout1 from TTLin2 |

The canonical list is `CMDS` in the Makefile — keep it in sync when adding a command under `cmd/`.

**Data flow**:
```
BBTK Device (USB/serial)
  → communication_with_bbtk.go  (serial I/O, raw data)
  → events.go                   (parse + process)
  → .dat → -dscevents.csv → -events.csv
```

**`-events.csv` columns**: `Type`, `Onset`, `Duration`, `DurationCorrected` — one row per event, sorted by onset, milliseconds. `DurationCorrected` subtracts the ~20 ms smoothing tail (`DefaultSmoothingDurationOffsetMs`) on the channels the programmed mask covers, clamping at zero; elsewhere it repeats `Duration`. `Duration` is never altered, so pre-existing captures stay comparable and a wrong offset can be undone. Onsets need no correction — smoothing extends the tail, not the leading edge.

Two asymmetries to preserve when touching this:
- `bbtk-capture` writes four columns via `SaveEventsToCSVWithCorrection()`, passing the same mask it programmed from `-s` — a mask disagreeing with the device corrects the wrong channels, so these must stay tied together. `ibbtk`'s `capture run` still writes three via `SaveEventsToCSV()`.
- `events-stats` (`readCSV` in `cmd/events-stats/main.go`) resolves columns by header name, not position, and reads `Duration` — its duration statistics are of uncorrected values. Keep new readers header-based; that is what made the column safe to add.

**Port definitions** (in `events.go`): 12 input ports (Keypad1-4, Opto1-4, TTLin1-2, Mic1-2) and 8 output ports (ActClose1-4, TTLout1-2, Sounder1-2).

## Environment Variables

- `BBTK_PORT` — Serial port path, used when `-p` is not given. Every device tool resolves its port through `bbtkv3.ResolvePort()` (`communication_with_bbtk.go`), in this order:
  1. `-p <port>` on the command line.
  2. `BBTK_PORT` (read via `bbtkv3.GetPortFromEnv()`).
  3. The udev symlink matching `/dev/serial/by-id/*BBTK*`, derived from the device's USB descriptors and therefore stable across replugs and power cycles, unlike `/dev/ttyUSBn`. Linux only — the glob matches nothing on macOS and Windows.
  4. `bbtkv3.DetectPort()`: a scan of every port `AvailablePorts()` reports — open, write `CONN`, keep what answers `BBTK;` — run in parallel, one second per silent port, announced on stderr. It is last because it writes to every serial port on the machine. `AvailablePorts()` is `serial.GetPortsList()` minus the `/dev/tty.*` half of macOS's call-in/call-out pairs, which block on open and are the wrong name to hand back.

  When all four fail, every device tool exits with `no serial port specified: use -p <port> or set BBTK_PORT`. There is no built-in device-name default; `/dev/ttyUSB0` used to be one for five of the tools, and on macOS that meant they failed while `bbtk-detect-port` worked.

  `bbtk-detect-port` is step 4 made explicit, over the ports named on its command line or all of them, reporting every BBTK rather than the first. Keep the probe itself in `ScanForBBTK`/`probeBBTK` — one CONN implementation, shared by the tool and the fallback.

  `bbtkv3.StablePortName()` is the inverse of step 3: given a `/dev/ttyUSBn` name it returns the by-id symlink pointing at it. `bbtk-detect-port` and `DetectPort()` both use it, so a scan reports and returns the stable name; it returns its argument unchanged when there is no `/dev/serial/by-id`.

  (`bbtk-detect-port`, `get-serial-port-list` and `events-stats` take no `-p` and ignore `BBTK_PORT`.)
- `DEBUG` — Enable debug logging

## Dependencies

- `go.bug.st/serial` — serial communication, used by the root library.
- `golang.org/x/term` — raw-mode key reading, in `communication_with_bbtk.go` and `bbtk-input-check`.
- `gonum.org/v1/plot` — plots in `events-stats` (by far the heaviest dependency).
- `github.com/turret-io/go-menu` — menu shell in `ibbtk`.

Only the first is listed as a direct require in `go.mod`; the others are marked `// indirect` despite being imported directly, so a `go mod tidy` will reclassify them.
