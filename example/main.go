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
		Time:    true,
		Height:  250,
	})

	return chart, last, err
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
		Time:    true,
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	_, err = fmt.Fprintf(w, pageTemplate, plot.HeadTags("/plot/"), liveHTML, requestsChart, statusChart, last)
	if err != nil {
		log.Println("write response:", err.Error())
	}
}

// pageTemplate takes: head tags, live chart, requests chart, status chart, time of the last point of the live chart.
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
