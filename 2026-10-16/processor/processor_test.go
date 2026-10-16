package processor

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func testProcessor(t *testing.T, workers int, timeout time.Duration) *Processor {
	t.Helper()
	p, err := New(Config{Workers: workers, TaskTimeout: timeout})
	if err != nil { t.Fatal(err) }
	return p
}

func TestProcessCollectsResultsAndErrors(t *testing.T) {
	p := testProcessor(t, 3, time.Second)
	tasks := make(chan Task, 3)
	tasks <- Task{ID: "ok", Fn: func(context.Context) (any, error) { return 42, nil }}
	tasks <- Task{ID: "error", Fn: func(context.Context) (any, error) { return nil, errors.New("boom") }}
	tasks <- Task{ID: "nil", Fn: nil}
	close(tasks)
	got := make(map[string]Result)
	for result := range p.Process(context.Background(), tasks) { got[result.TaskID] = result }
	if len(got) != 3 || got["ok"].Value != 42 || got["ok"].Err != nil { t.Fatalf("unexpected success results: %#v", got) }
	if got["error"].Err == nil || got["nil"].Err == nil { t.Fatalf("expected errors: %#v", got) }
}

func TestTaskTimeout(t *testing.T) {
	p := testProcessor(t, 1, 30*time.Millisecond)
	tasks := make(chan Task, 1)
	tasks <- Task{ID: "slow", Fn: func(ctx context.Context) (any, error) { <-ctx.Done(); return nil, ctx.Err() }}
	close(tasks)
	result := <-p.Process(context.Background(), tasks)
	if !errors.Is(result.Err, ErrTaskTimeout) { t.Fatalf("expected timeout, got %v", result.Err) }
}

func TestCancellationStopsWorkersAndDoesNotStartRemainingTasks(t *testing.T) {
	p := testProcessor(t, 2, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	tasks := make(chan Task, 10)
	var started atomic.Int32
	for i := 0; i < 10; i++ {
		tasks <- Task{ID: string(rune('a' + i)), Fn: func(ctx context.Context) (any, error) { started.Add(1); <-ctx.Done(); return nil, ctx.Err() }}
	}
	close(tasks)
	results := p.Process(ctx, tasks)
	time.Sleep(10 * time.Millisecond)
	cancel()
	for range results {}
	if started.Load() > 2 { t.Fatalf("more tasks started than workers: %d", started.Load()) }
}

func TestWorkersExitWhenTaskChannelCloses(t *testing.T) {
	before := runtime.NumGoroutine()
	p := testProcessor(t, 4, time.Second)
	tasks := make(chan Task)
	close(tasks)
	for range p.Process(context.Background(), tasks) {}
	time.Sleep(10 * time.Millisecond)
	if runtime.NumGoroutine() > before+2 { t.Fatalf("possible goroutine leak: before=%d after=%d", before, runtime.NumGoroutine()) }
}

func TestTaskPanicBecomesError(t *testing.T) {
	p := testProcessor(t, 1, time.Second)
	tasks := make(chan Task, 1)
	tasks <- Task{ID: "panic", Fn: func(context.Context) (any, error) { panic("bad task") }}
	close(tasks)
	result := <-p.Process(context.Background(), tasks)
	if result.Err == nil { t.Fatal("expected panic to become an error") }
}
