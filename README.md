# plot-go

[![Build Status](https://github.com/vearutop/plot-go/workflows/test-unit/badge.svg)](https://github.com/vearutop/plot-go/actions?query=branch%3Amaster+workflow%3Atest-unit)
[![Coverage Status](https://codecov.io/gh/vearutop/plot-go/branch/master/graph/badge.svg)](https://codecov.io/gh/vearutop/plot-go)
[![GoDevDoc](https://img.shields.io/badge/dev-doc-00ADD8?logo=go)](https://pkg.go.dev/github.com/vearutop/plot-go)
[![Time Tracker](https://wakatime.com/badge/github/vearutop/plot-go.svg)](https://wakatime.com/badge/github/vearutop/plot-go)
![Code lines](https://sloc.xyz/github/vearutop/plot-go/?category=code)
![Comments](https://sloc.xyz/github/vearutop/plot-go/?category=comments)

Renders interactive line, stacked-bar and pie charts (backed by [uPlot](https://github.com/leeoniya/uPlot))
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
}, plot.Options{Caption: "Requests per minute", Time: true})

fmt.Fprintf(w, `<!DOCTYPE html><html><head>%s</head><body>%s</body></html>`,
	plot.HeadTags("/plot/"), chart)
```

See [example/main.go](example/main.go) for a complete, runnable app:

```
go run ./example
```

## Data

`Data.Columns`/`Data.Values` is a plain table, first column generally the X axis:

```go
plot.Data{
	Columns: []string{"x", "y1", "y2"},
	Values:  [][]any{{1, 10, 20}, {2, 15, 18}},
}
```

## Options

| Field         | Effect                                                                               |
|---------------|---------------------------------------------------------------------------------------|
| `Kind`        | `plot.Line` (default) or `plot.Pie`                                                   |
| `Rows`        | Treat Data as `(x, y, label)` rows instead of `(x, y1, y2, ...)` columns (Line only)   |
| `StackedBars` | Render stacked bars instead of lines (Line only)                                      |
| `Time`        | X axis is UTC Unix timestamps (Line only)                                             |
| `TimeAxisFmt` | Override the X axis label format, e.g. `"{YYYY}-{MM}"` (only with `Time`)             |
| `Caption`     | Chart title                                                                           |
| `Height`      | Pixels, default 300 (Line only)                                                       |
| `PieTotal`    | Override the computed total, for a Pie chart showing a partial breakdown              |

For `Kind: Pie`, each row of `Data` is `(count, label)` or `(label, count)` -- whichever column
parses as a number is used as the count.
