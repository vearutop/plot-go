package plot_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vearutop/plot-go"
)

func TestRender(t *testing.T) {
	html, err := plot.Render("chart-1", plot.Data{
		Columns: []string{"ts", "value"},
		Values:  [][]any{{1, 2}, {3, 4}},
	}, plot.Options{Kind: plot.Pie, Caption: "Totals"})
	require.NoError(t, err)

	s := string(html)
	assert.Contains(t, s, `<div id="chart-1"></div>`)
	assert.Contains(t, s, `Plot.draw("chart-1"`)
	assert.Contains(t, s, `"kind":"pie"`)
	assert.Contains(t, s, `"caption":"Totals"`)
	assert.Contains(t, s, `"columns":["ts","value"]`)
}

func TestRender_heatmap(t *testing.T) {
	html, err := plot.Render("heat", plot.Data{
		Columns: []string{"x", "min", "max", "count", "percent"},
		Values:  [][]any{{1, 0, 10, 4, 100}},
	}, plot.Options{
		Kind:    plot.Heatmap,
		X:       plot.Axis{Type: plot.TimeAxis, Format: "{HH}:{mm}"},
		Value:   plot.Field{Unit: plot.Seconds},
		Heatmap: &plot.HeatmapOptions{Percent: true},
	})
	require.NoError(t, err)

	s := string(html)
	assert.Contains(t, s, `"kind":"heatmap"`)
	assert.Contains(t, s, `"x":{"type":"time","format":"{HH}:{mm}"}`)
	assert.Contains(t, s, `"value":{"unit":"s"}`)
	assert.Contains(t, s, `"heatmap":{"percent":true}`)
	assert.NotContains(t, s, `"y":`)
}

func TestRender_histogram(t *testing.T) {
	html, err := plot.Render("hist", plot.Data{
		Columns: []string{"series", "min", "max", "count", "percent"},
		Values:  [][]any{{"a", 0, 10, 4, 100}},
	}, plot.Options{
		Kind:      plot.Histogram,
		Value:     plot.Field{Unit: plot.Seconds, Label: "latency"},
		Histogram: &plot.HistogramOptions{Percent: true},
	})
	require.NoError(t, err)

	s := string(html)
	assert.Contains(t, s, `"kind":"histogram"`)
	assert.Contains(t, s, `"value":{"label":"latency","unit":"s"}`)
	assert.Contains(t, s, `"histogram":{"percent":true}`)
}

func TestRender_escapesID(t *testing.T) {
	html, err := plot.Render(`"><img>`, plot.Data{}, plot.Options{})
	require.NoError(t, err)

	assert.NotContains(t, string(html), "<img>")
}

func TestHandler(t *testing.T) {
	srv := httptest.NewServer(plot.Handler())
	t.Cleanup(srv.Close)

	for _, path := range []string{"/uPlot.iife.min.js", "/uPlot.min.css", "/plot.js"} {
		resp, err := srv.Client().Get(srv.URL + path)
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode, path)
		require.NoError(t, resp.Body.Close())
	}
}

func TestHeadTags(t *testing.T) {
	assert.True(t, strings.Contains(string(plot.HeadTags("/plot/")), "/plot/uPlot.iife.min.js"))
}
