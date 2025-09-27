// example/buff_http_bench_test.go
package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/J1407B-K/buff/buff"
)

func startBuff(b *testing.B) (string, func()) {
	ln, err := net.Listen("tcp", "127.0.0.1:0") // 随机端口，避免冲突
	if err != nil {
		b.Fatal(err)
	}

	r := buff.NewRouter()
	// 🚫基准里不要任何中间件（Logger/Recover/Timeout 都关）
	_ = r.Handle("GET", "/hello/:id", func(c *buff.Context) {
		c.Text(200, c.Param("id")) // 走 fast 只对静态路由；这条是 trie
	})

	srv := &http.Server{Handler: r}
	go func() { _ = srv.Serve(ln) }()
	url := "http://" + ln.Addr().String()
	stop := func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
	return url, stop
}

func BenchmarkBuff_HelloParam(b *testing.B) {
	baseURL, stop := startBuff(b)
	defer stop()

	// 共享 Transport/Client，强制 HTTP/1.1，调高空闲连接池，避免新建连接
	tr := &http.Transport{
		MaxIdleConns:        1024,
		MaxIdleConnsPerHost: 1024,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
		ForceAttemptHTTP2:   false,
		// DisableKeepAlives: false (默认)，切记别开
	}
	client := &http.Client{Transport: tr, Timeout: 2 * time.Second}

	req, _ := http.NewRequest("GET", baseURL+"/hello/12345", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatalf("do: %v", err)
		}
		// 必须读尽响应体再关，否则连接不会回到空闲池
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != 200 {
			b.Fatalf("status=%d", resp.StatusCode)
		}
	}
}
