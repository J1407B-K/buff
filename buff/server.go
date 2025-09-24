package buff

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Server struct{ *http.Server }

func (r *Router) Listen(addr string) error {
	srv := &http.Server{Addr: addr, Handler: r, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	return runGraceful(srv)
}

func runGraceful(srv *http.Server) error {
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-sigCh:
		log.Printf("[buff] shutting down: %v", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
