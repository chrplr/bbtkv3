// events-stats: compute descriptive statistics on *events.csv files produced by bbtk-capture.
//
// For each event type found in the input files it reports:
//   - percentiles 0 % (min), 10 %, 20 %, … 90 %, 100 % (max) of Duration
//   - the same percentiles of the inter-onset intervals (jitter) within each type
//
// Usage:
//
//	events-stats file1.events.csv [file2.events.csv ...]
package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

type row struct {
	typ      string
	onset    float64
	duration float64
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <file.events.csv> [file2.events.csv ...]\n", os.Args[0])
		os.Exit(1)
	}

	// Collect all rows, grouped by type.
	byType := map[string][]row{}
	typeOrder := []string{} // preserve first-seen order

	for _, path := range os.Args[1:] {
		rows, err := readCSV(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", path, err)
			os.Exit(1)
		}
		for _, r := range rows {
			if _, seen := byType[r.typ]; !seen {
				typeOrder = append(typeOrder, r.typ)
			}
			byType[r.typ] = append(byType[r.typ], r)
		}
	}

	if len(byType) == 0 {
		fmt.Fprintln(os.Stderr, "no events found")
		os.Exit(1)
	}

	pcts := []int{0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100}

	// ── Duration statistics ────────────────────────────────────────────────
	fmt.Println("=== Duration Statistics (ms) ===")
	fmt.Println()
	printTable(typeOrder, byType, pcts, func(rows []row) []float64 {
		vals := make([]float64, len(rows))
		for i, r := range rows {
			vals[i] = r.duration
		}
		return vals
	}, false)

	// ── Jitter (inter-onset interval) statistics ───────────────────────────
	fmt.Println()
	fmt.Println("=== Inter-Onset Interval / Jitter Statistics (ms) ===")
	fmt.Println()
	printTable(typeOrder, byType, pcts, jitterValues, true)
}

// printTable writes a percentile table to stdout.
// valuesFn extracts the sample values for a single type's rows.
// isJitter controls whether the N column shows n-1 (diffs) instead of n.
func printTable(
	typeOrder []string,
	byType map[string][]row,
	pcts []int,
	valuesFn func([]row) []float64,
	isJitter bool,
) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Header
	header := []string{"Type", "N"}
	for _, p := range pcts {
		if p == 0 {
			header = append(header, "Min")
		} else if p == 100 {
			header = append(header, "Max")
		} else {
			header = append(header, fmt.Sprintf("P%d", p))
		}
	}
	header = append(header, "Range", "P99.5-P0.5", "P95-P05", "SD")
	fmt.Fprintln(w, strings.Join(header, "\t"))

	// Separator
	seps := make([]string, len(header))
	for i, h := range header {
		seps[i] = strings.Repeat("-", len(h))
	}
	fmt.Fprintln(w, strings.Join(seps, "\t"))

	for _, typ := range typeOrder {
		rows := byType[typ]
		vals := valuesFn(rows)
		if len(vals) == 0 {
			continue
		}
		sort.Float64s(vals)

		cols := []string{typ, strconv.Itoa(len(vals))}
		for _, p := range pcts {
			cols = append(cols, fmt.Sprintf("%.3f", percentile(vals, float64(p))))
		}
		rng := vals[len(vals)-1] - vals[0]
		spread := percentile(vals, 99.5) - percentile(vals, 0.5)
		iqr := percentile(vals, 95) - percentile(vals, 5)
		cols = append(cols,
			fmt.Sprintf("%.3f", rng),
			fmt.Sprintf("%.3f", spread),
			fmt.Sprintf("%.3f", iqr),
			fmt.Sprintf("%.3f", stddev(vals)),
		)
		fmt.Fprintln(w, strings.Join(cols, "\t"))
	}
	w.Flush()
}

// jitterValues returns the successive differences of Onset values for a type,
// sorted by Onset first.
func jitterValues(rows []row) []float64 {
	if len(rows) < 2 {
		return nil
	}
	sorted := make([]row, len(rows))
	copy(sorted, rows)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].onset < sorted[j].onset })

	diffs := make([]float64, len(sorted)-1)
	for i := 1; i < len(sorted); i++ {
		diffs[i-1] = sorted[i].onset - sorted[i-1].onset
	}
	return diffs
}

// percentile computes the p-th percentile (0–100) of a pre-sorted slice using
// linear interpolation (equivalent to numpy's default and R's type 7).
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return math.NaN()
	}
	if n == 1 {
		return sorted[0]
	}
	idx := p / 100.0 * float64(n-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

// stddev computes the sample standard deviation (Bessel-corrected, n-1).
func stddev(vals []float64) float64 {
	n := len(vals)
	if n < 2 {
		return math.NaN()
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(n)
	var sq float64
	for _, v := range vals {
		d := v - mean
		sq += d * d
	}
	return math.Sqrt(sq / float64(n-1))
}

// readCSV reads a single events CSV file and returns its rows.
func readCSV(path string) ([]row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true

	// Read and validate header
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}
	typeCol, onsetCol, durCol := -1, -1, -1
	for i, h := range header {
		switch strings.TrimSpace(h) {
		case "Type":
			typeCol = i
		case "Onset":
			onsetCol = i
		case "Duration":
			durCol = i
		}
	}
	if typeCol < 0 || onsetCol < 0 || durCol < 0 {
		return nil, fmt.Errorf("missing required columns (Type, Onset, Duration); found: %v", header)
	}

	var rows []row
	lineNum := 1
	for {
		lineNum++
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
		onset, err := strconv.ParseFloat(strings.TrimSpace(rec[onsetCol]), 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: bad Onset %q: %w", lineNum, rec[onsetCol], err)
		}
		dur, err := strconv.ParseFloat(strings.TrimSpace(rec[durCol]), 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: bad Duration %q: %w", lineNum, rec[durCol], err)
		}
		rows = append(rows, row{
			typ:      strings.TrimSpace(rec[typeCol]),
			onset:    onset,
			duration: dur,
		})
	}
	return rows, nil
}
