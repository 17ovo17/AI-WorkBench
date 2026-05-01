package timeseries

func DetectSeasonality(data []float64) int {
	if len(data) < 48 {
		return 0
	}
	m := Mean(data)
	var variance float64
	for _, v := range data {
		d := v - m
		variance += d * d
	}
	if variance < 0.001 {
		return 0
	}
	maxCorr := 0.0
	bestLag := 0
	for _, lag := range []int{6, 12, 24, 168} {
		if lag >= len(data)/2 {
			continue
		}
		corr := autocorrelation(data, m, variance, lag)
		if corr > maxCorr && corr > 0.5 {
			maxCorr = corr
			bestLag = lag
		}
	}
	return bestLag
}

func autocorrelation(data []float64, mean, variance float64, lag int) float64 {
	n := len(data)
	var sum float64
	for i := 0; i < n-lag; i++ {
		sum += (data[i] - mean) * (data[i+lag] - mean)
	}
	return sum / variance
}
