package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/J1407B-K/buff/buff"
	gnet "github.com/panjf2000/gnet/v2"
)

func main() {
	const defaultAddr = ":8080"

	addr := flag.String("addr", defaultAddr, "listen address")
	engineName := flag.String("engine", "gnet", "engine to run: gnet or std")
	gnetMulticore := flag.Bool("gnet-multicore", true, "enable multicore event-loops when using gnet")
	gnetReusePort := flag.Bool("gnet-reuseport", true, "enable SO_REUSEPORT when using gnet")
	gnetLoops := flag.Int("gnet-loops", 0, "number of gnet event-loops to start (0 lets gnet decide)")
	flag.Parse()

	if *addr == defaultAddr {
		switch {
		case os.Getenv("BUFF_ADDR") != "":
			*addr = os.Getenv("BUFF_ADDR")
		case os.Getenv("PORT") != "":
			port := os.Getenv("PORT")
			if !strings.HasPrefix(port, ":") {
				port = ":" + port
			}
			*addr = port
		}
	}

	e := buff.NewEngine()
	e.Use(buff.Logger(), buff.Timeout(5*time.Second))

	e.GET("/ping", PongHandler)
	e.GET("/hello/:name", func(c *buff.Context) {
		c.JSON(200, map[string]string{"hi": c.Param("name")})
	})

	if err := e.R.Verify(); err != nil {
		log.Fatal(err)
	}

	switch strings.ToLower(*engineName) {
	case "gnet":
		var opts []buff.GNetRunOption
		if *gnetMulticore {
			opts = append(opts, buff.WithGNetOption(gnet.WithMulticore(true)))
		}
		if *gnetReusePort {
			opts = append(opts, buff.WithGNetOption(gnet.WithReusePort(true)))
		}
		if *gnetLoops > 0 {
			opts = append(opts, buff.WithGNetOption(gnet.WithNumEventLoop(*gnetLoops)))
		}
		log.Fatal(e.RunGNet(*addr, opts...))
	case "std", "nethttp", "http":
		log.Fatal(e.Run(*addr))
	default:
		log.Fatal(fmt.Errorf("unsupported engine %q (use gnet or std)", *engineName))
	}
}
