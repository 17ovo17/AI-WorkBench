package timeseries

import (
	"fmt"
	"math"
)

type Baseline struct {
	Mean float64 `json:"mean"`
	Std  float64 `json:"std"`
	P50  float64 `json:"p50"`
	P95  float64 `json:"p95"`
	P99  float64 `json:"p99"`
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
}

type AnalysisResult struct {
	AnomalyScore      float64       `json:"anomaly_score"`
	ChangePoints      []ChangePoint `json:"change_points"`
	TrendDirection    string        `json:"trend_direction"`
	TrendSlope        float64       `json:"trend_slope"`
	BaselineDeviation float64       `json:"baseline_deviation"`
	SeasonalPeriod    int           `json:"seasonal_period_hours"`
	Forecast          []DataPoint   `json:"forecast,omitempty"`
	Summary           string        `json:"summary"`
}

func Analyze(data []DataPoint, baseline *Baseline) *AnalysisResult {
	if len(data) < 3 {
		return &AnalysisResult{Summary: "数据点不足，无法分析"}
	}
	values := make([]float64, len(data))
	for i, d := range data {
		values[i] = d.Value
	}
	result := &AnalysisResult{}
	result.ChangePoints = DetectChangePoints(values, 3.0)
	result.TrendDirection, result.TrendSlope = DetectTrend(values)
	if baseline != nil {
		latest := values[len(values)-1]
		result.BaselineDeviation = (latest - baseline.Mean) / math.Max(baseline.Std, 0.001)
		result.AnomalyScore = Sigmoid(math.Abs(result.BaselineDeviation))
	} else {
		result.AnomalyScore = ZScoreAnomaly(values)
	}
	result.SeasonalPeriod = DetectSeasonality(values)
	result.Forecast = LinearForecast(data, 24)
	result.Summary = generateSummary(result)
	return result
}

func generateSummary(r *AnalysisResult) string {
	s := ""
	if r.AnomalyScore > 0.8 {
		s += "当前值严重偏离正常范围。"
	} else if r.AnomalyScore > 0.6 {
		s += "当前值存在一定偏离。"
	} else {
		s += "当前值在正常范围内。"
	}
	switch r.TrendDirection {
	case "increasing":
		s += fmt.Sprintf("呈上升趋势（斜率 %.4f）。", r.TrendSlope)
	case "decreasing":
		s += fmt.Sprintf("呈下降趋势（斜率 %.4f）。", r.TrendSlope)
	default:
		s += "趋势平稳。"
	}
	if len(r.ChangePoints) > 0 {
		s += fmt.Sprintf("检测到 %d 个突变点。", len(r.ChangePoints))
	}
	if r.SeasonalPeriod > 0 {
		s += fmt.Sprintf("存在 %d 小时周期性。", r.SeasonalPeriod)
	}
	return s
}
