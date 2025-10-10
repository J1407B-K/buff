package buff

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

type Router struct {
	root *node

	mw []Middleware

	notFound Handler

	pool sync.Pool

	fast map[string]map[string]Handler

	mu sync.RWMutex
}

func NewRouter() *Router {
	r := &Router{
		root: newNode("/"),
		mw:   make([]Middleware, 0),
		notFound: func(btx *Context) {
			btx.JSON(http.StatusNotFound, map[string]any{"error": "route not found"})
		},
		fast: make(map[string]map[string]Handler),
	}
	r.pool.New = func() any { return &Context{} }
	return r
}

func (r *Router) Use(m ...Middleware) { r.mw = append(r.mw, m...) }

func (r *Router) Handle(method, path string, h Handler, mws ...Middleware) error {
	if path == "" || path[0] != '/' {
		return errors.New("path must start with '/'")
	}
	method = strings.ToUpper(method)
	clean := normalize(path)

	final := chain(append(r.mw, mws...)...)(Recover()(h))

	r.mu.Lock()
	defer r.mu.Unlock()

	if !strings.ContainsAny(clean, ":*") {
		mm := r.fast[method]
		if mm == nil {
			mm = map[string]Handler{}
			r.fast[method] = mm
		}
		if _, ok := mm[clean]; ok {
			return fmt.Errorf("route exists: %s %s", method, clean)
		}
		mm[clean] = final
		return nil
	}

	parts := splitPath(clean)
	if err := r.root.add(method, parts, final, clean); err != nil {
		return err
	}
	return nil
}

type Group struct {
	r    *Router
	base string
	mw   []Middleware
}

func (r *Router) Group(prefix string, m ...Middleware) *Group {
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return &Group{r: r, base: strings.TrimRight(prefix, "/"), mw: m}
}

func (g *Group) Handle(method, path string, h Handler, mws ...Middleware) error {
	full := g.base
	if path != "" && path != "/" {
		full += "/" + strings.Trim(path, "/")
	}
	return g.r.Handle(method, full, h, append(g.mw, mws...)...)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	root := r.root
	r.mu.RUnlock()
	method := strings.ToUpper(req.Method)
	clean := normalize(req.URL.Path)

	// Fast path
	if mm := r.fast[method]; mm != nil {
		if h, ok := mm[clean]; ok {
			c := r.getCtx(w, req)
			c.Route = clean
			h(c)
			r.putCtx(c)
			return
		}
	}

	// Slow path
	c := r.getCtx(w, req)
	leaf, params := root.findPath(clean, 1, len(clean), c.params[:0])
	c.params = params
	if leaf == nil {
		c.Route = clean
		r.notFound(c)
		r.putCtx(c)
		return
	}
	c.params = c.params[:len(c.params)]
	h := leaf.handlers[method]
	if h == nil {
		c.Route = clean
		_ = c.JSON(http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		r.putCtx(c)
		return
	}
	if tpl, ok := leaf.tpls[method]; ok {
		c.Route = tpl
	} else {
		c.Route = clean
	}
	h(c)
	r.putCtx(c)
}

func (r *Router) getCtx(w http.ResponseWriter, req *http.Request) *Context {
	c := r.pool.Get().(*Context)
	c.sw = statusWriter{ResponseWriter: w}
	c.Writer, c.Request = &c.sw, req
	c.params = c.params[:0]
	c.Route = ""
	if c.store != nil {
		for k := range c.store {
			delete(c.store, k)
		}
	}
	return c
}
func (r *Router) putCtx(c *Context) { r.pool.Put(c) }

// Verify 基础健康检查
func (r *Router) Verify() error { return verifyNode(r.root) }

func verifyNode(n *node) error {
	if n.splat {
		if n.pchild != nil || n.schild != nil {
			return fmt.Errorf("splat node must be terminal: %s", n.part)
		}
	}
	if n.pchild != nil {
		if err := verifyNode(n.pchild); err != nil {
			return err
		}
	}
	if n.schild != nil {
		if err := verifyNode(n.schild); err != nil {
			return err
		}
	}
	return nil
}

func (r *Router) Dump() string { return dumpNode(r.root, 0) }
func dumpNode(n *node, depth int) string {
	pad := strings.Repeat(" ", depth)
	line := pad + "- '" + n.part + "'"
	if len(n.handlers) > 0 {
		line += " [H]"
	}
	out := line + "\n"
	if n.pchild != nil {
		out += dumpNode(n.pchild, depth+1)
	}
	if n.schild != nil {
		out += dumpNode(n.schild, depth+1)
	}
	return out
}
