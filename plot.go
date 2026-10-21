// Package plot renders interactive line, stacked-bar and pie charts (backed by uPlot) as an
// HTML fragment, so any Go web app can embed a chart by calling Render with tabular data.
//
// Mount Handler under a path prefix and include HeadTags(prefix) in the page's <head>; then
// Render returns a container element plus the inline script that draws into it once those
// assets have loaded:
//
//	mux.Handle("/plot/", http.StripPrefix("/plot/", plot.Handler()))
//	head := plot.HeadTags("/plot/")
//	chart, err := plot.Render("chart-1", plot.Data{Columns: cols, Values: rows}, plot.Options{Caption: "Requests"})
package plot

import (
	"embed"
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/vearutop/statigz"
)

//go:embed static/*
var staticAssets embed.FS

var staticServer = statigz.FileServer(staticAssets, statigz.FSPrefix("static"))

// Handler serves the package's static assets (uPlot library and the chart rendering script).
func Handler() http.Handler {
	return staticServer
}

// HeadTags returns the <script>/<link> tags to load into a page's <head>, with asset URLs
// rooted at prefix, the same prefix Handler is mounted under.
func HeadTags(prefix string) template.HTML {
	return template.HTML( //nolint:gosec
		`<script src="` + prefix + `uPlot.iife.min.js"></script>` +
			`<link rel="stylesheet" href="` + prefix + `uPlot.min.css">` +
			`<script src="` + prefix + `plot.js"></script>`,
	)
}

// Data is a generic tabular dataset to chart.
//
// In the default column shape, the first column is the X axis and each remaining column is
// one series. In row shape (Options.Rows), each row is (x, y, label) and rows sharing a
// label form one series. For Options.Kind Pie, each row is (count, label) or (label, count).
type Data struct {
	Columns []string `json:"columns"`
	Values  [][]any  `json:"values"`
}

// Kind selects the chart type.
type Kind string

// Supported chart kinds.
const (
	Line Kind = "line"
	Pie  Kind = "pie"
)

// Options configures how Data is rendered.
type Options struct {
	Kind Kind `json:"kind,omitempty"` // Default Line.

	// Rows treats Data as (x, y, label) rows instead of (x, y1, y2, ...) columns. Ignored for Pie.
	Rows bool `json:"rows,omitempty"`

	// StackedBars renders a stacked bar chart instead of lines. Ignored for Pie.
	StackedBars bool `json:"stackedBars,omitempty"`

	// Time treats the X axis as UTC Unix timestamps. Ignored for Pie.
	Time bool `json:"time,omitempty"`

	// TimeAxisFmt overrides the X axis label format, e.g. "{YYYY}-{MM}". Only applies with Time.
	TimeAxisFmt string `json:"timeAxisFmt,omitempty"`

	Caption string `json:"caption,omitempty"`
	Height  int    `json:"height,omitempty"` // Pixels, default 300.

	// PieTotal overrides the computed total, for a Pie chart showing a partial breakdown.
	PieTotal float64 `json:"pieTotal,omitempty"`
}

// Render returns an HTML fragment: a container element plus the inline script that draws
// the chart into it once the page has loaded the assets from Handler/HeadTags. id must be
// unique within the page.
func Render(id string, data Data, opts Options) (template.HTML, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	optsJSON, err := json.Marshal(opts)
	if err != nil {
		return "", err
	}

	idJSON, err := json.Marshal(id)
	if err != nil {
		return "", err
	}

	html := `<div id="` + template.HTMLEscapeString(id) + `"></div>` +
		`<script>Plot.draw(` + string(idJSON) + `, ` + string(payload) + `, ` + string(optsJSON) + `)</script>`

	return template.HTML(html), nil //nolint:gosec
}
