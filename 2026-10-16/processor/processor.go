// Package processor는 외부 라이브러리 없이 동시 작업을 처리하는 기능을 제공합니다.
package processor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrProcessorCanceled = errors.New("작업 시작 전에 작업 처리가 취소되었습니다")
var ErrTaskTimeout = errors.New("작업 시간이 초과되었습니다")

type Task struct {
	ID string
	Fn func(context.Context) (any, error)
}

type Result struct {
	TaskID   string
	WorkerID int
	Value    any
	Err      error
	Duration time.Duration
}

type Config struct {
	Workers     int
	TaskTimeout time.Duration
}

type Processor struct {
	workers int
	taskTimeout time.Duration
}

func New(cfg Config) (*Processor, error) {
	if cfg.Workers <= 0 {
		return nil, errors.New("Worker 수는 0보다 커야 합니다")
	}
	if cfg.TaskTimeout <= 0 {
		return nil, errors.New("작업 Timeout은 0보다 커야 합니다")
	}
	return &Processor{workers: cfg.Workers, taskTimeout: cfg.TaskTimeout}, nil
}

// Process는 작업을 소비하고 Processor가 소유·종료하는 결과 Channel을 반환합니다.
// 입력 Channel은 호출자가 소유하며, 작업 제출 후 닫아야 합니다.
func (p *Processor) Process(ctx context.Context, tasks <-chan Task) <-chan Result {
	results := make(chan Result, p.workers)
	var wg sync.WaitGroup
	wg.Add(p.workers)
	for workerID := 0; workerID < p.workers; workerID++ {
		go func(id int) {
			defer wg.Done()
			p.worker(ctx, id, tasks, results)
		}(workerID)
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	return results
}

func (p *Processor) worker(ctx context.Context, workerID int, tasks <-chan Task, results chan<- Result) {
	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-tasks:
			if !ok {
				return
			}
			if ctx.Err() != nil {
				return
			}
			result := p.execute(ctx, workerID, task)
			select {
			case results <- result:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (p *Processor) execute(parent context.Context, workerID int, task Task) Result {
	started := time.Now()
	result := Result{TaskID: task.ID, WorkerID: workerID}
	if task.Fn == nil {
		result.Err = errors.New("작업 함수가 nil입니다")
		result.Duration = time.Since(started)
		return result
	}
	taskCtx, cancel := context.WithTimeout(parent, p.taskTimeout)
	defer cancel()
	value, err := runSafely(taskCtx, task.Fn)
	if errors.Is(taskCtx.Err(), context.DeadlineExceeded) {
		result.Err = fmt.Errorf("%w: 작업 %q가 설정된 시간 %s를 초과했습니다", ErrTaskTimeout, task.ID, p.taskTimeout)
	} else if errors.Is(taskCtx.Err(), context.Canceled) && err == nil {
		result.Err = ErrProcessorCanceled
	} else {
		result.Value = value
		result.Err = err
	}
	result.Duration = time.Since(started)
	return result
}

func runSafely(ctx context.Context, fn func(context.Context) (any, error)) (value any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("작업에서 panic이 발생했습니다: %v", recovered)
		}
	}()
	return fn(ctx)
}
