package queue

import (
"context"
"fmt"
"testing"

"github.com/devaloi/workq/internal/domain"
)

func BenchmarkMemoryQueue_Enqueue(b *testing.B) {
mq := NewMemoryQueue()
defer mq.Close()
ctx := context.Background()

b.ResetTimer()
for i := 0; i < b.N; i++ {
job, _ := domain.NewJob("bench", []byte("payload"), 1)
if err := mq.Enqueue(ctx, job); err != nil {
b.Fatal(err)
}
}
}

func BenchmarkMemoryQueue_EnqueueDequeue(b *testing.B) {
mq := NewMemoryQueue()
defer mq.Close()
ctx := context.Background()

b.ResetTimer()
for i := 0; i < b.N; i++ {
job, _ := domain.NewJob("bench", []byte("payload"), 1)
if err := mq.Enqueue(ctx, job); err != nil {
b.Fatal(err)
}
if _, err := mq.Dequeue(ctx); err != nil {
b.Fatal(err)
}
}
}

func BenchmarkMemoryQueue_PriorityOrder(b *testing.B) {
ctx := context.Background()

b.ResetTimer()
for i := 0; i < b.N; i++ {
b.StopTimer()
mq := NewMemoryQueue()
for p := 10; p >= 1; p-- {
job, _ := domain.NewJob(fmt.Sprintf("job-%d", p), []byte("payload"), p)
_ = mq.Enqueue(ctx, job)
}
b.StartTimer()

for j := 0; j < 10; j++ {
if _, err := mq.Dequeue(ctx); err != nil {
b.Fatal(err)
}
}
mq.Close()
}
}
