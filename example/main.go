// Command example demonstrates embedding plot's charts in a plain net/http app -- e.g. an
// internal metrics page.
//
// Run with `go run ./example` and open http://localhost:8080.
package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/vearutop/plot-go"
)

type point struct {
	ts    int64
	value int
}

// live is a rolling series of per-second points, new ones are produced in background.
type live struct {
	mu     sync.Mutex
	points []point // In ascending order of time.
}

const liveKeep = 600

func newLive() *live {
	l := &live{}

	v := 100
	now := time.Now()

	for i := 60; i > 0; i-- {
		v = next(v)
		l.points = append(l.points, point{now.Add(-time.Duration(i) * time.Second).Unix(), v})
	}

	go func() {
		for t := range time.Tick(time.Second) {
			v = next(v)

			l.mu.Lock()
			l.points = append(l.points, point{t.Unix(), v})

			if len(l.points) > liveKeep {
				l.points = l.points[len(l.points)-liveKeep:]
			}
			l.mu.Unlock()
		}
	}()

	return l
}

// next is a random walk, so the chart looks alive.
func next(v int) int {
	return min(max(v+rand.Intn(21)-10, 20), 200) //nolint:gosec
}

// since returns points that are newer than ts, as chart rows.
func (l *live) since(ts int64) [][]any {
	l.mu.Lock()
	defer l.mu.Unlock()

	rows := [][]any{}

	for _, p := range l.points {
		if p.ts > ts {
			rows = append(rows, []any{p.ts, p.value})
		}
	}

	return rows
}

// liveChart renders the chart with the points collected so far, and the time of the last of them.
func liveChart(l *live) (template.HTML, int64, error) {
	rows := l.since(0)
	last := rows[len(rows)-1][0].(int64) //nolint:forcetypeassert,errcheck // Rows are made by since.

	chart, err := plot.Render("live", plot.Data{Columns: []string{"time", "requests per second"}, Values: rows}, plot.Options{
		Caption: "Requests per second (live, new points are appended every second)",
		X:       plot.Axis{Type: plot.TimeAxis},
		Height:  250,
	})

	return chart, last, err
}

// sampleSeconds returns a random duration with a long tail (median about 20s), e.g. a click to install time.
// Shift moves the distribution, so the heatmap has something to show.
func sampleSeconds(shift float64) float64 {
	return math.Exp(rand.NormFloat64()*1.2+3+shift) + 1 //nolint:gosec
}

// conversionHistogram counts samples of two series in equal-width 10s buckets up to 5 minutes. A row is
// (series, min, max, count, percent of all observations), series are stacked. For a single series omit the first column.
func conversionHistogram() plot.Data {
	const width, buckets = 10, 30

	data := plot.Data{Columns: []string{"series", "min", "max", "count", "percent"}}
	counts := map[string][]int{"organic": make([]int, buckets), "paid": make([]int, buckets)}
	total := 0

	for _, series := range []struct {
		name  string
		shift float64
	}{{"organic", 0}, {"paid", 0.5}} {
		for range 3000 {
			if b := int(sampleSeconds(series.shift) / width); b < buckets {
				counts[series.name][b]++
				total++
			}
		}
	}

	for _, name := range []string{"organic", "paid"} {
		for i, c := range counts[name] {
			if c > 0 {
				data.Values = append(data.Values, []any{name, i * width, (i + 1) * width, c, float64(c) / float64(total) * 100})
			}
		}
	}

	return data
}

