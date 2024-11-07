package taskrunner

import (
	"context"
	"sync"
)

/**
* Task Runner is a framework that could decouple the task without
* bothering about every piece of error handling.
**/
type TaskExecutor[T any] func(ctx context.Context, taskReq *T) error
type ParallelExecutor[T any] func(ctx context.Context, taskReq *T, mu *sync.RWMutex) error
type TaskRunner[T any] interface {
	Then(taskExec TaskExecutor[T]) TaskRunner[T]
	Parallel(parallelExec ParallelExecutor[T]) TaskRunner[T]
	Result() (T, error)
}

