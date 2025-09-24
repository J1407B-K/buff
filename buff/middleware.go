package buff

import (
	"context"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

type Middleware func(Handler) Handler

func chain(mws ...Middleware) func(Handler) Handler {
	return func(h Handler) Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			h = mws[i](h)
		}
		return h
	}
}

func Recover() Middleware {
	return func(next Handler) Handler {
		return func(btx *Context) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[buff] panic: %v", r)
					_ = btx.JSON(http.StatusInternalServerError, map[string]any{"error": "internal error"})
				}
			}()
			next(btx)
		}
	}
}

func Logger() Middleware {
	return func(next Handler) Handler {
		return func(c *Context) {
			start := time.Now()
			next(c)
			dur := time.Since(start)
			status := http.StatusOK
			if sw, ok := c.Writer.(*statusWriter); ok {
				status = sw.Status()
			}
			route, _ := c.Get("route")
			log.Printf("%s %s route=%v %d %s", c.Request.Method, c.Request.URL.Path, route, status, dur)

		}
	}
}

type timeoutWriter struct {
	http.ResponseWriter
	timedOut atomic.Bool
}

func (tw *timeoutWriter) markTimedOut() bool {
	return tw.timedOut.CompareAndSwap(false, true)
}
func (tw *timeoutWriter) WriteHeader(code int) {
	if tw.timedOut.Load() {
		return
	}
	tw.ResponseWriter.WriteHeader(code)
}
func (tw *timeoutWriter) Write(p []byte) (int, error) {
	if tw.timedOut.Load() {
		return 0, http.ErrHandlerTimeout
	}
	return tw.ResponseWriter.Write(p)
}

func Timeout(d time.Duration) Middleware {
	return func(next Handler) Handler {
		return func(c *Context) {
			ctx, cancel := context.WithTimeout(c.Request.Context(), d)
			defer cancel()
			c.Request = c.Request.WithContext(ctx)

			if sw, ok := c.Writer.(*statusWriter); ok {
				c.Writer = &timeoutWriter{ResponseWriter: sw}
			} else {
				c.Writer = &timeoutWriter{ResponseWriter: c.Writer}
			}
			tw := c.Writer.(*timeoutWriter)

			done := make(chan struct{}, 1)
			go func() {
				next(c)
				done <- struct{}{} // 通知完成
			}()

			select {
			case <-done:
				return
			case <-ctx.Done():
				if tw.markTimedOut() {
					_ = (&Context{Writer: tw, Request: c.Request}).
						JSON(http.StatusGatewayTimeout, map[string]string{"error": "timeout"})
				}
				return
			}
		}
	}
}
