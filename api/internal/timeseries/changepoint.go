package timeseries

import "math"

type ChangePoint struct {
	Index     int     `json:"index"`
	Timestamp int64   `json:"timestamp,omitempty"`
	Direction string  `json:"direction"`
	Magnitude float64 `json:"magnitude"`
}

func DetectChangePoints(data []float64, threshold float64) []ChangePoint {
	if len(data) < 5 {
		return nil
	}
	m := Mean(data)
	s := Std(data)
	if s < 0.001 {
		return nil
	}
	cusumPos, cusumNeg := 0.0, 0.0
	var points []ChangePoint
	for i, v := range data {
		z := (v - m) / s
		cusumPos = math.Max(0, cusumPos+z-threshold*0.5)
		cusumNeg = math.Min(0, cusumNeg+z+threshold*0.5)
		if cusumPos > threshold*3 {
			points = append(points, ChangePoint{Index: i, Direction: "increase", Magnitude: z})
			cusumPos = 0
		}
		if cusumNeg < -threshold*3 {
			points = append(points, ChangePoint{Index: i, Direction: "decrease", Magnitude: math.Abs(z)})
			cusumNeg = 0
		}
	}
	return points
}
