package stream_utils

import (
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	ErrTest = errors.New("a test error")
)

func TestMapRunner(t *testing.T) {
	// Create a map with some values
	floatingStrings := []string{"0.1", "0.2", "22", "22.1"}

	res, err := NewTransformer[string, float64](floatingStrings).
		Map(MapIt[string, float64](func(item string) (float64, error) { return strconv.ParseFloat(item, 64) })).
		Map(MapIt[float64, float64](func(item float64) (float64, error) { return item * 10, nil })).
		Result()
	if err != nil {
		t.Errorf("Testcase failed with error : %v", err)
		return
	}
	// Output: [0.1 0.2 22 22.1]
	t.Logf("Result: %v", res)
	assert.ElementsMatch(t, []any{float64(1), float64(2), float64(220), float64(221)}, res)

}

func TestFilterIt(t *testing.T) {
	// Create a map with some values
	floatingStrings := []string{"0.1", "0.2", "22", "22.1"}

	res, err := NewTransformer[string, int64](floatingStrings).
		Map(MapIt[string, float64](func(item string) (float64, error) {return strconv.ParseFloat(item, 64)})).
		Map(MapIt[float64, float64](func(item float64) (float64, error) { return item * 10, nil })).
		Map(MapIt[float64, int64](func(item float64) (int64, error) { return int64(item), nil })).
		Map(FilterIt[int64](func(item int64) (bool, error) { return item%2 == 0, nil })).
		Result()
	if err != nil {
		t.Errorf("Testcase failed with error : %v", err)
		return
	}
	// Output: [2 220]
	t.Logf("Result: %v", res)
	assert.ElementsMatch(t, []any{int64(2), int64(220)}, res)	
}

func TestMapRunnerForError(t *testing.T) {
	// Create a map with some values
	floatingStrings := []string{"0.1", "1", "22", "22.1"}

	_, err := NewTransformer[string, float64](floatingStrings).
		Map(MapIt[string, float64](func(item string) (float64, error) { return strconv.ParseFloat(item, 64) })).
		Map(MapIt[float64, float64](func(item float64) (float64, error) { return item, ErrTest })).
		Result()
	assert.Equal(t, err, ErrTest)

}

func TestFilterItSimple(t *testing.T) {
	// Create a map with some values
	floatingStrings := []string{"0.1", "0.2", "22", "22.1"}

	res, err := NewTransformer[string, string](floatingStrings).
		Map(MapIt[string, float64](func(item string) (float64, error) {return strconv.ParseFloat(item, 64)})).
		Map(MapIt[float64, float64](func(item float64) (float64, error) { return item * 10, nil })).
		Map(MapItSimple[float64, int64](func(item float64) int64 { return int64(item) })).
		Map(FilterItSimple[int64](func(item int64) bool { return item%2 == 0 })).
		Map(MapIt[int64, string](func(item int64) (string, error) { return strconv.FormatInt(item, 10), nil })).
		Map(FilterIt[string](func(item string) (bool, error) { return item == "2", nil })).
		Result()
	if err != nil {
		t.Errorf("Testcase failed with error : %v", err)
		return
	}
	assert.ElementsMatch(t, []any{"2"}, res)
}

func TestMapItSimple(t *testing.T) {
	
	// Create a map with some values
	floatingStrings := []string{"0.1", "0.2", "22", "22.1"}

	res, err := NewTransformer[string, float64](floatingStrings).
		Map(MapIt[string, float64](func(item string) (float64, error) {return strconv.ParseFloat(item, 64)})).
		Map(MapItSimple[float64, float64](func(item float64) float64 { return item * 10 })).
		Result()
	if err != nil {
		t.Errorf("Testcase failed with error : %v", err)
		return
	}
	t.Logf("Result: %v", res)
	assert.ElementsMatch(t, []any{float64(1), float64(2), float64(220), float64(221)}, res)
}

