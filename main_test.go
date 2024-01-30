package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func BenchmarkDirectAccessSendString(b *testing.B) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	ctx := &TupaContext{
		request:  req,
		response: w,
	}

	start := time.Now()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx.SendString(http.StatusOK, "Hello, World!")
	}

	elapsed := time.Since(start)
	opsPerSec := float64(b.N) / elapsed.Seconds()
	b.ReportMetric(opsPerSec, "ops/sec")
}

func TestConcurrentSendStringCtxExample(t *testing.T) {
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			req, _ := http.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			ctx := context.WithValue(context.Background(), "value", i)
			tupaCtx := &TupaContext{
				request:  req,
				response: w,
				Context:  ctx,
			}

			err := tupaCtx.SendString(http.StatusOK, "Hello, World!")
			if err != nil {
				panic(err)
			}

		}(i)
	}

	wg.Wait()
}

// func TestSendStringWithValues(t *testing.T) {
// 	req, _ := http.NewRequest(http.MethodGet, "/", nil)
// 	w := httptest.NewRecorder()
// 	tupaCtx := &TupaContext{
// 		request:  req,
// 		response: w,
// 	}

// 	tupaCtx.WithValues(map[any]any{
// 		"nome":    "Victor",
// 		"empresa": "Iuptec",
// 	})
// 	ctx := context.Background()

// 	err := tupaCtx.SendString(http.StatusOK, "Hello, World!")
// 	if err != nil {
// 		t.Error(err)
// 	}
// }
