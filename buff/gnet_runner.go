package buff

import (
	"fmt"
	"log"

	gnet "github.com/panjf2000/gnet/v2"
)

// RunGNet starts an HTTP server backed by gnet and blocks until shutdown.
func (e *Engine) RunGNet(addr string, opts ...GNetRunOption) error {
	if addr == "" {
		return fmt.Errorf("missing address")
	}
	cfg := defaultGNetRunConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	handler := newGNetHTTPHandler(e.R, cfg)
	protoAddr := ensureProtoAddr(addr)
	log.Printf("buff gnet listening on %s", protoAddr)
	return gnet.Run(handler, protoAddr, cfg.opts...)
}
