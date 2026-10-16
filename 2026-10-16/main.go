package main

import (
	"context"
	"fmt"
	"time"

	"concurrenttaskprocessor/processor"
)

func main() {
	const taskCount = 12

	for _, workers := range []int{1, 2, 4} {
		p, err := processor.New(processor.Config{
			Workers:     workers,
			TaskTimeout: 500 * time.Millisecond,
		})
		if err != nil {
			panic(err)
		}

		tasks := make(chan processor.Task, taskCount)
		for i := 0; i < taskCount; i++ {
			id := i
			tasks <- processor.Task{
				ID: fmt.Sprintf("task-%02d", id),
				Fn: func(context.Context) (any, error) {
					time.Sleep(50 * time.Millisecond)
					return id, nil
				},
			}
		}
		close(tasks)

		started := time.Now()
		success, failed := 0, 0
		for result := range p.Process(context.Background(), tasks) {
			if result.Err != nil {
				failed++
			} else {
				success++
			}
		}
		fmt.Printf("workers=%d elapsed=%s success=%d failed=%d\n",
			workers, time.Since(started).Round(time.Millisecond), success, failed)
	}
}
