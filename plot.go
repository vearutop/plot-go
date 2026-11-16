// Package plot renders interactive line, stacked-bar, pie and heatmap charts (backed by uPlot) as an
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

	// Heatmap draws a histogram per X slice, e.g. a latency distribution over time. Each row of Data is
	// (x, min, max, count, percent): [min, max] is a value bucket, percent is the share of count in its X slice.
	// Buckets should be the same in all slices, see HeatmapOptions.
	Heatmap Kind = "heatmap"

	// Histogram draws bars of value buckets: equally spaced, labelled by the observed range of a bucket. Each row of
	// Data is (min, max, count, percent), or (series, min, max, count, percent) to stack series (all series should
	// have the same buckets). Percent is the share of all observations.
	Histogram Kind = "histogram"
)

// AxisType selects how values of an axis are interpreted.
type AxisType string

// Supported axis types.
const (
	NumberAxis   AxisType = "number" // Default.
	TimeAxis     AxisType = "time"   // Values are UTC Unix timestamps.
	CategoryAxis AxisType = "category"
)

// Unit is a unit of values, it defines how they are shown.
type Unit string

// Supported units, other values are shown as is.
const (
	Seconds Unit = "s" // Go-style durations: 1h34m2s, 5d23h12m5s, 1.5s, 300ms.
)

// Axis configures an axis.
type Axis struct {
	// Type defaults to NumberAxis. TimeAxis and CategoryAxis are for X only.
	Type AxisType `json:"type,omitempty"`

	// Unit shows values of the axis in the unit, e.g. Seconds. Ignored for TimeAxis.
	Unit Unit `json:"unit,omitempty"`

	// Format overrides the label format of a TimeAxis, e.g. "{YYYY}-{MM}".
	Format string `json:"format,omitempty"`
}

// Field describes a measured value that is not tied to one axis of every chart.
type Field struct {
	// Label names the value, e.g. in an axis title and tooltips of a Histogram.
	Label string `json:"label,omitempty"`

	// Unit of the value. For Heatmap the value is on the Y axis (and X is the slice), it is shown compactly,
	// with two most significant units (1d23h46m as 1d23h).
	Unit Unit `json:"unit,omitempty"`
}

// HeatmapOptions configures Heatmap charts.
type HeatmapOptions struct {
	// Percent colors cells by percent of their slice (normalizes busy and quiet slices), otherwise by count.
	Percent bool `json:"percent,omitempty"`
}

// HistogramOptions configures Histogram charts.
type HistogramOptions struct {
	// Percent shows percent of all observations on the Y axis instead of counts.
	Percent bool `json:"percent,omitempty"`
}

// PieOptions configures Pie charts.
type PieOptions struct {
	// Total overrides the computed total, for a Pie chart showing a partial breakdown.
	Total float64 `json:"total,omitempty"`
}

// Options configures how Data is rendered.
//
// Which axis holds the measured value depends on the chart: for a line chart of durations it is Y (use Y.Unit),
// for a Histogram it is X (Y is a count) and for a Heatmap it is Y (X is the slice), use Value.Unit for both.
type Options struct {
	Kind Kind `json:"kind,omitempty"` // Default Line.

	// Rows treats Data as (x, y, label) rows instead of (x, y1, y2, ...) columns. Ignored for Pie.
	Rows bool `json:"rows,omitempty"`

	// StackedBars renders a stacked bar chart instead of lines. Ignored for Pie.
	StackedBars bool `json:"stackedBars,omitempty"`

	Caption string `json:"caption,omitempty"`
	Height  int    `json:"height,omitempty"` // Pixels, default 300.

	X Axis `json:"x,omitempty,omitzero"`
	Y Axis `json:"y,omitempty,omitzero"`

	// Value is the measured value of a Heatmap or Histogram.
	Value Field `json:"value,omitempty,omitzero"`

	Heatmap   *HeatmapOptions   `json:"heatmap,omitempty"`   // For Heatmap.
	Histogram *HistogramOptions `json:"histogram,omitempty"` // For Histogram.
	Pie       *PieOptions       `json:"pie,omitempty"`       // For Pie.
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
