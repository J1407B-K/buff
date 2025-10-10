package buff

import (
	"os"
	"syscall"
	"time"

	gnet "github.com/panjf2000/gnet/v2"
)

const (
	defaultMaxHeaderBytes  = 8 << 10
	defaultShutdownTimeout = 5 * time.Second
	defaultServerHeader    = "buff-gnet"
)

type gnetRunConfig struct {
	maxHeaderBytes  int
	shutdownSignals []os.Signal
	shutdownTimeout time.Duration
	opts            []gnet.Option
	serverHeader    string
}

func defaultGNetRunConfig() gnetRunConfig {
	return gnetRunConfig{
		maxHeaderBytes:  defaultMaxHeaderBytes,
		shutdownSignals: []os.Signal{syscall.SIGINT, syscall.SIGTERM},
		shutdownTimeout: defaultShutdownTimeout,
		serverHeader:    defaultServerHeader,
	}
}

// GNetRunOption configures RunGNet behaviour.
type GNetRunOption func(*gnetRunConfig)

// WithGNetMaxHeaderBytes sets the maximum allowed header size for incoming requests.
func WithGNetMaxHeaderBytes(n int) GNetRunOption {
	return func(cfg *gnetRunConfig) {
		if n > 0 {
			cfg.maxHeaderBytes = n
		}
	}
}

// WithGNetShutdownSignals overrides the OS signals that trigger graceful shutdown.
func WithGNetShutdownSignals(signals ...os.Signal) GNetRunOption {
	return func(cfg *gnetRunConfig) {
		if len(signals) > 0 {
			cfg.shutdownSignals = signals
		}
	}
}

// WithGNetShutdownTimeout overrides the graceful shutdown timeout.
func WithGNetShutdownTimeout(d time.Duration) GNetRunOption {
	return func(cfg *gnetRunConfig) {
		if d > 0 {
			cfg.shutdownTimeout = d
		}
	}
}

// WithGNetServerHeader overrides the default Server response header.
func WithGNetServerHeader(header string) GNetRunOption {
	return func(cfg *gnetRunConfig) {
		if header != "" {
			cfg.serverHeader = header
		}
	}
}

// WithGNetOption forwards a gnet.Option to the underlying event engine.
func WithGNetOption(opt gnet.Option) GNetRunOption {
	return func(cfg *gnetRunConfig) {
		cfg.opts = append(cfg.opts, opt)
	}
}
