package stream_utils

import (
	"fmt"
	"reflect"
)

type MappingFn[T any, R any] func(item T) (R, error)
type FilterFn[T any] func(item T) (bool, error)
type SimpleMapper[T any, R any] func(item T) R
type SimpleFilter[T any] func(item T) bool

type ObjectMapper interface {
	Result(items any) (any, error)
}

type MapRunner[T, R any] struct {
	mappingFn    MappingFn[T, R]
	filterFn     FilterFn[T]
	simpleMapper SimpleMapper[T, R]
	simpleFilter SimpleFilter[T]
	err          error
}

func MapIt[T, R any](fn MappingFn[T, R]) *MapRunner[T, R] {
	return &MapRunner[T, R]{
		mappingFn: fn,
		err:       nil,
	}
}

func MapItSimple[T, R any](fn SimpleMapper[T, R]) *MapRunner[T, R] {
	return &MapRunner[T, R]{
		simpleMapper: fn,
		err:          nil,
	}
}

func FilterIt[T any](fn FilterFn[T]) *MapRunner[T, T] {
	return &MapRunner[T, T]{
		filterFn: fn,
		err:      nil,
	}
}

func FilterItSimple[T any](fn SimpleFilter[T]) *MapRunner[T, T] {
	return &MapRunner[T, T]{
		simpleFilter: fn,
		err:          nil,
	}
}

func (m *MapRunner[T, R]) Result(items any) (any, error) {
	var results []R
	if _, ok := items.([]T); !ok {
		var t T
		return nil, fmt.Errorf("not able to typecast items : %v", reflect.TypeOf(t).Name())
	}
	for _, item := range (items).([]T) {
		if m.mappingFn != nil {
			res, err := m.mappingFn(item)
			if err != nil {
				return nil, err
			}
			results = append(results, res)
		} else if m.filterFn != nil {
			ok, err := m.filterFn(item)
			if err != nil {
				return nil, err
			}
			if ok {
				var res any
				res = item
				results = append(results, res.(R))
			}
		} else if m.simpleMapper != nil {
			res := m.simpleMapper(item)
			results = append(results, res)
		} else if m.simpleFilter != nil {
			ok := m.simpleFilter(item)
			if ok {
				var res any
				res = item
				results = append(results, res.(R))
			}
		}
	}
	return results, nil
}

type Transformer[T any, R any] struct {
	items   any
	mappers []ObjectMapper
}

func NewTransformer[T, R any](items []T) *Transformer[T, R] {
	return &Transformer[T, R]{
		items: items,
	}
}

func (t *Transformer[T, R]) Transform(mapper ObjectMapper) *Transformer[T, R] {
	t.mappers = append(t.mappers, mapper)
	return t
}

func (t *Transformer[T, R]) Result() (r []R, err error) {
	for _, mapper := range t.mappers {
		items, err := mapper.Result(t.items)
		if err != nil {
			return nil, err
		}
		t.items = items
	}

	if _, ok := t.items.([]R); !ok {
		var r R
		return nil, fmt.Errorf("bad type casting %v", reflect.TypeOf(r).Name())
	}
	return t.items.([]R), nil
}
