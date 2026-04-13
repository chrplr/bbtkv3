// events-stats: compute descriptive statistics on *events.csv files produced by bbtk-capture.
//
// For each event type found in the input files it reports:
//   - percentiles 0 % (min), 10 %, 20 %, … 90 %, 100 % (max) of Duration
//   - the same percentiles of the inter-onset intervals (jitter) within each type
//   - the same percentiles of onset differences between temporally paired events
//     (event2.Onset − event1.Onset for the nearest following event2 after each event1)
//
// Usage:
//
//	events-stats [-event1 TYPE] [-event2 TYPE] file1.events.csv [file2.events.csv ...]
package main

import (
	"encoding/csv"
	"flag"
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
	event1 := flag.String("event1", "TTLin1", "first event type for paired-difference analysis")
	event2 := flag.String("event2", "Opto1", "second event type for paired-difference analysis (difference = event2 − event1)")
	outlierK := flag.Float64("detect-outliers", 0, "exclude values more than this many ms away from the median (set to 0 to disable)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [-event1 TYPE] [-event2 TYPE] [-detect-outliers MS] <file.events.csv> [file2.events.csv ...]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	// Collect all rows, grouped by type, and also in a flat slice for pairing.
	byType := map[string][]row{}
	typeOrder := []string{} // preserve first-seen order
	var allRows []row

	for _, path := range flag.Args() {
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
		allRows = append(allRows, rows...)
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
	}, false, true, *outlierK)

	// ── Jitter (inter-onset interval) statistics ───────────────────────────
	fmt.Println()
	fmt.Println("=== Inter-Onset Interval / Jitter Statistics (ms) ===")
	fmt.Println()
	printTable(typeOrder, byType, pcts, jitterValues, true, true, *outlierK)

	// ── Paired-event onset difference statistics ───────────────────────────
	diffs := pairDiffs(allRows, *event1, *event2)
	fmt.Println()
	fmt.Printf("=== Paired-Event Onset Differences: %s → %s (ms) ===\n", *event1, *event2)
	fmt.Println()
	if len(diffs) == 0 {
		fmt.Printf("  (no pairs found — check that both %q and %q events exist in the input)\n", *event1, *event2)
	} else {
		printSingleRow(fmt.Sprintf("%s→%s", *event1, *event2), diffs, pcts, *outlierK)
	}
}

type outlierWarning struct {
	typ     string
	n       int
	maxDist float64
}

// printTable writes a percentile table to stdout, followed by outlier warnings
// and per-type histograms. outlierK is the maximum distance from the median in
// ms; values beyond it are excluded (0 = off).
// valuesFn extracts the sample values for a single type's rows.
func printTable(
	typeOrder []string,
	byType map[string][]row,
	pcts []int,
	valuesFn func([]row) []float64,
	isJitter bool,
	withHistogram bool,
	outlierK float64,
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

	// Collect (filtered) sorted values per type for warnings and histograms.
	valsByType := make(map[string][]float64, len(typeOrder))
	var warnings []outlierWarning

	for _, typ := range typeOrder {
		rows := byType[typ]
		vals := valuesFn(rows)
		if len(vals) == 0 {
			continue
		}
		sort.Float64s(vals)

		filtered, nOut := filterOutliers(vals, outlierK)
		if nOut > 0 {
			warnings = append(warnings, outlierWarning{typ, nOut, outlierK})
		}
		vals = filtered
		valsByType[typ] = vals

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

	for _, warn := range warnings {
		fmt.Printf("Warning: %d outliers detected in %s (> %.3f ms away from the median)\n",
			warn.n, warn.typ, warn.maxDist)
	}

	if withHistogram {
		for _, typ := range typeOrder {
			vals := valsByType[typ]
			if len(vals) == 0 {
				continue
			}
			fmt.Printf("\n  %s:\n", typ)
			printHistogram(vals)
		}
	}
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

// filterOutliers removes values more than maxDist ms away from the median.
// vals must be pre-sorted. Returns the filtered slice and the count of removed
// values. When maxDist≤0, no filtering is applied.
func filterOutliers(sorted []float64, maxDist float64) ([]float64, int) {
	if maxDist <= 0 {
		return sorted, 0
	}
	median := percentile(sorted, 50)

	filtered := sorted[:0:0] // reuse backing array type but start empty
	for _, v := range sorted {
		if math.Abs(v-median) <= maxDist {
			filtered = append(filtered, v)
		}
	}
	return filtered, len(sorted) - len(filtered)
}

// pairDiffs returns onset differences (event2.Onset − event1.Onset) for each
// event1, paired with the nearest following event2 (by Onset). If multiple
// event1s fall before the same event2, each gets paired independently.
func pairDiffs(allRows []row, typ1, typ2 string) []float64 {
	var onsets1, onsets2 []float64
	for _, r := range allRows {
		switch r.typ {
		case typ1:
			onsets1 = append(onsets1, r.onset)
		case typ2:
			onsets2 = append(onsets2, r.onset)
		}
	}
	if len(onsets1) == 0 || len(onsets2) == 0 {
		return nil
	}
	sort.Float64s(onsets1)
	sort.Float64s(onsets2)

	var diffs []float64
	for _, t1 := range onsets1 {
		// Binary search for the first event2 onset >= t1.
		idx := sort.SearchFloat64s(onsets2, t1)
		if idx >= len(onsets2) {
			continue // no following event2
		}
		diffs = append(diffs, onsets2[idx]-t1)
	}
	return diffs
}

// printSingleRow writes a single-row percentile table for the given label and
// values, followed by outlier warnings and a histogram.
func printSingleRow(label string, vals []float64, pcts []int, outlierK float64) {
	fakeByType := map[string][]row{label: {}}
	precomputed := make([]float64, len(vals))
	copy(precomputed, vals)
	printTable([]string{label}, fakeByType, pcts, func(_ []row) []float64 {
		return precomputed
	}, false, true, outlierK)
}

// printHistogram prints a 10-bin ASCII histogram of vals to stdout.
// Copied from github.com/chrplr/goxpyriment/tests/internal/timingstats.
func printHistogram(vals []float64) {
	const nBins = 10
	const barWidth = 40
	n := len(vals)
	if n == 0 {
		return
	}
	mn, mx := vals[0], vals[0]
	for _, v := range vals {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	binW := (mx - mn) / nBins
	if binW == 0 {
		binW = 1
	}
	counts := make([]int, nBins)
	for _, v := range vals {
		b := int((v - mn) / binW)
		if b >= nBins {
			b = nBins - 1
		}
		counts[b]++
	}
	maxCount := 0
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}
	fmt.Printf("  histogram (%d bins):\n", nBins)
	for i := 0; i < nBins; i++ {
		lo := mn + float64(i)*binW
		hi := lo + binW
		bar := ""
		if maxCount > 0 {
			stars := counts[i] * barWidth / maxCount
			for j := 0; j < stars; j++ {
				bar += "*"
			}
		}
		fmt.Printf("  [%8.3f, %8.3f) ms : %5d  %s\n", lo, hi, counts[i], bar)
	}
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
