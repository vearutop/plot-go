// Plot draws a line/stacked-bar/pie chart (backed by uPlot) into a container element.
//
// Entry point: Plot.draw(containerId, data, opts).
//   data: {columns: [...], values: [[...], ...]}, first column is the X axis (or, for
//         opts.rows, each row is [x, y, label]). For opts.kind === "pie", each row is
//         [count, label] (or [label, count]).
//   opts: {kind: "line"|"pie", rows, stackedBars, time, timeAxisFmt, caption, height, pieTotal}
var Plot = (function () {
    function splitmix32(a) {
        return function () {
            a |= 0;
            a = a + 0x9e3779b9 | 0;
            let t = a ^ a >>> 16;
            t = Math.imul(t, 0x21f0aaad);
            t = t ^ t >>> 15;
            t = Math.imul(t, 0x735a2d97);
            return ((t = t ^ t >>> 15) >>> 0) / 4294967296;
        }
    }

    // Deterministic random number generator, so series colors are stable across renders.
    const prng = splitmix32(12345)

    function getDarkColor() {
        var color = '#';
        for (var i = 0; i < 6; i++) {
            color += Math.floor(2 + prng() * 11).toString(16);
        }
        return color;
    }

    function toNumberOrNull(v) {
        if (v === null || typeof v === 'undefined') {
            return null;
        }
        if (typeof v === 'string' && v.trim() === '') {
            return null;
        }
        let n = parseFloat(v);
        return Number.isFinite(n) ? n : null;
    }

    function uplotOpts(container) {
        return {
            width: container.clientWidth,
            height: 300,
            tzDate: ts => uPlot.tzDate(new Date(ts * 1e3), 'Etc/UTC'),
            scales: {
                x: {
                    time: false,
                },
            },
            series: [],
            axes: [
                {},
                {
                    labelGap: 8,
                    labelSize: 8 + 12 + 8,
                    size(self, values, axisIdx, cycleNum) {
                        let axis = self.axes[axisIdx];

                        // bail out, force convergence
                        if (cycleNum > 1)
                            return axis._size;

                        let axisSize = axis.ticks.size + axis.gap;

                        // find longest value
                        let longestVal = (values ?? []).reduce((acc, val) => (
                            val.length > acc.length ? val : acc
                        ), "");

                        if (longestVal !== "") {
                            self.ctx.font = axis.font[0];
                            axisSize += self.ctx.measureText(longestVal).width / devicePixelRatio;
                        }

                        return Math.ceil(axisSize);
                    },
                }
            ],
        };
    }

    function applyLegendSortOnHover(uplot_opts) {
        uplot_opts.hooks = uplot_opts.hooks || {};
        uplot_opts.hooks.init = uplot_opts.hooks.init || [];
        uplot_opts.hooks.init.push((u) => {
            const legend = u.root.querySelector(".u-legend");
            if (!legend) {
                return;
            }
            const rows = Array.from(legend.querySelectorAll(".u-series"));
            let needsIdx = false;
            for (let i = 0; i < rows.length; i++) {
                if (!rows[i].hasAttribute("data-idx")) {
                    needsIdx = true;
                    break;
                }
            }
            if (needsIdx) {
                for (let i = 0; i < rows.length && i < u.series.length; i++) {
                    rows[i].setAttribute("data-idx", String(i));
                }
            }

            if (uplot_opts._legendSortData) {
                u._legendSortData = uplot_opts._legendSortData;
            }
        });

        uplot_opts.hooks.setLegend = uplot_opts.hooks.setLegend || [];
        uplot_opts.hooks.setLegend.push((u) => {
            if (!u.cursor || u.cursor.idx == null) {
                return;
            }

            const legend = u.root.querySelector(".u-legend");
            if (!legend) {
                return;
            }

            const rows = Array.from(legend.querySelectorAll(".u-series"));
            if (rows.length <= 2) {
                return;
            }

            const rowMap = new Map();
            rows.forEach((row) => {
                const idxAttr = row.getAttribute("data-idx");
                const idx = idxAttr != null ? parseInt(idxAttr, 10) : null;
                if (Number.isFinite(idx)) {
                    rowMap.set(idx, row);
                }
            });

            const idxs = u.cursor.idxs || [];
            const hasValue = (si) => idxs[si] != null;
            const dataSource = u._legendSortData || u.data;
            const valueFor = (si) => {
                const idx = idxs[si];
                if (idx == null) {
                    return null;
                }
                const v = dataSource[si] ? dataSource[si][idx] : null;
                if (v === null || typeof v === 'undefined') {
                    return null;
                }
                const n = +v;
                return Number.isFinite(n) ? n : 0;
            };

            const order = [0];
            const withVals = [];
            const withoutVals = [];

            for (let si = 1; si < u.series.length; si++) {
                if (!rowMap.has(si)) {
                    continue;
                }
                if (hasValue(si)) {
                    withVals.push(si);
                } else {
                    withoutVals.push(si);
                }
            }

            withVals.sort((a, b) => valueFor(b) - valueFor(a));
            order.push(...withVals, ...withoutVals);

            for (let i = 0; i < order.length; i++) {
                const row = rowMap.get(order[i]);
                if (row) {
                    legend.appendChild(row);
                }
            }
        });
    }

    function applyTimeAxis(uplot_opts, axisFmt) {
        function utcDate(ts) {
            if (!ts) {
                return null;
            }

            return uPlot.tzDate(new Date(ts * 1e3), 'Etc/UTC');
        }

        const axisDateFmt = axisFmt ? uPlot.fmtDate(axisFmt) : null;
        const fullDate = uPlot.fmtDate("{HH}:{mm}\n{YYYY}-{MM}-{DD}");
        const dayDate = uPlot.fmtDate("{HH}:{mm}\n{MM}-{DD}");
        const hourDate = uPlot.fmtDate("{HH}:{mm}");
        const legendDate = uPlot.fmtDate("{YYYY}-{MM}-{DD} {HH}:{mm}:{ss}");

        uplot_opts.axes[0].values = (u, vals, space) => {
            return vals.map((v, i) => {
                const d = utcDate(v);
                if (axisDateFmt) {
                    return axisDateFmt(d)
                }
                const prev = i > 0 ? vals[i - 1] : null;
                if (!prev || !v) {
                    return fullDate(d)
                }

                const prevUTCDate = utcDate(prev);

                if (d.getFullYear() !== prevUTCDate.getFullYear()) { // year changed
                    return fullDate(d)
                } else if (d.getDate() !== prevUTCDate.getDate()) { // date changed
                    return dayDate(d)
                } else { // same day
                    return hourDate(d)
                }
            });
        };

        if (uplot_opts.series[0]) {
            uplot_opts.series[0].value = function (u, ts) {
                if (!ts) {
                    return null;
                }

                return legendDate(utcDate(ts));
            };
        }
    }

    function stack(data, omit, fillNulls) {
        let data2 = [];
        let bands = [];
        let d0Len = data[0].length;
        let accum = Array(d0Len);

        for (let i = 0; i < d0Len; i++) {
            accum[i] = 0;
        }

        for (let i = 1; i < data.length; i++) {
            data2.push(omit(i) ? data[i] : data[i].map((v, i) => {
                if (v === null || typeof v === 'undefined') {
                    return fillNulls ? accum[i] : null;
                }
                let n = +v;
                if (!Number.isFinite(n)) {
                    return fillNulls ? accum[i] : null;
                }
                return (accum[i] += n);
            }));
        }

        for (let i = 1; i < data.length; i++) {
            !omit(i) && bands.push({
                series: [
                    data.findIndex((s, j) => j > i && !omit(j)),
                    i,
                ],
            });
        }

        bands = bands.filter(b => b.series[0] > -1 && b.series[1] > -1);

        return {
            data: [data[0]].concat(data2),
            bands,
        };
    }

    function getStackedOpts(uplot_opts, series, data) {
        uplot_opts.series = series;
        uplot_opts._legendSortData = data;

        let stacked = stack(data, i => false, true);
        uplot_opts.bands = stacked.bands;

        uplot_opts.cursor = uplot_opts.cursor || {};
        uplot_opts.cursor.dataIdx = (u, seriesIdx, closestIdx, xValue) => {
            return data[seriesIdx][closestIdx] == null ? null : closestIdx;
        };

        uplot_opts.series.forEach((s, si) => {
            s.value = (u, v, si, i) => data[si][i];

            s.points = s.points || {};

            // scan raw unstacked data to return only real points
            s.points.filter = (u, seriesIdx, show, gaps) => {
                if (show) {
                    let pts = [];
                    data[seriesIdx].forEach((v, i) => {
                        v != null && pts.push(i);
                    });
                    return pts;
                }
            }
        });

        // force 0 to be the sum minimum instead of the bottom series
        uplot_opts.scales.y = {
            range: (u, min, max) => {
                let minMax = uPlot.rangeNum(0, max, 0.1, true);
                return [0, minMax[1]];
            }
        };

        // restack on toggle
        uplot_opts.hooks = uplot_opts.hooks || {};
        uplot_opts.hooks.setSeries = uplot_opts.hooks.setSeries || [];
        uplot_opts.hooks.setSeries.push((u, i) => {
            let stacked = stack(data, i => !u.series[i].show, true);
            u.delBand(null);
            stacked.bands.forEach(b => u.addBand(b));
            u.setData(stacked.data);
        });

        return {opts: uplot_opts, data: stacked.data};
    }

    function buildStackedBarsSeries(columns) {
        const {bars} = uPlot.paths;
        const barsPath = bars({size: [0.95, 100]});

        let series = [{
            label: columns[0],
        }];

        for (let i = 1; i < columns.length; i++) {
            series.push({
                label: columns[i],
                width: 1,
                fill: getDarkColor(),
                paths: barsPath,
                points: {show: false},
            });
        }

        return series;
    }

    // Build rows-based data (x, y, label) into uPlot columnar data.
    function buildRowsData(data) {
        let timedData = {}
        let labels = {}

        for (let i in data.values) {
            let row = data.values[i]
            let t = toNumberOrNull(row[0])
            let val = toNumberOrNull(row[1])
            let label = row[2]

            if (t === null || typeof label === 'undefined') {
                continue;
            }

            labels[label] = 1
            let key = String(t)
            if (!timedData[key]) {
                timedData[key] = {};
            }
            timedData[key][label] = val
        }

        let labelsArr = Object.keys(labels).sort()

        let uplot_data = []
        for (let i = 0; i < 1 + labelsArr.length; i++) {
            uplot_data.push([])
        }

        let xVals = Object.keys(timedData)
            .map(k => parseFloat(k))
            .filter(Number.isFinite)
        xVals.sort((a, b) => a - b)

        for (let xi = 0; xi < xVals.length; xi++) {
            let t = xVals[xi]
            uplot_data[0].push(t)

            let values = timedData[String(t)] || {}
            for (let li = 0; li < labelsArr.length; li++) {
                let label = labelsArr[li]
                let val = values[label]
                uplot_data[1 + li].push(typeof val === 'undefined' ? null : val)
            }
        }

        return {uplot_data, labelsArr}
    }

    function uplotRowsData(data, uplot_opts) {
        let built = buildRowsData(data)
        let uplot_data = built.uplot_data
        let labelsArr = built.labelsArr

        uplot_opts.series.push({
            label: data.columns[0],
        })

        for (let i = 0; i < labelsArr.length; i++) {
            uplot_opts.series.push({
                stroke: getDarkColor(),
                label: labelsArr[i],
            })
        }

        return uplot_data
    }

    function uplotBarsRowsData(data, uplot_opts) {
        let built = buildRowsData(data)
        let uplot_data = built.uplot_data
        let labelsArr = built.labelsArr
        let series = buildStackedBarsSeries([data.columns[0]].concat(labelsArr))

        let stacked = getStackedOpts(uplot_opts, series, uplot_data);

        return stacked.data;
    }

    function uplotBarsColumnsData(data, uplot_opts) {
        let series = buildStackedBarsSeries(data.columns);
        let uplot_data = [];
        for (let i = 0; i < data.columns.length; i++) {
            uplot_data.push([]) // Separate vector for each column (X + multiple Y).
        }

        let sortable = [];
        for (let i in data.values) {
            let item = data.values[i];
            let x = toNumberOrNull(item[0]);
            if (x === null) {
                continue;
            }
            sortable.push({x, item});
        }

        // Sorting data by first column (X axis) ascending.
        sortable.sort(function (a, b) {
            return a.x - b.x
        });

        for (let i = 0; i < sortable.length; i++) {
            let item = sortable[i].item;
            uplot_data[0].push(sortable[i].x);

            for (let j = 1; j < data.columns.length; j++) {
                let val = (j < item.length) ? toNumberOrNull(item[j]) : null;
                uplot_data[j].push(val);
            }
        }

        let stacked = getStackedOpts(uplot_opts, series, uplot_data);

        return stacked.data;
    }

    function uplotColumnsData(data, uplot_opts) {
        let uplot_data = [];
        for (let i = 0; i < data.columns.length; i++) {
            uplot_data.push([]) // Separate vector for each column (X + multiple Y).

            if (i === 0) {
                uplot_opts.series.push({
                    label: data.columns[i],
                })
            } else {
                uplot_opts.series.push({
                    stroke: getDarkColor(),
                    label: data.columns[i],
                })
            }
        }

        // Sorting data by first column (X axis) ascending.
        let sortedValues = data.values.sort(function (a, b) {
            return a[0] - b[0]
        });

        for (let i in sortedValues) {
            let item = sortedValues[i]

            for (let j = 0; j < item.length; j++) {
                uplot_data[j].push(parseFloat(item[j]))
            }
        }

        return uplot_data
    }

    function drawLine(container, data, opts) {
        let uplot_opts = uplotOpts(container)
        applyLegendSortOnHover(uplot_opts)

        if (opts.caption) {
            uplot_opts.title = opts.caption
        }

        if (opts.height) {
            uplot_opts.height = opts.height
        }

        if (opts.time) {
            uplot_opts.scales.x.time = true
        }

        let uplot_data
        if (opts.stackedBars) {
            uplot_data = opts.rows ? uplotBarsRowsData(data, uplot_opts) : uplotBarsColumnsData(data, uplot_opts)
        } else if (opts.rows) {
            uplot_data = uplotRowsData(data, uplot_opts)
        } else {
            uplot_data = uplotColumnsData(data, uplot_opts)
        }

        if (opts.time) {
            applyTimeAxis(uplot_opts, opts.timeAxisFmt || null)
        }

        new uPlot(uplot_opts, uplot_data, container);
    }

    // drawPieChart renders a pie chart, courtesy of deepseek-r1:32b with minor changes.
    function drawPieChart(data, total, container) {
        // Set chart dimensions
        const width = 500;
        const height = 500;
        const margin = 0;
        const chartWidth = width - 2 * margin;
        const chartHeight = height - 2 * margin;

        // Create SVG element
        const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
        svg.setAttribute('width', container.clientWidth);
        svg.setAttribute('height', height);
        container.appendChild(svg);

        // Calculate totals
        if (!total) {
            total = data.reduce((sum, item) => sum + item.value, 0);
        }

        // Create pie chart slices
        let currentAngle = 0;
        const centerX = margin + chartWidth / 2;
        const centerY = margin + chartHeight / 2;

        const radius = Math.min(chartWidth - margin * 2, chartHeight - margin * 2) / 2;

        data.forEach(item => {
            const percent = (item.value / total) * 100;
            const angle = (percent / 100) * 2 * Math.PI;

            // Create slice path
            const slice = document.createElementNS("http://www.w3.org/2000/svg", "path");
            slice.classList.add("slice");

            const d = [
                `M ${centerX} ${centerY}`,
                `L ${centerX + radius * Math.cos(currentAngle)} ${centerY + radius * Math.sin(currentAngle)}`,
                `A ${radius} ${radius} 0 ${(angle > Math.PI ? 1 : 0)} 1 ${centerX + radius * Math.cos(currentAngle + angle)} ${centerY + radius *
                Math.sin(currentAngle + angle)}`,
                `L ${centerX} ${centerY}`
            ].join(' ');

            slice.setAttributeNS(null, 'd', d);
            slice.style.fill = item.color;

            // Add title (tooltip)
            const tooltip = document.createElementNS("http://www.w3.org/2000/svg", "title");
            tooltip.textContent = `${item.label}: ${item.value} (${percent.toFixed(1)}%)`;
            slice.appendChild(tooltip);

            svg.appendChild(slice);

            currentAngle += angle;
        });

        // Add legend
        const legendG = document.createElementNS("http://www.w3.org/2000/svg", "g");
        legendG.setAttribute('transform', `translate(520,${margin})`);

        data.forEach((item, index) => {
            const legendItem = document.createElementNS("http://www.w3.org/2000/svg", 'g');
            legendItem.classList.add('legend-item');

            // Legend color swatch
            const rect = document.createElementNS("http://www.w3.org/2000/svg", "rect");
            rect.setAttributeNS(null, 'x', 0);
            rect.setAttributeNS(null, 'y', index * 20);
            rect.style.fill = item.color;

            // Legend text
            const text = document.createElementNS("http://www.w3.org/2000/svg", "text");
            text.setAttributeNS(null, 'x', 24);
            text.setAttributeNS(null, 'y', index * 20 + 12);
            const percent = (item.value / total) * 100;
            text.textContent = `${item.label}: ${item.value} (${percent.toFixed(1)}%)`;

            legendItem.appendChild(rect);
            legendItem.appendChild(text);
            legendG.appendChild(legendItem);
        });

        svg.appendChild(legendG);
    }

    function drawPie(container, data, opts) {
        let values = (data.values || []).slice()
        if (values.length <= 1) {
            return
        }

        // Sorting data by first column (count) descending.
        let sortedValues = values.sort(function (a, b) {
            return b[0] - a[0]
        });

        let v = 0 // 0 col for value
        let l = 1 // 1 col for label

        if (isNaN(parseFloat(sortedValues[0][v]))) {
            v = 1
            l = 0
        }

        let pieData = [];

        // Transposing results from being an array of rows to array of columns.
        for (let i in sortedValues) {
            let item = sortedValues[i]

            pieData.push(
                {label: item[l], value: parseFloat(item[v]), color: getDarkColor()}
            )
        }

        drawPieChart(pieData, opts.pieTotal || 0, container)
    }

    // draw renders data into the element identified by containerId, per opts.kind.
    function draw(containerId, data, opts) {
        opts = opts || {}

        let container = document.getElementById(containerId)
        if (!container) {
            return
        }

        if (opts.kind === 'pie') {
            drawPie(container, data, opts)
        } else {
            drawLine(container, data, opts)
        }
    }

    return {draw: draw};
})();
