package buff

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	gnet "github.com/panjf2000/gnet/v2"
	"github.com/valyala/bytebufferpool"
)

type gnetHTTPHandler struct {
	gnet.BuiltinEventEngine

	router          *Router
	maxHeaderBytes  int
	shutdownSignals []os.Signal
	shutdownTimeout time.Duration
	serverHeader    string

	engine  gnet.Engine
	bufPool *bytebufferpool.Pool
}

func newGNetHTTPHandler(r *Router, cfg gnetRunConfig) *gnetHTTPHandler {
	return &gnetHTTPHandler{
		router:          r,
		maxHeaderBytes:  cfg.maxHeaderBytes,
		shutdownSignals: cfg.shutdownSignals,
		shutdownTimeout: cfg.shutdownTimeout,
		serverHeader:    cfg.serverHeader,
		bufPool:         &bytebufferpool.Pool{},
	}
}

func (h *gnetHTTPHandler) OnBoot(engine gnet.Engine) (action gnet.Action) {
	h.engine = engine
	if len(h.shutdownSignals) > 0 {
		go h.handleSignals()
	}
	return gnet.None
}

func (h *gnetHTTPHandler) handleSignals() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, h.shutdownSignals...)
	sig := <-sigCh
	log.Printf("[buff] gnet shutting down: %v", sig)
	timeout := h.shutdownTimeout
	if timeout <= 0 {
		timeout = defaultShutdownTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := h.engine.Stop(ctx); err != nil {
		log.Printf("[buff] gnet stop error: %v", err)
	}
}

func (h *gnetHTTPHandler) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	c.SetContext(&gnetConnContext{})
	return nil, gnet.None
}

func (h *gnetHTTPHandler) OnClose(c gnet.Conn, err error) (action gnet.Action) {
	if ctx, ok := c.Context().(*gnetConnContext); ok {
		ctx.reset()
	}
	return gnet.None
}

func (h *gnetHTTPHandler) OnTraffic(c gnet.Conn) gnet.Action {
	ctx, _ := c.Context().(*gnetConnContext)
	if ctx == nil {
		ctx = &gnetConnContext{}
		c.SetContext(ctx)
	}

	if n := c.InboundBuffered(); n > 0 {
		data, err := c.Next(n)
		if err != nil {
			h.writeError(c, http.StatusInternalServerError, "read error")
			return gnet.Close
		}
		ctx.append(data)
	}

	for len(ctx.buf) > 0 {
		req, consumed, closeAfter, err := parseHTTPRequest(ctx.buf, h.maxHeaderBytes)
		if err != nil {
			if errors.Is(err, errNeedMoreData) {
				break
			}
			if errors.Is(err, errHeaderTooLarge) {
				h.writeError(c, http.StatusRequestHeaderFieldsTooLarge, err.Error())
			} else {
				h.writeError(c, http.StatusBadRequest, err.Error())
			}
			return gnet.Close
		}

		req.RemoteAddr = c.RemoteAddr().String()

		writer := acquireGNetResponseWriter(h.bufPool)
		writer.serverHdr = h.serverHeader
		h.router.ServeHTTP(writer, req)
		_ = req.Body.Close()

		respBuf := h.bufPool.Get()
		respBuf.Reset()
		respBuf, shouldClose := writer.finalize(req, closeAfter, respBuf)
		if _, err := c.Write(respBuf.Bytes()); err != nil {
			h.bufPool.Put(respBuf)
			releaseGNetResponseWriter(h.bufPool, writer)
			return gnet.Close
		}
		h.bufPool.Put(respBuf)
		releaseGNetResponseWriter(h.bufPool, writer)

		ctx.discard(consumed)

		if shouldClose {
			return gnet.Close
		}
	}
	return gnet.None
}

func (h *gnetHTTPHandler) writeError(c gnet.Conn, status int, msg string) {
	if msg == "" {
		msg = http.StatusText(status)
	}
	body := []byte(msg + "\n")
	buf := h.bufPool.Get()
	buf.Reset()
	fmt.Fprintf(buf, "HTTP/1.1 %d %s%s", status, http.StatusText(status), crlf)
	fmt.Fprintf(buf, "Content-Type: text/plain; charset=utf-8%s", crlf)
	fmt.Fprintf(buf, "Content-Length: %d%s", len(body), crlf)
	buf.WriteString("Connection: close\r\n")
	buf.WriteString(crlf)
	buf.Write(body)
	_, _ = c.Write(buf.Bytes())
	h.bufPool.Put(buf)
}
