package bbtkv3

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLineEnding(t *testing.T) {
	// A real file, which is an *os.File but not a terminal — the case that
	// distinguishes "raw mode is on" from "the output is a terminal". Getting
	// this wrong writes stray CRs into redirected output.
	f, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer f.Close()

	for _, tc := range []struct {
		name string
		w    interface{ Write([]byte) (int, error) }
		raw  bool
		want string
	}{
		{"cooked mode leaves \\n alone", f, false, "\n"},
		{"raw mode, output redirected to a file", f, true, "\n"},
		{"raw mode, output not a file at all", &bytes.Buffer{}, true, "\n"},
		{"cooked mode, buffer", &bytes.Buffer{}, false, "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := LineEnding(tc.w, tc.raw); got != tc.want {
				t.Errorf("LineEnding(%T, %v) = %q, want %q", tc.w, tc.raw, got, tc.want)
			}
		})
	}
}
