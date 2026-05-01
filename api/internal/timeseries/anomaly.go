package timeseries

import (
	"math"
	"sort"
)

func ZScoreAnomaly(data []float64) float64 {
	if len(data) < 3 {
		return 0
	}
	m := Mean(data)
	s := Std(data)
	if s < 0.001 {
		return 0
	}
	latest := data[len(data)-1]
	z := math.Abs((latest - m) / s)
	return Sigmoid(z)
}

func IQRAnomaly(data []float64) float64 {
	if len(data) < 4 {
		return 0
	}
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)
	q1 := Percentile(sorted, 25)
	q3 := Percentile(sorted, 75)
	iqr := q3 - q1
	if iqr < 0.001 {
		return 0
	}
	latest := data[len(data)-1]
	lower := q1 - 1.5*iqr
	upper := q3 + 1.5*iqr
	if latest < lower || latest > upper {
		deviation := math.Max(lower-latest, latest-upper) / iqr
		return Sigmoid(deviation)
	}
	return 0
}
