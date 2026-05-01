package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronExpr 表示一个 5 字段 cron 表达式（分 时 日 月 周）。
type CronExpr struct {
	Minute  []int
	Hour    []int
	Day     []int
	Month   []int
	Weekday []int
}

// ParseCron 解析 cron 表达式，支持 *、具体数字、*/N 三种格式。
func ParseCron(expr string) (*CronExpr, error) {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return nil, fmt.Errorf("cron: need 5 fields, got %d", len(parts))
	}
	c := &CronExpr{}
	var err error
	if c.Minute, err = parseField(parts[0], 0, 59); err != nil {
		return nil, err
	}
	if c.Hour, err = parseField(parts[1], 0, 23); err != nil {
		return nil, err
	}
	if c.Day, err = parseField(parts[2], 1, 31); err != nil {
		return nil, err
	}
	if c.Month, err = parseField(parts[3], 1, 12); err != nil {
		return nil, err
	}
	if c.Weekday, err = parseField(parts[4], 0, 6); err != nil {
		return nil, err
	}
	return c, nil
}

// parseField 解析单个 cron 字段。nil 表示"任意值"。
func parseField(field string, min, max int) ([]int, error) {
	if field == "*" {
		return nil, nil
	}
	if strings.HasPrefix(field, "*/") {
		step, err := strconv.Atoi(field[2:])
		if err != nil || step <= 0 {
			return nil, fmt.Errorf("cron: invalid step %s", field)
		}
		var vals []int
		for i := min; i <= max; i += step {
			vals = append(vals, i)
		}
		return vals, nil
	}
	val, err := strconv.Atoi(field)
	if err != nil {
		return nil, fmt.Errorf("cron: invalid value %s", field)
	}
	if val < min || val > max {
		return nil, fmt.Errorf("cron: %d out of range [%d,%d]", val, min, max)
	}
	return []int{val}, nil
}

// Matches 判断给定时间是否匹配该 cron 表达式。
func (c *CronExpr) Matches(t time.Time) bool {
	return matchField(c.Minute, t.Minute()) &&
		matchField(c.Hour, t.Hour()) &&
		matchField(c.Day, t.Day()) &&
		matchField(c.Month, int(t.Month())) &&
		matchField(c.Weekday, int(t.Weekday()))
}

func matchField(allowed []int, val int) bool {
	if allowed == nil {
		return true
	}
	for _, a := range allowed {
		if a == val {
			return true
		}
	}
	return false
}
