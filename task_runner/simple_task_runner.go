package taskrunner

import (
	"context"
	"errors"
	"sync"
)

/**
* Simple Task Runner is a simple implementation of the
* Task Runner interface. It runs tasks in the order they are
* added to the task list.
 */
type SimpleTaskRunner[T any] struct {
	ctx context.Context
	taskReq T
	mu sync.RWMutex
	tasks []TaskExecutor[T]
	parallelTasks []ParallelExecutor[T]
}

func NewSimpleTaskRunner[T any](ctx context.Context, taskReq T) *SimpleTaskRunner[T] {
	return &SimpleTaskRunner[T]{
		ctx: ctx,
		taskReq: taskReq,
		mu: sync.RWMutex{},
	}
}

func (s *SimpleTaskRunner[T]) Then(taskExec TaskExecutor[T]) *SimpleTaskRunner[T] {
	s.tasks = append(s.tasks, taskExec)
	return s
}

func (s *SimpleTaskRunner[T]) Parallel(parallelExec ParallelExecutor[T]) *SimpleTaskRunner[T] {
	s.parallelTasks = append(s.parallelTasks, parallelExec)
	return s
}

func (s *SimpleTaskRunner[T]) Result() (T, error) {
	err := s.serialExecutor()
	err = errors.Join(err, s.parallelExecutor())
	return s.taskReq, err
}


func (s* SimpleTaskRunner[T]) serialExecutor() error {
	for _, task := range(s.tasks) {
		err := task(s.ctx, &s.taskReq)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SimpleTaskRunner[T]) parallelExecutor() error {
	errChan := make(chan error)
	wg := sync.WaitGroup{}
	var err error
	for _, task := range(s.parallelTasks) {
		wg.Add(1)
		go func(ctx context.Context, mu *sync.RWMutex, taskReq *T) {
			defer wg.Done()
			err := task(ctx, taskReq, mu)
			errChan <- err
		}(s.ctx, &s.mu, &s.taskReq)
	}
	
	go func ()  {
		wg.Wait()
		close(errChan)
	}()

	// Collect errors from errChan
	for goErr := range errChan {
		if goErr != nil {
			err = errors.Join(err, goErr)
		}
	}
	return err
}