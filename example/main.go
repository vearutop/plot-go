// Command example demonstrates embedding plot's charts in a plain net/http app -- e.g. an
// internal metrics page.
//
// Run with `go run ./example` and open http://localhost:8080.
package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/vearutop/plot-go"
)

func main() {
	mux := http.NewServeMux()

	mux.Handle("/plot/", http.StripPrefix("/plot/", plot.Handler()))
	mux.HandleFunc("/", index)

	log.Println("Listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux)) //nolint:gosec
}

func index(w http.ResponseWriter, _ *http.Request) {
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

	_, err = fmt.Fprintf(w, `<!DOCTYPE html>
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
</body>
</html>
`, plot.HeadTags("/plot/"), requestsChart, statusChart)
	if err != nil {
		log.Println("write response:", err.Error())
	}
}
