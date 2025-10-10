package buff

type gnetConnContext struct {
	buf []byte
}

func (g *gnetConnContext) append(p []byte) {
	g.buf = append(g.buf, p...)
}

func (g *gnetConnContext) discard(n int) {
	switch {
	case n >= len(g.buf):
		g.buf = g.buf[:0]
	case n > 0:
		copy(g.buf, g.buf[n:])
		g.buf = g.buf[:len(g.buf)-n]
	}
}

func (g *gnetConnContext) reset() {
	g.buf = nil
}
