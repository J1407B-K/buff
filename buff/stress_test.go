package buff

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"testing"
	"time"

	gnet "github.com/panjf2000/gnet/v2"
)

type benchServer struct {
	url  string
	stop func(testing.TB)
}

func BenchmarkEngineRun(b *testing.B) {
	benchmarkServer(b, startNetHTTPServer)
}

func BenchmarkEngineRunGNet(b *testing.B) {
	if runtime.GOOS == "windows" {
		b.Skip("gnet is not supported on Windows")
	}
	benchmarkServer(b, startGNetServer)
}

type serverStarter func(*testing.B) benchServer

func benchmarkServer(b *testing.B, start serverStarter) {
	b.Helper()

	srv := start(b)
	defer func() {
		b.StopTimer()
		srv.stop(b)
	}()

	transport := &http.Transport{
		MaxConnsPerHost:     256,
		MaxIdleConns:        256,
		MaxIdleConnsPerHost: 256,
		DisableCompression:  true,
	}
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	defer transport.CloseIdleConnections()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.url, nil)
	if err != nil {
		b.Fatalf("new request: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := client.Do(req.Clone(context.Background()))
			if err != nil {
				b.Fatalf("http request: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				_ = resp.Body.Close()
				b.Fatalf("unexpected status: %d", resp.StatusCode)
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
	})
}

func startNetHTTPServer(b *testing.B) benchServer {
	b.Helper()

	e := newBenchmarkEngine()
	port := freePort(b)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	baseURL := fmt.Sprintf("http://%s/ping", addr)

	errCh := make(chan error, 1)
	go func() {
		errCh <- e.Run(addr)
	}()

	waitForServer(b, baseURL)

	return benchServer{
		url: baseURL,
		stop: func(tb testing.TB) {
			tb.Helper()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := e.httpServer.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) {
				tb.Fatalf("shutdown net/http server: %v", err)
			}
			if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
				tb.Fatalf("net/http server error: %v", err)
			}
		},
	}
}

func startGNetServer(b *testing.B) benchServer {
	b.Helper()

	e := newBenchmarkEngine()
	port := freePort(b)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	baseURL := fmt.Sprintf("http://%s/ping", addr)
	protoAddr := ensureProtoAddr(addr)

	errCh := make(chan error, 1)
	go func() {
		errCh <- e.RunGNet(addr)
	}()

	waitForServer(b, baseURL)

	return benchServer{
		url: baseURL,
		stop: func(tb testing.TB) {
			tb.Helper()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := gnet.Stop(ctx, protoAddr); err != nil {
				tb.Fatalf("stop gnet server: %v", err)
			}
			if err := <-errCh; err != nil {
				tb.Fatalf("gnet server error: %v", err)
			}
		},
	}
}

func newBenchmarkEngine() *Engine {
	e := NewEngine()
	e.GET("/ping", func(btx *Context) {
		_ = btx.Text(http.StatusOK, "pong")
	})
	return e
}

func freePort(tb testing.TB) int {
	tb.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("listen for free port: %v", err)
	}
	defer func() { _ = l.Close() }()
	tcpAddr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		tb.Fatalf("unexpected addr type: %T", l.Addr())
	}
	return tcpAddr.Port
}

func waitForServer(tb testing.TB, url string) {
	tb.Helper()
	client := &http.Client{Timeout: 100 * time.Millisecond}
	defer client.CloseIdleConnections()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				return
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}
	tb.Fatalf("server %s not ready in time", url)
}
