package goctx

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// TaskContext wraps a context.Context and adds thread-safe error handling
// for task execution and error collection
type TaskContext struct {
	context.Context
	mu       sync.RWMutex
	err      error
	multiErr []error
}

// NewTaskContext returns a new TaskContext that wraps the parent context.
func NewTaskContext(parent context.Context) *TaskContext {
	if parent == nil {
		panic("cannot create context from nil parent")
	}
	return &TaskContext{Context: parent}
}

// WithError sets the first error on the context and joins with existing errors
func (c *TaskContext) WithError(err error) *TaskContext {
	if err == nil {
		return c
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.multiErr = append(c.multiErr, err)
	if c.err == nil {
		c.err = err
	} else {
		c.err = errors.Join(c.err, err)
	}
	return c
}

// AddError adds an error to the multi-error collection
func (c *TaskContext) AddError(err error) {
	if err == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.multiErr = append(c.multiErr, err)
	if c.err == nil {
		c.err = err
	} else {
		c.err = errors.Join(c.err, err)
	}
}

// Err returns the first error stored in the context
func (c *TaskContext) Err() error {
	// First check parent context's error (timeout, cancel, etc)
	if err := c.Context.Err(); err != nil {
		return err
	}

	// Then check parent's custom errors if it's a TaskContext
	if tc, ok := c.Context.(*TaskContext); ok {
		if err := tc.err; err != nil {
			return err
		}
	}

	// Finally check our own custom error
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.err
}

// Errors returns all collected errors joined together
func (c *TaskContext) Errors() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.multiErr) == 0 {
		return nil
	}
	return errors.Join(c.multiErr...)
}

// Define RunFn type at the top with other types
type RunFn[T any] func() (T, error)

// Update Run to use RunFn
func Run[T any](ctx *TaskContext, fn RunFn[T]) T {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero
	}

	result, err := fn()
	if err != nil {
		ctx.WithError(err)
		return zero
	}
	return result
}

// Update RunParallel to use RunFn
func RunParallel[T any](ctx *TaskContext, fns ...RunFn[T]) ([]T, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	results := make([]T, len(fns))
	var resultsMu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(fns))

	for i, fn := range fns {
		i, fn := i, fn
		go func() {
			defer wg.Done()
			result, err := fn()
			if err != nil {
				ctx.AddError(fmt.Errorf("task %d: %w", i+1, err))
			} else {
				resultsMu.Lock()
				results[i] = result
				resultsMu.Unlock()
			}
		}()
	}

	wg.Wait()
	return results, ctx.Errors()
}

// Update RunParallelWithLimit to use RunFn
func RunParallelWithLimit[T any](ctx *TaskContext, limit int, fns ...RunFn[T]) ([]T, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	results := make([]T, len(fns))
	var resultsMu sync.Mutex
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	wg.Add(len(fns))

	for i, fn := range fns {
		i, fn := i, fn
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := fn()
			if err != nil {
				ctx.AddError(fmt.Errorf("task %d: %w", i+1, err))
			} else {
				resultsMu.Lock()
				results[i] = result
				resultsMu.Unlock()
			}
		}()
	}

	wg.Wait()
	return results, ctx.Errors()
}