package goctx

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewTaskContext(t *testing.T) {

	t.Run("creates new context", func(t *testing.T) {
		ctx := NewTaskContext(context.Background())
		assert.NotNil(t, ctx)
		assert.Nil(t, ctx.Err())
	})
}

func TestTaskContext_WithError(t *testing.T) {
	t.Run("single error", func(t *testing.T) {
		ctx := NewTaskContext(context.Background())
		err := errors.New("test error")
		ctx.WithError(err)
		assert.Equal(t, err, ctx.Err())
	})

	t.Run("multiple errors are joined", func(t *testing.T) {
		ctx := NewTaskContext(context.Background())
		err1 := errors.New("error 1")
		err2 := errors.New("error 2")

		ctx.WithError(err1)
		ctx.WithError(err2)

		assert.True(t, errors.Is(ctx.Err(), err1))
		assert.Contains(t, ctx.Err().Error(), "error 1")
		assert.Contains(t, ctx.Err().Error(), "error 2")
	})
}

func TestTaskContext_AddError(t *testing.T) {
	t.Run("adds multiple errors", func(t *testing.T) {
		ctx := NewTaskContext(context.Background())
		err1 := errors.New("error 1")
		err2 := errors.New("error 2")

		ctx.AddError(err1)
		ctx.AddError(err2)

		allErrors := ctx.Errors()
		assert.Contains(t, allErrors.Error(), err1.Error())
		assert.Contains(t, allErrors.Error(), err2.Error())
	})

	t.Run("thread safety", func(t *testing.T) {
		ctx := NewTaskContext(context.Background())
		var wg sync.WaitGroup

		for i := 0; i < 100; i++ {
			wg.Add(1)
			i := i
			go func() {
				defer wg.Done()
				ctx.AddError(fmt.Errorf("error %d", i))
			}()
		}

		wg.Wait()
		assert.NotNil(t, ctx.Errors())
	})
}

func TestRun(t *testing.T) {
	t.Run("successful execution", func(t *testing.T) {
		ctx := NewTaskContext(context.Background())
		result := Run(ctx, func() (string, error) {
			return "success", nil
		})
		assert.NoError(t, ctx.Err())
		assert.Equal(t, "success", result)
	})

	t.Run("function error", func(t *testing.T) {
		ctx := NewTaskContext(context.Background())
		expectedErr := errors.New("test error")
		_ = Run(ctx, func() (string, error) {
			return "", expectedErr
		})
		_ = Run(ctx, func() (int64, error) {
			return 20, nil
		})
		assert.Equal(t, expectedErr, ctx.Err())
	})
}

func TestRunParallel(t *testing.T) {
	t.Run("successful parallel execution", func(t *testing.T) {
		ctx := NewTaskContext(context.Background())
		results, err := RunParallel(ctx,
			func() (int, error) { return 1, nil },
			func() (int, error) { return 2, nil },
			func() (int, error) { return 3, nil },
		)
		assert.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, results)
	})

	t.Run("handles errors", func(t *testing.T) {
		ctx := NewTaskContext(context.Background())
		_, err := RunParallel(ctx,
			func() (int, error) { return 0, errors.New("error 1") },
			func() (int, error) { return 2, nil },
			func() (int, error) { return 0, errors.New("error 3") },
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task 1")
		assert.Contains(t, err.Error(), "task 3")
	})
}

func TestRunParallelWithLimit(t *testing.T) {
	t.Run("respects concurrency limit", func(t *testing.T) {
		ctx := NewTaskContext(context.Background())
		concurrent := atomic.Int32{}
		maxConcurrent := atomic.Int32{}

		tasks := make([]RunFn[int], 10)
		for i := range tasks {
			tasks[i] = func() (int, error) {
				curr := concurrent.Add(1)
				if curr > maxConcurrent.Load() {
					maxConcurrent.Store(curr)
				}
				time.Sleep(10 * time.Millisecond)
				concurrent.Add(-1)
				return 1, nil
			}
		}

		_, err := RunParallelWithLimit(ctx, 3, tasks...)
		assert.NoError(t, err)
		assert.LessOrEqual(t, maxConcurrent.Load(), int32(3))
	})
}

func ExampleTaskContext() {
	ctx := NewTaskContext(context.Background())

	// Single execution
	result := Run(ctx, func() (string, error) {
		return "success", nil
	})
	fmt.Printf("Single execution: %s\n", result)

	// Parallel execution with error
	results, _ := RunParallel(ctx,
		func() (int, error) { return 0, errors.New("task failed") },
		func() (int, error) { return 42, nil },
	)
	fmt.Printf("Results: %v\n", results)
	fmt.Printf("Error: %v\n", ctx.Err())

	// Output:
	// Single execution: success
	// Results: [0 42]
	// Error: task 1: task failed
}

func TestDerivedContextFromTaskContext(t *testing.T) {
	//Test for the derived context from TaskContext
	// Check if the derived context function is working as expected

	ctx := NewTaskContext(context.Background())
	derivedCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	// sleep for 2 seconds
	time.Sleep(2 * time.Second)
	assert.ErrorIs(t, derivedCtx.Err(), context.DeadlineExceeded)
}
