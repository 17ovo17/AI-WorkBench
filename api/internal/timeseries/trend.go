package timeseries

import "math"

type DataPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

func DetectTrend(data []float64) (string, float64) {
	n := float64(len(data))
	if n < 3 {
		return "stable", 0
	}
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for i, v := range data {
		x := float64(i)
		sumX += x
		sumY += v
		sumXY += x * v
		sumX2 += x * x
	}
	denom := n*sumX2 - sumX*sumX
	if math.Abs(denom) < 1e-10 {
		return "stable", 0
	}
	slope := (n*sumXY - sumX*sumY) / denom
	m := sumY / n
	if math.Abs(m) > 0.001 {
		ns := slope / m
		if ns > 0.01 {
			return "increasing", slope
		}
		if ns < -0.01 {
			return "decreasing", slope
		}
	}
	return "stable", slope
}

func LinearForecast(data []DataPoint, steps int) []DataPoint {
	if len(data) < 3 || steps <= 0 {
		return nil
	}
	values := make([]float64, len(data))
	for i, d := range data {
		values[i] = d.Value
	}
	_, slope := DetectTrend(values)
	n := len(data)
	lastTS := data[n-1].Timestamp
	lastVal := data[n-1].Value
	interval := int64(3600)
	if n > 1 {
		interval = (data[n-1].Timestamp - data[0].Timestamp) / int64(n-1)
	}
	if interval <= 0 {
		interval = 3600
	}
	forecast := make([]DataPoint, steps)
	for i := 0; i < steps; i++ {
		forecast[i] = DataPoint{
			Timestamp: lastTS + int64(i+1)*interval,
			Value:     lastVal + slope*float64(i+1),
		}
	}
	return forecast
}
