package buff

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Engine struct {
	R          *Router
	mws        []Middleware
	httpServer *http.Server
}

func NewEngine() *Engine { return &Engine{R: NewRouter()} }

func (e *Engine) Use(mw ...Middleware) { e.mws = append(e.mws, mw...) }

func (e *Engine) GET(path string, h Handler)    { _ = e.R.Handle(http.MethodGet, path, h, e.mws...) }
func (e *Engine) POST(path string, h Handler)   { _ = e.R.Handle(http.MethodPost, path, h, e.mws...) }
func (e *Engine) PUT(path string, h Handler)    { _ = e.R.Handle(http.MethodPut, path, h, e.mws...) }
func (e *Engine) PATCH(path string, h Handler)  { _ = e.R.Handle(http.MethodPatch, path, h, e.mws...) }
func (e *Engine) DELETE(path string, h Handler) { _ = e.R.Handle(http.MethodDelete, path, h, e.mws...) }

// ServeHTTP just delegates to the underlying Router
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) { e.R.ServeHTTP(w, r) }

// Run starts a net/http server with graceful shutdown
func (e *Engine) Run(addr string) error {
	e.httpServer = &http.Server{Addr: addr, Handler: e, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := e.httpServer.Shutdown(ctx); err != nil {
			log.Printf("server shutdown: %v", err)
		}
	}()
	log.Printf("buff listening on %s", addr)
	return e.httpServer.ListenAndServe()
}
