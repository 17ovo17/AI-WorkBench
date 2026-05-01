package timeseries

import (
	"math"
	"sort"
)

func Mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

func Std(data []float64) float64 {
	if len(data) < 2 {
		return 0
	}
	m := Mean(data)
	sum := 0.0
	for _, v := range data {
		d := v - m
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(data)))
}

func Percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	k := (p / 100.0) * float64(len(sorted)-1)
	f := math.Floor(k)
	c := math.Ceil(k)
	if int(f) == int(c) {
		return sorted[int(k)]
	}
	return sorted[int(f)]*(c-k) + sorted[int(c)]*(k-f)
}

func Median(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	s := make([]float64, len(data))
	copy(s, data)
	sort.Float64s(s)
	return Percentile(s, 50)
}

func Sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}
