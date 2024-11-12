package examples

import (
	"strconv"
	"testing"

	streams "github.com/mahadev-k/go-utils/stream_utils"
	"github.com/stretchr/testify/assert"
)

func TestMapRunnerLib(t *testing.T) {
	// Create a map with some values
	floatingStrings := []string{"0.1", "0.2", "22", "22.1"}

	res, err := streams.NewTransformer[string, int64](floatingStrings).
		Map(streams.MapIt[string, float64](func(item string) (float64, error) { return strconv.ParseFloat(item, 64) })).
		Map(streams.MapIt[float64, float64](func(item float64) (float64, error) { return item * 10, nil })).
		Map(streams.MapIt[float64, int64](func(item float64) (int64, error) { return int64(item), nil })).
		Map(streams.FilterIt[int64](func(item int64) (bool, error) { return item%2 == 0, nil })).
		Result()
	if err != nil {
		t.Errorf("Testcase failed with error : %v", err)
		return
	}
	// Output: [2 220]
	t.Logf("Result: %v", res)
	assert.ElementsMatch(t, []any{int64(2), int64(220)}, res)
}
