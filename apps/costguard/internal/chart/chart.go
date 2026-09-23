// Copyright 2026 Schwarz Digits Cloud GmbH & Co. KG
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package chart renders the 30-day cost series as a PNG and
// uploads it to STACKIT Object Storage with a 7-day presigned URL
//. The chart is included in every run — dry-run and
// live alike. S3 is optional: when unconfigured the report simply has no
// ChartURL.
package chart

import (
	"bytes"
	"fmt"
	"time"

	"github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"

	"github.com/stackitcloud/professional-service/apps/costguard/internal/report"
)

// dateLayout is the daily-cost date format produced by the billing scan.
const dateLayout = "2006-01-02"

// Render renders the daily cost series as a PNG: a single time series
// line (blue, 25% opacity area fill), euro y-axis, MM-DD x-axis.
func Render(daily []report.DailyCost) (*bytes.Buffer, error) {
	if len(daily) == 0 {
		return nil, fmt.Errorf("no cost data provided to chart")
	}
	var xValues []time.Time
	var yValues []float64
	for _, c := range daily {
		t, err := time.Parse(dateLayout, c.Date)
		if err != nil {
			continue
		}
		xValues = append(xValues, t)
		yValues = append(yValues, c.CostEUR)
	}
	if len(xValues) == 0 {
		return nil, fmt.Errorf("no parseable dates in cost data")
	}

	mainSeries := chart.TimeSeries{
		Name: "Daily Cost",
		Style: chart.Style{
			StrokeColor: drawing.ColorFromHex("3273DC"),
			StrokeWidth: 2,
			FillColor:   drawing.ColorFromHex("3273DC").WithAlpha(64),
		},
		XValues: xValues,
		YValues: yValues,
	}

	graph := chart.Chart{
		Title:      fmt.Sprintf("%d-Day Cost Overview", len(daily)),
		TitleStyle: chart.Style{FontSize: 12},
		Height:     400,
		Width:      1000,
		Background: chart.Style{
			Padding: chart.Box{Top: 40, Bottom: 20, Left: 20, Right: 20},
		},
		XAxis: chart.XAxis{
			ValueFormatter: chart.TimeValueFormatterWithFormat("01-02"),
		},
		YAxis: chart.YAxis{
			ValueFormatter: func(v interface{}) string {
				if f, ok := v.(float64); ok {
					return fmt.Sprintf("€%.2f", f)
				}
				return fmt.Sprintf("%v", v)
			},
		},
		Series: []chart.Series{mainSeries},
	}

	buffer := bytes.NewBuffer([]byte{})
	if err := graph.Render(chart.PNG, buffer); err != nil {
		return nil, fmt.Errorf("rendering chart: %w", err)
	}
	return buffer, nil
}
