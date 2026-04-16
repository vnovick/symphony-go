package schedule

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type fieldSpec struct {
	any    bool
	values map[int]struct{}
}

type Expression struct {
	minute fieldSpec
	hour   fieldSpec
	day    fieldSpec
	month  fieldSpec
	week   fieldSpec
}

func Parse(expr string) (Expression, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return Expression{}, fmt.Errorf("expected 5 cron fields")
	}
	minute, err := parseField(fields[0], 0, 59)
	if err != nil {
		return Expression{}, fmt.Errorf("minute: %w", err)
	}
	hour, err := parseField(fields[1], 0, 23)
	if err != nil {
		return Expression{}, fmt.Errorf("hour: %w", err)
	}
	day, err := parseField(fields[2], 1, 31)
	if err != nil {
		return Expression{}, fmt.Errorf("day-of-month: %w", err)
	}
	month, err := parseField(fields[3], 1, 12)
	if err != nil {
		return Expression{}, fmt.Errorf("month: %w", err)
	}
	week, err := parseField(fields[4], 0, 6)
	if err != nil {
		return Expression{}, fmt.Errorf("day-of-week: %w", err)
	}
	return Expression{
		minute: minute,
		hour:   hour,
		day:    day,
		month:  month,
		week:   week,
	}, nil
}

func (e Expression) Matches(t time.Time) bool {
	if !matchesField(e.minute, t.Minute()) || !matchesField(e.hour, t.Hour()) || !matchesField(e.month, int(t.Month())) {
		return false
	}
	dayMatch := matchesField(e.day, t.Day())
	weekMatch := matchesField(e.week, int(t.Weekday()))
	switch {
	case e.day.any && e.week.any:
		return true
	case e.day.any:
		return weekMatch
	case e.week.any:
		return dayMatch
	default:
		return dayMatch || weekMatch
	}
}

func matchesField(spec fieldSpec, value int) bool {
	if spec.any {
		return true
	}
	_, ok := spec.values[value]
	return ok
}

func parseField(expr string, min, max int) (fieldSpec, error) {
	if expr == "*" {
		return fieldSpec{any: true}, nil
	}
	values := make(map[int]struct{})
	parts := strings.Split(expr, ",")
	for _, part := range parts {
		if err := addFieldPart(values, strings.TrimSpace(part), min, max); err != nil {
			return fieldSpec{}, err
		}
	}
	if len(values) == 0 {
		return fieldSpec{}, fmt.Errorf("no values")
	}
	return fieldSpec{values: values}, nil
}

func addFieldPart(values map[int]struct{}, expr string, min, max int) error {
	if expr == "" {
		return fmt.Errorf("empty field part")
	}
	step := 1
	base := expr
	if strings.Contains(expr, "/") {
		pieces := strings.Split(expr, "/")
		if len(pieces) != 2 {
			return fmt.Errorf("invalid step expression %q", expr)
		}
		base = pieces[0]
		n, err := strconv.Atoi(pieces[1])
		if err != nil || n <= 0 {
			return fmt.Errorf("invalid step value %q", pieces[1])
		}
		step = n
	}

	start, end, err := parseRange(base, min, max)
	if err != nil {
		return err
	}
	for value := start; value <= end; value += step {
		values[value] = struct{}{}
	}
	return nil
}

func parseRange(expr string, min, max int) (int, int, error) {
	switch {
	case expr == "" || expr == "*":
		return min, max, nil
	case strings.Contains(expr, "-"):
		pieces := strings.Split(expr, "-")
		if len(pieces) != 2 {
			return 0, 0, fmt.Errorf("invalid range %q", expr)
		}
		start, err := parseBound(pieces[0], min, max)
		if err != nil {
			return 0, 0, err
		}
		end, err := parseBound(pieces[1], min, max)
		if err != nil {
			return 0, 0, err
		}
		if end < start {
			return 0, 0, fmt.Errorf("descending range %q", expr)
		}
		return start, end, nil
	default:
		value, err := parseBound(expr, min, max)
		if err != nil {
			return 0, 0, err
		}
		return value, value, nil
	}
}

func parseBound(expr string, min, max int) (int, error) {
	value, err := strconv.Atoi(expr)
	if err != nil {
		return 0, fmt.Errorf("invalid integer %q", expr)
	}
	if value < min || value > max {
		return 0, fmt.Errorf("value %d out of range [%d,%d]", value, min, max)
	}
	return value, nil
}
