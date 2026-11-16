# plot-go

[![Build Status](https://github.com/vearutop/plot-go/workflows/test-unit/badge.svg)](https://github.com/vearutop/plot-go/actions?query=branch%3Amaster+workflow%3Atest-unit)
[![Coverage Status](https://codecov.io/gh/vearutop/plot-go/branch/master/graph/badge.svg)](https://codecov.io/gh/vearutop/plot-go)
[![GoDevDoc](https://img.shields.io/badge/dev-doc-00ADD8?logo=go)](https://pkg.go.dev/github.com/vearutop/plot-go)
[![Time Tracker](https://wakatime.com/badge/github/vearutop/plot-go.svg)](https://wakatime.com/badge/github/vearutop/plot-go)
![Code lines](https://sloc.xyz/github/vearutop/plot-go/?category=code)
![Comments](https://sloc.xyz/github/vearutop/plot-go/?category=comments)

Renders interactive line, stacked-bar, pie, histogram and heatmap charts (backed by [uPlot](https://github.com/leeoniya/uPlot))
as an HTML fragment, so any Go web app can embed a chart by handing it tabular data - no
server-side pixel rendering, no JS to write by hand.

## Usage

```
go get github.com/vearutop/plot-go
```

Mount `Handler` under a path prefix, include `HeadTags` with the same prefix in the page's
`<head>`, then `Render` returns a container element plus the inline script that draws into it
once those assets have loaded:

```go
mux.Handle("/plot/", http.StripPrefix("/plot/", plot.Handler()))

chart, err := plot.Render("requests", plot.Data{
	Columns: []string{"time", "requests"},
	Values:  [][]any{{1700000000, 12}, {1700000060, 18}},
}, plot.Options{Caption: "Requests per minute", X: plot.Axis{Type: plot.TimeAxis}})

fmt.Fprintf(w, `<!DOCTYPE html><html><head>%s</head><body>%s</body></html>`,
	plot.HeadTags("/plot/"), chart)
```

See [example/main.go](example/main.go) for a complete, runnable app:

```
go run ./example
```

The example also has a live chart: the server produces a point every second, and the page polls
`/points?since=<time of the last point>` and calls `Plot.append` with the new rows
(see [Live updates](#live-updates)).

## Live updates

`Render` draws a chart once. Later (e.g. from a polling script) the chart can be changed in place with two JS calls,
so cursor, legend state and user zoom are kept, and the X axis follows new points unless the chart is zoomed in:

```js
// Replace the whole dataset with a new full one.
Plot.replace("requests", {columns: ["time", "requests"], values: [[1700000000, 12], [1700000060, 18]]}, opts)

// Add rows to the existing dataset, rows have the shape of `values`, X is expected to be greater than existing.
Plot.append("requests", [[1700000120, 21]])
```

- `Plot.replace(id, data, opts)` takes the same `data` as `Render`, `opts` is only used if the chart has not been
  drawn yet, so it can also be used for the first draw. A changed point (e.g. the current, still growing, minute)
  is simply a part of the new list.
- `Plot.append(id, rows)` needs a drawn chart and returns `false` if there is none. For line charts with ascending X
  the new values are pushed right to the chart (uPlot has no append call of its own, this is its `setData` over
  the same arrays), so the cost does not depend on the number of points already there. Otherwise (rows shape,
  stacked bars, X is not greater than the last one) the chart data is rebuilt from all the rows.

Line and stacked-bar charts are supported, if the set of series changes the chart is rebuilt.

## Data

`Data.Columns`/`Data.Values` is a plain table, first column generally the X axis:

```go
plot.Data{
	Columns: []string{"x", "y1", "y2"},
	Values:  [][]any{{1, 10, 20}, {2, 15, 18}},
}
```

## Options

| Field         | Effect                                                                                  |
|---------------|------------------------------------------------------------------------------------------|
| `Kind`        | `plot.Line` (default), `plot.Pie`, `plot.Histogram` or `plot.Heatmap`                    |
| `Rows`        | Treat Data as `(x, y, label)` rows instead of `(x, y1, y2, ...)` columns (Line only)      |
| `StackedBars` | Render stacked bars instead of lines (Line only)                                         |
| `Caption`     | Chart title                                                                              |
| `Height`      | Pixels, default 300 (Line and Heatmap)                                                   |
| `X`, `Y`      | `plot.Axis{Type, Unit, Format}`, see [Axes and units](#axes-and-units)                   |
| `Value`       | `plot.Field{Unit, Label}`, the measured value of a Histogram or Heatmap                  |
| `Histogram`   | `*plot.HistogramOptions{Percent}` for `Kind: plot.Histogram`                             |
| `Heatmap`     | `*plot.HeatmapOptions{Percent}` for `Kind: plot.Heatmap`                                 |
| `Pie`         | `*plot.PieOptions{Total}`, overrides the computed total of a partial breakdown           |

For `Kind: Pie`, each row of `Data` is `(count, label)` or `(label, count)` -- whichever column
parses as a number is used as the count.

The same options are JSON (the struct tags, `x`, `y`, `value`, `heatmap`, `pie`), `Plot.draw` takes them as is.

## Axes and units

`Axis.Type` is `plot.NumberAxis` (default), `plot.TimeAxis` (X values are UTC Unix timestamps, `Format` overrides
the label format, e.g. `"{YYYY}-{MM}"`) or `plot.CategoryAxis` (Heatmap only: equal-width columns in row order).

`Unit` shows values of an axis as Go-style durations with `plot.Seconds` (`1h34m2s`, `5d23h12m5s`, `1.5s`,
`300ms`) instead of large numbers. On X it also formats the legend value, and it is ignored for `TimeAxis`.

For histograms and heatmaps put the unit on the measured value, `Value.Unit`, the chart knows which axis it is on
(X for a histogram, where Y is a count, and Y for a heatmap, where X is the slice). For other charts use the axis
that holds the values: `Y.Unit` for a line chart of durations over X.

```go
plot.Render("hist", histogramData, plot.Options{Kind: plot.Histogram, Value: plot.Field{Unit: plot.Seconds}})
```

Both charts are in [example/main.go](example/main.go).

## Histogram

`Kind: plot.Histogram` draws a bar per value bucket. Bars are equally spaced and labelled by the observed range of a
bucket, so adaptive or skewed (e.g. exponential) buckets stay readable. Data rows are `(min, max, count, percent)`,
where `percent` is the share of all observations. To stack series add the series name as the first column,
`(series, min, max, count, percent)`: every series should use the same buckets, series are stacked in the order of
appearance, the legend toggles them, the tooltip lists them.

```go
plot.Render("hist", plot.Data{
	Columns: []string{"series", "min", "max", "count", "percent"},
	Values:  [][]any{{"organic", 0, 10, 4, 40}, {"paid", 0, 10, 2, 20}, {"organic", 10, 20, 3, 30}, {"paid", 10, 20, 1, 10}},
}, plot.Options{
	Kind:      plot.Histogram,
	Value:     plot.Field{Unit: plot.Seconds, Label: "install time"},
	Histogram: &plot.HistogramOptions{Percent: false},
})
```

`Histogram.Percent` shows percent of all observations on the Y axis instead of counts, `Value.Label` names the value
in the axis title and tooltips.

## Heatmap

`Kind: plot.Heatmap` draws a histogram per X slice, e.g. a latency distribution over time. Data rows are
`(x, min, max, count, percent)`, where `[min, max]` is a value bucket and `percent` is the share of the count in its
X slice:

```go
plot.Render("heat", plot.Data{
	Columns: []string{"x", "min", "max", "count", "percent"},
	Values:  [][]any{{1700000000, 0, 10, 4, 80}, {1700000000, 10, 100, 1, 20}, {1700003600, 0, 10, 3, 100}},
}, plot.Options{
	Kind:    plot.Heatmap,
	X:       plot.Axis{Type: plot.TimeAxis},
	Value:   plot.Field{Unit: plot.Seconds},
	Heatmap: &plot.HeatmapOptions{Percent: true},
})
```

- X is positioned by number (cells are as wide as the smallest gap between slices, `TimeAxis` formats it as UTC
  time). With `CategoryAxis` X is categorical: equal-width columns in row order, labelled by the value.
- `Percent`: color by percent of the slice (normalizes busy and quiet slices), otherwise by count.
- Buckets should be the same in all slices. They are drawn as equal-height rows ordered by `min`, labelled by their
  upper bound on the axis (the tooltip has the full range), so skewed (e.g. exponential) buckets are readable.
- Cells are colored with a light-to-dark blue ramp, the tooltip shows the count and the percent of the slice.
- `Value.Unit` formats the value axis compactly (two most significant units, e.g. `1d23h`), `X.Unit` the X axis.
