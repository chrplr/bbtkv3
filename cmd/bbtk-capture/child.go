// Author: Christophe Pallier <christophe@pallier.org>
// LICENSE: GPL-3.0

// Running a stimulus program inside the capture window.
//
// The problem this solves: the device needs 11-40 s of setup before it starts
// recording, and nothing about that instant is predictable — the internal-memory
// erase takes a variable time. A stimulus started too early is recorded
// partially; started too late it runs past the end of the window. Launching it
// from here, at the instant RUDS is acknowledged, removes the guesswork and the
// out-of-process handshake that used to reconstruct it.

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// childGrace is how long a child gets to exit after SIGTERM before it is
// killed. Generous enough for an SDL program to tear down its window and close
// its data file, short enough not to stall the download that follows.
const childGrace = 5 * time.Second

// splitArgv divides a command line at the first "--". The second result is the
// child's argv, nil if there was no "--".
//
// This is done before flag.Parse rather than left to the flag package: flag
// stops at the first non-flag argument, so whether "--" survives into
// flag.Args() depends on where the basefilename sits relative to it. Splitting
// here makes the boundary explicit, and guarantees a child flag such as -d is
// never read as ours.
func splitArgv(argv []string) (host, child []string) {
	for i, a := range argv {
		if a == "--" {
			return argv[:i], argv[i+1:]
		}
	}
	return argv, nil
}

// child supervises the stimulus process for one capture.
type child struct {
	argv []string
	out  io.Writer // where to report, matching the capture's progress stream

	cmd *exec.Cmd

	// done carries the single result of cmd.Wait(). Buffered so the watcher
	// goroutine never blocks, whether or not anybody reads it.
	done chan error

	// result caches what came out of done. The channel holds exactly one value,
	// so both finish() and exitCode() reading it directly would leave whichever
	// ran second to conclude the child was still running. Written only from the
	// main goroutine, which is the only one that reaps.
	result    error
	resultSet bool

	// abort requests that the capture stop. Shared with the signal handler.
	abort chan<- struct{}
}

// reap reports whether the child has exited, caching the result. It never
// blocks.
func (c *child) reap() (exited bool, err error) {
	if c.resultSet {
		return true, c.result
	}
	select {
	case e := <-c.done:
		c.result, c.resultSet = e, true
		return true, e
	default:
		return false, nil
	}
}

// waitReap blocks until the child has exited, caching the result.
func (c *child) waitReap() error {
	if !c.resultSet {
		c.result, c.resultSet = <-c.done, true
	}
	return c.result
}

func newChild(argv []string, out io.Writer, abort chan<- struct{}) *child {
	return &child{argv: argv, out: out, done: make(chan error, 1), abort: abort}
}

// start launches the child and begins watching it. It is called from
// CaptureEvents at the instant recording begins, so it must not block.
//
// A child that exits non-zero aborts the capture. The reasoning is that the
// recording is worthless without a stimulus that ran to completion, and an
// aborted capture cannot be salvaged anyway — so there is nothing to weigh
// against stopping early and freeing the device for the retry. A child that
// exits zero is left alone: it is expected to finish before the window does,
// because the window deliberately carries a margin at each end.
func (c *child) start() {
	c.cmd = exec.Command(c.argv[0], c.argv[1:]...)
	// The child inherits the terminal. That is why the capture runs with
	// NoKeyAbort: raw mode would clear ISIG and ONLCR for this same terminal,
	// costing the child its Ctrl-C and staggering its line output.
	c.cmd.Stdin = os.Stdin
	c.cmd.Stdout = os.Stdout
	c.cmd.Stderr = os.Stderr

	if err := c.cmd.Start(); err != nil {
		// Recording has already started, so there is no way to decline
		// gracefully; report it and stop the capture.
		fmt.Fprintf(c.out, "\n!! cannot start %s: %v\n", c.argv[0], err)
		c.done <- err
		c.requestAbort()
		return
	}
	fmt.Fprintf(c.out, "started: %v\n", c.argv)

	go func() {
		err := c.cmd.Wait()
		c.done <- err
		if err != nil {
			fmt.Fprintf(c.out, "\n!! %s failed: %v\n", c.argv[0], err)
			fmt.Fprintf(c.out, "   aborting the capture; no data will be saved.\n")
			c.requestAbort()
		}
	}()
}

func (c *child) requestAbort() {
	select {
	case c.abort <- struct{}{}:
	default:
	}
}

// finish settles the child once the capture window has closed, and reports
// whether the run should be considered a failure.
//
// A child still running at this point outran the capture. The recording is
// nevertheless complete and valid — the window ran its full length — so it is
// still downloaded and saved by the caller; only the exit status marks the
// mismatch. Discarding good data over a miscalculated duration would be the
// worse trade, since the duration cannot be corrected after the fact.
func (c *child) finish() (failed bool) {
	if exited, err := c.reap(); exited {
		return err != nil
	}

	fmt.Fprintf(c.out, "\n!! %s was still running when the capture window closed.\n", c.argv[0])
	fmt.Fprintf(c.out, "   The stimulus is longer than the recording: raise -d, or shorten it.\n")
	fmt.Fprintf(c.out, "   The data below covers only the recorded window.\n")

	if err := c.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintf(c.out, "   (SIGTERM failed: %v)\n", err)
	}
	select {
	case e := <-c.done:
		c.result, c.resultSet = e, true
	case <-time.After(childGrace):
		fmt.Fprintf(c.out, "   (still alive after %v — killing it)\n", childGrace)
		_ = c.cmd.Process.Kill()
		c.waitReap()
	}
	return true
}

// exitCode is the status to exit with when the child failed: its own, so a
// caller inspecting $? sees what the stimulus reported. Falls back to 1 when
// there is no code to propagate (killed by a signal, or never started).
func (c *child) exitCode() int {
	exited, err := c.reap()
	if !exited {
		return 1
	}
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if code := ee.ExitCode(); code > 0 {
			return code
		}
	}
	return 1
}
