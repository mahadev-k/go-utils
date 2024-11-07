package taskrunner

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	errFoo error = fmt.Errorf("error in processFoo")
)

func processFoo(ctx context.Context, taskReq *struct{isFoo bool; isBar bool}) error {
	taskReq.isFoo = true
	return nil
}

func processBar(ctx context.Context, taskReq *struct{isFoo bool; isBar bool}) error {
	taskReq.isBar = true
	return nil
}

func TestSimpleTaskRunner(t *testing.T) {
	// Create a new task runner
	req := struct {
		isFoo bool
		isBar bool
	}{}
	ctx := context.TODO()
	runner := NewSimpleTaskRunner(ctx, req)
	res, err := runner.
		Then(processFoo).
		Then(processBar).
		Result()
	// Check the result
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	// Check the result
	assert.Equal(t, true, res.isFoo)
	assert.Equal(t, true, res.isBar)

}

func processFooError(ctx context.Context, taskReq *struct{isFoo bool; isBar bool}) error {
	taskReq.isFoo = true
	return errFoo
}

func TestSimpleTaskRunnerForErrorHandling(t *testing.T) {
	// Create a new task runner
	req := struct {
		isFoo bool
		isBar bool
	}{}
	ctx := context.TODO()
	runner := NewSimpleTaskRunner(ctx, req)
	_, err := runner.
		Then(processFooError).
		Then(processBar).
		Result()
	
	assert.Error(t, err, errFoo)
}

func processFooParallel(ctx context.Context, taskReq *struct{isFoo bool; isBar bool}, mu *sync.RWMutex) error {
	return processFoo(ctx, taskReq)
}

func processBarParallel(ctx context.Context, taskReq *struct{isFoo bool; isBar bool}, mu *sync.RWMutex) error {
	mu.Lock()
	defer mu.Unlock()
	taskReq.isBar = true
	return nil
}

func processFooParallelError(ctx context.Context, taskReq *struct{isFoo bool; isBar bool}, mu *sync.RWMutex) error {
	return errFoo
}

func TestSimpleTaskRunnerParallel(t *testing.T) {
	req := struct {
		isFoo bool
		isBar bool
	}{}
	ctx := context.TODO()
	runner := NewSimpleTaskRunner(ctx, req)
	// Test parallel execution
	res, err := runner.
		Parallel(processBarParallel).
		Parallel(processFooParallel).
		Result()
	
	if err != nil {
		t.Errorf("error occurred %v", err)
	}
	assert.Equal(t, true, res.isFoo)
	assert.Equal(t, true, res.isBar)
}

func TestSimpleTaskRunnerParallelError(t *testing.T) {
	req := struct {
		isFoo bool
		isBar bool
	}{}
	ctx := context.TODO()
	runner := NewSimpleTaskRunner(ctx, req)
	// Test parallel execution with error
	res, err := runner.
		Parallel(processFooParallelError).
		Parallel(processBarParallel).
		Result()
	
	assert.Error(t, errFoo, err)
	assert.Equal(t, true, res.isBar)
}