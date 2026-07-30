// Author: Christophe Pallier <christophe@pallier.org>
// LICENSE: GPL-3.0

package main

import (
	"io"
	"reflect"
	"testing"
	"time"
)

func TestSplitArgv(t *testing.T) {
	cases := []struct {
		name        string
		argv        []string
		host        []string
		child       []string
		wantNoChild bool
	}{
		{
			name:        "no separator",
			argv:        []string{"bbtk-capture", "-d", "20", "base"},
			host:        []string{"bbtk-capture", "-d", "20", "base"},
			wantNoChild: true,
		},
		{
			name:  "separator after basefilename",
			argv:  []string{"bbtk-capture", "-d", "20", "base", "--", "./stim", "-x"},
			host:  []string{"bbtk-capture", "-d", "20", "base"},
			child: []string{"./stim", "-x"},
		},
		{
			// The child's own -d must not be parsed as ours. Splitting before
			// flag.Parse is what guarantees it.
			name:  "child reuses our flag names",
			argv:  []string{"bbtk-capture", "-d", "20", "base", "--", "./stim", "-d", "1", "-p", "9"},
			host:  []string{"bbtk-capture", "-d", "20", "base"},
			child: []string{"./stim", "-d", "1", "-p", "9"},
		},
		{
			// Only the FIRST -- separates; later ones belong to the child.
			name:  "second separator is the child's",
			argv:  []string{"bbtk-capture", "base", "--", "sh", "-c", "prog -- x"},
			host:  []string{"bbtk-capture", "base"},
			child: []string{"sh", "-c", "prog -- x"},
		},
		{
			name:        "trailing separator with no command",
			argv:        []string{"bbtk-capture", "base", "--"},
			host:        []string{"bbtk-capture", "base"},
			child:       []string{},
			wantNoChild: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			host, child := splitArgv(c.argv)
			if !reflect.DeepEqual(host, c.host) {
				t.Errorf("host argv = %q, want %q", host, c.host)
			}
			if c.wantNoChild {
				if len(child) != 0 {
					t.Errorf("child argv = %q, want none", child)
				}
				return
			}
			if !reflect.DeepEqual(child, c.child) {
				t.Errorf("child argv = %q, want %q", child, c.child)
			}
		})
	}
}

// newTestChild builds a child wired to a buffered abort channel the test can
// inspect, discarding its progress text.
func newTestChild(argv ...string) (*child, chan struct{}) {
	abort := make(chan struct{}, 1)
	return newChild(argv, io.Discard, abort), abort
}

// waitAbort reports whether an abort was requested within the timeout.
func waitAbort(abort <-chan struct{}, timeout time.Duration) bool {
	select {
	case <-abort:
		return true
	case <-time.After(timeout):
		return false
	}
}

func TestChildSuccessDoesNotAbort(t *testing.T) {
	c, abort := newTestChild("true")
	c.start()

	if waitAbort(abort, 2*time.Second) {
		t.Fatal("a child that exited 0 requested an abort")
	}
	if failed := c.finish(); failed {
		t.Error("finish() reported failure for a child that exited 0")
	}
	if code := c.exitCode(); code != 0 {
		t.Errorf("exitCode() = %d, want 0", code)
	}
}

func TestChildFailureAbortsAndPropagatesStatus(t *testing.T) {
	c, abort := newTestChild("sh", "-c", "exit 3")
	c.start()

	if !waitAbort(abort, 5*time.Second) {
		t.Fatal("a child that exited non-zero did not request an abort")
	}
	if failed := c.finish(); !failed {
		t.Error("finish() did not report failure")
	}
	if code := c.exitCode(); code != 3 {
		t.Errorf("exitCode() = %d, want the child's 3", code)
	}
}

func TestChildThatCannotStartAborts(t *testing.T) {
	c, abort := newTestChild("./definitely-not-a-real-program")
	c.start()

	if !waitAbort(abort, 2*time.Second) {
		t.Fatal("a child that could not be started did not request an abort")
	}
	// Recording has already begun at this point, so the run must be reported as
	// failed rather than silently producing a stimulus-free recording.
	if failed := c.finish(); !failed {
		t.Error("finish() did not report failure")
	}
	if code := c.exitCode(); code == 0 {
		t.Error("exitCode() = 0 for a child that never ran")
	}
}

// A child still running when the window closes is killed and the run reported
// as failed, but this is the path where the caller still saves its data.
func TestChildOutrunningTheWindowIsStopped(t *testing.T) {
	c, abort := newTestChild("sleep", "60")
	c.start()

	// Nothing should be aborted while it is merely still running: the capture
	// window must be allowed to complete.
	if waitAbort(abort, time.Second) {
		t.Fatal("a running child requested an abort")
	}

	start := time.Now()
	if failed := c.finish(); !failed {
		t.Error("finish() did not report failure for a child that outran the window")
	}
	// SIGTERM kills sleep outright, so this must not have waited out childGrace.
	if elapsed := time.Since(start); elapsed >= childGrace {
		t.Errorf("finish() took %v; SIGTERM should have ended it well before the %v grace", elapsed, childGrace)
	}
	if c.cmd.ProcessState == nil || c.cmd.ProcessState.Success() {
		t.Error("child was not actually terminated")
	}
}

// finish() and exitCode() each need the child's result, and the channel holds
// exactly one value — so whichever ran second used to see "still running".
func TestChildResultSurvivesRepeatedReaping(t *testing.T) {
	c, _ := newTestChild("sh", "-c", "exit 7")
	c.start()
	c.waitReap()

	for i := 0; i < 3; i++ {
		if failed := c.finish(); !failed {
			t.Errorf("finish() #%d lost the failure", i+1)
		}
		if code := c.exitCode(); code != 7 {
			t.Errorf("exitCode() #%d = %d, want 7", i+1, code)
		}
	}
}
