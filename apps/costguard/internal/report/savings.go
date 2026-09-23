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

package report

import (
	"math"
)

// daysPerMonth normalizes the monthly figure to a daily one. A flat 30
// days keeps the savings line comparable across reports; the underlying
// prices are monthly rates.
const daysPerMonth = 30.0

// round2 rounds to cents — money figures must not show float noise.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// EstimateMonthlySavings computes the estimated savings if all deletion
// candidates in the report were removed: detached volume
// sizes at the configured per-GB rate, plus the idle public IP
// reservation cost per IP. The daily figure is the monthly figure over 30
// days. It returns (monthly, daily) in EUR.
func EstimateMonthlySavings(volumes []DetachedVolume, idleIPs []IdlePublicIP, publicIPCostEURPerMonth float64) (monthly, daily float64) {
	for _, v := range volumes {
		monthly += v.EstimatedMonthlyCostEUR
	}
	monthly += float64(len(idleIPs)) * publicIPCostEURPerMonth
	monthly = round2(monthly)
	daily = round2(monthly / daysPerMonth)
	return monthly, daily
}

// FillSavings computes and stores the estimated savings on the report.
func (r *Report) FillSavings(publicIPCostEURPerMonth float64) {
	r.EstimatedMonthlySavingsEUR, r.EstimatedDailySavingsEUR =
		EstimateMonthlySavings(r.DetachedVolumes, r.IdlePublicIPs, publicIPCostEURPerMonth)
}
