package processor

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func BenchmarkWorkers(b *testing.B) {
	for _, workers := range []int{1, 2, 4, 8} {
		b.Run("workers="+strconv.Itoa(workers), func(b *testing.B) {
			p, _ := New(Config{Workers: workers, TaskTimeout: time.Second})
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				tasks := make(chan Task, 8)
				for i := 0; i < 8; i++ {
					tasks <- Task{Fn: func(context.Context) (any, error) { time.Sleep(time.Millisecond); return nil, nil }}
				}
				close(tasks)
				for range p.Process(context.Background(), tasks) {}
			}
		})
	}
}