// conversionHeatmap is a histogram per hour for a day. All hours share the same exponential buckets
// (1s, 2s, 4s, ... 1d), a row is (x, min, max, count, percent of the hour).
func conversionHeatmap() plot.Data {
	edges := []float64{0}
	for e := 1.0; e <= 86400; e *= 2 {
		edges = append(edges, e)
	}

	edges = append(edges, math.Inf(1))

	data := plot.Data{Columns: []string{"hour", "min", "max", "count", "percent"}}
	start := time.Now().Add(-24 * time.Hour).Truncate(time.Hour)

	for h := range 24 {
		counts := make([]int, len(edges)-1)
		total := 0

		for range 300 {
			v := sampleSeconds(0.8 * math.Sin(float64(h)/24*2*math.Pi))

			for i := range counts {
				if v >= edges[i] && v < edges[i+1] {
					counts[i]++
					total++

					break
				}
			}
		}

		for i, c := range counts {
			if c == 0 {
				continue
			}

			upper := edges[i+1]
			if math.IsInf(upper, 1) {
				upper = edges[i] * 2
			}

			data.Values = append(data.Values, []any{
				start.Add(time.Duration(h) * time.Hour).Unix(), edges[i], upper, c, float64(c) / float64(total) * 100,
			})
		}
	}

	return data
}

func main() {
	mux := http.NewServeMux()
	l := newLive()

	mux.Handle("/plot/", http.StripPrefix("/plot/", plot.Handler()))
	mux.HandleFunc("/points", func(w http.ResponseWriter, r *http.Request) {
		ts, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64) //nolint:errcheck // Zero means all points.

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(l.since(ts)); err != nil {
			log.Println("write points:", err.Error())
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { index(w, r, l) })

	log.Println("Listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux)) //nolint:gosec
}

func index(w http.ResponseWriter, _ *http.Request, l *live) {
	requests := plot.Data{Columns: []string{"time", "requests"}}

	start := time.Now().Add(-time.Hour)
	for i := range 60 {
		requests.Values = append(requests.Values, []any{
			start.Add(time.Duration(i) * time.Minute).Unix(),
			50 + rand.Intn(50), //nolint:gosec
		})
	}

	statuses := plot.Data{
		Columns: []string{"count", "status"},
		Values: [][]any{
			{982, "200 OK"},
			{41, "404 Not Found"},
			{7, "500 Internal Error"},
		},
	}

	// Initial dataset of the live chart, further points are appended by the page script.
	liveHTML, last, err := liveChart(l)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	requestsChart, err := plot.Render("requests", requests, plot.Options{
		Caption: "Requests per minute",
		X:       plot.Axis{Type: plot.TimeAxis},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	statusChart, err := plot.Render("statuses", statuses, plot.Options{
		Kind:    plot.Pie,
		Caption: "Response statuses",
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	histogramChart, err := plot.Render("histogram", conversionHistogram(), plot.Options{
		Kind:    plot.Histogram,
		Caption: "Click to install time (stacked histogram)",
		// The unit belongs to the measured value, the chart knows which axis it is on.
		Value: plot.Field{Unit: plot.Seconds, Label: "install time"},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	heatmapChart, err := plot.Render("heatmap", conversionHeatmap(), plot.Options{
		Kind:    plot.Heatmap,
		Caption: "Click to install time by hour (heatmap)",
		// X is the time slice, formatted as time. The measured value (seconds) is on the Y axis: set Value.Unit.
		X:       plot.Axis{Type: plot.TimeAxis},
		Value:   plot.Field{Unit: plot.Seconds},
		Heatmap: &plot.HeatmapOptions{Percent: true},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	_, err = fmt.Fprintf(w, pageTemplate, plot.HeadTags("/plot/"), liveHTML, requestsChart, statusChart, histogramChart, heatmapChart, last)
	if err != nil {
		log.Println("write response:", err.Error())
	}
}

// pageTemplate takes: head tags, live, requests, status, histogram and heatmap charts, time of the last point of the live chart.
const pageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<title>App Metrics</title>
%s
</head>
<body style="margin: 2em">
<h1>App Metrics</h1>
%s
%s
%s
%s
%s
<script>
// Ask for points newer than the last one that is on the chart, and append them.
let last = %d
setInterval(async () => {
	const rows = await (await fetch('/points?since=' + last)).json()
	if (rows.length > 0 && Plot.append('live', rows)) {
		last = rows[rows.length - 1][0]
	}
}, 1000)
</script>
</body>
</html>
`
