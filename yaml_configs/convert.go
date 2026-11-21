package yaml_configs

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// String returns the value for key as a string. Scalars are formatted.
func (c *Config) String(key string) (string, error) { return GetAs[string](c, key) }

// Bool returns the value for key as a bool ("true", "1", "false", "0", ...).
func (c *Config) Bool(key string) (bool, error) { return GetAs[bool](c, key) }

// Int returns the value for key as an int.
func (c *Config) Int(key string) (int, error) { return GetAs[int](c, key) }

// Int64 returns the value for key as an int64.
func (c *Config) Int64(key string) (int64, error) { return GetAs[int64](c, key) }

// Float64 returns the value for key as a float64.
func (c *Config) Float64(key string) (float64, error) { return GetAs[float64](c, key) }

// Duration returns the value for key parsed with time.ParseDuration ("5s", "1m30s").
func (c *Config) Duration(key string) (time.Duration, error) { return GetAs[time.Duration](c, key) }

// StringSlice returns the value for key from a YAML list or a comma-separated
// string. Items are trimmed and empty items dropped.
func (c *Config) StringSlice(key string) ([]string, error) { return GetAs[[]string](c, key) }

func convert[T any](v any) (T, error) {
	var zero T
	var out any
	var err error
	switch any(zero).(type) {
	case string:
		out, err = toString(v)
	case bool:
		out, err = toBool(v)
	case int:
		var n int64
		if n, err = toInt64(v); err == nil {
			if n < math.MinInt || n > math.MaxInt {
				err = fmt.Errorf("%w: %d overflows int", ErrInvalidValue, n)
			}
			out = int(n)
		}
	case int64:
		out, err = toInt64(v)
	case float64:
		out, err = toFloat64(v)
	case time.Duration:
		out, err = toDuration(v)
	case []string:
		out, err = toStringSlice(v)
	default:
		if t, ok := v.(T); ok {
			return t, nil
		}
		return zero, fmt.Errorf("%w: cannot convert %T to %T", ErrInvalidValue, v, zero)
	}
	if err != nil {
		return zero, err
	}
	return out.(T), nil
}

func invalid(v any, target string) error {
	return fmt.Errorf("%w: cannot convert %T (%v) to %s", ErrInvalidValue, v, v, target)
}

func toString(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case bool:
		return strconv.FormatBool(x), nil
	case int:
		return strconv.Itoa(x), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case uint64:
		return strconv.FormatUint(x, 10), nil
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), nil
	case time.Time:
		return x.Format(time.RFC3339Nano), nil
	}
	return "", invalid(v, "string")
}

func toBool(v any) (bool, error) {
	switch x := v.(type) {
	case bool:
		return x, nil
	case string:
		b, err := strconv.ParseBool(strings.TrimSpace(x))
		if err != nil {
			return false, invalid(v, "bool")
		}
		return b, nil
	}
	return false, invalid(v, "bool")
}

func toInt64(v any) (int64, error) {
	switch x := v.(type) {
	case int:
		return int64(x), nil
	case int64:
		return x, nil
	case uint64:
		if x > math.MaxInt64 {
			return 0, invalid(v, "int64")
		}
		return int64(x), nil
	case float64:
		if x != math.Trunc(x) || x < math.MinInt64 || x >= math.MaxInt64 {
			return 0, invalid(v, "int64")
		}
		return int64(x), nil
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		if err != nil {
			return 0, invalid(v, "int64")
		}
		return n, nil
	}
	return 0, invalid(v, "int64")
}

func toFloat64(v any) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case int:
		return float64(x), nil
	case int64:
		return float64(x), nil
	case uint64:
		return float64(x), nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		if err != nil {
			return 0, invalid(v, "float64")
		}
		return f, nil
	}
	return 0, invalid(v, "float64")
}

func toDuration(v any) (time.Duration, error) {
	switch x := v.(type) {
	case time.Duration:
		return x, nil
	case string:
		d, err := time.ParseDuration(strings.TrimSpace(x))
		if err != nil {
			return 0, invalid(v, "time.Duration")
		}
		return d, nil
	}
	// Bare numbers are rejected: the unit (ns? s?) would be a guess.
	return 0, invalid(v, "time.Duration (use a string such as \"5s\")")
}

func toStringSlice(v any) ([]string, error) {
	switch x := v.(type) {
	case []string:
		return x, nil
	case string:
		var out []string
		for _, s := range strings.Split(x, ",") {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return out, nil
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			s, err := toString(item)
			if err != nil {
				return nil, invalid(v, "[]string")
			}
			out = append(out, s)
		}
		return out, nil
	}
	return nil, invalid(v, "[]string")
}
