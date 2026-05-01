package timeseries

import "math"

func PearsonCorrelation(x, y []float64) float64 {
	n := len(x)
	if n != len(y) || n < 3 {
		return 0
	}
	mx, my := Mean(x), Mean(y)
	var sumXY, sumX2, sumY2 float64
	for i := 0; i < n; i++ {
		dx := x[i] - mx
		dy := y[i] - my
		sumXY += dx * dy
		sumX2 += dx * dx
		sumY2 += dy * dy
	}
	denom := math.Sqrt(sumX2 * sumY2)
	if denom < 1e-10 {
		return 0
	}
	return sumXY / denom
}
