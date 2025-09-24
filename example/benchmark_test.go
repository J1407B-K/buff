package main

import (
	"net/http/httptest"
	"testing"

	"github.com/J1407B-K/buff/buff"
)

func BenchmarkRoute_HelloParam(b *testing.B) {
	r := buff.NewRouter() // 或 NewRouter()
	// 注意：基准时不要 Use(Logger/Timeout) 之类的中间件，否则会拖慢很多
	_ = r.Handle("GET", "/hello/:id", func(c *buff.Context) {
		c.Text(200, c.Param("id"))
	})

	req := httptest.NewRequest("GET", "/hello/12345", nil)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			b.Fatalf("unexpected code: %d", w.Code)
		}
	}
}
