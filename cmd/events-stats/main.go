// events-stats: compute descriptive statistics on *-events.csv files produced by bbtk-capture.
//
// For each event type found in the input files it reports:
//   - percentiles 0 % (min), 10 %, 20 %, … 90 %, 100 % (max) of Duration
//   - the same percentiles of the inter-onset intervals (jitter) within each type
//   - the same percentiles of onset differences between temporally paired events
//     (event2.Onset − event1.Onset for the nearest following event2 after each event1)
//
// Usage:
//
//	events-stats [-event1 TYPE] [-no-md] file1-events.csv [file2-events.csv ...]
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"image/color"

	"github.com/chrplr/bbtkv3"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// Variables to be passed on the compilation command line with "-X main.Version=${VERSION} -X main.Build=${BUILD}"
var (
	Version string
	Build   string
)

type row struct {
	typ      string
	onset    float64
	duration float64
}

func main() {
	event1 := flag.String("event1", "TTLin1", "reference event type; onset differences are reported for every other event type relative to this one")
	outlierK := flag.Float64("detect-outliers", 50, "exclude values more than this many ms away from the median (set to 0 to disable)")
	noMD := flag.Bool("no-md", false, "skip writing the markdown report")
	noHTML := flag.Bool("no-html", false, "skip writing the HTML report")
	versionPtr := flag.Bool("V", false, "display version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [-event1 TYPE] [-detect-outliers MS] [-no-md] [-no-html] <file-events.csv> [file2-events.csv ...]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *versionPtr {
		fmt.Println(bbtkv3.VersionString(Version, Build))
		os.Exit(0)
	}

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

	pcts := []int{0, 5, 25, 50, 75, 95, 100}

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
	fmt.Println()
	fmt.Printf("=== Paired-Event Onset Differences relative to %s (ms) ===\n", *event1)
	fmt.Println()
	otherTypes := otherEventTypes(typeOrder, *event1)
	if len(otherTypes) == 0 {
		fmt.Printf("  (no other event types found to pair with %q)\n", *event1)
	} else {
		for _, typ2 := range otherTypes {
			diffs := pairDiffs(allRows, *event1, typ2)
			if len(diffs) == 0 {
				fmt.Printf("  (no pairs found for %s → %s)\n", *event1, typ2)
				continue
			}
			printSingleRow(fmt.Sprintf("%s→%s", *event1, typ2), diffs, pcts, *outlierK)
			fmt.Println()
		}
	}

	// ── Markdown report ────────────────────────────────────────────────────
	base := csvBasename(flag.Arg(0))
	if !*noMD {
		mdOut := base + ".md"
		if err := writeMarkdownReport(mdOut, typeOrder, byType, allRows, pcts, *event1, *outlierK); err != nil {
			fmt.Fprintf(os.Stderr, "error writing markdown report: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "markdown report written to %s\n", mdOut)
	}

	// ── HTML report ────────────────────────────────────────────────────────
	if !*noHTML {
		htmlOut := base + ".html"
		if err := writeHTMLReport(htmlOut, typeOrder, byType, allRows, pcts, *event1, *outlierK); err != nil {
			fmt.Fprintf(os.Stderr, "error writing HTML report: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "HTML report written to %s\n", htmlOut)
	}
}

// ── Markdown report ──────────────────────────────────────────────────────────

func writeMarkdownReport(
	mdPath string,
	typeOrder []string,
	byType map[string][]row,
	allRows []row,
	pcts []int,
	event1 string,
	outlierK float64,
) error {
	f, err := os.Create(mdPath)
	if err != nil {
		return err
	}
	defer f.Close()

	mdDir := filepath.Dir(mdPath)
	mdBase := strings.TrimSuffix(filepath.Base(mdPath), filepath.Ext(mdPath))

	// Helper: save a PNG histogram and return a relative path for the markdown.
	savePNG := func(vals []float64, title, xlabel, key string) (string, error) {
		fname := fmt.Sprintf("%s_%s.png", mdBase, sanitizeFilename(key))
		fpath := filepath.Join(mdDir, fname)
		if err := saveHistogramPNG(vals, title, xlabel, fpath); err != nil {
			return "", err
		}
		return fname, nil
	}

	// Helper: save a timeline scatter PNG and return a relative path.
	saveTimelinePNGRel := func(xs, ys []float64, title, xlabel, ylabel, key string) (string, error) {
		fname := fmt.Sprintf("%s_%s.png", mdBase, sanitizeFilename(key))
		fpath := filepath.Join(mdDir, fname)
		if err := saveTimelinePNG(xs, ys, title, xlabel, ylabel, fpath); err != nil {
			return "", err
		}
		return fname, nil
	}

	valuesFnDuration := func(rows []row) []float64 {
		vals := make([]float64, len(rows))
		for i, r := range rows {
			vals[i] = r.duration
		}
		return vals
	}

	fmt.Fprintln(f, "# Events Statistics Report")
	fmt.Fprintln(f)

	// ── Duration ────────────────────────────────────────────────────────────
	fmt.Fprintln(f, "## Duration Statistics (ms)")
	fmt.Fprintln(f)
	writeMDTable(f, typeOrder, byType, pcts, valuesFnDuration, false, outlierK)

	fmt.Fprintln(f)
	fmt.Fprintln(f, "### Duration Histograms")
	fmt.Fprintln(f)
	for _, typ := range typeOrder {
		vals, warn := filteredVals(byType[typ], valuesFnDuration, outlierK)
		if len(vals) == 0 {
			continue
		}
		if warn != nil {
			fmt.Fprintf(f, "> **Warning:** %d outlier(s) excluded from %s (> %.3f ms from median)\n\n", warn.n, typ, warn.maxDist)
		}
		title := fmt.Sprintf("Duration – %s", typ)
		pngRel, err := savePNG(vals, title, "Duration (ms)", "dur_"+typ)
		if err != nil {
			return err
		}
		fmt.Fprintf(f, "![%s](%s)\n\n", title, pngRel)
	}

	// ── Jitter ──────────────────────────────────────────────────────────────
	fmt.Fprintln(f, "## Inter-Onset Interval / Jitter Statistics (ms)")
	fmt.Fprintln(f)
	writeMDTable(f, typeOrder, byType, pcts, jitterValues, true, outlierK)

	fmt.Fprintln(f)
	fmt.Fprintln(f, "### Jitter Histograms")
	fmt.Fprintln(f)
	for _, typ := range typeOrder {
		vals, warn := filteredVals(byType[typ], jitterValues, outlierK)
		if len(vals) == 0 {
			continue
		}
		if warn != nil {
			fmt.Fprintf(f, "> **Warning:** %d outlier(s) excluded from %s (> %.3f ms from median)\n\n", warn.n, typ, warn.maxDist)
		}
		title := fmt.Sprintf("Jitter (IOI) – %s", typ)
		pngRel, err := savePNG(vals, title, "Inter-Onset Interval (ms)", "jitter_"+typ)
		if err != nil {
			return err
		}
		fmt.Fprintf(f, "![%s](%s)\n\n", title, pngRel)
	}

	// ── Paired differences ──────────────────────────────────────────────────
	fmt.Fprintf(f, "## Paired-Event Onset Differences relative to %s (ms)\n\n", event1)
	otherTypes := otherEventTypes(typeOrder, event1)
	if len(otherTypes) == 0 {
		fmt.Fprintf(f, "_No other event types found to pair with `%s`._\n\n", event1)
	} else {
		fmt.Fprintln(f, "### Tables")
		fmt.Fprintln(f)
		// Build a combined table with one row per pairing.
		type pairResult struct {
			label string
			vals  []float64
		}
		var pairs []pairResult
		for _, typ2 := range otherTypes {
			diffs := pairDiffs(allRows, event1, typ2)
			if len(diffs) == 0 {
				continue
			}
			pairs = append(pairs, pairResult{fmt.Sprintf("%s→%s", event1, typ2), diffs})
		}
		for _, pr := range pairs {
			label := pr.label
			precomputed := pr.vals
			fakeByType := map[string][]row{label: {}}
			writeMDTable(f, []string{label}, fakeByType, pcts, func(_ []row) []float64 { return precomputed }, false, outlierK)
			fmt.Fprintln(f)
		}

		fmt.Fprintln(f, "### Paired-Event Histograms")
		fmt.Fprintln(f)
		for _, pr := range pairs {
			vals, warn := filteredVals(nil, func(_ []row) []float64 { return pr.vals }, outlierK)
			if warn != nil {
				fmt.Fprintf(f, "> **Warning:** %d outlier(s) excluded from %s (> %.3f ms from median)\n\n", warn.n, pr.label, warn.maxDist)
			}
			if len(vals) == 0 {
				continue
			}
			title := fmt.Sprintf("Onset Diff – %s", pr.label)
			key := "diff_" + sanitizeFilename(pr.label)
			pngRel, err := savePNG(vals, title, "Onset Difference (ms)", key)
			if err != nil {
				return err
			}
			fmt.Fprintf(f, "![%s](%s)\n\n", title, pngRel)
		}
	}

	// ── Timeline plots ──────────────────────────────────────────────────────
	fmt.Fprintln(f, "## Timeline Plots")
	fmt.Fprintln(f)
	for _, typ := range typeOrder {
		rows := byType[typ]
		if len(rows) == 0 {
			continue
		}
		// Sort rows by onset for consistent x ordering.
		sorted := make([]row, len(rows))
		copy(sorted, rows)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].onset < sorted[j].onset })

		// Duration vs. time.
		xs := make([]float64, len(sorted))
		ys := make([]float64, len(sorted))
		for i, r := range sorted {
			xs[i] = r.onset
			ys[i] = r.duration
		}
		title := fmt.Sprintf("Duration over time – %s", typ)
		pngRel, err := saveTimelinePNGRel(xs, ys, title, "Onset (ms)", "Duration (ms)", "timeline_dur_"+typ)
		if err != nil {
			return err
		}
		fmt.Fprintf(f, "![%s](%s)\n\n", title, pngRel)

		// SOA vs. time (x = onset[i], y = onset[i+1] − onset[i]).
		if len(sorted) < 2 {
			continue
		}
		soaXs := make([]float64, len(sorted)-1)
		soaYs := make([]float64, len(sorted)-1)
		for i := 0; i < len(sorted)-1; i++ {
			soaXs[i] = sorted[i].onset
			soaYs[i] = sorted[i+1].onset - sorted[i].onset
		}
		title = fmt.Sprintf("SOA over time – %s", typ)
		pngRel, err = saveTimelinePNGRel(soaXs, soaYs, title, "Onset (ms)", "SOA (ms)", "timeline_soa_"+typ)
		if err != nil {
			return err
		}
		fmt.Fprintf(f, "![%s](%s)\n\n", title, pngRel)
	}

	return nil
}

// ── HTML report ──────────────────────────────────────────────────────────────

const htmlHeader = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Events Statistics Report</title>
<style>
  body { font-family: sans-serif; max-width: 1100px; margin: 2em auto; color: #222; }
  h1 { border-bottom: 2px solid #444; padding-bottom: 0.3em; }
  h2 { border-bottom: 1px solid #aaa; margin-top: 2em; }
  table { border-collapse: collapse; margin: 1em 0; font-size: 0.9em; }
  th, td { border: 1px solid #ccc; padding: 0.35em 0.7em; text-align: right; }
  th { background: #f0f0f0; text-align: center; }
  td:first-child, th:first-child { text-align: left; }
  img { max-width: 100%; margin: 0.5em 0 1.5em; display: block; }
  blockquote { background: #fff8e1; border-left: 4px solid #f5a623; margin: 0.5em 0; padding: 0.4em 0.8em; }
</style>
</head>
<body>
<h1>Events Statistics Report</h1>
`

const htmlFooter = `</body>
</html>
`

func writeHTMLReport(
	htmlPath string,
	typeOrder []string,
	byType map[string][]row,
	allRows []row,
	pcts []int,
	event1 string,
	outlierK float64,
) error {
	f, err := os.Create(htmlPath)
	if err != nil {
		return err
	}
	defer f.Close()

	htmlDir := filepath.Dir(htmlPath)
	htmlBase := strings.TrimSuffix(filepath.Base(htmlPath), filepath.Ext(htmlPath))

	savePNG := func(vals []float64, title, xlabel, key string) (string, error) {
		fname := fmt.Sprintf("%s_%s.png", htmlBase, sanitizeFilename(key))
		fpath := filepath.Join(htmlDir, fname)
		if err := saveHistogramPNG(vals, title, xlabel, fpath); err != nil {
			return "", err
		}
		return fname, nil
	}

	saveTimelinePNGRel := func(xs, ys []float64, title, xlabel, ylabel, key string) (string, error) {
		fname := fmt.Sprintf("%s_%s.png", htmlBase, sanitizeFilename(key))
		fpath := filepath.Join(htmlDir, fname)
		if err := saveTimelinePNG(xs, ys, title, xlabel, ylabel, fpath); err != nil {
			return "", err
		}
		return fname, nil
	}

	valuesFnDuration := func(rows []row) []float64 {
		vals := make([]float64, len(rows))
		for i, r := range rows {
			vals[i] = r.duration
		}
		return vals
	}

	fmt.Fprint(f, htmlHeader)

	// ── Duration ────────────────────────────────────────────────────────────
	fmt.Fprintln(f, "<h2>Duration Statistics (ms)</h2>")
	writeHTMLTable(f, typeOrder, byType, pcts, valuesFnDuration, false, outlierK)

	fmt.Fprintln(f, "<h3>Duration Histograms</h3>")
	for _, typ := range typeOrder {
		vals, warn := filteredVals(byType[typ], valuesFnDuration, outlierK)
		if len(vals) == 0 {
			continue
		}
		if warn != nil {
			fmt.Fprintf(f, "<blockquote><strong>Warning:</strong> %d outlier(s) excluded from %s (&gt; %.3f ms from median)</blockquote>\n", warn.n, typ, warn.maxDist)
		}
		title := fmt.Sprintf("Duration – %s", typ)
		pngRel, err := savePNG(vals, title, "Duration (ms)", "dur_"+typ)
		if err != nil {
			return err
		}
		fmt.Fprintf(f, "<img src=\"%s\" alt=\"%s\">\n", pngRel, title)
	}

	// ── Jitter ──────────────────────────────────────────────────────────────
	fmt.Fprintln(f, "<h2>Inter-Onset Interval / Jitter Statistics (ms)</h2>")
	writeHTMLTable(f, typeOrder, byType, pcts, jitterValues, true, outlierK)

	fmt.Fprintln(f, "<h3>Jitter Histograms</h3>")
	for _, typ := range typeOrder {
		vals, warn := filteredVals(byType[typ], jitterValues, outlierK)
		if len(vals) == 0 {
			continue
		}
		if warn != nil {
			fmt.Fprintf(f, "<blockquote><strong>Warning:</strong> %d outlier(s) excluded from %s (&gt; %.3f ms from median)</blockquote>\n", warn.n, typ, warn.maxDist)
		}
		title := fmt.Sprintf("Jitter (IOI) – %s", typ)
		pngRel, err := savePNG(vals, title, "Inter-Onset Interval (ms)", "jitter_"+typ)
		if err != nil {
			return err
		}
		fmt.Fprintf(f, "<img src=\"%s\" alt=\"%s\">\n", pngRel, title)
	}

	// ── Paired differences ──────────────────────────────────────────────────
	fmt.Fprintf(f, "<h2>Paired-Event Onset Differences relative to %s (ms)</h2>\n", event1)
	otherTypes := otherEventTypes(typeOrder, event1)
	if len(otherTypes) == 0 {
		fmt.Fprintf(f, "<p><em>No other event types found to pair with <code>%s</code>.</em></p>\n", event1)
	} else {
		fmt.Fprintln(f, "<h3>Tables</h3>")
		type pairResult struct {
			label string
			vals  []float64
		}
		var pairs []pairResult
		for _, typ2 := range otherTypes {
			diffs := pairDiffs(allRows, event1, typ2)
			if len(diffs) == 0 {
				continue
			}
			pairs = append(pairs, pairResult{fmt.Sprintf("%s→%s", event1, typ2), diffs})
		}
		for _, pr := range pairs {
			label := pr.label
			precomputed := pr.vals
			fakeByType := map[string][]row{label: {}}
			writeHTMLTable(f, []string{label}, fakeByType, pcts, func(_ []row) []float64 { return precomputed }, false, outlierK)
		}

		fmt.Fprintln(f, "<h3>Paired-Event Histograms</h3>")
		for _, pr := range pairs {
			vals, warn := filteredVals(nil, func(_ []row) []float64 { return pr.vals }, outlierK)
			if warn != nil {
				fmt.Fprintf(f, "<blockquote><strong>Warning:</strong> %d outlier(s) excluded from %s (&gt; %.3f ms from median)</blockquote>\n", warn.n, pr.label, warn.maxDist)
			}
			if len(vals) == 0 {
				continue
			}
			title := fmt.Sprintf("Onset Diff – %s", pr.label)
			key := "diff_" + sanitizeFilename(pr.label)
			pngRel, err := savePNG(vals, title, "Onset Difference (ms)", key)
			if err != nil {
				return err
			}
			fmt.Fprintf(f, "<img src=\"%s\" alt=\"%s\">\n", pngRel, title)
		}
	}

	// ── Timeline plots ──────────────────────────────────────────────────────
	fmt.Fprintln(f, "<h2>Timeline Plots</h2>")
	for _, typ := range typeOrder {
		rows := byType[typ]
		if len(rows) == 0 {
			continue
		}
		sorted := make([]row, len(rows))
		copy(sorted, rows)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].onset < sorted[j].onset })

		xs := make([]float64, len(sorted))
		ys := make([]float64, len(sorted))
		for i, r := range sorted {
			xs[i] = r.onset
			ys[i] = r.duration
		}
		title := fmt.Sprintf("Duration over time – %s", typ)
		pngRel, err := saveTimelinePNGRel(xs, ys, title, "Onset (ms)", "Duration (ms)", "timeline_dur_"+typ)
		if err != nil {
			return err
		}
		fmt.Fprintf(f, "<img src=\"%s\" alt=\"%s\">\n", pngRel, title)

		if len(sorted) < 2 {
			continue
		}
		soaXs := make([]float64, len(sorted)-1)
		soaYs := make([]float64, len(sorted)-1)
		for i := 0; i < len(sorted)-1; i++ {
			soaXs[i] = sorted[i].onset
			soaYs[i] = sorted[i+1].onset - sorted[i].onset
		}
		title = fmt.Sprintf("SOA over time – %s", typ)
		pngRel, err = saveTimelinePNGRel(soaXs, soaYs, title, "Onset (ms)", "SOA (ms)", "timeline_soa_"+typ)
		if err != nil {
			return err
		}
		fmt.Fprintf(f, "<img src=\"%s\" alt=\"%s\">\n", pngRel, title)
	}

	fmt.Fprint(f, htmlFooter)
	return nil
}

// writeHTMLTable writes an HTML <table> of percentile statistics to w.
func writeHTMLTable(
	w io.Writer,
	typeOrder []string,
	byType map[string][]row,
	pcts []int,
	valuesFn func([]row) []float64,
	isJitter bool,
	outlierK float64,
) {
	header := []string{"Type", "N"}
	for _, p := range pcts {
		switch p {
		case 0:
			header = append(header, "Min")
		case 100:
			header = append(header, "Max")
		default:
			header = append(header, fmt.Sprintf("P%d", p))
		}
	}
	header = append(header, "Range", "P95-P05", "Mean", "SD")

	fmt.Fprintln(w, "<table>")
	fmt.Fprint(w, "<tr>")
	for _, h := range header {
		fmt.Fprintf(w, "<th>%s</th>", h)
	}
	fmt.Fprintln(w, "</tr>")

	for _, typ := range typeOrder {
		rows := byType[typ]
		vals := valuesFn(rows)
		if len(vals) == 0 {
			continue
		}
		sort.Float64s(vals)
		filtered, _ := filterOutliers(vals, outlierK)
		vals = filtered

		cols := []string{typ, strconv.Itoa(len(vals))}
		for _, p := range pcts {
			cols = append(cols, fmt.Sprintf("%.1f", percentile(vals, float64(p))))
		}
		rng := vals[len(vals)-1] - vals[0]
		avg := average(vals)
		iqr := percentile(vals, 95) - percentile(vals, 5)
		cols = append(cols,
			fmt.Sprintf("%.1f", rng),
			fmt.Sprintf("%.1f", iqr),
			fmt.Sprintf("%.1f", avg),
			fmt.Sprintf("%.2f", stddev(vals)),
		)
		fmt.Fprint(w, "<tr>")
		for _, c := range cols {
			fmt.Fprintf(w, "<td>%s</td>", c)
		}
		fmt.Fprintln(w, "</tr>")
	}
	fmt.Fprintln(w, "</table>")
}

// filteredVals extracts and filters values for a single type.
// rows may be nil when vals are pre-computed via valuesFn ignoring rows.
func filteredVals(rows []row, valuesFn func([]row) []float64, outlierK float64) ([]float64, *outlierWarning) {
	vals := valuesFn(rows)
	if len(vals) == 0 {
		return nil, nil
	}
	sort.Float64s(vals)
	filtered, outliers := filterOutliers(vals, outlierK)
	if len(outliers) > 0 {
		return filtered, &outlierWarning{n: len(outliers), maxDist: outlierK, vals: outliers}
	}
	return filtered, nil
}

// stickPlotter draws a vertical line from y=0 to each (x, y) point.
type stickPlotter struct {
	pts       plotter.XYs
	lineStyle draw.LineStyle
}

func newStickPlotter(xs, ys []float64) *stickPlotter {
	pts := make(plotter.XYs, len(xs))
	for i := range xs {
		pts[i].X = xs[i]
		pts[i].Y = ys[i]
	}
	return &stickPlotter{
		pts: pts,
		lineStyle: draw.LineStyle{
			Color: color.RGBA{R: 70, G: 130, B: 180, A: 255},
			Width: vg.Points(0.5),
		},
	}
}

func (s *stickPlotter) Plot(c draw.Canvas, plt *plot.Plot) {
	trX, trY := plt.Transforms(&c)
	y0 := trY(0)
	for _, pt := range s.pts {
		x := trX(pt.X)
		y1 := trY(pt.Y)
		c.StrokeLine2(s.lineStyle, x, y0, x, y1)
	}
}

func (s *stickPlotter) DataRange() (xmin, xmax, ymin, ymax float64) {
	xmin, xmax, ymin, ymax = plotter.XYRange(s.pts)
	if ymin > 0 {
		ymin = 0
	}
	return
}

// saveTimelinePNG saves a stick plot of ys vs xs to path.
func saveTimelinePNG(xs, ys []float64, title, xlabel, ylabel, path string) error {
	p := plot.New()
	p.Title.Text = title
	p.X.Label.Text = xlabel
	p.Y.Label.Text = ylabel
	p.Add(newStickPlotter(xs, ys))
	return p.Save(7*vg.Inch, 3*vg.Inch, path)
}

// logHistPlotter draws a histogram with a log Y axis.
// Empty bins are rendered at floor so the log axis stays well-defined.
type logHistPlotter struct {
	edges  []float64 // nBins+1 bin edges
	counts []float64 // nBins heights (>= floor)
	floor  float64
	color  color.Color
}

func newLogHistPlotter(vals []float64, nBins int, floor float64) *logHistPlotter {
	mn, mx := vals[0], vals[0]
	for _, v := range vals {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	binW := (mx - mn) / float64(nBins)
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
	edges := make([]float64, nBins+1)
	for i := range edges {
		edges[i] = mn + float64(i)*binW
	}
	heights := make([]float64, nBins)
	for i, c := range counts {
		if c == 0 {
			heights[i] = floor
		} else {
			heights[i] = float64(c)
		}
	}
	return &logHistPlotter{
		edges:  edges,
		counts: heights,
		floor:  floor,
		color:  color.RGBA{R: 100, G: 149, B: 237, A: 200},
	}
}

func (h *logHistPlotter) Plot(c draw.Canvas, plt *plot.Plot) {
	trX, trY := plt.Transforms(&c)
	for i, cnt := range h.counts {
		x0 := trX(h.edges[i])
		x1 := trX(h.edges[i+1])
		y0 := trY(h.floor)
		y1 := trY(cnt)
		pts := []vg.Point{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
		c.FillPolygon(h.color, pts)
		c.StrokeLines(draw.LineStyle{Color: color.Gray{Y: 80}, Width: vg.Points(0.3)},
			[]vg.Point{{X: x0, Y: y0}, {X: x0, Y: y1}, {X: x1, Y: y1}, {X: x1, Y: y0}})
	}
}

func (h *logHistPlotter) DataRange() (xmin, xmax, ymin, ymax float64) {
	xmin = h.edges[0]
	xmax = h.edges[len(h.edges)-1]
	ymin = h.floor
	ymax = h.floor
	for _, c := range h.counts {
		if c > ymax {
			ymax = c
		}
	}
	return
}

// saveHistogramPNG saves a log-scale histogram PNG of vals to path.
func saveHistogramPNG(vals []float64, title, xlabel, path string) error {
	const nBins = 20
	const floor = 0.5

	p := plot.New()
	p.Title.Text = title
	p.X.Label.Text = xlabel
	p.Y.Label.Text = "Count (log₁₀)"
	p.Y.Scale = plot.LogScale{}
	p.Y.Tick.Marker = plot.LogTicks{}
	p.Add(newLogHistPlotter(vals, nBins, floor))

	return p.Save(5*vg.Inch, 3*vg.Inch, path)
}

// writeMDTable writes a markdown-formatted percentile table to w.
func writeMDTable(
	w io.Writer,
	typeOrder []string,
	byType map[string][]row,
	pcts []int,
	valuesFn func([]row) []float64,
	isJitter bool,
	outlierK float64,
) {
	// Header
	header := []string{"Type", "N"}
	for _, p := range pcts {
		switch p {
		case 0:
			header = append(header, "Min")
		case 100:
			header = append(header, "Max")
		default:
			header = append(header, fmt.Sprintf("P%d", p))
		}
	}
	header = append(header, "Range", "P95-P05", "Mean", "SD")

	fmt.Fprintln(w, "| "+strings.Join(header, " | ")+" |")
	seps := make([]string, len(header))
	for i := range header {
		seps[i] = "---"
	}
	fmt.Fprintln(w, "| "+strings.Join(seps, " | ")+" |")

	for _, typ := range typeOrder {
		rows := byType[typ]
		vals := valuesFn(rows)
		if len(vals) == 0 {
			continue
		}
		sort.Float64s(vals)
		filtered, _ := filterOutliers(vals, outlierK)
		vals = filtered

		cols := []string{typ, strconv.Itoa(len(vals))}
		for _, p := range pcts {
			cols = append(cols, fmt.Sprintf("%.1f", percentile(vals, float64(p))))
		}
		rng := vals[len(vals)-1] - vals[0]
		avg := average(vals)
		iqr := percentile(vals, 95) - percentile(vals, 5)
		cols = append(cols,
			fmt.Sprintf("%.1f", rng),
			fmt.Sprintf("%.1f", iqr),
			fmt.Sprintf("%.1f", avg),
			fmt.Sprintf("%.2f", stddev(vals)),
		)
		fmt.Fprintln(w, "| "+strings.Join(cols, " | ")+" |")
	}
}

// otherEventTypes returns typeOrder minus the reference type, in original order.
func otherEventTypes(typeOrder []string, ref string) []string {
	var out []string
	for _, t := range typeOrder {
		if t != ref {
			out = append(out, t)
		}
	}
	return out
}

// csvBasename strips -events.csv (or .events.csv or .csv) from path and returns the result,
// preserving the directory so output lands next to the input file.
func csvBasename(path string) string {
	base := filepath.Base(path)
	for _, suffix := range []string{"-events.csv", ".events.csv", ".csv"} {
		if strings.HasSuffix(base, suffix) {
			base = strings.TrimSuffix(base, suffix)
			break
		}
	}
	return filepath.Join(filepath.Dir(path), base)
}

// sanitizeFilename replaces characters unsafe for filenames with underscores.
// sanitizeFilename maps a display label to a portable file name. The result is
// restricted to ASCII letters, digits, '.', '_' and '-': Go module archives
// reject paths containing non-ASCII characters, so a committed report figure
// named after a pair label (e.g. "TTLin1→Mic1") would make the whole module
// impossible to `go install`.
func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '→':
			b.WriteString("_to_")
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// ── Stdout helpers ───────────────────────────────────────────────────────────

func average(vals []float64) float64 {
	n := len(vals)
	if n == 0 {
		panic("Cannot compute the average of a empty vector")
	}
	sum := 0.0
	for _, x := range vals {
		sum += x
	}
	return sum / float64(n)
}

type outlierWarning struct {
	typ     string
	n       int
	maxDist float64
	vals    []float64
}

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
	header = append(header, "Range", "P95-P05", "Mean", "SD")
	fmt.Fprintln(w, strings.Join(header, "\t"))

	seps := make([]string, len(header))
	for i, h := range header {
		seps[i] = strings.Repeat("-", len(h))
	}
	fmt.Fprintln(w, strings.Join(seps, "\t"))

	valsByType := make(map[string][]float64, len(typeOrder))
	var warnings []outlierWarning

	for _, typ := range typeOrder {
		rows := byType[typ]
		vals := valuesFn(rows)
		if len(vals) == 0 {
			continue
		}
		sort.Float64s(vals)

		filtered, outliers := filterOutliers(vals, outlierK)
		if len(outliers) > 0 {
			warnings = append(warnings, outlierWarning{typ, len(outliers), outlierK, outliers})
		}
		vals = filtered
		valsByType[typ] = vals

		cols := []string{typ, strconv.Itoa(len(vals))}
		for _, p := range pcts {
			cols = append(cols, fmt.Sprintf("%.1f", percentile(vals, float64(p))))
		}
		rng := vals[len(vals)-1] - vals[0]
		avg := average(vals)
		iqr := percentile(vals, 95) - percentile(vals, 5)
		cols = append(cols,
			fmt.Sprintf("%.1f", rng),
			fmt.Sprintf("%.1f", iqr),
			fmt.Sprintf("%.1f", avg),
			fmt.Sprintf("%.2f", stddev(vals)),
		)
		fmt.Fprintln(w, strings.Join(cols, "\t"))
	}
	w.Flush()

	for _, warn := range warnings {
		strs := make([]string, len(warn.vals))
		for i, v := range warn.vals {
			strs[i] = fmt.Sprintf("%.3f", v)
		}
		fmt.Printf("Warning: %d outliers detected in %s (> %.3f ms away from the median): %s\n",
			warn.n, warn.typ, warn.maxDist, strings.Join(strs, ", "))
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

func filterOutliers(sorted []float64, maxDist float64) (filtered, outliers []float64) {
	if maxDist <= 0 {
		return sorted, nil
	}
	median := percentile(sorted, 50)
	for _, v := range sorted {
		if math.Abs(v-median) <= maxDist {
			filtered = append(filtered, v)
		} else {
			outliers = append(outliers, v)
		}
	}
	return filtered, outliers
}

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
		idx := sort.SearchFloat64s(onsets2, t1)
		if idx >= len(onsets2) {
			continue
		}
		diffs = append(diffs, onsets2[idx]-t1)
	}
	return diffs
}

func printSingleRow(label string, vals []float64, pcts []int, outlierK float64) {
	fakeByType := map[string][]row{label: {}}
	precomputed := make([]float64, len(vals))
	copy(precomputed, vals)
	printTable([]string{label}, fakeByType, pcts, func(_ []row) []float64 {
		return precomputed
	}, false, true, outlierK)
}

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

func readCSV(path string) ([]row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true

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
